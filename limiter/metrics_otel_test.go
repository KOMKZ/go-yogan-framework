package limiter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestNewOTelMetrics(t *testing.T) {
	t.Run("creates with config", func(t *testing.T) {
		cfg := MetricsConfig{
			Enabled:          true,
			RecordTokens:     true,
			RecordRejectRate: true,
		}
		m := NewOTelMetrics(cfg)

		assert.NotNil(t, m)
		assert.True(t, m.config.Enabled)
		assert.False(t, m.IsRegistered())
	})
}

func TestOTelMetrics_MetricsProvider(t *testing.T) {
	t.Run("MetricsName returns limiter", func(t *testing.T) {
		m := NewOTelMetrics(MetricsConfig{Enabled: true})
		assert.Equal(t, "limiter", m.MetricsName())
	})

	t.Run("IsMetricsEnabled reflects config", func(t *testing.T) {
		m1 := NewOTelMetrics(MetricsConfig{Enabled: true})
		assert.True(t, m1.IsMetricsEnabled())

		m2 := NewOTelMetrics(MetricsConfig{Enabled: false})
		assert.False(t, m2.IsMetricsEnabled())
	})
}

func TestOTelMetrics_RegisterMetrics(t *testing.T) {
	t.Run("registers all metrics", func(t *testing.T) {
		mp := noop.NewMeterProvider()
		meter := mp.Meter("test")

		m := NewOTelMetrics(MetricsConfig{
			Enabled:      true,
			RecordTokens: true,
		})
		err := m.RegisterMetrics(meter)

		require.NoError(t, err)
		assert.True(t, m.IsRegistered())
		assert.NotNil(t, m.requestsTotal)
		assert.NotNil(t, m.allowedTotal)
		assert.NotNil(t, m.rejectedTotal)
	})

	t.Run("idempotent registration", func(t *testing.T) {
		mp := noop.NewMeterProvider()
		meter := mp.Meter("test")

		m := NewOTelMetrics(MetricsConfig{Enabled: true})

		err1 := m.RegisterMetrics(meter)
		require.NoError(t, err1)

		err2 := m.RegisterMetrics(meter)
		require.NoError(t, err2)
	})
}

func TestOTelMetrics_RecordMethods(t *testing.T) {
	mp := noop.NewMeterProvider()
	meter := mp.Meter("test")

	m := NewOTelMetrics(MetricsConfig{Enabled: true})
	err := m.RegisterMetrics(meter)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("RecordAllowed does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordAllowed(ctx, "test-resource", "token_bucket")
		})
	})

	t.Run("RecordRejected does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordRejected(ctx, "test-resource", "token_bucket", "rate_exceeded")
		})
	})

	t.Run("methods no-op when not registered", func(t *testing.T) {
		unregistered := NewOTelMetrics(MetricsConfig{Enabled: true})
		assert.NotPanics(t, func() {
			unregistered.RecordAllowed(ctx, "test", "algo")
			unregistered.RecordRejected(ctx, "test", "algo", "reason")
		})
	})
}

func TestOTelMetrics_TokenCallbacks(t *testing.T) {
	m := NewOTelMetrics(MetricsConfig{Enabled: true, RecordTokens: true})

	t.Run("register and unregister callbacks", func(t *testing.T) {
		callback := func() int64 { return 100 }

		m.RegisterTokenCallback("resource1", callback)
		assert.Len(t, m.tokenCallbacks, 1)

		m.RegisterTokenCallback("resource2", callback)
		assert.Len(t, m.tokenCallbacks, 2)

		m.UnregisterTokenCallback("resource1")
		assert.Len(t, m.tokenCallbacks, 1)

		m.UnregisterTokenCallback("resource2")
		assert.Len(t, m.tokenCallbacks, 0)
	})
}
