package limiter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Manager 限流器管理器
type Manager struct {
	config   Config
	store    Store
	limiters map[string]*rateLimiter
	eventBus EventBus
	provider AdaptiveProvider
	logger   *logger.CtxZapLogger
	mu       sync.RWMutex
}

// rateLimiter 单个资源的限流器
type rateLimiter struct {
	resource  string
	config    ResourceConfig
	algorithm Algorithm
	metrics   MetricsCollector
}

// NewManager 创建限流器管理器
func NewManager(config Config) (*Manager, error) {
	return NewManagerWithLogger(config, nil, nil, nil)
}

// NewManagerWithLogger 创建带logger的限流器管理器
func NewManagerWithLogger(config Config, ctxLogger *logger.CtxZapLogger, redisClient *redis.Client, provider AdaptiveProvider) (*Manager, error) {
	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// 如果没有提供logger，使用默认的
	if ctxLogger == nil {
		ctxLogger = logger.GetLogger("yogan")
	}

	ctx := context.Background()

	// 如果未启用，返回空管理器
	if !config.Enabled {
		ctxLogger.DebugCtx(ctx, "⏭️  限流器未启用，所有调用将直接执行")
		return &Manager{
			config:   config,
			limiters: make(map[string]*rateLimiter),
			logger:   ctxLogger,
		}, nil
	}

	// 创建存储
	var store Store
	switch StoreType(config.StoreType) {
	case StoreTypeMemory:
		store = NewMemoryStore()
		ctxLogger.DebugCtx(ctx, "✅ 使用内存存储")
	case StoreTypeRedis:
		if redisClient == nil {
			return nil, fmt.Errorf("redis client is required for redis store")
		}
		store = NewRedisStore(redisClient, config.Redis.KeyPrefix)
		ctxLogger.DebugCtx(ctx, "✅ 使用 Redis 存储",
			zap.String("key_prefix", config.Redis.KeyPrefix))
	default:
		return nil, fmt.Errorf("unsupported store type: %s", config.StoreType)
	}

	// 创建事件总线
	eventBus := NewEventBus(config.EventBusBuffer)

	ctxLogger.DebugCtx(ctx, "🎯 限流器管理器初始化",
		zap.String("store_type", config.StoreType),
		zap.Int("event_bus_buffer", config.EventBusBuffer))

	return &Manager{
		config:   config,
		store:    store,
		limiters: make(map[string]*rateLimiter),
		eventBus: eventBus,
		provider: provider,
		logger:   ctxLogger,
	}, nil
}

// Allow 检查是否允许请求
func (m *Manager) Allow(ctx context.Context, resource string) (bool, error) {
	return m.AllowN(ctx, resource, 1)
}

// AllowN 检查是否允许N个请求
func (m *Manager) AllowN(ctx context.Context, resource string, n int64) (bool, error) {
	if m.logger != nil {
		m.logger.DebugCtx(ctx, "🔍 [LimiterManager] AllowN called",
			zap.Bool("enabled", m.config.Enabled),
			zap.String("resource", resource),
			zap.Int64("n", n))
	}

	// 如果未启用，直接允许
	if !m.config.Enabled {
		return true, nil
	}

	// 🎯 检查资源是否在配置中定义
	_, exists := m.config.Resources[resource]

	// 如果资源未配置
	if !exists {
		// 尝试使用 default 配置
		if err := m.config.Default.Validate(); err != nil {
			// default 配置无效或未配置，直接放行
			if m.logger != nil {
				m.logger.DebugCtx(ctx, "🔓 [LimiterManager] Resource not configured and default config is invalid, auto-allowing",
					zap.String("resource", resource))
			}
			return true, nil
		}

		// default 配置有效，使用 default 配置限流
		if m.logger != nil {
			m.logger.DebugCtx(ctx, "🎯 [LimiterManager] Applying default config to unknown resource",
				zap.String("resource", resource),
				zap.String("algorithm", m.config.Default.Algorithm),
				zap.Int64("rate", m.config.Default.Rate))
		}
		// 继续执行限流逻辑（使用 default 配置）
	}

	// 获取或创建限流器
	limiter := m.getOrCreateLimiter(resource)

	// 调用算法检查
	resp, err := limiter.algorithm.Allow(ctx, m.store, resource, n, limiter.config)
	if err != nil {
		return false, fmt.Errorf("algorithm allow failed: %w", err)
	}

	// 记录指标
	if resp.Allowed {
		limiter.metrics.RecordAllowed(resp.Remaining)

		// 发布允许事件
		if m.eventBus != nil {
			m.eventBus.Publish(&AllowedEvent{
				BaseEvent: NewBaseEvent(EventAllowed, resource, ctx),
				Remaining: resp.Remaining,
				Limit:     resp.Limit,
			})
		}
	} else {
		limiter.metrics.RecordRejected("limit exceeded")

		// 发布拒绝事件
		if m.eventBus != nil {
			m.eventBus.Publish(&RejectedEvent{
				BaseEvent:  NewBaseEvent(EventRejected, resource, ctx),
				RetryAfter: resp.RetryAfter,
				Reason:     "limit exceeded",
			})
		}
	}

	return resp.Allowed, nil
}

