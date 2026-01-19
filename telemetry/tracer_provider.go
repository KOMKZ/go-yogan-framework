package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/sdk/trace"
)

// createTracerProvider 创建 TracerProvider
func (m *Manager) createTracerProvider(ctx context.Context) (
	*trace.TracerProvider, func(context.Context) error, error) {

	// 1. 创建 Resource（服务信息）
	res, err := m.createResource(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("create resource failed: %w", err)
	}

	// 2. 创建 Exporter
	exporter, err := m.createExporter(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("create exporter failed: %w", err)
	}

	// 3. 创建 Sampler
	sampler := m.createSampler()

	// 4. 配置 TracerProvider 选项
	opts := []trace.TracerProviderOption{
		trace.WithResource(res),
		trace.WithSampler(sampler),
	}

	// 5. 配置 SpanProcessor（批处理或同步）
	if m.config.Batch.Enabled {
		// 批处理模式（推荐用于生产）
		batchOpts := []trace.BatchSpanProcessorOption{
			trace.WithMaxQueueSize(m.config.Batch.MaxQueueSize),
			trace.WithMaxExportBatchSize(m.config.Batch.MaxExportBatchSize),
			trace.WithBatchTimeout(m.config.Batch.ScheduleDelay),
			trace.WithExportTimeout(m.config.Batch.ExportTimeout),
		}
		opts = append(opts, trace.WithBatcher(exporter, batchOpts...))
	} else {
		// 同步模式（仅用于调试）
		opts = append(opts, trace.WithSyncer(exporter))
	}

	// 6. 配置 Span 限制
	if m.config.Span.MaxAttributes > 0 {
		opts = append(opts, trace.WithSpanLimits(trace.SpanLimits{
			AttributeCountLimit:        m.config.Span.MaxAttributes,
			EventCountLimit:            m.config.Span.MaxEvents,
			LinkCountLimit:             m.config.Span.MaxLinks,
			AttributeValueLengthLimit:  m.config.Span.MaxAttributeLength,
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
func (m *Manager) createSampler() trace.Sampler {
	switch m.config.Sampler.Type {
	case "always_on":
		return trace.AlwaysSample()
	case "always_off":
		return trace.NeverSample()
	case "trace_id_ratio":
		return trace.TraceIDRatioBased(m.config.Sampler.Ratio)
	case "parent_based_always_on":
		return trace.ParentBased(trace.AlwaysSample())
	default:
		// 默认使用 parent_based_always_on
		return trace.ParentBased(trace.AlwaysSample())
	}
}

