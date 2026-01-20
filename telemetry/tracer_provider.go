package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/sdk/trace"
)

// createTracerProvider Creates a TracerProvider
func (m *Manager) createTracerProvider(ctx context.Context) (
	*trace.TracerProvider, func(context.Context) error, error) {

	// 1. Create Resource (service information)
	res, err := m.createResource(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("create resource failed: %w", err)
	}

	// 2. Create Exporter
	exporter, err := m.createExporter(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("create exporter failed: %w", err)
	}

	// 3. Create Sampler
	sampler := m.createSampler()

	// 4. Configure TracerProvider options
	opts := []trace.TracerProviderOption{
		trace.WithResource(res),
		trace.WithSampler(sampler),
	}

	// 5. Configure SpanProcessor (batch or synchronous)
	if m.config.Batch.Enabled {
		// Batch processing mode (recommended for production)
		batchOpts := []trace.BatchSpanProcessorOption{
			trace.WithMaxQueueSize(m.config.Batch.MaxQueueSize),
			trace.WithMaxExportBatchSize(m.config.Batch.MaxExportBatchSize),
			trace.WithBatchTimeout(m.config.Batch.ScheduleDelay),
			trace.WithExportTimeout(m.config.Batch.ExportTimeout),
		}
		opts = append(opts, trace.WithBatcher(exporter, batchOpts...))
	} else {
		// Synchronous mode (for debugging only)
		opts = append(opts, trace.WithSyncer(exporter))
	}

	// Configure span limit
	if m.config.Span.MaxAttributes > 0 {
		opts = append(opts, trace.WithSpanLimits(trace.SpanLimits{
			AttributeCountLimit:        m.config.Span.MaxAttributes,
			EventCountLimit:            m.config.Span.MaxEvents,
			LinkCountLimit:             m.config.Span.MaxLinks,
			AttributeValueLengthLimit:  m.config.Span.MaxAttributeLength,
		}))
	}

	// 7. Create TracerProvider
	tp := trace.NewTracerProvider(opts...)

	// 8. Return the shutdown function
	shutdownFn := func(ctx context.Context) error {
		if err := tp.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdown tracer provider failed: %w", err)
		}
		return nil
	}

	return tp, shutdownFn, nil
}

// createSampler Create Sampler
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
		// Use parent_based_always_on by default
		return trace.ParentBased(trace.AlwaysSample())
	}
}

