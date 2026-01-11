package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/sdk/trace"
)

// createTracerProvider 创建 TracerProvider
func (c *Component) createTracerProvider(ctx context.Context) (
	*trace.TracerProvider, func(context.Context) error, error) {

	// 1. 创建 Resource（服务信息）
	res, err := c.createResource(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("create resource failed: %w", err)
	}

	// 2. 创建 Exporter
	exporter, err := c.createExporter(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("create exporter failed: %w", err)
	}

	// 3. 创建 Sampler
	sampler := c.createSampler()

	// 4. 配置 TracerProvider 选项
	opts := []trace.TracerProviderOption{
		trace.WithResource(res),
		trace.WithSampler(sampler),
	}

	// 5. 配置 SpanProcessor（批处理或同步）
	if c.config.Batch.Enabled {
		// 批处理模式（推荐用于生产）
		batchOpts := []trace.BatchSpanProcessorOption{
			trace.WithMaxQueueSize(c.config.Batch.MaxQueueSize),
			trace.WithMaxExportBatchSize(c.config.Batch.MaxExportBatchSize),
			trace.WithBatchTimeout(c.config.Batch.ScheduleDelay),
			trace.WithExportTimeout(c.config.Batch.ExportTimeout),
		}
		opts = append(opts, trace.WithBatcher(exporter, batchOpts...))
	} else {
		// 同步模式（仅用于调试）
		opts = append(opts, trace.WithSyncer(exporter))
	}

	// 6. 配置 Span 限制
	if c.config.Span.MaxAttributes > 0 {
		opts = append(opts, trace.WithSpanLimits(trace.SpanLimits{
			AttributeCountLimit:        c.config.Span.MaxAttributes,
			EventCountLimit:            c.config.Span.MaxEvents,
			LinkCountLimit:             c.config.Span.MaxLinks,
			AttributeValueLengthLimit:  c.config.Span.MaxAttributeLength,
		}))
	}

	// 7. 创建 TracerProvider
	tp := trace.NewTracerProvider(opts...)

	// 8. 返回 shutdown 函数
	shutdownFn := func(ctx context.Context) error {
		if err := tp.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdown tracer provider failed: %w", err)
		}
		return nil
	}

	return tp, shutdownFn, nil
}

// createSampler 创建 Sampler
func (c *Component) createSampler() trace.Sampler {
	switch c.config.Sampler.Type {
	case "always_on":
		return trace.AlwaysSample()
	case "always_off":
		return trace.NeverSample()
	case "trace_id_ratio":
		return trace.TraceIDRatioBased(c.config.Sampler.Ratio)
	case "parent_based_always_on":
		return trace.ParentBased(trace.AlwaysSample())
	default:
		// 默认使用 parent_based_always_on
		return trace.ParentBased(trace.AlwaysSample())
	}
}

