package redis

import (
	"context"
	"sync"
	"time"

	"github.com/KOMKZ/go-yogan-framework/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// RedisMetricsConfig holds configuration for Redis metrics
type RedisMetricsConfig struct {
	Enabled          bool
	RecordHitMiss    bool
	RecordPoolStats  bool
	RecordLatencyP99 bool
}

// RedisMetrics implements component.MetricsProvider for Redis instrumentation.
// 使用 telemetry.OperationMetrics + telemetry.CacheMetrics 模板减少样板代码
type RedisMetrics struct {
	config     RedisMetricsConfig
	meter      metric.Meter
	registered bool
	mu         sync.RWMutex

	// 使用预定义模板
	operations *telemetry.OperationMetrics // 命令操作指标
	cache      *telemetry.CacheMetrics     // 缓存命中指标（可选）

	// 连接池指标（需要 callback，保留原始实现）
	connectionsActive metric.Int64ObservableGauge
	connectionsIdle   metric.Int64ObservableGauge

	// Pool stats callbacks
	poolCallbacks map[string]func() PoolStats
	poolMu        sync.RWMutex
}

// PoolStats represents Redis connection pool statistics
type PoolStats struct {
	ActiveCount int64
	IdleCount   int64
}

// NewRedisMetrics creates a new Redis metrics provider
func NewRedisMetrics(cfg RedisMetricsConfig) *RedisMetrics {
	return &RedisMetrics{
		config:        cfg,
		poolCallbacks: make(map[string]func() PoolStats),
	}
}

// MetricsName returns the metrics group name
func (m *RedisMetrics) MetricsName() string {
	return "redis"
}

// IsMetricsEnabled returns whether metrics collection is enabled
func (m *RedisMetrics) IsMetricsEnabled() bool {
	return m.config.Enabled
}

// RegisterMetrics registers all Redis metrics with the provided Meter
// 使用 MetricsBuilder 模板，代码量从 80+ 行减少到 30 行
func (m *RedisMetrics) RegisterMetrics(meter metric.Meter) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.registered {
		return nil
	}

	m.meter = meter
	builder := telemetry.NewMetricsBuilder(meter, "redis")

	// 使用 OperationMetrics 模板创建命令操作指标
	operations, err := builder.NewOperationMetrics("command")
	if err != nil {
		return err
	}
	m.operations = operations

	// Optional: cache hits/misses 使用 CacheMetrics 模板
	if m.config.RecordHitMiss {
		cache, err := builder.NewCacheMetrics("")
		if err != nil {
			return err
		}
		m.cache = cache
	}

	// Optional: connection pool stats（需要 callback，保留原始实现）
	if m.config.RecordPoolStats {
		m.connectionsActive, err = meter.Int64ObservableGauge(
			"redis_connections_active",
			metric.WithDescription("Number of active Redis connections"),
			metric.WithUnit("{connection}"),
			metric.WithInt64Callback(m.collectActiveConnections),
		)
		if err != nil {
			return err
		}

		m.connectionsIdle, err = meter.Int64ObservableGauge(
			"redis_connections_idle",
			metric.WithDescription("Number of idle Redis connections"),
			metric.WithUnit("{connection}"),
			metric.WithInt64Callback(m.collectIdleConnections),
		)
		if err != nil {
			return err
		}
	}

	m.registered = true
	return nil
}

// collectActiveConnections collects active connection counts
func (m *RedisMetrics) collectActiveConnections(_ context.Context, observer metric.Int64Observer) error {
	m.poolMu.RLock()
	defer m.poolMu.RUnlock()

	for instance, callback := range m.poolCallbacks {
		stats := callback()
		observer.Observe(stats.ActiveCount,
			metric.WithAttributes(attribute.String("instance", instance)),
		)
	}
	return nil
}

// collectIdleConnections collects idle connection counts
func (m *RedisMetrics) collectIdleConnections(_ context.Context, observer metric.Int64Observer) error {
	m.poolMu.RLock()
	defer m.poolMu.RUnlock()

	for instance, callback := range m.poolCallbacks {
		stats := callback()
		observer.Observe(stats.IdleCount,
			metric.WithAttributes(attribute.String("instance", instance)),
		)
	}
	return nil
}

// RegisterPoolCallback registers a pool stats callback for an instance
func (m *RedisMetrics) RegisterPoolCallback(instance string, callback func() PoolStats) {
	m.poolMu.Lock()
	defer m.poolMu.Unlock()
	m.poolCallbacks[instance] = callback
}

// UnregisterPoolCallback removes a pool stats callback
func (m *RedisMetrics) UnregisterPoolCallback(instance string) {
	m.poolMu.Lock()
	defer m.poolMu.Unlock()
	delete(m.poolCallbacks, instance)
}

// RecordCommand records a command execution
func (m *RedisMetrics) RecordCommand(ctx context.Context, instance, command string, duration time.Duration, err error) {
	if !m.registered || m.operations == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("instance", instance),
		attribute.String("command", command),
	}

	m.operations.Record(ctx, duration.Seconds(), err, attrs...)
}

// RecordCacheHit records a cache hit
func (m *RedisMetrics) RecordCacheHit(ctx context.Context, instance string) {
	if !m.registered || m.cache == nil {
		return
	}
	m.cache.RecordHit(ctx, attribute.String("instance", instance))
}

// RecordCacheMiss records a cache miss
func (m *RedisMetrics) RecordCacheMiss(ctx context.Context, instance string) {
	if !m.registered || m.cache == nil {
		return
	}
	m.cache.RecordMiss(ctx, attribute.String("instance", instance))
}

// IsRegistered returns whether metrics have been registered
func (m *RedisMetrics) IsRegistered() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.registered
}
