package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	t.Run("creates manager with config", func(t *testing.T) {
		cfg := Config{
			Enabled:     true,
			ServiceName: "test-service",
		}
		m := NewManager(cfg, nil)
		require.NotNil(t, m)
		assert.Equal(t, "test-service", m.config.ServiceName)
		assert.True(t, m.config.Enabled)
	})

	t.Run("creates manager with default logger", func(t *testing.T) {
		cfg := Config{Enabled: false}
		m := NewManager(cfg, nil)
		require.NotNil(t, m)
		assert.NotNil(t, m.logger)
	})
}

func TestManager_Start_Disabled(t *testing.T) {
	t.Run("skips initialization when disabled", func(t *testing.T) {
		cfg := Config{Enabled: false}
		m := NewManager(cfg, nil)

		err := m.Start(context.Background())
		require.NoError(t, err)

		assert.Nil(t, m.tracerProvider)
		assert.Nil(t, m.metricsManager)
	})
}

func TestManager_Start_Enabled(t *testing.T) {
	t.Run("initializes tracer with stdout exporter", func(t *testing.T) {
		cfg := Config{
			Enabled:     true,
			ServiceName: "test-service",
			Exporter: ExporterConfig{
				Type:    "stdout",
				Timeout: 5 * time.Second,
			},
			Sampler: SamplerConfig{
				Type: "always_on",
			},
		}
		m := NewManager(cfg, nil)

		err := m.Start(context.Background())
		require.NoError(t, err)

		assert.NotNil(t, m.tracerProvider)
		assert.NotNil(t, m.shutdownFn)

		// Cleanup
		_ = m.Shutdown(context.Background())
	})

	t.Run("initializes metrics when metrics enabled", func(t *testing.T) {
		cfg := Config{
			Enabled:     true,
			ServiceName: "test-service",
			Exporter: ExporterConfig{
				Type:    "stdout",
				Timeout: 5 * time.Second,
			},
			Sampler: SamplerConfig{
				Type: "always_on",
			},
			Metrics: MetricsConfig{
				Enabled:        true,
				ExportInterval: 5 * time.Second,
				ExportTimeout:  10 * time.Second,
			},
		}
		m := NewManager(cfg, nil)

		err := m.Start(context.Background())
		require.NoError(t, err)

		assert.NotNil(t, m.tracerProvider)
		assert.NotNil(t, m.metricsManager)
		assert.True(t, m.metricsManager.IsEnabled())

		// Cleanup
		_ = m.Shutdown(context.Background())
	})

	t.Run("skips metrics when metrics disabled", func(t *testing.T) {
		cfg := Config{
			Enabled:     true,
			ServiceName: "test-service",
			Exporter: ExporterConfig{
				Type:    "stdout",
				Timeout: 5 * time.Second,
			},
			Sampler: SamplerConfig{
				Type: "always_on",
			},
			Metrics: MetricsConfig{
				Enabled: false,
			},
		}
		m := NewManager(cfg, nil)

		err := m.Start(context.Background())
		require.NoError(t, err)

		assert.NotNil(t, m.tracerProvider)
		assert.Nil(t, m.metricsManager)

		// Cleanup
		_ = m.Shutdown(context.Background())
	})
}

func TestManager_Shutdown(t *testing.T) {
	t.Run("shutdown nil manager", func(t *testing.T) {
		var m *Manager = nil
		err := m.Shutdown(context.Background())
		require.NoError(t, err)
	})

	t.Run("shutdown when not started", func(t *testing.T) {
		cfg := Config{Enabled: false}
		m := NewManager(cfg, nil)

		err := m.Shutdown(context.Background())
		require.NoError(t, err)
	})

	t.Run("shutdown after start", func(t *testing.T) {
		cfg := Config{
			Enabled:     true,
			ServiceName: "test-service",
			Exporter: ExporterConfig{
				Type:    "stdout",
				Timeout: 5 * time.Second,
			},
			Sampler: SamplerConfig{
				Type: "always_on",
			},
			Metrics: MetricsConfig{
				Enabled:        true,
				ExportInterval: 5 * time.Second,
				ExportTimeout:  10 * time.Second,
			},
		}
		m := NewManager(cfg, nil)

		err := m.Start(context.Background())
		require.NoError(t, err)

		err = m.Shutdown(context.Background())
		require.NoError(t, err)
	})
}

func TestManager_GetTracer(t *testing.T) {
	t.Run("returns tracer from provider", func(t *testing.T) {
		cfg := Config{
			Enabled:     true,
			ServiceName: "test-service",
			Exporter: ExporterConfig{
				Type:    "stdout",
				Timeout: 5 * time.Second,
			},
			Sampler: SamplerConfig{
				Type: "always_on",
			},
		}
		m := NewManager(cfg, nil)
		_ = m.Start(context.Background())
		defer m.Shutdown(context.Background())

		tracer := m.GetTracer("test-tracer")
		assert.NotNil(t, tracer)
	})

	t.Run("returns global tracer when provider is nil", func(t *testing.T) {
		cfg := Config{Enabled: false}
		m := NewManager(cfg, nil)

		tracer := m.GetTracer("test-tracer")
		assert.NotNil(t, tracer)
	})
}

func TestManager_GetMetricsManager(t *testing.T) {
	t.Run("returns metrics manager after initialization", func(t *testing.T) {
		cfg := Config{
			Enabled:     true,
			ServiceName: "test-service",
			Exporter: ExporterConfig{
				Type:    "stdout",
				Timeout: 5 * time.Second,
			},
			Sampler: SamplerConfig{
				Type: "always_on",
			},
			Metrics: MetricsConfig{
				Enabled:        true,
				ExportInterval: 5 * time.Second,
				ExportTimeout:  10 * time.Second,
			},
		}
		m := NewManager(cfg, nil)
		_ = m.Start(context.Background())
		defer m.Shutdown(context.Background())

		mm := m.GetMetricsManager()
		assert.NotNil(t, mm)
		assert.True(t, mm.IsEnabled())
	})

	t.Run("returns nil when not initialized", func(t *testing.T) {
		cfg := Config{Enabled: false}
		m := NewManager(cfg, nil)

		mm := m.GetMetricsManager()
		assert.Nil(t, mm)
	})
}

func TestManager_IsEnabled(t *testing.T) {
	t.Run("returns config enabled state", func(t *testing.T) {
		m := NewManager(Config{Enabled: true}, nil)
		assert.True(t, m.IsEnabled())

		m = NewManager(Config{Enabled: false}, nil)
		assert.False(t, m.IsEnabled())
	})
}

func TestManager_GetConfig(t *testing.T) {
	t.Run("returns config", func(t *testing.T) {
		cfg := Config{
			Enabled:     true,
			ServiceName: "my-service",
		}
		m := NewManager(cfg, nil)

		result := m.GetConfig()
		assert.Equal(t, "my-service", result.ServiceName)
		assert.True(t, result.Enabled)
	})
}
