package event

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// EventMetricsConfig holds configuration for Event metrics
type EventMetricsConfig struct {
	Enabled         bool
	RecordQueueSize bool
}

// EventMetrics implements component.MetricsProvider for Event instrumentation.
type EventMetrics struct {
	config     EventMetricsConfig
	meter      metric.Meter
	registered bool
	mu         sync.RWMutex

	// Metrics instruments
	eventsDispatched  metric.Int64Counter       // Events dispatched
	eventsHandled     metric.Int64Counter       // Events handled
	dispatchDuration  metric.Float64Histogram   // Dispatch duration
	queueSize         metric.Int64ObservableGauge // Queue size (optional)

	// Queue size callback
	queueSizeCallback func() int64
}

// NewEventMetrics creates a new Event metrics provider
func NewEventMetrics(cfg EventMetricsConfig) *EventMetrics {
	return &EventMetrics{
		config: cfg,
	}
}

// MetricsName returns the metrics group name
func (m *EventMetrics) MetricsName() string {
	return "event"
}

// IsMetricsEnabled returns whether metrics collection is enabled
func (m *EventMetrics) IsMetricsEnabled() bool {
	return m.config.Enabled
}

// RegisterMetrics registers all Event metrics with the provided Meter
func (m *EventMetrics) RegisterMetrics(meter metric.Meter) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.registered {
		return nil
	}

	m.meter = meter
	var err error

	// Counter: events dispatched
	m.eventsDispatched, err = meter.Int64Counter(
		"event_dispatched_total",
		metric.WithDescription("Total number of events dispatched"),
		metric.WithUnit("{event}"),
	)
	if err != nil {
		return err
	}

	// Counter: events handled
	m.eventsHandled, err = meter.Int64Counter(
		"event_handled_total",
		metric.WithDescription("Total number of events handled"),
		metric.WithUnit("{event}"),
	)
	if err != nil {
		return err
	}

	// Histogram: dispatch duration
	m.dispatchDuration, err = meter.Float64Histogram(
		"event_dispatch_duration_seconds",
		metric.WithDescription("Event dispatch duration distribution"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	// Optional: queue size
	if m.config.RecordQueueSize {
		m.queueSize, err = meter.Int64ObservableGauge(
			"event_queue_size",
			metric.WithDescription("Current event queue size"),
			metric.WithUnit("{event}"),
			metric.WithInt64Callback(m.collectQueueSize),
		)
		if err != nil {
			return err
		}
	}

	m.registered = true
	return nil
}

// collectQueueSize collects the current queue size
func (m *EventMetrics) collectQueueSize(_ context.Context, observer metric.Int64Observer) error {
	if m.queueSizeCallback != nil {
		observer.Observe(m.queueSizeCallback())
	}
	return nil
}

// SetQueueSizeCallback sets the queue size callback
func (m *EventMetrics) SetQueueSizeCallback(callback func() int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queueSizeCallback = callback
}

// RecordDispatched records an event dispatch
func (m *EventMetrics) RecordDispatched(ctx context.Context, topic string, duration time.Duration) {
	if !m.registered {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("topic", topic),
	}

	m.eventsDispatched.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.dispatchDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

// RecordHandled records an event being handled
func (m *EventMetrics) RecordHandled(ctx context.Context, topic, handler, result string) {
	if !m.registered {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("topic", topic),
		attribute.String("handler", handler),
		attribute.String("result", result),
	}

	m.eventsHandled.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// IsRegistered returns whether metrics have been registered
func (m *EventMetrics) IsRegistered() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.registered
}
