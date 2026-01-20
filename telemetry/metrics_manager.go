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

// Metrics Manager
type MetricsManager struct {
	meterProvider *sdkmetric.MeterProvider
	config        MetricsConfig
	enabled       bool
}

// Create Metrics Manager
func NewMetricsManager(cfg Config, res *resource.Resource) (*MetricsManager, error) {
	if !cfg.Enabled || !cfg.Metrics.Enabled {
		return &MetricsManager{
			enabled: false,
			config:  cfg.Metrics,
		}, nil
	}

	// Create Exporter
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

		// Add custom headers (for authentication)
		if len(cfg.Exporter.Headers) > 0 {
			opts = append(opts, otlpmetricgrpc.WithHeaders(cfg.Exporter.Headers))
		}

		exporter, err = otlpmetricgrpc.New(context.Background(), opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP metrics exporter: %w", err)
		}

	case "stdout":
		// Stdout Exporter (for debugging)
		exporter, err = stdoutmetric.New()
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout metrics exporter: %w", err)
		}

	default:
		return nil, fmt.Errorf("unsupported metrics exporter type: %s", cfg.Exporter.Type)
	}

	// Create MeterProvider
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

	// Set global MeterProvider
	otel.SetMeterProvider(mp)

	return &MetricsManager{
		meterProvider: mp,
		config:        cfg.Metrics,
		enabled:       true,
	}, nil
}

// Shut down metrics
func (m *MetricsManager) Shutdown(ctx context.Context) error {
	if m.meterProvider != nil {
		return m.meterProvider.Shutdown(ctx)
	}
	return nil
}

// GetMeter retrieve Meter (for application use)
func (m *MetricsManager) GetMeter(name string) metric.Meter {
	return otel.Meter(name)
}

// IsEnabled whether enabled
func (m *MetricsManager) IsEnabled() bool {
	return m.enabled
}

// GetConfig Retrieve configuration
func (m *MetricsManager) GetConfig() MetricsConfig {
	return m.config
}

// IsHTTPMetricsEnabled Whether HTTP metrics are enabled
func (m *MetricsManager) IsHTTPMetricsEnabled() bool {
	return m.enabled && m.config.HTTP.Enabled
}

// IsDBMetricsEnabled Database metrics enabled flag
func (m *MetricsManager) IsDBMetricsEnabled() bool {
	return m.enabled && m.config.Database.Enabled
}

// Is GRPC Metrics Enabled
func (m *MetricsManager) IsGRPCMetricsEnabled() bool {
	return m.enabled && m.config.GRPC.Enabled
}

