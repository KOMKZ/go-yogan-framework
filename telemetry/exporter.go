package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc/credentials/insecure"
)

// createExporter creates an Exporter (wrapped with circuit breaker)
func (m *Manager) createExporter(ctx context.Context) (trace.SpanExporter, error) {
	// Create main exporter
	primaryExporter, err := m.createRawExporter(ctx, m.config.Exporter.Type)
	if err != nil {
		return nil, fmt.Errorf("create primary exporter failed: %w", err)
	}

	// If the circuit breaker is not enabled, return the main outcome directly
	if !m.config.CircuitBreaker.Enabled {
		return primaryExporter, nil
	}

	// Create fallback exporter
	fallbackExporter, err := m.createRawExporter(ctx, m.config.CircuitBreaker.FallbackExporterType)
	if err != nil {
		m.logger.WarnCtx(ctx, "Failed to create fallback exporter, using noop",
			zap.Error(err),
			zap.String("fallback_type", m.config.CircuitBreaker.FallbackExporterType),
		)
		fallbackExporter = &noopExporter{}
	}

	// Wrap fuse functionality
	circuitBreaker := NewCircuitBreaker(
		m.config.CircuitBreaker,
		m.logger.GetZapLogger(),
		primaryExporter,
		fallbackExporter,
	)

	m.circuitBreaker = circuitBreaker

	m.logger.InfoCtx(ctx, "✅ Circuit breaker enabled for telemetry exporter",
		zap.Int("failure_threshold", m.config.CircuitBreaker.FailureThreshold),
		zap.Int("success_threshold", m.config.CircuitBreaker.SuccessThreshold),
		zap.Duration("timeout", m.config.CircuitBreaker.Timeout),
		zap.String("fallback_exporter", m.config.CircuitBreaker.FallbackExporterType),
	)

	return circuitBreaker, nil
}

// createRawExporter Creates the raw Exporter (without circuit breaker wrapping)
func (m *Manager) createRawExporter(ctx context.Context, exporterType string) (trace.SpanExporter, error) {
	switch exporterType {
	case "otlp":
		return m.createOTLPExporter(ctx)
	case "stdout":
		return m.createStdoutExporter()
	case "noop":
		return &noopExporter{}, nil
	default:
		return nil, fmt.Errorf("unsupported exporter type: %s", exporterType)
	}
}

// Create OTLP exporter
func (m *Manager) createOTLPExporter(ctx context.Context) (trace.SpanExporter, error) {
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(m.config.Exporter.Endpoint),
		otlptracegrpc.WithTimeout(m.config.Exporter.Timeout),
	}

	// If using an insecure connection
	if m.config.Exporter.Insecure {
		opts = append(opts, otlptracegrpc.WithTLSCredentials(insecure.NewCredentials()))
	}

	// addTarget custom Headers (for OpenObserve authentication etc.)
	if len(m.config.Exporter.Headers) > 0 {
		opts = append(opts, otlptracegrpc.WithHeaders(m.config.Exporter.Headers))
	}

	// Create gRPC client
	client := otlptracegrpc.NewClient(opts...)

	// Create OTLP Exporter
	return otlptrace.New(ctx, client)
}

// CreateStdoutExporter Create Stdout Exporter (for debugging)
func (m *Manager) createStdoutExporter() (trace.SpanExporter, error) {
	return stdouttrace.New(
		stdouttrace.WithPrettyPrint(), // format output
	)
}

// noopExporter dummy exporter (does nothing)
type noopExporter struct{}

func (n *noopExporter) ExportSpans(ctx context.Context, spans []trace.ReadOnlySpan) error {
	return nil
}

func (n *noopExporter) Shutdown(ctx context.Context) error {
	return nil
}

