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
	// FusedOpenError
	ErrCircuitOpen = errors.New("circuit breaker is open")

	// ErrTooManyRequests Too many requests in half-open state
	ErrTooManyRequests = errors.New("too many requests in half-open state")
)

// circuit breaker implementation
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

// Create circuit breaker instance
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

// Execute the protected operation
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

	// Check if execution is allowed
	if !cb.stateMgr.CanAttempt(cb.config) {
		if cb.logger != nil {
			cb.logger.WarnCtx(ctx, "⛔ [CircuitBreaker] Request rejected",
				zap.String("resource", cb.resource),
				zap.String("state", currentState.String()))
		}
		cb.metrics.RecordRejection()

		// Publish rejection event
		if cb.eventBus != nil {
			cb.eventBus.Publish(&RejectedEvent{
				BaseEvent:    NewBaseEvent(EventCallRejected, cb.resource, ctx),
				CurrentState: cb.stateMgr.GetState(),
			})
		}

		// Try to execute fallback scenario
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

	// Perform the actual operation
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

// handle success
func (cb *circuitBreaker) handleSuccess(ctx context.Context, duration time.Duration) {
	if cb.logger != nil {
		cb.logger.DebugCtx(ctx, "✅ [CircuitBreaker] handleSuccess",
			zap.String("resource", cb.resource),
			zap.Duration("duration", duration))
	}

	cb.metrics.RecordSuccess(duration)

	// Publish successful event
	if cb.eventBus != nil {
		cb.eventBus.Publish(&CallEvent{
			BaseEvent: NewBaseEvent(EventCallSuccess, cb.resource, ctx),
			Success:   true,
			Duration:  duration,
		})
	}

	// Update status
	changed, fromState, toState := cb.stateMgr.RecordSuccess(cb.config)
	if changed {
		cb.publishStateChangedEvent(ctx, fromState, toState, "success threshold reached")
	}

	// If it's a consecutive failure strategy, reset the counter
	if s, ok := cb.strategy.(*consecutiveFailuresStrategy); ok {
		s.RecordSuccess()
	}
}

// handleFailure Handle failure
func (cb *circuitBreaker) handleFailure(ctx context.Context, duration time.Duration, err error) {
	// Check if timeout
	isTimeout := errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)

	if cb.logger != nil {
		cb.logger.DebugCtx(ctx, "❌ [CircuitBreaker] handleFailure",
			zap.String("resource", cb.resource),
			zap.Bool("timeout", isTimeout),
			zap.Error(err))
	}

	// Record metrics
	if isTimeout {
		cb.metrics.RecordTimeout(duration)

		// Publish timeout event
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

		// Publish failure event
		if cb.eventBus != nil {
			cb.eventBus.Publish(&CallEvent{
				BaseEvent: NewBaseEvent(EventCallFailure, cb.resource, ctx),
				Success:   false,
				Duration:  duration,
				Error:     err,
			})
		}
	}

	// Update status
	changed, fromState, toState := cb.stateMgr.RecordFailure()
	if changed {
		cb.publishStateChangedEvent(ctx, fromState, toState, "failure in half-open state")
		return
	}

	// If it's a consecutive failure strategy, record the failure
	if s, ok := cb.strategy.(*consecutiveFailuresStrategy); ok {
		s.RecordFailure()
	}

	// Check if the circuit breaker should be triggered
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

// executeFallback Execute fallback
func (cb *circuitBreaker) executeFallback(ctx context.Context, req *Request, originalErr error) (interface{}, error) {
	start := time.Now()
	result, err := req.Fallback(ctx, originalErr)
	duration := time.Since(start)

	// Publish degrading event
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

// publishStateChangedEvent Publish state change event
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

// GetState Retrieve circuit breaker status
func (cb *circuitBreaker) GetState() State {
	return cb.stateMgr.GetState()
}

// GetMetrics Retrieve metric snapshot
func (cb *circuitBreaker) GetMetrics() *MetricsSnapshot {
	return cb.metrics.GetSnapshot()
}

// Reset fuse status and metrics
func (cb *circuitBreaker) Reset() {
	cb.stateMgr.Reset()
	cb.metrics.Reset()
}

// Circuit breaker manager
type Manager struct {
	config   Config
	breakers map[string]*circuitBreaker
	eventBus EventBus
	logger   *logger.CtxZapLogger
	mu       sync.RWMutex
}

// Create circuit breaker manager
func NewManager(config Config) (*Manager, error) {
	return NewManagerWithLogger(config, nil)
}

// Create a circuit breaker manager with logger
func NewManagerWithLogger(config Config, ctxLogger *logger.CtxZapLogger) (*Manager, error) {
	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// If no logger is provided, use the default one
	if ctxLogger == nil {
		ctxLogger = logger.GetLogger("yogan")
	}

	ctx := context.Background()

	// If not enabled, return empty manager
	if !config.Enabled {
		ctxLogger.DebugCtx(ctx, "⏭️  English: 🚫 Circuit breaker is not enabled, all calls will be executed directly，English: 🚫 Circuit breaker is not enabled, all calls will be executed directly")
		return &Manager{
			config:   config,
			breakers: make(map[string]*circuitBreaker),
			logger:   ctxLogger,
		}, nil
	}

	// Create event bus
	eventBus := NewEventBus(config.EventBusBuffer)

	ctxLogger.DebugCtx(ctx, "🎯 Circuit breaker manager initialization",
		zap.Int("event_bus_buffer", config.EventBusBuffer))

	return &Manager{
		config:   config,
		breakers: make(map[string]*circuitBreaker),
		eventBus: eventBus,
		logger:   ctxLogger,
	}, nil
}

// Execute the protected operation
func (m *Manager) Execute(ctx context.Context, req *Request) (interface{}, error) {
	if m.logger != nil {
		m.logger.DebugCtx(ctx, "🔍 [BreakerManager] Execute called",
			zap.Bool("enabled", m.config.Enabled),
			zap.String("resource", req.Resource))
	}

	// If not enabled, execute directly
	if !m.config.Enabled {
		if m.logger != nil {
			m.logger.DebugCtx(ctx, "🔍 [BreakerManager] Not enabled, executing directly",
				zap.String("resource", req.Resource))
		}
		return req.Execute(ctx)
	}

	// Get or create the circuit breaker
	breaker := m.getOrCreateBreaker(req.Resource)
	if m.logger != nil {
		m.logger.DebugCtx(ctx, "🔍 [BreakerManager] Getting circuit breaker",
			zap.String("resource", req.Resource),
			zap.String("state", breaker.GetState().String()))
	}

	// Perform operation
	result, err := breaker.Execute(ctx, req)
	if m.logger != nil {
		m.logger.DebugCtx(ctx, "🔍 [BreakerManager] Execution completed",
			zap.String("resource", req.Resource),
			zap.Error(err))
	}
	return result, err
}

// GetBreaker Get the circuit breaker instance for the specified resource (internal type)
func (m *Manager) GetBreaker(resource string) *circuitBreaker {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.breakers[resource]
}

// GetState Retrieve circuit breaker status
func (m *Manager) GetState(resource string) State {
	breaker := m.getOrCreateBreaker(resource)
	return breaker.GetState()
}

// GetMetrics retrieves circuit breaker metrics
func (m *Manager) GetMetrics(resource string) *MetricsSnapshot {
	breaker := m.getOrCreateBreaker(resource)
	return breaker.GetMetrics()
}

// GetEventBus obtain event bus
func (m *Manager) GetEventBus() EventBus {
	return m.eventBus
}

// SubscribeMetrics subscribe metric updates
func (m *Manager) SubscribeMetrics(resource string, observer MetricsObserver) ObserverID {
	breaker := m.getOrCreateBreaker(resource)
	return breaker.metrics.Subscribe(observer)
}

// Close Manager
func (m *Manager) Close() {
	if m.eventBus != nil {
		m.eventBus.Close()
	}
}

// getOrCreateBreaker Get or create breaker (thread-safe)
func (m *Manager) getOrCreateBreaker(resource string) *circuitBreaker {
	// Try to read first
	m.mu.RLock()
	if breaker, exists := m.breakers[resource]; exists {
		m.mu.RUnlock()
		return breaker
	}
	m.mu.RUnlock()

	// Need to create, obtain write lock
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double check
	if breaker, exists := m.breakers[resource]; exists {
		return breaker
	}

	// Get resource configuration
	resourceConfig := m.config.GetResourceConfig(resource)

	// Create new circuit breaker (pass in logger)
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
