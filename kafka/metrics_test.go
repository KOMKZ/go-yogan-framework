package kafka

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestNewKafkaMetrics(t *testing.T) {
	t.Run("creates with config", func(t *testing.T) {
		cfg := KafkaMetricsConfig{
			Enabled:   true,
			RecordLag: true,
		}
		m := NewKafkaMetrics(cfg)

		assert.NotNil(t, m)
		assert.True(t, m.config.Enabled)
		assert.False(t, m.IsRegistered())
	})
}

func TestKafkaMetrics_MetricsProvider(t *testing.T) {
	t.Run("MetricsName returns kafka", func(t *testing.T) {
		m := NewKafkaMetrics(KafkaMetricsConfig{Enabled: true})
		assert.Equal(t, "kafka", m.MetricsName())
	})

	t.Run("IsMetricsEnabled reflects config", func(t *testing.T) {
		m1 := NewKafkaMetrics(KafkaMetricsConfig{Enabled: true})
		assert.True(t, m1.IsMetricsEnabled())

		m2 := NewKafkaMetrics(KafkaMetricsConfig{Enabled: false})
		assert.False(t, m2.IsMetricsEnabled())
	})
}

func TestKafkaMetrics_RegisterMetrics(t *testing.T) {
	t.Run("registers all metrics", func(t *testing.T) {
		mp := noop.NewMeterProvider()
		meter := mp.Meter("test")

		m := NewKafkaMetrics(KafkaMetricsConfig{
			Enabled:   true,
			RecordLag: true,
		})
		err := m.RegisterMetrics(meter)

		require.NoError(t, err)
		assert.True(t, m.IsRegistered())
		assert.NotNil(t, m.messagesProduced)
		assert.NotNil(t, m.produceDuration)
		assert.NotNil(t, m.messagesConsumed)
		assert.NotNil(t, m.consumeDuration)
	})

	t.Run("idempotent registration", func(t *testing.T) {
		mp := noop.NewMeterProvider()
		meter := mp.Meter("test")

		m := NewKafkaMetrics(KafkaMetricsConfig{Enabled: true})

		err1 := m.RegisterMetrics(meter)
		require.NoError(t, err1)

		err2 := m.RegisterMetrics(meter)
		require.NoError(t, err2)
	})
}

func TestKafkaMetrics_RecordMethods(t *testing.T) {
	mp := noop.NewMeterProvider()
	meter := mp.Meter("test")

	m := NewKafkaMetrics(KafkaMetricsConfig{Enabled: true})
	err := m.RegisterMetrics(meter)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("RecordProduce does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordProduce(ctx, "test-topic", 0, 10*time.Millisecond, nil)
		})
	})

	t.Run("RecordConsume does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordConsume(ctx, "test-topic", "test-group", 0, 20*time.Millisecond, nil)
		})
	})

	t.Run("methods no-op when not registered", func(t *testing.T) {
		unregistered := NewKafkaMetrics(KafkaMetricsConfig{Enabled: true})
		assert.NotPanics(t, func() {
			unregistered.RecordProduce(ctx, "topic", 0, time.Second, nil)
			unregistered.RecordConsume(ctx, "topic", "group", 0, time.Second, nil)
		})
	})
}

func TestKafkaMetrics_LagCallbacks(t *testing.T) {
	m := NewKafkaMetrics(KafkaMetricsConfig{Enabled: true, RecordLag: true})

	t.Run("register and unregister callbacks", func(t *testing.T) {
		callback := func() int64 { return 100 }

		m.RegisterLagCallback("group1", callback)
		assert.Len(t, m.lagCallbacks, 1)

		m.RegisterLagCallback("group2", callback)
		assert.Len(t, m.lagCallbacks, 2)

		m.UnregisterLagCallback("group1")
		assert.Len(t, m.lagCallbacks, 1)
	})
}
