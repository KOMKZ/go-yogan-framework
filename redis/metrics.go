package redis

import (
	"context"
	"sync"
	"time"

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
type RedisMetrics struct {
	config     RedisMetricsConfig
	meter      metric.Meter
	registered bool
	mu         sync.RWMutex

	// Metrics instruments
	commandsTotal    metric.Int64Counter       // Total commands executed
	commandDuration  metric.Float64Histogram   // Command duration
	errorsTotal      metric.Int64Counter       // Total errors
	connectionsActive metric.Int64ObservableGauge // Active connections
	connectionsIdle  metric.Int64ObservableGauge // Idle connections
	cacheHits        metric.Int64Counter       // Cache hits (optional)
	cacheMisses      metric.Int64Counter       // Cache misses (optional)

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
func (m *RedisMetrics) RegisterMetrics(meter metric.Meter) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.registered {
		return nil
	}

	m.meter = meter
	var err error

	// Counter: total commands
	m.commandsTotal, err = meter.Int64Counter(
		"redis_commands_total",
		metric.WithDescription("Total number of Redis commands executed"),
		metric.WithUnit("{command}"),
	)
	if err != nil {
		return err
	}

	// Histogram: command duration
	m.commandDuration, err = meter.Float64Histogram(
		"redis_command_duration_seconds",
		metric.WithDescription("Redis command duration distribution"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	// Counter: errors
	m.errorsTotal, err = meter.Int64Counter(
		"redis_errors_total",
		metric.WithDescription("Total number of Redis errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return err
	}

	// Optional: cache hits/misses
	if m.config.RecordHitMiss {
		m.cacheHits, err = meter.Int64Counter(
			"redis_cache_hits_total",
			metric.WithDescription("Total number of cache hits"),
			metric.WithUnit("{hit}"),
		)
		if err != nil {
			return err
		}

		m.cacheMisses, err = meter.Int64Counter(
			"redis_cache_misses_total",
			metric.WithDescription("Total number of cache misses"),
			metric.WithUnit("{miss}"),
		)
		if err != nil {
			return err
		}
	}

	// Optional: connection pool stats
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
	if !m.registered {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("instance", instance),
		attribute.String("command", command),
	}

	m.commandsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.commandDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))

	if err != nil {
		m.errorsTotal.Add(ctx, 1, metric.WithAttributes(
			attribute.String("instance", instance),
			attribute.String("error_type", "command_error"),
		))
	}
}

// RecordCacheHit records a cache hit
func (m *RedisMetrics) RecordCacheHit(ctx context.Context, instance string) {
	if !m.registered || m.cacheHits == nil {
		return
	}
	m.cacheHits.Add(ctx, 1, metric.WithAttributes(attribute.String("instance", instance)))
}

// RecordCacheMiss records a cache miss
func (m *RedisMetrics) RecordCacheMiss(ctx context.Context, instance string) {
	if !m.registered || m.cacheMisses == nil {
		return
	}
	m.cacheMisses.Add(ctx, 1, metric.WithAttributes(attribute.String("instance", instance)))
}

// IsRegistered returns whether metrics have been registered
func (m *RedisMetrics) IsRegistered() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.registered
}
