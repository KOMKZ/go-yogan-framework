package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

// MetricsManager Metrics 管理器
type MetricsManager struct {
	meterProvider *sdkmetric.MeterProvider
	config        MetricsConfig
	enabled       bool
}

// NewMetricsManager 创建 Metrics 管理器
func NewMetricsManager(cfg Config, res *resource.Resource) (*MetricsManager, error) {
	if !cfg.Enabled || !cfg.Metrics.Enabled {
		return &MetricsManager{
			enabled: false,
			config:  cfg.Metrics,
		}, nil
	}

	// 创建 Exporter
	var exporter sdkmetric.Exporter
	var err error

	switch cfg.Exporter.Type {
	case "otlp":
		// OTLP gRPC Exporter
		opts := []otlpmetricgrpc.Option{
			otlpmetricgrpc.WithEndpoint(cfg.Exporter.Endpoint),
			otlpmetricgrpc.WithTimeout(cfg.Exporter.Timeout),
		}

		if cfg.Exporter.Insecure {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		}

		// 添加自定义 Headers（用于认证）
		if len(cfg.Exporter.Headers) > 0 {
			opts = append(opts, otlpmetricgrpc.WithHeaders(cfg.Exporter.Headers))
		}

		exporter, err = otlpmetricgrpc.New(context.Background(), opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP metrics exporter: %w", err)
		}

	case "stdout":
		// Stdout Exporter（用于调试）
		exporter, err = stdoutmetric.New()
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout metrics exporter: %w", err)
		}

	default:
		return nil, fmt.Errorf("unsupported metrics exporter type: %s", cfg.Exporter.Type)
	}

	// 创建 MeterProvider
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(
				exporter,
				sdkmetric.WithInterval(cfg.Metrics.ExportInterval),
				sdkmetric.WithTimeout(cfg.Metrics.ExportTimeout),
			),
		),
	)

	// 设置全局 MeterProvider
	otel.SetMeterProvider(mp)

	return &MetricsManager{
		meterProvider: mp,
		config:        cfg.Metrics,
		enabled:       true,
	}, nil
}

// Shutdown 关闭 Metrics
func (m *MetricsManager) Shutdown(ctx context.Context) error {
	if m.meterProvider != nil {
		return m.meterProvider.Shutdown(ctx)
	}
	return nil
}

// GetMeter 获取 Meter（供应用使用）
func (m *MetricsManager) GetMeter(name string) metric.Meter {
	return otel.Meter(name)
}

// IsEnabled 是否启用
func (m *MetricsManager) IsEnabled() bool {
	return m.enabled
}

// GetConfig 获取配置
func (m *MetricsManager) GetConfig() MetricsConfig {
	return m.config
}

// IsHTTPMetricsEnabled HTTP 指标是否启用
func (m *MetricsManager) IsHTTPMetricsEnabled() bool {
	return m.enabled && m.config.HTTP.Enabled
}

// IsDBMetricsEnabled 数据库指标是否启用
func (m *MetricsManager) IsDBMetricsEnabled() bool {
	return m.enabled && m.config.Database.Enabled
}

// IsGRPCMetricsEnabled gRPC 指标是否启用
func (m *MetricsManager) IsGRPCMetricsEnabled() bool {
	return m.enabled && m.config.GRPC.Enabled
}

