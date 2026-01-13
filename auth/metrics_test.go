package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestNewAuthMetrics(t *testing.T) {
	t.Run("creates with config", func(t *testing.T) {
		cfg := AuthMetricsConfig{Enabled: true}
		m := NewAuthMetrics(cfg)

		assert.NotNil(t, m)
		assert.True(t, m.config.Enabled)
		assert.False(t, m.IsRegistered())
	})
}

func TestAuthMetrics_MetricsProvider(t *testing.T) {
	t.Run("MetricsName returns auth", func(t *testing.T) {
		m := NewAuthMetrics(AuthMetricsConfig{Enabled: true})
		assert.Equal(t, "auth", m.MetricsName())
	})

	t.Run("IsMetricsEnabled reflects config", func(t *testing.T) {
		m1 := NewAuthMetrics(AuthMetricsConfig{Enabled: true})
		assert.True(t, m1.IsMetricsEnabled())

		m2 := NewAuthMetrics(AuthMetricsConfig{Enabled: false})
		assert.False(t, m2.IsMetricsEnabled())
	})
}

func TestAuthMetrics_RegisterMetrics(t *testing.T) {
	t.Run("registers all metrics", func(t *testing.T) {
		mp := noop.NewMeterProvider()
		meter := mp.Meter("test")

		m := NewAuthMetrics(AuthMetricsConfig{Enabled: true})
		err := m.RegisterMetrics(meter)

		require.NoError(t, err)
		assert.True(t, m.IsRegistered())
		assert.NotNil(t, m.loginsTotal)
		assert.NotNil(t, m.loginDuration)
		assert.NotNil(t, m.passwordValidations)
		assert.NotNil(t, m.failedAttempts)
	})

	t.Run("idempotent registration", func(t *testing.T) {
		mp := noop.NewMeterProvider()
		meter := mp.Meter("test")

		m := NewAuthMetrics(AuthMetricsConfig{Enabled: true})

		err1 := m.RegisterMetrics(meter)
		require.NoError(t, err1)

		err2 := m.RegisterMetrics(meter)
		require.NoError(t, err2)
	})
}

func TestAuthMetrics_RecordMethods(t *testing.T) {
	mp := noop.NewMeterProvider()
	meter := mp.Meter("test")

	m := NewAuthMetrics(AuthMetricsConfig{Enabled: true})
	err := m.RegisterMetrics(meter)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("RecordLogin does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordLogin(ctx, "password", "success", 100*time.Millisecond)
		})
	})

	t.Run("RecordPasswordValidation does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordPasswordValidation(ctx, "valid")
		})
	})

	t.Run("RecordFailedAttempt does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordFailedAttempt(ctx, "invalid_password")
		})
	})

	t.Run("methods no-op when not registered", func(t *testing.T) {
		unregistered := NewAuthMetrics(AuthMetricsConfig{Enabled: true})
		assert.NotPanics(t, func() {
			unregistered.RecordLogin(ctx, "password", "success", time.Second)
			unregistered.RecordPasswordValidation(ctx, "valid")
			unregistered.RecordFailedAttempt(ctx, "reason")
		})
	})
}
