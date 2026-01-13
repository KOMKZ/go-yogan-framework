package limiter

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// OTelMetrics implements MetricsProvider for OpenTelemetry integration.
// This replaces the legacy atomic-based metrics with OTel instrumentation.
type OTelMetrics struct {
	config     MetricsConfig
	meter      metric.Meter
	registered bool
	mu         sync.RWMutex

	// Metrics instruments
	requestsTotal   metric.Int64Counter       // Total requests
	allowedTotal    metric.Int64Counter       // Allowed requests
	rejectedTotal   metric.Int64Counter       // Rejected requests
	currentTokens   metric.Int64ObservableGauge // Current token count
	rejectRate      metric.Float64ObservableGauge // Current reject rate
	
	// State tracking for gauges
	tokenCallbacks  map[string]func() int64
	tokenMu         sync.RWMutex
}

// MetricsConfig holds configuration for limiter metrics
type MetricsConfig struct {
	Enabled          bool
	RecordTokens     bool
	RecordRejectRate bool
}

// NewOTelMetrics creates a new OTel metrics provider for limiter
func NewOTelMetrics(cfg MetricsConfig) *OTelMetrics {
	return &OTelMetrics{
		config:         cfg,
		tokenCallbacks: make(map[string]func() int64),
	}
}

// MetricsName returns the metrics group name
func (m *OTelMetrics) MetricsName() string {
	return "limiter"
}

// IsMetricsEnabled returns whether metrics collection is enabled
func (m *OTelMetrics) IsMetricsEnabled() bool {
	return m.config.Enabled
}

// RegisterMetrics registers all limiter metrics with the provided Meter
func (m *OTelMetrics) RegisterMetrics(meter metric.Meter) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.registered {
		return nil
	}

	m.meter = meter
	var err error

	// Counter: total requests
	m.requestsTotal, err = meter.Int64Counter(
		"limiter_requests_total",
		metric.WithDescription("Total number of rate limit requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return err
	}

	// Counter: allowed requests
	m.allowedTotal, err = meter.Int64Counter(
		"limiter_allowed_total",
		metric.WithDescription("Total number of allowed requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return err
	}

	// Counter: rejected requests
	m.rejectedTotal, err = meter.Int64Counter(
		"limiter_rejected_total",
		metric.WithDescription("Total number of rejected requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return err
	}

	// Optional: current tokens gauge
	if m.config.RecordTokens {
		m.currentTokens, err = meter.Int64ObservableGauge(
			"limiter_current_tokens",
			metric.WithDescription("Current available tokens"),
			metric.WithUnit("{token}"),
			metric.WithInt64Callback(m.collectTokens),
		)
		if err != nil {
			return err
		}
	}

	m.registered = true
	return nil
}

// collectTokens is the callback for the observable gauge
func (m *OTelMetrics) collectTokens(_ context.Context, observer metric.Int64Observer) error {
	m.tokenMu.RLock()
	defer m.tokenMu.RUnlock()

	for resource, callback := range m.tokenCallbacks {
		tokens := callback()
		observer.Observe(tokens,
			metric.WithAttributes(attribute.String("resource", resource)),
		)
	}
	return nil
}

// RegisterTokenCallback registers a callback for a resource's token count
func (m *OTelMetrics) RegisterTokenCallback(resource string, callback func() int64) {
	m.tokenMu.Lock()
	defer m.tokenMu.Unlock()
	m.tokenCallbacks[resource] = callback
}

// UnregisterTokenCallback removes a resource's token callback
func (m *OTelMetrics) UnregisterTokenCallback(resource string) {
	m.tokenMu.Lock()
	defer m.tokenMu.Unlock()
	delete(m.tokenCallbacks, resource)
}

// RecordAllowed records an allowed request
func (m *OTelMetrics) RecordAllowed(ctx context.Context, resource, algorithm string) {
	if !m.registered {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("resource", resource),
		attribute.String("algorithm", algorithm),
	}

	m.requestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.allowedTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordRejected records a rejected request
func (m *OTelMetrics) RecordRejected(ctx context.Context, resource, algorithm, reason string) {
	if !m.registered {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("resource", resource),
		attribute.String("algorithm", algorithm),
		attribute.String("reason", reason),
	}

	m.requestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.rejectedTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// IsRegistered returns whether metrics have been registered
func (m *OTelMetrics) IsRegistered() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.registered
}