// Wait 等待获取许可
func (m *Manager) Wait(ctx context.Context, resource string) error {
	return m.WaitN(ctx, resource, 1)
}

// WaitN 等待获取N个许可
func (m *Manager) WaitN(ctx context.Context, resource string, n int64) error {
	if m.logger != nil {
		m.logger.DebugCtx(ctx, "🔍 [LimiterManager] WaitN called",
			zap.Bool("enabled", m.config.Enabled),
			zap.String("resource", resource),
			zap.Int64("n", n))
	}

	// 如果未启用，直接返回
	if !m.config.Enabled {
		return nil
	}

	// 获取或创建限流器
	limiter := m.getOrCreateLimiter(resource)

	// 发布等待开始事件
	start := time.Now()
	if m.eventBus != nil {
		m.eventBus.Publish(&WaitEvent{
			BaseEvent: NewBaseEvent(EventWaitStart, resource, ctx),
			Success:   false,
			Waited:    0,
		})
	}

	// 调用算法等待
	timeout := limiter.config.Timeout
	if timeout <= 0 {
		timeout = 1 * time.Second
	}

	err := limiter.algorithm.Wait(ctx, m.store, resource, n, limiter.config, timeout)
	waited := time.Since(start)

	// 发布等待结果事件
	if m.eventBus != nil {
		eventType := EventWaitSuccess
		if err != nil {
			eventType = EventWaitTimeout
		}
		m.eventBus.Publish(&WaitEvent{
			BaseEvent: NewBaseEvent(eventType, resource, ctx),
			Success:   err == nil,
			Waited:    waited,
		})
	}

	if err != nil {
		limiter.metrics.RecordRejected("wait timeout")
		return err
	}

	limiter.metrics.RecordAllowed(0)
	return nil
}

// GetMetrics 获取限流器指标
func (m *Manager) GetMetrics(resource string) *MetricsSnapshot {
	m.mu.RLock()
	limiter, exists := m.limiters[resource]
	m.mu.RUnlock()

	if !exists {
		return &MetricsSnapshot{
			Resource:  resource,
			Algorithm: "unknown",
		}
	}

	snapshot := limiter.metrics.GetSnapshot()

	// 获取算法指标
	algoMetrics, err := limiter.algorithm.GetMetrics(context.Background(), m.store, resource)
	if err == nil && algoMetrics != nil {
		snapshot.CurrentValue = algoMetrics.Current
		snapshot.Limit = algoMetrics.Limit
		snapshot.Remaining = algoMetrics.Remaining
	}

	return snapshot
}

// GetEventBus 获取事件总线
func (m *Manager) GetEventBus() EventBus {
	return m.eventBus
}

// Reset 重置限流器状态
func (m *Manager) Reset(resource string) {
	m.mu.RLock()
	limiter, exists := m.limiters[resource]
	m.mu.RUnlock()

	if !exists {
		return
	}

	// 重置算法状态
	limiter.algorithm.Reset(context.Background(), m.store, resource)

	// 重置指标
	limiter.metrics.Reset()
}

// Close 关闭管理器
func (m *Manager) Close() error {
	// 关闭事件总线
	if m.eventBus != nil {
		m.eventBus.Close()
	}

	// 关闭存储
	if m.store != nil {
		return m.store.Close()
	}

	return nil
}

// IsEnabled 检查限流器是否启用
func (m *Manager) IsEnabled() bool {
	return m.config.Enabled
}

// getOrCreateLimiter 获取或创建限流器（线程安全）
func (m *Manager) getOrCreateLimiter(resource string) *rateLimiter {
	// 先尝试读取
	m.mu.RLock()
	if limiter, exists := m.limiters[resource]; exists {
		m.mu.RUnlock()
		return limiter
	}
	m.mu.RUnlock()

	// 需要创建，获取写锁
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double check
	if limiter, exists := m.limiters[resource]; exists {
		return limiter
	}

	// 获取资源配置
	resourceConfig := m.config.GetResourceConfig(resource)

	// 创建算法实例
	algorithm := GetAlgorithm(resourceConfig, m.provider)

	// 创建指标采集器
	metrics := NewMetricsCollector(resource, resourceConfig.Algorithm)

	// 创建新限流器
	limiter := &rateLimiter{
		resource:  resource,
		config:    resourceConfig,
		algorithm: algorithm,
		metrics:   metrics,
	}
	m.limiters[resource] = limiter

	if m.logger != nil {
		m.logger.DebugCtx(context.Background(), "🎯 Creating limiter instance",
			zap.String("resource", resource),
			zap.String("algorithm", resourceConfig.Algorithm))
	}

	return limiter
}
