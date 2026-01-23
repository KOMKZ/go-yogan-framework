package redis

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestNewRedisMetrics(t *testing.T) {
	t.Run("creates with config", func(t *testing.T) {
		cfg := RedisMetricsConfig{
			Enabled:       true,
			RecordHitMiss: true,
			RecordPoolStats: true,
		}
		m := NewRedisMetrics(cfg)

		assert.NotNil(t, m)
		assert.True(t, m.config.Enabled)
		assert.False(t, m.IsRegistered())
	})
}

func TestRedisMetrics_MetricsProvider(t *testing.T) {
	t.Run("MetricsName returns redis", func(t *testing.T) {
		m := NewRedisMetrics(RedisMetricsConfig{Enabled: true})
		assert.Equal(t, "redis", m.MetricsName())
	})

	t.Run("IsMetricsEnabled reflects config", func(t *testing.T) {
		m1 := NewRedisMetrics(RedisMetricsConfig{Enabled: true})
		assert.True(t, m1.IsMetricsEnabled())

		m2 := NewRedisMetrics(RedisMetricsConfig{Enabled: false})
		assert.False(t, m2.IsMetricsEnabled())
	})
}

func TestRedisMetrics_RegisterMetrics(t *testing.T) {
	t.Run("registers all metrics", func(t *testing.T) {
		mp := noop.NewMeterProvider()
		meter := mp.Meter("test")

		m := NewRedisMetrics(RedisMetricsConfig{
			Enabled:         true,
			RecordHitMiss:   true,
			RecordPoolStats: true,
		})
		err := m.RegisterMetrics(meter)

		require.NoError(t, err)
		assert.True(t, m.IsRegistered())
		// 使用 MetricsBuilder 模板后，检查模板是否创建成功
		assert.NotNil(t, m.operations) // OperationMetrics 模板
		assert.NotNil(t, m.cache)      // CacheMetrics 模板
	})

	t.Run("idempotent registration", func(t *testing.T) {
		mp := noop.NewMeterProvider()
		meter := mp.Meter("test")

		m := NewRedisMetrics(RedisMetricsConfig{Enabled: true})

		err1 := m.RegisterMetrics(meter)
		require.NoError(t, err1)

		err2 := m.RegisterMetrics(meter)
		require.NoError(t, err2)
	})
}

func TestRedisMetrics_RecordMethods(t *testing.T) {
	mp := noop.NewMeterProvider()
	meter := mp.Meter("test")

	m := NewRedisMetrics(RedisMetricsConfig{
		Enabled:       true,
		RecordHitMiss: true,
	})
	err := m.RegisterMetrics(meter)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("RecordCommand does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordCommand(ctx, "main", "GET", 10*time.Millisecond, nil)
		})
	})

	t.Run("RecordCacheHit does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordCacheHit(ctx, "main")
		})
	})

	t.Run("RecordCacheMiss does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordCacheMiss(ctx, "main")
		})
	})

	t.Run("methods no-op when not registered", func(t *testing.T) {
		unregistered := NewRedisMetrics(RedisMetricsConfig{Enabled: true})
		assert.NotPanics(t, func() {
			unregistered.RecordCommand(ctx, "main", "GET", time.Second, nil)
			unregistered.RecordCacheHit(ctx, "main")
			unregistered.RecordCacheMiss(ctx, "main")
		})
	})
}

func TestRedisMetrics_PoolCallbacks(t *testing.T) {
	m := NewRedisMetrics(RedisMetricsConfig{Enabled: true, RecordPoolStats: true})

	t.Run("register and unregister callbacks", func(t *testing.T) {
		callback := func() PoolStats { return PoolStats{ActiveCount: 10, IdleCount: 5} }

		m.RegisterPoolCallback("main", callback)
		assert.Len(t, m.poolCallbacks, 1)

		m.RegisterPoolCallback("cache", callback)
		assert.Len(t, m.poolCallbacks, 2)

		m.UnregisterPoolCallback("main")
		assert.Len(t, m.poolCallbacks, 1)
	})
}

func TestRedisMetrics_RecordCommand_WithError(t *testing.T) {
	mp := noop.NewMeterProvider()
	meter := mp.Meter("test")

	m := NewRedisMetrics(RedisMetricsConfig{
		Enabled:       true,
		RecordHitMiss: true,
	})
	err := m.RegisterMetrics(meter)
	require.NoError(t, err)

	ctx := context.Background()

	// 测试有错误的命令
	assert.NotPanics(t, func() {
		m.RecordCommand(ctx, "main", "GET", 10*time.Millisecond, assert.AnError)
	})
}

func TestRedisMetrics_RegisterMetrics_Disabled(t *testing.T) {
	mp := noop.NewMeterProvider()
	meter := mp.Meter("test")

	// 未启用的 metrics
	m := NewRedisMetrics(RedisMetricsConfig{
		Enabled:         false,
		RecordHitMiss:   false,
		RecordPoolStats: false,
	})
	err := m.RegisterMetrics(meter)

	require.NoError(t, err)
}

func TestRedisMetrics_RecordHitMiss_Disabled(t *testing.T) {
	mp := noop.NewMeterProvider()
	meter := mp.Meter("test")

	m := NewRedisMetrics(RedisMetricsConfig{
		Enabled:       true,
		RecordHitMiss: false, // 禁用 hit/miss
	})
	err := m.RegisterMetrics(meter)
	require.NoError(t, err)

	ctx := context.Background()

	// 即使禁用了 hit/miss，也不应该 panic
	assert.NotPanics(t, func() {
		m.RecordCacheHit(ctx, "main")
		m.RecordCacheMiss(ctx, "main")
	})
}

func TestRedisMetrics_RegisterWithPoolStats(t *testing.T) {
	mp := noop.NewMeterProvider()
	meter := mp.Meter("test")

	m := NewRedisMetrics(RedisMetricsConfig{
		Enabled:         true,
		RecordPoolStats: true,
	})

	// 注册回调
	callback := func() PoolStats { return PoolStats{ActiveCount: 10, IdleCount: 5} }
	m.RegisterPoolCallback("main", callback)
	m.RegisterPoolCallback("cache", func() PoolStats { return PoolStats{ActiveCount: 20, IdleCount: 10} })

	// 注册 metrics 会创建 gauge 并使用 callbacks
	err := m.RegisterMetrics(meter)
	require.NoError(t, err)

	assert.True(t, m.IsRegistered())
}

// 使用 _ 忽略 metric 包的导入检查
var _ = metric.Int64Observable(nil)
