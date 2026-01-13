package breaker

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// OTelBreakerMetrics implements component.MetricsProvider for OpenTelemetry integration.
type OTelBreakerMetrics struct {
	config     BreakerMetricsConfig
	meter      metric.Meter
	registered bool
	mu         sync.RWMutex

	// Metrics instruments
	requestsTotal   metric.Int64Counter       // Total requests
	successesTotal  metric.Int64Counter       // Successful requests
	failuresTotal   metric.Int64Counter       // Failed requests
	rejectionsTotal metric.Int64Counter       // Rejected requests
	latency         metric.Float64Histogram   // Request latency
	stateGauge      metric.Int64ObservableGauge // Current state (0=closed, 1=open, 2=half-open)

	// State tracking for gauge
	stateCallbacks map[string]func() int64
	stateMu        sync.RWMutex
}

// BreakerMetricsConfig holds configuration for breaker metrics
type BreakerMetricsConfig struct {
	Enabled           bool
	RecordState       bool
	RecordSuccessRate bool
}

// NewOTelBreakerMetrics creates a new OTel metrics provider for breaker
func NewOTelBreakerMetrics(cfg BreakerMetricsConfig) *OTelBreakerMetrics {
	return &OTelBreakerMetrics{
		config:         cfg,
		stateCallbacks: make(map[string]func() int64),
	}
}

// MetricsName returns the metrics group name
func (m *OTelBreakerMetrics) MetricsName() string {
	return "breaker"
}

// IsMetricsEnabled returns whether metrics collection is enabled
func (m *OTelBreakerMetrics) IsMetricsEnabled() bool {
	return m.config.Enabled
}

// RegisterMetrics registers all breaker metrics with the provided Meter
func (m *OTelBreakerMetrics) RegisterMetrics(meter metric.Meter) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.registered {
		return nil
	}

	m.meter = meter
	var err error

	// Counter: total requests
	m.requestsTotal, err = meter.Int64Counter(
		"breaker_requests_total",
		metric.WithDescription("Total number of circuit breaker requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return err
	}

	// Counter: successes
	m.successesTotal, err = meter.Int64Counter(
		"breaker_successes_total",
		metric.WithDescription("Total number of successful requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return err
	}

	// Counter: failures
	m.failuresTotal, err = meter.Int64Counter(
		"breaker_failures_total",
		metric.WithDescription("Total number of failed requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return err
	}

	// Counter: rejections
	m.rejectionsTotal, err = meter.Int64Counter(
		"breaker_rejections_total",
		metric.WithDescription("Total number of rejected requests (circuit open)"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return err
	}

	// Histogram: latency
	m.latency, err = meter.Float64Histogram(
		"breaker_latency_seconds",
		metric.WithDescription("Request latency distribution"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	// Optional: state gauge
	if m.config.RecordState {
		m.stateGauge, err = meter.Int64ObservableGauge(
			"breaker_state",
			metric.WithDescription("Current circuit breaker state (0=closed, 1=open, 2=half-open)"),
			metric.WithInt64Callback(m.collectState),
		)
		if err != nil {
			return err
		}
	}

	m.registered = true
	return nil
}

// collectState is the callback for the observable gauge
func (m *OTelBreakerMetrics) collectState(_ context.Context, observer metric.Int64Observer) error {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()

	for resource, callback := range m.stateCallbacks {
		state := callback()
		observer.Observe(state,
			metric.WithAttributes(attribute.String("resource", resource)),
		)
	}
	return nil
}

// RegisterStateCallback registers a callback for a resource's state
func (m *OTelBreakerMetrics) RegisterStateCallback(resource string, callback func() int64) {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	m.stateCallbacks[resource] = callback
}

// UnregisterStateCallback removes a resource's state callback
func (m *OTelBreakerMetrics) UnregisterStateCallback(resource string) {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	delete(m.stateCallbacks, resource)
}

// RecordSuccess records a successful request
func (m *OTelBreakerMetrics) RecordSuccess(ctx context.Context, resource string, duration time.Duration) {
	if !m.registered {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("resource", resource),
		attribute.String("result", "success"),
	}

	m.requestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.successesTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("resource", resource)))
	m.latency.Record(ctx, duration.Seconds(), metric.WithAttributes(attribute.String("resource", resource)))
}

// RecordFailure records a failed request
func (m *OTelBreakerMetrics) RecordFailure(ctx context.Context, resource string, duration time.Duration, errorType string) {
	if !m.registered {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("resource", resource),
		attribute.String("result", "failure"),
	}

	m.requestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.failuresTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.String("resource", resource),
		attribute.String("error_type", errorType),
	))
	m.latency.Record(ctx, duration.Seconds(), metric.WithAttributes(attribute.String("resource", resource)))
}

// RecordRejection records a rejected request
func (m *OTelBreakerMetrics) RecordRejection(ctx context.Context, resource string) {
	if !m.registered {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("resource", resource),
		attribute.String("result", "rejected"),
	}

	m.requestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.rejectionsTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("resource", resource)))
}

// IsRegistered returns whether metrics have been registered
func (m *OTelBreakerMetrics) IsRegistered() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.registered
}
