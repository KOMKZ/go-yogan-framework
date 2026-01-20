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

// Rate Limiter Manager
type Manager struct {
	config   Config
	store    Store
	limiters map[string]*rateLimiter
	eventBus EventBus
	provider AdaptiveProvider
	logger   *logger.CtxZapLogger
	mu       sync.RWMutex
}

// rateLimiter rate limiter for a single resource
type rateLimiter struct {
	resource  string
	config    ResourceConfig
	algorithm Algorithm
	metrics   MetricsCollector
}

// Create rate limiter manager
func NewManager(config Config) (*Manager, error) {
	return NewManagerWithLogger(config, nil, nil, nil)
}

// Create a rate limiter manager with logger
func NewManagerWithLogger(config Config, ctxLogger *logger.CtxZapLogger, redisClient *redis.Client, provider AdaptiveProvider) (*Manager, error) {
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
		ctxLogger.DebugCtx(ctx, "⏭️  English: Throttler not enabled, all calls will be executed directly，English: Throttler not enabled, all calls will be executed directly")
		return &Manager{
			config:   config,
			limiters: make(map[string]*rateLimiter),
			logger:   ctxLogger,
		}, nil
	}

	// Create storage
	var store Store
	switch StoreType(config.StoreType) {
	case StoreTypeMemory:
		store = NewMemoryStore()
		ctxLogger.DebugCtx(ctx, "✅ English: ✔ Using in-memory storage")
	case StoreTypeRedis:
		if redisClient == nil {
			return nil, fmt.Errorf("redis client is required for redis store")
		}
		store = NewRedisStore(redisClient, config.Redis.KeyPrefix)
		ctxLogger.DebugCtx(ctx, "✅ English: √ Using Redis for storage Redis English: √ Using Redis for storage",
			zap.String("key_prefix", config.Redis.KeyPrefix))
	default:
		return nil, fmt.Errorf("unsupported store type: %s", config.StoreType)
	}

	// Create event bus
	eventBus := NewEventBus(config.EventBusBuffer)

	ctxLogger.DebugCtx(ctx, "🎯 English: 🎲 Rate limiter manager initialization",
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

// Allow check if the request is permitted
func (m *Manager) Allow(ctx context.Context, resource string) (bool, error) {
	return m.AllowN(ctx, resource, 1)
}

// AllowN checks if N requests are permitted
func (m *Manager) AllowN(ctx context.Context, resource string, n int64) (bool, error) {
	if m.logger != nil {
		m.logger.DebugCtx(ctx, "🔍 [LimiterManager] AllowN called",
			zap.Bool("enabled", m.config.Enabled),
			zap.String("resource", resource),
			zap.Int64("n", n))
	}

	// If not enabled, allow directly
	if !m.config.Enabled {
		return true, nil
	}

	// 🎯 Check if the resource is defined in the configuration
	_, exists := m.config.Resources[resource]

	// If the resource is not configured
	if !exists {
		// Try using default configuration
		if err := m.config.Default.Validate(); err != nil {
			// default configuration is invalid or not set, allow directly
			if m.logger != nil {
				m.logger.DebugCtx(ctx, "🔓 [LimiterManager] Resource not configured and default config is invalid, auto-allowing",
					zap.String("resource", resource))
			}
			return true, nil
		}

		// default configuration is effective, rate limiting using default configuration
		if m.logger != nil {
			m.logger.DebugCtx(ctx, "🎯 [LimiterManager] Applying default config to unknown resource",
				zap.String("resource", resource),
				zap.String("algorithm", m.config.Default.Algorithm),
				zap.Int64("rate", m.config.Default.Rate))
		}
		// Continue with rate limiting logic (using default configuration)
	}

	// Get or create the rate limiter
	limiter := m.getOrCreateLimiter(resource)

	// Call the algorithm to check
	resp, err := limiter.algorithm.Allow(ctx, m.store, resource, n, limiter.config)
	if err != nil {
		return false, fmt.Errorf("algorithm allow failed: %w", err)
	}

	// Record metrics
	if resp.Allowed {
		limiter.metrics.RecordAllowed(resp.Remaining)

		// Publish allow event
		if m.eventBus != nil {
			m.eventBus.Publish(&AllowedEvent{
				BaseEvent: NewBaseEvent(EventAllowed, resource, ctx),
				Remaining: resp.Remaining,
				Limit:     resp.Limit,
			})
		}
	} else {
		limiter.metrics.RecordRejected("limit exceeded")

		// Publish rejection event
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

// Wait for permission to be acquired
func (m *Manager) Wait(ctx context.Context, resource string) error {
	return m.WaitN(ctx, resource, 1)
}

// Wait for N licenses to be acquired
func (m *Manager) WaitN(ctx context.Context, resource string, n int64) error {
	if m.logger != nil {
		m.logger.DebugCtx(ctx, "🔍 [LimiterManager] WaitN called",
			zap.Bool("enabled", m.config.Enabled),
			zap.String("resource", resource),
			zap.Int64("n", n))
	}

	// If not enabled, return directly
	if !m.config.Enabled {
		return nil
	}

	// Get or create the rate limiter
	limiter := m.getOrCreateLimiter(resource)

	// Publish wait start event
	start := time.Now()
	if m.eventBus != nil {
		m.eventBus.Publish(&WaitEvent{
			BaseEvent: NewBaseEvent(EventWaitStart, resource, ctx),
			Success:   false,
			Waited:    0,
		})
	}

	// Call algorithm and wait
	timeout := limiter.config.Timeout
	if timeout <= 0 {
		timeout = 1 * time.Second
	}

	err := limiter.algorithm.Wait(ctx, m.store, resource, n, limiter.config, timeout)
	waited := time.Since(start)

	// Publish waiting result event
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

// GetMetrics retrieves throttling metrics
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

	// Get algorithm metrics
	algoMetrics, err := limiter.algorithm.GetMetrics(context.Background(), m.store, resource)
	if err == nil && algoMetrics != nil {
		snapshot.CurrentValue = algoMetrics.Current
		snapshot.Limit = algoMetrics.Limit
		snapshot.Remaining = algoMetrics.Remaining
	}

	return snapshot
}

// GetEventBus obtain event bus
func (m *Manager) GetEventBus() EventBus {
	return m.eventBus
}

// Reset the rate limiter status
func (m *Manager) Reset(resource string) {
	m.mu.RLock()
	limiter, exists := m.limiters[resource]
	m.mu.RUnlock()

	if !exists {
		return
	}

	// Reset algorithm state
	limiter.algorithm.Reset(context.Background(), m.store, resource)

	// Reset metrics
	limiter.metrics.Reset()
}

// Close Manager
func (m *Manager) Close() error {
	// Close event bus
	if m.eventBus != nil {
		m.eventBus.Close()
	}

	// Close storage
	if m.store != nil {
		return m.store.Close()
	}

	return nil
}

// Implements the samber/do.Shutdownable interface for shutdown functionality
func (m *Manager) Shutdown() error {
	return m.Close()
}

// Check if the rate limiter is enabled
func (m *Manager) IsEnabled() bool {
	return m.config.Enabled
}

// GetConfig retrieve rate limiter configuration
func (m *Manager) GetConfig() Config {
	return m.config
}

// Get or create limiter (thread-safe)
func (m *Manager) getOrCreateLimiter(resource string) *rateLimiter {
	// Try to read first
	m.mu.RLock()
	if limiter, exists := m.limiters[resource]; exists {
		m.mu.RUnlock()
		return limiter
	}
	m.mu.RUnlock()

	// Need to create, obtain write lock
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double check
	if limiter, exists := m.limiters[resource]; exists {
		return limiter
	}

	// Get resource configuration
	resourceConfig := m.config.GetResourceConfig(resource)

	// Create algorithm instance
	algorithm := GetAlgorithm(resourceConfig, m.provider)

	// Create metric collector
	metrics := NewMetricsCollector(resource, resourceConfig.Algorithm)

	// Create new rate limiter
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
