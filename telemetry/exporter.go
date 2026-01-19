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

// createExporter 创建 Exporter（带熔断器包装）
func (m *Manager) createExporter(ctx context.Context) (trace.SpanExporter, error) {
	// 创建主导出器
	primaryExporter, err := m.createRawExporter(ctx, m.config.Exporter.Type)
	if err != nil {
		return nil, fmt.Errorf("create primary exporter failed: %w", err)
	}

	// 如果熔断器未启用，直接返回主导出器
	if !m.config.CircuitBreaker.Enabled {
		return primaryExporter, nil
	}

	// 创建降级导出器
	fallbackExporter, err := m.createRawExporter(ctx, m.config.CircuitBreaker.FallbackExporterType)
	if err != nil {
		m.logger.WarnCtx(ctx, "Failed to create fallback exporter, using noop",
			zap.Error(err),
			zap.String("fallback_type", m.config.CircuitBreaker.FallbackExporterType),
		)
		fallbackExporter = &noopExporter{}
	}

	// 包装熔断器
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

// createRawExporter 创建原始 Exporter（不包装熔断器）
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

// createOTLPExporter 创建 OTLP Exporter
func (m *Manager) createOTLPExporter(ctx context.Context) (trace.SpanExporter, error) {
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(m.config.Exporter.Endpoint),
		otlptracegrpc.WithTimeout(m.config.Exporter.Timeout),
	}

	// 如果使用不安全连接
	if m.config.Exporter.Insecure {
		opts = append(opts, otlptracegrpc.WithTLSCredentials(insecure.NewCredentials()))
	}

	// 🎯 添加自定义 Headers（用于 OpenObserve 认证等）
	if len(m.config.Exporter.Headers) > 0 {
		opts = append(opts, otlptracegrpc.WithHeaders(m.config.Exporter.Headers))
	}

	// 创建 gRPC 客户端
	client := otlptracegrpc.NewClient(opts...)

	// 创建 OTLP Exporter
	return otlptrace.New(ctx, client)
}

// createStdoutExporter 创建 Stdout Exporter（调试用）
func (m *Manager) createStdoutExporter() (trace.SpanExporter, error) {
	return stdouttrace.New(
		stdouttrace.WithPrettyPrint(), // 格式化输出
	)
}

// noopExporter 空导出器（什么都不做）
type noopExporter struct{}

func (n *noopExporter) ExportSpans(ctx context.Context, spans []trace.ReadOnlySpan) error {
	return nil
}

func (n *noopExporter) Shutdown(ctx context.Context) error {
	return nil
}

