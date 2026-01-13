package event

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestNewEventMetrics(t *testing.T) {
	t.Run("creates with config", func(t *testing.T) {
		cfg := EventMetricsConfig{
			Enabled:         true,
			RecordQueueSize: true,
		}
		m := NewEventMetrics(cfg)

		assert.NotNil(t, m)
		assert.True(t, m.config.Enabled)
		assert.False(t, m.IsRegistered())
	})
}

func TestEventMetrics_MetricsProvider(t *testing.T) {
	t.Run("MetricsName returns event", func(t *testing.T) {
		m := NewEventMetrics(EventMetricsConfig{Enabled: true})
		assert.Equal(t, "event", m.MetricsName())
	})

	t.Run("IsMetricsEnabled reflects config", func(t *testing.T) {
		m1 := NewEventMetrics(EventMetricsConfig{Enabled: true})
		assert.True(t, m1.IsMetricsEnabled())

		m2 := NewEventMetrics(EventMetricsConfig{Enabled: false})
		assert.False(t, m2.IsMetricsEnabled())
	})
}

func TestEventMetrics_RegisterMetrics(t *testing.T) {
	t.Run("registers all metrics", func(t *testing.T) {
		mp := noop.NewMeterProvider()
		meter := mp.Meter("test")

		m := NewEventMetrics(EventMetricsConfig{
			Enabled:         true,
			RecordQueueSize: true,
		})
		err := m.RegisterMetrics(meter)

		require.NoError(t, err)
		assert.True(t, m.IsRegistered())
		assert.NotNil(t, m.eventsDispatched)
		assert.NotNil(t, m.eventsHandled)
		assert.NotNil(t, m.dispatchDuration)
	})

	t.Run("idempotent registration", func(t *testing.T) {
		mp := noop.NewMeterProvider()
		meter := mp.Meter("test")

		m := NewEventMetrics(EventMetricsConfig{Enabled: true})

		err1 := m.RegisterMetrics(meter)
		require.NoError(t, err1)

		err2 := m.RegisterMetrics(meter)
		require.NoError(t, err2)
	})
}

func TestEventMetrics_RecordMethods(t *testing.T) {
	mp := noop.NewMeterProvider()
	meter := mp.Meter("test")

	m := NewEventMetrics(EventMetricsConfig{Enabled: true})
	err := m.RegisterMetrics(meter)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("RecordDispatched does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordDispatched(ctx, "user.created", 5*time.Millisecond)
		})
	})

	t.Run("RecordHandled does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordHandled(ctx, "user.created", "SendWelcomeEmail", "success")
		})
	})

	t.Run("methods no-op when not registered", func(t *testing.T) {
		unregistered := NewEventMetrics(EventMetricsConfig{Enabled: true})
		assert.NotPanics(t, func() {
			unregistered.RecordDispatched(ctx, "topic", time.Second)
			unregistered.RecordHandled(ctx, "topic", "handler", "success")
		})
	})
}

func TestEventMetrics_QueueSizeCallback(t *testing.T) {
	m := NewEventMetrics(EventMetricsConfig{Enabled: true, RecordQueueSize: true})

	t.Run("set queue size callback", func(t *testing.T) {
		callback := func() int64 { return 42 }
		m.SetQueueSizeCallback(callback)
		assert.NotNil(t, m.queueSizeCallback)
	})
}
