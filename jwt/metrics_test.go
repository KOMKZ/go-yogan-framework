package jwt

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestNewJWTMetrics(t *testing.T) {
	t.Run("creates with config", func(t *testing.T) {
		cfg := JWTMetricsConfig{Enabled: true}
		m := NewJWTMetrics(cfg)

		assert.NotNil(t, m)
		assert.True(t, m.config.Enabled)
		assert.False(t, m.IsRegistered())
	})
}

func TestJWTMetrics_MetricsProvider(t *testing.T) {
	t.Run("MetricsName returns jwt", func(t *testing.T) {
		m := NewJWTMetrics(JWTMetricsConfig{Enabled: true})
		assert.Equal(t, "jwt", m.MetricsName())
	})

	t.Run("IsMetricsEnabled reflects config", func(t *testing.T) {
		m1 := NewJWTMetrics(JWTMetricsConfig{Enabled: true})
		assert.True(t, m1.IsMetricsEnabled())

		m2 := NewJWTMetrics(JWTMetricsConfig{Enabled: false})
		assert.False(t, m2.IsMetricsEnabled())
	})
}

func TestJWTMetrics_RegisterMetrics(t *testing.T) {
	t.Run("registers all metrics", func(t *testing.T) {
		mp := noop.NewMeterProvider()
		meter := mp.Meter("test")

		m := NewJWTMetrics(JWTMetricsConfig{Enabled: true})
		err := m.RegisterMetrics(meter)

		require.NoError(t, err)
		assert.True(t, m.IsRegistered())
		assert.NotNil(t, m.tokensGenerated)
		assert.NotNil(t, m.tokensVerified)
		assert.NotNil(t, m.tokensRefreshed)
		assert.NotNil(t, m.tokensRevoked)
		assert.NotNil(t, m.verificationDuration)
	})

	t.Run("idempotent registration", func(t *testing.T) {
		mp := noop.NewMeterProvider()
		meter := mp.Meter("test")

		m := NewJWTMetrics(JWTMetricsConfig{Enabled: true})

		err1 := m.RegisterMetrics(meter)
		require.NoError(t, err1)

		err2 := m.RegisterMetrics(meter)
		require.NoError(t, err2)
	})
}

func TestJWTMetrics_RecordMethods(t *testing.T) {
	mp := noop.NewMeterProvider()
	meter := mp.Meter("test")

	m := NewJWTMetrics(JWTMetricsConfig{Enabled: true})
	err := m.RegisterMetrics(meter)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("RecordGenerated does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordGenerated(ctx, "access")
		})
	})

	t.Run("RecordVerified does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordVerified(ctx, "success", 5*time.Millisecond)
		})
	})

	t.Run("RecordRefreshed does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordRefreshed(ctx, "success")
		})
	})

	t.Run("RecordRevoked does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordRevoked(ctx)
		})
	})

	t.Run("methods no-op when not registered", func(t *testing.T) {
		unregistered := NewJWTMetrics(JWTMetricsConfig{Enabled: true})
		assert.NotPanics(t, func() {
			unregistered.RecordGenerated(ctx, "access")
			unregistered.RecordVerified(ctx, "success", time.Second)
			unregistered.RecordRefreshed(ctx, "success")
			unregistered.RecordRevoked(ctx)
		})
	})
}
