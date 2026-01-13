// Package component provides component interface definitions
package component

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// MetricsProvider defines the interface for components that provide metrics.
// Components can optionally implement this interface to register their metrics
// with the centralized MetricsRegistry.
//
// Example implementation:
//
//	func (c *Component) MetricsName() string {
//	    return "redis"
//	}
//
//	func (c *Component) RegisterMetrics(meter metric.Meter) error {
//	    counter, err := meter.Int64Counter("redis_commands_total")
//	    if err != nil {
//	        return err
//	    }
//	    c.commandsCounter = counter
//	    return nil
//	}
//
//	func (c *Component) IsMetricsEnabled() bool {
//	    return c.config.Metrics.Enabled
//	}
type MetricsProvider interface {
	// MetricsName returns the metrics group name (used for Meter naming).
	// Should be a short, lowercase identifier like "redis", "kafka", "jwt".
	MetricsName() string

	// RegisterMetrics registers all metrics for this component.
	// Called by MetricsRegistry after component Init.
	// The meter is pre-configured with the component's namespace.
	RegisterMetrics(meter metric.Meter) error

	// IsMetricsEnabled returns whether metrics collection is enabled for this component.
	IsMetricsEnabled() bool
}

// MetricsCollector defines the interface for the centralized metrics registry.
// This interface is implemented by telemetry.MetricsRegistry.
type MetricsCollector interface {
	// Register registers a MetricsProvider with the registry.
	// The provider's RegisterMetrics will be called with a pre-configured Meter.
	Register(provider MetricsProvider) error

	// GetMeter returns a Meter for the given component name.
	// The meter is pre-configured with base labels.
	GetMeter(name string) metric.Meter

	// GetBaseLabels returns the global base labels (service_name, env, etc.).
	GetBaseLabels() []attribute.KeyValue

	// IsEnabled returns whether metrics collection is globally enabled.
	IsEnabled() bool
}
