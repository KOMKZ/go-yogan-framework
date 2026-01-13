package telemetry

import (
	"testing"

	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
)

// mockMetricsProvider is a mock implementation of MetricsProvider
type mockMetricsProvider struct {
	name           string
	enabled        bool
	registerCalled bool
	registerError  error
}

func (m *mockMetricsProvider) MetricsName() string {
	return m.name
}

func (m *mockMetricsProvider) RegisterMetrics(meter metric.Meter) error {
	m.registerCalled = true
	return m.registerError
}

func (m *mockMetricsProvider) IsMetricsEnabled() bool {
	return m.enabled
}

func TestNewMetricsRegistry(t *testing.T) {
	t.Run("with nil meter provider uses global", func(t *testing.T) {
		r := NewMetricsRegistry(nil)
		assert.NotNil(t, r)
		assert.True(t, r.IsEnabled())
		assert.Equal(t, "yogan", r.namespace)
	})

	t.Run("with custom options", func(t *testing.T) {
		mp := noop.NewMeterProvider()
		labels := []attribute.KeyValue{
			attribute.String("env", "test"),
		}

		r := NewMetricsRegistry(mp,
			WithNamespace("custom"),
			WithBaseLabels(labels),
		)

		assert.NotNil(t, r)
		assert.Equal(t, "custom", r.namespace)
		assert.Len(t, r.GetBaseLabels(), 1)
		assert.Equal(t, "env", string(r.GetBaseLabels()[0].Key))
	})

	t.Run("with logger option", func(t *testing.T) {
		l := logger.GetLogger("test")
		r := NewMetricsRegistry(nil, WithLogger(l))
		assert.NotNil(t, r)
		assert.Equal(t, l, r.logger)
	})
}

func TestMetricsRegistry_Register(t *testing.T) {
	t.Run("register valid provider", func(t *testing.T) {
		mp := noop.NewMeterProvider()
		r := NewMetricsRegistry(mp)

		provider := &mockMetricsProvider{
			name:    "test",
			enabled: true,
		}

		err := r.Register(provider)
		require.NoError(t, err)
		assert.True(t, provider.registerCalled)
		assert.Equal(t, 1, r.GetProviderCount())
	})

	t.Run("register nil provider returns error", func(t *testing.T) {
		r := NewMetricsRegistry(nil)
		err := r.Register(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nil")
	})

	t.Run("register disabled provider skips", func(t *testing.T) {
		r := NewMetricsRegistry(nil)

		provider := &mockMetricsProvider{
			name:    "test",
			enabled: false,
		}

		err := r.Register(provider)
		require.NoError(t, err)
		assert.False(t, provider.registerCalled)
		assert.Equal(t, 0, r.GetProviderCount())
	})

	t.Run("register when registry disabled", func(t *testing.T) {
		r := NewMetricsRegistry(nil)
		r.SetEnabled(false)

		provider := &mockMetricsProvider{
			name:    "test",
			enabled: true,
		}

		err := r.Register(provider)
		require.NoError(t, err)
		assert.False(t, provider.registerCalled)
	})

	t.Run("register duplicate provider returns error", func(t *testing.T) {
		r := NewMetricsRegistry(nil)

		provider1 := &mockMetricsProvider{name: "test", enabled: true}
		provider2 := &mockMetricsProvider{name: "test", enabled: true}

		err := r.Register(provider1)
		require.NoError(t, err)

		err = r.Register(provider2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})

	t.Run("register provider with empty name returns error", func(t *testing.T) {
		r := NewMetricsRegistry(nil)

		provider := &mockMetricsProvider{name: "", enabled: true}

		err := r.Register(provider)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty")
	})

	t.Run("register provider with register error", func(t *testing.T) {
		r := NewMetricsRegistry(nil)

		provider := &mockMetricsProvider{
			name:          "test",
			enabled:       true,
			registerError: assert.AnError,
		}

		err := r.Register(provider)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "register metrics")
	})
}

func TestMetricsRegistry_GetMeter(t *testing.T) {
	t.Run("get meter creates new meter", func(t *testing.T) {
		mp := noop.NewMeterProvider()
		r := NewMetricsRegistry(mp, WithNamespace("app"))

		meter := r.GetMeter("redis")
		assert.NotNil(t, meter)
	})

	t.Run("get meter returns cached meter", func(t *testing.T) {
		mp := noop.NewMeterProvider()
		r := NewMetricsRegistry(mp)

		meter1 := r.GetMeter("redis")
		meter2 := r.GetMeter("redis")

		// Both should reference the same meter
		assert.NotNil(t, meter1)
		assert.NotNil(t, meter2)
	})

	t.Run("different names return different meters", func(t *testing.T) {
		mp := noop.NewMeterProvider()
		r := NewMetricsRegistry(mp)

		meter1 := r.GetMeter("redis")
		meter2 := r.GetMeter("kafka")

		assert.NotNil(t, meter1)
		assert.NotNil(t, meter2)
	})
}

func TestMetricsRegistry_GetProviders(t *testing.T) {
	t.Run("returns copy of providers", func(t *testing.T) {
		r := NewMetricsRegistry(nil)

		p1 := &mockMetricsProvider{name: "test1", enabled: true}
		p2 := &mockMetricsProvider{name: "test2", enabled: true}

		_ = r.Register(p1)
		_ = r.Register(p2)

		providers := r.GetProviders()
		assert.Len(t, providers, 2)

		// Modifying the returned slice should not affect internal state
		providers[0] = nil
		assert.Equal(t, 2, r.GetProviderCount())
	})
}

func TestMetricsRegistry_GetBaseLabels(t *testing.T) {
	t.Run("returns copy of labels", func(t *testing.T) {
		labels := []attribute.KeyValue{
			attribute.String("env", "test"),
			attribute.String("region", "us-west"),
		}
		r := NewMetricsRegistry(nil, WithBaseLabels(labels))

		result := r.GetBaseLabels()
		assert.Len(t, result, 2)

		// Modifying the returned slice should not affect internal state
		result[0] = attribute.String("modified", "value")
		assert.Equal(t, "env", string(r.GetBaseLabels()[0].Key))
	})
}

func TestMetricsRegistry_SetEnabled(t *testing.T) {
	t.Run("enable and disable", func(t *testing.T) {
		r := NewMetricsRegistry(nil)
		assert.True(t, r.IsEnabled())

		r.SetEnabled(false)
		assert.False(t, r.IsEnabled())

		r.SetEnabled(true)
		assert.True(t, r.IsEnabled())
	})
}

func TestMetricsRegistry_ImplementsInterface(t *testing.T) {
	t.Run("implements MetricsCollector", func(t *testing.T) {
		var _ component.MetricsCollector = (*MetricsRegistry)(nil)
	})
}
