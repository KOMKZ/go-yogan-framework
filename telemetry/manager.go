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

	// Create TracerProvider
	tp, shutdownFn, err := m.createTracerProvider(ctx)
	if err != nil {
		return err
	}

	m.tracerProvider = tp
	m.shutdownFn = shutdownFn

	// Set global TracerProvider
	otel.SetTracerProvider(tp)

	m.logger.InfoCtx(ctx, "✅ Telemetry started",
		zap.String("service_name", m.config.ServiceName),
		zap.String("exporter", m.config.Exporter.Type),
	)

	return nil
}

// Shut down telemetry component
func (m *Manager) Shutdown(ctx context.Context) error {
	if m.shutdownFn != nil {
		return m.shutdownFn(ctx)
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
