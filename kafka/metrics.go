package kafka

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// KafkaMetricsConfig holds configuration for Kafka metrics
type KafkaMetricsConfig struct {
	Enabled         bool
	RecordLag       bool
	RecordBatchSize bool
}

// KafkaMetrics implements component.MetricsProvider for Kafka instrumentation.
type KafkaMetrics struct {
	config     KafkaMetricsConfig
	meter      metric.Meter
	registered bool
	mu         sync.RWMutex

	// Producer metrics
	messagesProduced  metric.Int64Counter     // Messages produced
	produceDuration   metric.Float64Histogram // Produce duration
	produceErrors     metric.Int64Counter     // Produce errors

	// Consumer metrics
	messagesConsumed  metric.Int64Counter       // Messages consumed
	consumeDuration   metric.Float64Histogram   // Consume processing duration
	consumerLag       metric.Int64ObservableGauge // Consumer lag (optional)
	consumeErrors     metric.Int64Counter       // Consume errors

	// Lag callbacks
	lagCallbacks map[string]func() int64
	lagMu        sync.RWMutex
}

// NewKafkaMetrics creates a new Kafka metrics provider
func NewKafkaMetrics(cfg KafkaMetricsConfig) *KafkaMetrics {
	return &KafkaMetrics{
		config:       cfg,
		lagCallbacks: make(map[string]func() int64),
	}
}

// MetricsName returns the metrics group name
func (m *KafkaMetrics) MetricsName() string {
	return "kafka"
}

// IsMetricsEnabled returns whether metrics collection is enabled
func (m *KafkaMetrics) IsMetricsEnabled() bool {
	return m.config.Enabled
}

// RegisterMetrics registers all Kafka metrics with the provided Meter
func (m *KafkaMetrics) RegisterMetrics(meter metric.Meter) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.registered {
		return nil
	}

	m.meter = meter
	var err error

	// Producer: messages produced
	m.messagesProduced, err = meter.Int64Counter(
		"kafka_messages_produced_total",
		metric.WithDescription("Total number of messages produced"),
		metric.WithUnit("{message}"),
	)
	if err != nil {
		return err
	}

	// Producer: duration
	m.produceDuration, err = meter.Float64Histogram(
		"kafka_produce_duration_seconds",
		metric.WithDescription("Kafka produce duration distribution"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	// Producer: errors
	m.produceErrors, err = meter.Int64Counter(
		"kafka_produce_errors_total",
		metric.WithDescription("Total number of produce errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return err
	}

	// Consumer: messages consumed
	m.messagesConsumed, err = meter.Int64Counter(
		"kafka_messages_consumed_total",
		metric.WithDescription("Total number of messages consumed"),
		metric.WithUnit("{message}"),
	)
	if err != nil {
		return err
	}

	// Consumer: duration
	m.consumeDuration, err = meter.Float64Histogram(
		"kafka_consume_duration_seconds",
		metric.WithDescription("Kafka consume processing duration distribution"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	// Consumer: errors
	m.consumeErrors, err = meter.Int64Counter(
		"kafka_consume_errors_total",
		metric.WithDescription("Total number of consume errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return err
	}

	// Optional: consumer lag
	if m.config.RecordLag {
		m.consumerLag, err = meter.Int64ObservableGauge(
			"kafka_consumer_lag",
			metric.WithDescription("Consumer lag (messages behind)"),
			metric.WithUnit("{message}"),
			metric.WithInt64Callback(m.collectLag),
		)
		if err != nil {
			return err
		}
	}

	m.registered = true
	return nil
}

// collectLag collects consumer lag
func (m *KafkaMetrics) collectLag(_ context.Context, observer metric.Int64Observer) error {
	m.lagMu.RLock()
	defer m.lagMu.RUnlock()

	for key, callback := range m.lagCallbacks {
		lag := callback()
		observer.Observe(lag, metric.WithAttributes(attribute.String("consumer_group", key)))
	}
	return nil
}

// RegisterLagCallback registers a lag callback for a consumer group
func (m *KafkaMetrics) RegisterLagCallback(group string, callback func() int64) {
	m.lagMu.Lock()
	defer m.lagMu.Unlock()
	m.lagCallbacks[group] = callback
}

// UnregisterLagCallback removes a lag callback
func (m *KafkaMetrics) UnregisterLagCallback(group string) {
	m.lagMu.Lock()
	defer m.lagMu.Unlock()
	delete(m.lagCallbacks, group)
}

// RecordProduce records a message production
func (m *KafkaMetrics) RecordProduce(ctx context.Context, topic string, partition int32, duration time.Duration, err error) {
	if !m.registered {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("topic", topic),
		attribute.Int("partition", int(partition)),
	}

	m.messagesProduced.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.produceDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))

	if err != nil {
		m.produceErrors.Add(ctx, 1, metric.WithAttributes(
			attribute.String("topic", topic),
			attribute.String("error_type", "produce_error"),
		))
	}
}

// RecordConsume records a message consumption
func (m *KafkaMetrics) RecordConsume(ctx context.Context, topic, group string, partition int32, duration time.Duration, err error) {
	if !m.registered {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("topic", topic),
		attribute.String("group", group),
		attribute.Int("partition", int(partition)),
	}

	m.messagesConsumed.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.consumeDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))

	if err != nil {
		m.consumeErrors.Add(ctx, 1, metric.WithAttributes(
			attribute.String("topic", topic),
			attribute.String("group", group),
			attribute.String("error_type", "consume_error"),
		))
	}
}

// IsRegistered returns whether metrics have been registered
func (m *KafkaMetrics) IsRegistered() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.registered
}
