package telemetry

import (
	"fmt"
	"sync"

	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

// MetricsRegistry is the centralized metrics registration center.
// It manages MetricsProvider registration and provides unified Meter access.
type MetricsRegistry struct {
	meterProvider metric.MeterProvider
	meters        map[string]metric.Meter
	providers     []component.MetricsProvider
	baseLabels    []attribute.KeyValue
	namespace     string
	enabled       bool
	logger        *logger.CtxZapLogger
	mu            sync.RWMutex
}

// MetricsRegistryOption configures the MetricsRegistry.
type MetricsRegistryOption func(*MetricsRegistry)

// WithNamespace sets the metrics namespace prefix.
func WithNamespace(namespace string) MetricsRegistryOption {
	return func(r *MetricsRegistry) {
		r.namespace = namespace
	}
}

// WithBaseLabels sets the global base labels.
func WithBaseLabels(labels []attribute.KeyValue) MetricsRegistryOption {
	return func(r *MetricsRegistry) {
		r.baseLabels = labels
	}
}

// WithLogger sets the logger for the registry.
func WithLogger(l *logger.CtxZapLogger) MetricsRegistryOption {
	return func(r *MetricsRegistry) {
		r.logger = l
	}
}

// NewMetricsRegistry creates a new MetricsRegistry.
// If meterProvider is nil, the global MeterProvider will be used.
func NewMetricsRegistry(mp metric.MeterProvider, opts ...MetricsRegistryOption) *MetricsRegistry {
	if mp == nil {
		mp = otel.GetMeterProvider()
	}

	r := &MetricsRegistry{
		meterProvider: mp,
		meters:        make(map[string]metric.Meter),
		providers:     make([]component.MetricsProvider, 0),
		baseLabels:    make([]attribute.KeyValue, 0),
		namespace:     "yogan",
		enabled:       true,
		logger:        logger.GetLogger("yogan"),
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Register registers a MetricsProvider with the registry.
// It creates a dedicated Meter for the provider and calls RegisterMetrics.
func (r *MetricsRegistry) Register(provider component.MetricsProvider) error {
	if provider == nil {
		return fmt.Errorf("metrics provider is nil")
	}

	if !r.enabled {
		return nil
	}

	if !provider.IsMetricsEnabled() {
		r.logger.Debug("metrics disabled for provider",
			zap.String("provider", provider.MetricsName()))
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	name := provider.MetricsName()
	if name == "" {
		return fmt.Errorf("metrics provider name is empty")
	}

	// Check for duplicate registration
	for _, p := range r.providers {
		if p.MetricsName() == name {
			return fmt.Errorf("metrics provider %q already registered", name)
		}
	}

	// Create or get the Meter for this provider
	meter := r.getMeterLocked(name)

	// Register the provider's metrics
	if err := provider.RegisterMetrics(meter); err != nil {
		return fmt.Errorf("register metrics for %q failed: %w", name, err)
	}

	r.providers = append(r.providers, provider)
	r.logger.Info("metrics provider registered", zap.String("provider", name))

	return nil
}

// GetMeter returns a Meter for the given component name.
// The meter name follows the pattern: {namespace}_{name}
func (r *MetricsRegistry) GetMeter(name string) metric.Meter {
	r.mu.RLock()
	if meter, ok := r.meters[name]; ok {
		r.mu.RUnlock()
		return meter
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.getMeterLocked(name)
}

// getMeterLocked creates or returns a Meter (must hold lock).
func (r *MetricsRegistry) getMeterLocked(name string) metric.Meter {
	if meter, ok := r.meters[name]; ok {
		return meter
	}

	// Create meter with namespace prefix
	meterName := name
	if r.namespace != "" {
		meterName = r.namespace + "_" + name
	}

	meter := r.meterProvider.Meter(meterName)
	r.meters[name] = meter
	return meter
}

// GetBaseLabels returns the global base labels.
func (r *MetricsRegistry) GetBaseLabels() []attribute.KeyValue {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]attribute.KeyValue{}, r.baseLabels...)
}

// IsEnabled returns whether metrics collection is enabled.
func (r *MetricsRegistry) IsEnabled() bool {
	return r.enabled
}

// SetEnabled enables or disables metrics collection.
func (r *MetricsRegistry) SetEnabled(enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enabled = enabled
}

// GetProviders returns all registered providers.
func (r *MetricsRegistry) GetProviders() []component.MetricsProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]component.MetricsProvider{}, r.providers...)
}

// GetProviderCount returns the number of registered providers.
func (r *MetricsRegistry) GetProviderCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.providers)
}

// Ensure MetricsRegistry implements MetricsCollector interface.
var _ component.MetricsCollector = (*MetricsRegistry)(nil)
