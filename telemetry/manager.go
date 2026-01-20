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

// Manager 遥测管理器（替代 Component）
// 管理 TracerProvider、MetricsRegistry 等
type Manager struct {
	config         Config
	logger         *logger.CtxZapLogger
	tracerProvider *trace.TracerProvider
	circuitBreaker *CircuitBreaker
	metricsManager *MetricsManager
	shutdownFn     func(context.Context) error
	mu             sync.RWMutex
}

// NewManager 创建遥测管理器
func NewManager(config Config, log *logger.CtxZapLogger) *Manager {
	if log == nil {
		log = logger.GetLogger("yogan")
	}
	return &Manager{
		config: config,
		logger: log,
	}
}

// Start 启动遥测组件
func (m *Manager) Start(ctx context.Context) error {
	if !m.config.Enabled {
		m.logger.InfoCtx(ctx, "Telemetry disabled, skipping initialization")
		return nil
	}

	// 创建 TracerProvider
	tp, shutdownFn, err := m.createTracerProvider(ctx)
	if err != nil {
		return err
	}

	m.tracerProvider = tp
	m.shutdownFn = shutdownFn

	// 设置全局 TracerProvider
	otel.SetTracerProvider(tp)

	m.logger.InfoCtx(ctx, "✅ Telemetry started",
		zap.String("service_name", m.config.ServiceName),
		zap.String("exporter", m.config.Exporter.Type),
	)

	return nil
}

// Shutdown 关闭遥测组件
func (m *Manager) Shutdown(ctx context.Context) error {
	if m.shutdownFn != nil {
		return m.shutdownFn(ctx)
	}
	return nil
}

// GetTracer 获取 Tracer
func (m *Manager) GetTracer(name string) otelTrace.Tracer {
	if m.tracerProvider == nil {
		return otel.GetTracerProvider().Tracer(name)
	}
	return m.tracerProvider.Tracer(name)
}

// GetCircuitBreaker 获取熔断器
func (m *Manager) GetCircuitBreaker() *CircuitBreaker {
	return m.circuitBreaker
}

// GetMetricsManager 获取 Metrics 管理器
func (m *Manager) GetMetricsManager() *MetricsManager {
	return m.metricsManager
}

// IsEnabled 是否启用
func (m *Manager) IsEnabled() bool {
	return m.config.Enabled
}

// GetConfig 获取配置
func (m *Manager) GetConfig() Config {
	return m.config
}
