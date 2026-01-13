package telemetry

import (
	"context"
	"fmt"

	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace"
	otelTrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const ComponentName = "telemetry"

// Component OpenTelemetry 组件
type Component struct {
	config          Config
	logger          *logger.CtxZapLogger
	tracerProvider  *trace.TracerProvider
	shutdownFn      func(context.Context) error
	circuitBreaker  *CircuitBreaker  // 熔断器
	metricsManager  *MetricsManager  // Metrics 管理器
	metricsRegistry *MetricsRegistry // 统一 Metrics 注册中心
}

// NewComponent 创建 Telemetry 组件
func NewComponent() *Component {
	return &Component{
		logger: logger.GetLogger("yogan"),
		config: DefaultConfig(), // 🔧 使用默认配置初始化
	}
}

// Name 返回组件名称
func (c *Component) Name() string {
	return ComponentName
}

// DependsOn 返回依赖的组件
func (c *Component) DependsOn() []string {
	return []string{
		component.ComponentConfig,
		component.ComponentLogger,
	}
}

// Init 初始化组件
func (c *Component) Init(ctx context.Context, loader component.ConfigLoader) error {
	// 加载默认配置
	c.config = DefaultConfig()

	// 读取配置（如果配置文件中有 telemetry 配置）
	if loader.IsSet("telemetry") {
		var loadedConfig Config
		if err := loader.Unmarshal("telemetry", &loadedConfig); err != nil {
			c.logger.ErrorCtx(ctx, "telemetry config exists but unmarshal failed", zap.Error(err))
			return fmt.Errorf("unmarshal telemetry config failed: %w", err)
		}
		c.config = loadedConfig
	}
	// 否则使用构造函数中初始化的默认配置（enabled=false）

	// 验证配置
	if err := c.config.Validate(); err != nil {
		return fmt.Errorf("validate telemetry config failed: %w", err)
	}

	// 检查是否启用
	if !c.config.Enabled {
		c.logger.InfoCtx(ctx, "OpenTelemetry is disabled")
		return nil
	}

	// 创建 TracerProvider
	tp, shutdownFn, err := c.createTracerProvider(ctx)
	if err != nil {
		return fmt.Errorf("create tracer provider failed: %w", err)
	}

	c.tracerProvider = tp
	c.shutdownFn = shutdownFn

	// 设置全局 TracerProvider
	otel.SetTracerProvider(tp)

	// 🎯 设置全局 TextMapPropagator（用于跨服务 trace context 传播）
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// 🎯 初始化 Metrics（使用相同的 Resource）
	if c.config.Metrics.Enabled {
		resource, err := c.createResource(ctx)
		if err != nil {
			return fmt.Errorf("create resource for metrics failed: %w", err)
		}

		metricsManager, err := NewMetricsManager(c.config, resource)
		if err != nil {
			c.logger.ErrorCtx(ctx, "❌ Failed to create Metrics manager", zap.Error(err))
			return fmt.Errorf("create metrics manager failed: %w", err)
		}

		c.metricsManager = metricsManager

		// 创建统一 Metrics 注册中心
		c.metricsRegistry = c.createMetricsRegistry()

		c.logger.InfoCtx(ctx, "✅ Metrics initialized",
			zap.Bool("http_enabled", c.config.Metrics.HTTP.Enabled),
			zap.Bool("db_enabled", c.config.Metrics.Database.Enabled),
			zap.Bool("grpc_enabled", c.config.Metrics.GRPC.Enabled),
			zap.String("namespace", c.config.Metrics.Namespace),
			zap.Duration("export_interval", c.config.Metrics.ExportInterval),
		)
	}

	c.logger.InfoCtx(ctx, "✅ OpenTelemetry initialized",
		zap.String("service_name", c.config.ServiceName),
		zap.String("service_version", c.config.ServiceVersion),
		zap.String("exporter_type", c.config.Exporter.Type),
		zap.String("exporter_endpoint", c.config.Exporter.Endpoint),
		zap.String("sampler_type", c.config.Sampler.Type),
	)

	return nil
}

// Start 启动组件
func (c *Component) Start(ctx context.Context) error {
	if !c.config.Enabled {
		return nil
	}

	c.logger.InfoCtx(ctx, "OpenTelemetry started")
	return nil
}

// Stop 停止组件
func (c *Component) Stop(ctx context.Context) error {
	if !c.config.Enabled {
		return nil
	}

	c.logger.InfoCtx(ctx, "Shutting down OpenTelemetry...")

	// 关闭 Metrics
	if c.metricsManager != nil {
		if err := c.metricsManager.Shutdown(ctx); err != nil {
			c.logger.ErrorCtx(ctx, "Failed to shutdown Metrics", zap.Error(err))
		}
	}

	// 关闭 Tracer
	if c.shutdownFn != nil {
		if err := c.shutdownFn(ctx); err != nil {
			c.logger.ErrorCtx(ctx, "Failed to shutdown OpenTelemetry", zap.Error(err))
			return err
		}
	}

	c.logger.InfoCtx(ctx, "✅ OpenTelemetry stopped")
	return nil
}

// GetTracerProvider 获取 TracerProvider
func (c *Component) GetTracerProvider() otelTrace.TracerProvider {
	if c.tracerProvider == nil {
		return otel.GetTracerProvider() // 返回全局的（no-op）
	}
	return c.tracerProvider
}

// GetTracer 获取 Tracer
func (c *Component) GetTracer(name string) otelTrace.Tracer {
	return c.GetTracerProvider().Tracer(name)
}

// IsEnabled 是否启用
func (c *Component) IsEnabled() bool {
	return c.config.Enabled
}

// GetConfig 获取配置（用于测试）
func (c *Component) GetConfig() Config {
	return c.config
}

// GetMetricsManager 获取 Metrics 管理器
func (c *Component) GetMetricsManager() *MetricsManager {
	return c.metricsManager
}

// GetMetricsRegistry 获取统一 Metrics 注册中心
func (c *Component) GetMetricsRegistry() *MetricsRegistry {
	return c.metricsRegistry
}

// createMetricsRegistry 创建 Metrics 注册中心
func (c *Component) createMetricsRegistry() *MetricsRegistry {
	// 构建全局标签
	baseLabels := c.buildBaseLabels()

	return NewMetricsRegistry(
		otel.GetMeterProvider(),
		WithNamespace(c.config.Metrics.Namespace),
		WithBaseLabels(baseLabels),
		WithLogger(c.logger),
	)
}

// buildBaseLabels 构建全局基础标签
func (c *Component) buildBaseLabels() []attribute.KeyValue {
	labels := []attribute.KeyValue{
		attribute.String("service.name", c.config.ServiceName),
		attribute.String("service.version", c.config.ServiceVersion),
	}

	// 添加配置中的自定义标签
	for k, v := range c.config.Metrics.Labels {
		labels = append(labels, attribute.String(k, v))
	}

	return labels
}

// GetCircuitBreaker 获取熔断器（用于监控）
func (c *Component) GetCircuitBreaker() *CircuitBreaker {
	return c.circuitBreaker
}

// GetCircuitBreakerStats 获取熔断器统计信息
func (c *Component) GetCircuitBreakerStats() map[string]interface{} {
	if c.circuitBreaker == nil {
		return map[string]interface{}{
			"enabled": false,
		}
	}
	stats := c.circuitBreaker.GetStats()
	stats["enabled"] = true
	return stats
}
