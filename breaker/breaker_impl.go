package breaker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.uber.org/zap"
)

var (
	// ErrCircuitOpen 熔断器打开错误
	ErrCircuitOpen = errors.New("circuit breaker is open")

	// ErrTooManyRequests 半开状态请求过多
	ErrTooManyRequests = errors.New("too many requests in half-open state")
)

// circuitBreaker 熔断器实现
type circuitBreaker struct {
	resource string
	config   ResourceConfig
	stateMgr *stateManager
	metrics  MetricsCollector
	strategy Strategy
	eventBus EventBus
	logger   *logger.CtxZapLogger
	mu       sync.RWMutex
}

// newCircuitBreaker 创建熔断器实例
func newCircuitBreaker(resource string, config ResourceConfig, eventBus EventBus, log *logger.CtxZapLogger) *circuitBreaker {
	stateMgr := newStateManager()
	metrics := newSlidingWindowMetrics(resource, config, stateMgr)
	strategy := GetStrategyByName(config.Strategy)

	return &circuitBreaker{
		resource: resource,
		config:   config,
		stateMgr: stateMgr,
		metrics:  metrics,
		strategy: strategy,
		eventBus: eventBus,
		logger:   log,
	}
}

// Execute 执行受保护的操作
func (cb *circuitBreaker) Execute(ctx context.Context, req *Request) (interface{}, error) {
	currentState := cb.stateMgr.GetState()
	snapshot := cb.metrics.GetSnapshot()

	if cb.logger != nil {
		cb.logger.DebugCtx(ctx, "🔍 [CircuitBreaker] Execute",
			zap.String("resource", cb.resource),
			zap.String("state", currentState.String()),
			zap.Int64("requests", snapshot.TotalRequests),
			zap.Int64("successes", snapshot.Successes),
			zap.Int64("failures", snapshot.Failures))
	}

	// 检查是否允许执行
	if !cb.stateMgr.CanAttempt(cb.config) {
		if cb.logger != nil {
			cb.logger.WarnCtx(ctx, "⛔ [CircuitBreaker] Request rejected",
				zap.String("resource", cb.resource),
				zap.String("state", currentState.String()))
		}
		cb.metrics.RecordRejection()

		// 发布拒绝事件
		if cb.eventBus != nil {
			cb.eventBus.Publish(&RejectedEvent{
				BaseEvent:    NewBaseEvent(EventCallRejected, cb.resource, ctx),
				CurrentState: cb.stateMgr.GetState(),
			})
		}

		// 尝试执行降级
		if req.Fallback != nil {
			return cb.executeFallback(ctx, req, ErrCircuitOpen)
		}

		return nil, ErrCircuitOpen
	}

	if cb.logger != nil {
		cb.logger.DebugCtx(ctx, "✅ [CircuitBreaker] Execution allowed",
			zap.String("resource", cb.resource),
			zap.String("state", currentState.String()))
	}

	// 执行实际操作
	start := time.Now()
	result, err := req.Execute(ctx)
	duration := time.Since(start)

	if err != nil {
		if cb.logger != nil {
			cb.logger.DebugCtx(ctx, "❌ [CircuitBreaker] Call failed",
				zap.String("resource", cb.resource),
				zap.Duration("duration", duration),
				zap.Error(err))
		}
		cb.handleFailure(ctx, duration, err)
	} else {
		if cb.logger != nil {
			cb.logger.DebugCtx(ctx, "✅ [CircuitBreaker] Call succeeded",
				zap.String("resource", cb.resource),
				zap.Duration("duration", duration))
		}
		cb.handleSuccess(ctx, duration)
	}

	return result, err
}

// handleSuccess 处理成功
func (cb *circuitBreaker) handleSuccess(ctx context.Context, duration time.Duration) {
	if cb.logger != nil {
		cb.logger.DebugCtx(ctx, "✅ [CircuitBreaker] handleSuccess",
			zap.String("resource", cb.resource),
			zap.Duration("duration", duration))
	}

	cb.metrics.RecordSuccess(duration)

	// 发布成功事件
	if cb.eventBus != nil {
		cb.eventBus.Publish(&CallEvent{
			BaseEvent: NewBaseEvent(EventCallSuccess, cb.resource, ctx),
			Success:   true,
			Duration:  duration,
		})
	}

	// 更新状态
	changed, fromState, toState := cb.stateMgr.RecordSuccess(cb.config)
	if changed {
		cb.publishStateChangedEvent(ctx, fromState, toState, "success threshold reached")
	}

	// 如果是连续失败策略，重置计数
	if s, ok := cb.strategy.(*consecutiveFailuresStrategy); ok {
		s.RecordSuccess()
	}
}

