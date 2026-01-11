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
func (c *Component) createExporter(ctx context.Context) (trace.SpanExporter, error) {
	// 创建主导出器
	primaryExporter, err := c.createRawExporter(ctx, c.config.Exporter.Type)
	if err != nil {
		return nil, fmt.Errorf("create primary exporter failed: %w", err)
	}

	// 如果熔断器未启用，直接返回主导出器
	if !c.config.CircuitBreaker.Enabled {
		return primaryExporter, nil
	}

	// 创建降级导出器
	fallbackExporter, err := c.createRawExporter(ctx, c.config.CircuitBreaker.FallbackExporterType)
	if err != nil {
		c.logger.WarnCtx(ctx, "Failed to create fallback exporter, using noop",
			zap.Error(err),
			zap.String("fallback_type", c.config.CircuitBreaker.FallbackExporterType),
		)
		fallbackExporter = &noopExporter{}
	}

	// 包装熔断器
	circuitBreaker := NewCircuitBreaker(
		c.config.CircuitBreaker,
		c.logger.GetZapLogger(),
		primaryExporter,
		fallbackExporter,
	)

	c.circuitBreaker = circuitBreaker

	c.logger.InfoCtx(ctx, "✅ Circuit breaker enabled for telemetry exporter",
		zap.Int("failure_threshold", c.config.CircuitBreaker.FailureThreshold),
		zap.Int("success_threshold", c.config.CircuitBreaker.SuccessThreshold),
		zap.Duration("timeout", c.config.CircuitBreaker.Timeout),
		zap.String("fallback_exporter", c.config.CircuitBreaker.FallbackExporterType),
	)

	return circuitBreaker, nil
}

// createRawExporter 创建原始 Exporter（不包装熔断器）
func (c *Component) createRawExporter(ctx context.Context, exporterType string) (trace.SpanExporter, error) {
	switch exporterType {
	case "otlp":
		return c.createOTLPExporter(ctx)
	case "stdout":
		return c.createStdoutExporter()
	case "noop":
		return &noopExporter{}, nil
	default:
		return nil, fmt.Errorf("unsupported exporter type: %s", exporterType)
	}
}

// createOTLPExporter 创建 OTLP Exporter
func (c *Component) createOTLPExporter(ctx context.Context) (trace.SpanExporter, error) {
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(c.config.Exporter.Endpoint),
		otlptracegrpc.WithTimeout(c.config.Exporter.Timeout),
	}

	// 如果使用不安全连接
	if c.config.Exporter.Insecure {
		opts = append(opts, otlptracegrpc.WithTLSCredentials(insecure.NewCredentials()))
	}

	// 🎯 添加自定义 Headers（用于 OpenObserve 认证等）
	if len(c.config.Exporter.Headers) > 0 {
		opts = append(opts, otlptracegrpc.WithHeaders(c.config.Exporter.Headers))
	}

	// 创建 gRPC 客户端
	client := otlptracegrpc.NewClient(opts...)

	// 创建 OTLP Exporter
	return otlptrace.New(ctx, client)
}

// createStdoutExporter 创建 Stdout Exporter（调试用）
func (c *Component) createStdoutExporter() (trace.SpanExporter, error) {
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

