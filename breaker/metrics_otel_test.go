package breaker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestNewOTelBreakerMetrics(t *testing.T) {
	t.Run("creates with config", func(t *testing.T) {
		cfg := BreakerMetricsConfig{
			Enabled:           true,
			RecordState:       true,
			RecordSuccessRate: true,
		}
		m := NewOTelBreakerMetrics(cfg)

		assert.NotNil(t, m)
		assert.True(t, m.config.Enabled)
		assert.False(t, m.IsRegistered())
	})
}

func TestOTelBreakerMetrics_MetricsProvider(t *testing.T) {
	t.Run("MetricsName returns breaker", func(t *testing.T) {
		m := NewOTelBreakerMetrics(BreakerMetricsConfig{Enabled: true})
		assert.Equal(t, "breaker", m.MetricsName())
	})

	t.Run("IsMetricsEnabled reflects config", func(t *testing.T) {
		m1 := NewOTelBreakerMetrics(BreakerMetricsConfig{Enabled: true})
		assert.True(t, m1.IsMetricsEnabled())

		m2 := NewOTelBreakerMetrics(BreakerMetricsConfig{Enabled: false})
		assert.False(t, m2.IsMetricsEnabled())
	})
}

func TestOTelBreakerMetrics_RegisterMetrics(t *testing.T) {
	t.Run("registers all metrics", func(t *testing.T) {
		mp := noop.NewMeterProvider()
		meter := mp.Meter("test")

		m := NewOTelBreakerMetrics(BreakerMetricsConfig{
			Enabled:     true,
			RecordState: true,
		})
		err := m.RegisterMetrics(meter)

		require.NoError(t, err)
		assert.True(t, m.IsRegistered())
		assert.NotNil(t, m.requestsTotal)
		assert.NotNil(t, m.successesTotal)
		assert.NotNil(t, m.failuresTotal)
		assert.NotNil(t, m.rejectionsTotal)
		assert.NotNil(t, m.latency)
	})

	t.Run("idempotent registration", func(t *testing.T) {
		mp := noop.NewMeterProvider()
		meter := mp.Meter("test")

		m := NewOTelBreakerMetrics(BreakerMetricsConfig{Enabled: true})

		err1 := m.RegisterMetrics(meter)
		require.NoError(t, err1)

		err2 := m.RegisterMetrics(meter)
		require.NoError(t, err2)
	})
}

func TestOTelBreakerMetrics_RecordMethods(t *testing.T) {
	mp := noop.NewMeterProvider()
	meter := mp.Meter("test")

	m := NewOTelBreakerMetrics(BreakerMetricsConfig{Enabled: true})
	err := m.RegisterMetrics(meter)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("RecordSuccess does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordSuccess(ctx, "test-resource", 100*time.Millisecond)
		})
	})

	t.Run("RecordFailure does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordFailure(ctx, "test-resource", 50*time.Millisecond, "timeout")
		})
	})

	t.Run("RecordRejection does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordRejection(ctx, "test-resource")
		})
	})

	t.Run("methods no-op when not registered", func(t *testing.T) {
		unregistered := NewOTelBreakerMetrics(BreakerMetricsConfig{Enabled: true})
		assert.NotPanics(t, func() {
			unregistered.RecordSuccess(ctx, "test", time.Second)
			unregistered.RecordFailure(ctx, "test", time.Second, "error")
			unregistered.RecordRejection(ctx, "test")
		})
	})
}

func TestOTelBreakerMetrics_StateCallbacks(t *testing.T) {
	m := NewOTelBreakerMetrics(BreakerMetricsConfig{Enabled: true, RecordState: true})

	t.Run("register and unregister callbacks", func(t *testing.T) {
		callback := func() int64 { return 0 } // 0 = closed

		m.RegisterStateCallback("resource1", callback)
		assert.Len(t, m.stateCallbacks, 1)

		m.RegisterStateCallback("resource2", callback)
		assert.Len(t, m.stateCallbacks, 2)

		m.UnregisterStateCallback("resource1")
		assert.Len(t, m.stateCallbacks, 1)

		m.UnregisterStateCallback("resource2")
		assert.Len(t, m.stateCallbacks, 0)
	})
}