// handleFailure 处理失败
func (cb *circuitBreaker) handleFailure(ctx context.Context, duration time.Duration, err error) {
	// 判断是否超时
	isTimeout := errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)

	if cb.logger != nil {
		cb.logger.DebugCtx(ctx, "❌ [CircuitBreaker] handleFailure",
			zap.String("resource", cb.resource),
			zap.Bool("timeout", isTimeout),
			zap.Error(err))
	}

	// 记录指标
	if isTimeout {
		cb.metrics.RecordTimeout(duration)

		// 发布超时事件
		if cb.eventBus != nil {
			cb.eventBus.Publish(&CallEvent{
				BaseEvent: NewBaseEvent(EventCallTimeout, cb.resource, ctx),
				Success:   false,
				Duration:  duration,
				Error:     err,
			})
		}
	} else {
		cb.metrics.RecordFailure(duration, err)

		// 发布失败事件
		if cb.eventBus != nil {
			cb.eventBus.Publish(&CallEvent{
				BaseEvent: NewBaseEvent(EventCallFailure, cb.resource, ctx),
				Success:   false,
				Duration:  duration,
				Error:     err,
			})
		}
	}

	// 更新状态
	changed, fromState, toState := cb.stateMgr.RecordFailure()
	if changed {
		cb.publishStateChangedEvent(ctx, fromState, toState, "failure in half-open state")
		return
	}

	// 如果是连续失败策略，记录失败
	if s, ok := cb.strategy.(*consecutiveFailuresStrategy); ok {
		s.RecordFailure()
	}

	// 检查是否应该触发熔断
	snapshot := cb.metrics.GetSnapshot()
	shouldOpen := cb.strategy.ShouldOpen(snapshot, cb.config)

	if cb.logger != nil {
		cb.logger.DebugCtx(ctx, "🔍 [CircuitBreaker] Checking if open",
			zap.String("resource", cb.resource),
			zap.Bool("shouldOpen", shouldOpen),
			zap.Int64("totalReqs", snapshot.TotalRequests),
			zap.Int64("failures", snapshot.Failures),
			zap.Float64("errorRate", snapshot.ErrorRate))
	}

	if shouldOpen {
		if cb.logger != nil {
			cb.logger.WarnCtx(ctx, "⛔ [CircuitBreaker] Circuit breaker triggered!",
				zap.String("resource", cb.resource))
		}
		changed, fromState, toState := cb.stateMgr.ShouldOpen(true)
		if changed {
			cb.publishStateChangedEvent(ctx, fromState, toState, "error threshold exceeded")
		}
	}
}

// executeFallback 执行降级
func (cb *circuitBreaker) executeFallback(ctx context.Context, req *Request, originalErr error) (interface{}, error) {
	start := time.Now()
	result, err := req.Fallback(ctx, originalErr)
	duration := time.Since(start)

	// 发布降级事件
	if cb.eventBus != nil {
		eventType := EventFallbackSuccess
		if err != nil {
			eventType = EventFallbackFailure
		}

		cb.eventBus.Publish(&FallbackEvent{
			BaseEvent: NewBaseEvent(eventType, cb.resource, ctx),
			Success:   err == nil,
			Duration:  duration,
			Error:     err,
		})
	}

	return result, err
}

// publishStateChangedEvent 发布状态变化事件
func (cb *circuitBreaker) publishStateChangedEvent(ctx context.Context, fromState, toState State, reason string) {
	if cb.eventBus != nil {
		cb.eventBus.Publish(&StateChangedEvent{
			BaseEvent: NewBaseEvent(EventStateChanged, cb.resource, ctx),
			FromState: fromState,
			ToState:   toState,
			Reason:    reason,
			Metrics:   cb.metrics.GetSnapshot(),
		})
	}
}

// GetState 获取熔断器状态
func (cb *circuitBreaker) GetState() State {
	return cb.stateMgr.GetState()
}

// GetMetrics 获取指标快照
func (cb *circuitBreaker) GetMetrics() *MetricsSnapshot {
	return cb.metrics.GetSnapshot()
}

// Manager 熔断器管理器
type Manager struct {
	config   Config
	breakers map[string]*circuitBreaker
	eventBus EventBus
	logger   *logger.CtxZapLogger
	mu       sync.RWMutex
}

// NewManager 创建熔断器管理器
func NewManager(config Config) (*Manager, error) {
	return NewManagerWithLogger(config, nil)
}

