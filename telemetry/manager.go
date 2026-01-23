package telemetry

import (
	"context"
	"sync"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	otelTrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Manager Telemetry Manager (replacing Component)
// Manage TracerProvider, MetricsRegistry etc.
type Manager struct {
	config         Config
	logger         *logger.CtxZapLogger
	tracerProvider *trace.TracerProvider
	circuitBreaker *CircuitBreaker
	metricsManager *MetricsManager
	shutdownFn     func(context.Context) error
	mu             sync.RWMutex
}

// Create telemetry manager
func NewManager(config Config, log *logger.CtxZapLogger) *Manager {
	if log == nil {
		log = logger.GetLogger("yogan")
	}
	return &Manager{
		config: config,
		logger: log,
	}
}

// Start telemetry component
func (m *Manager) Start(ctx context.Context) error {
	if !m.config.Enabled {
		m.logger.InfoCtx(ctx, "Telemetry disabled, skipping initialization")
		return nil
	}

	// 1. Create Resource (shared by Traces and Metrics)
	res, err := m.createResource(ctx)
	if err != nil {
		return err
	}

	// 2. Create TracerProvider
	tp, shutdownFn, err := m.createTracerProvider(ctx)
	if err != nil {
		return err
	}

	m.tracerProvider = tp
	m.shutdownFn = shutdownFn

	// Set global TracerProvider
	otel.SetTracerProvider(tp)

	m.logger.InfoCtx(ctx, "✅ Telemetry Traces started",
		zap.String("service_name", m.config.ServiceName),
		zap.String("exporter", m.config.Exporter.Type),
	)

	// 3. Create MetricsManager (if metrics enabled)
	if m.config.Metrics.Enabled {
		mm, err := NewMetricsManager(m.config, res)
		if err != nil {
			m.logger.WarnCtx(ctx, "⚠️ MetricsManager creation failed, metrics disabled",
				zap.Error(err),
			)
			// Don't return error, allow traces to work without metrics
		} else {
			m.metricsManager = mm
			m.logger.InfoCtx(ctx, "✅ Telemetry Metrics started",
				zap.Duration("export_interval", m.config.Metrics.ExportInterval),
			)
		}
	}

	return nil
}

// Shut down telemetry component
func (m *Manager) Shutdown(ctx context.Context) error {
	if m == nil {
		return nil
	}

	var errs []error

	// Shutdown MetricsManager first
	if m.metricsManager != nil {
		if err := m.metricsManager.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	// Shutdown TracerProvider
	if m.shutdownFn != nil {
		if err := m.shutdownFn(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errs[0] // Return first error
	}
	return nil
}

// GetTracer obtain tracer
func (m *Manager) GetTracer(name string) otelTrace.Tracer {
	if m.tracerProvider == nil {
		return otel.GetTracerProvider().Tracer(name)
	}
	return m.tracerProvider.Tracer(name)
}

// GetCircuitBreaker obtain circuit breaker
func (m *Manager) GetCircuitBreaker() *CircuitBreaker {
	return m.circuitBreaker
}

// GetMetricsManager obtain Metrics manager
func (m *Manager) GetMetricsManager() *MetricsManager {
	return m.metricsManager
}

// IsEnabled whether enabled
func (m *Manager) IsEnabled() bool {
	return m.config.Enabled
}

// GetConfig Retrieve configuration
func (m *Manager) GetConfig() Config {
	return m.config
}