// NewManagerWithLogger 创建带logger的熔断器管理器
func NewManagerWithLogger(config Config, ctxLogger *logger.CtxZapLogger) (*Manager, error) {
	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// 如果没有提供 logger，使用默认的
	if ctxLogger == nil {
		ctxLogger = logger.GetLogger("yogan")
	}

	ctx := context.Background()

	// 如果未启用，返回空管理器
	if !config.Enabled {
		ctxLogger.DebugCtx(ctx, "⏭️  熔断器未启用，所有调用将直接执行")
		return &Manager{
			config:   config,
			breakers: make(map[string]*circuitBreaker),
			logger:   ctxLogger,
		}, nil
	}

	// 创建事件总线
	eventBus := NewEventBus(config.EventBusBuffer)

	ctxLogger.DebugCtx(ctx, "🎯 熔断器管理器初始化",
		zap.Int("event_bus_buffer", config.EventBusBuffer))

	return &Manager{
		config:   config,
		breakers: make(map[string]*circuitBreaker),
		eventBus: eventBus,
		logger:   ctxLogger,
	}, nil
}

// Execute 执行受保护的操作
func (m *Manager) Execute(ctx context.Context, req *Request) (interface{}, error) {
	if m.logger != nil {
		m.logger.DebugCtx(ctx, "🔍 [BreakerManager] Execute called",
			zap.Bool("enabled", m.config.Enabled),
			zap.String("resource", req.Resource))
	}

	// 如果未启用，直接执行
	if !m.config.Enabled {
		if m.logger != nil {
			m.logger.DebugCtx(ctx, "🔍 [BreakerManager] Not enabled, executing directly",
				zap.String("resource", req.Resource))
		}
		return req.Execute(ctx)
	}

	// 获取或创建熔断器
	breaker := m.getOrCreateBreaker(req.Resource)
	if m.logger != nil {
		m.logger.DebugCtx(ctx, "🔍 [BreakerManager] Getting circuit breaker",
			zap.String("resource", req.Resource),
			zap.String("state", breaker.GetState().String()))
	}

	// 执行操作
	result, err := breaker.Execute(ctx, req)
	if m.logger != nil {
		m.logger.DebugCtx(ctx, "🔍 [BreakerManager] Execution completed",
			zap.String("resource", req.Resource),
			zap.Error(err))
	}
	return result, err
}

// GetBreaker 获取指定资源的熔断器实例(内部类型)
func (m *Manager) GetBreaker(resource string) *circuitBreaker {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.breakers[resource]
}

// GetState 获取熔断器状态
func (m *Manager) GetState(resource string) State {
	breaker := m.getOrCreateBreaker(resource)
	return breaker.GetState()
}

// GetMetrics 获取熔断器指标
func (m *Manager) GetMetrics(resource string) *MetricsSnapshot {
	breaker := m.getOrCreateBreaker(resource)
	return breaker.GetMetrics()
}

// GetEventBus 获取事件总线
func (m *Manager) GetEventBus() EventBus {
	return m.eventBus
}

// SubscribeMetrics 订阅指标更新
func (m *Manager) SubscribeMetrics(resource string, observer MetricsObserver) ObserverID {
	breaker := m.getOrCreateBreaker(resource)
	return breaker.metrics.Subscribe(observer)
}

// Close 关闭管理器
func (m *Manager) Close() {
	if m.eventBus != nil {
		m.eventBus.Close()
	}
}

// getOrCreateBreaker 获取或创建熔断器（线程安全）
func (m *Manager) getOrCreateBreaker(resource string) *circuitBreaker {
	// 先尝试读取
	m.mu.RLock()
	if breaker, exists := m.breakers[resource]; exists {
		m.mu.RUnlock()
		return breaker
	}
	m.mu.RUnlock()

	// 需要创建，获取写锁
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double check
	if breaker, exists := m.breakers[resource]; exists {
		return breaker
	}

	// 获取资源配置
	resourceConfig := m.config.GetResourceConfig(resource)

	// 创建新熔断器（传入 logger）
	breaker := newCircuitBreaker(resource, resourceConfig, m.eventBus, m.logger)
	m.breakers[resource] = breaker

	if m.logger != nil {
		m.logger.DebugCtx(context.Background(), "🎯 Creating circuit breaker instance",
			zap.String("resource", resource),
			zap.String("strategy", resourceConfig.Strategy),
			zap.Duration("timeout", resourceConfig.Timeout))
	}

	return breaker
}
