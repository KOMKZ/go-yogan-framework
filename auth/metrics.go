package auth

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// AuthMetricsConfig holds configuration for Auth metrics
type AuthMetricsConfig struct {
	Enabled bool
}

// AuthMetrics implements component.MetricsProvider for Auth instrumentation.
type AuthMetrics struct {
	config     AuthMetricsConfig
	meter      metric.Meter
	registered bool
	mu         sync.RWMutex

	// Metrics instruments
	loginsTotal          metric.Int64Counter     // Login attempts
	loginDuration        metric.Float64Histogram // Login duration
	passwordValidations  metric.Int64Counter     // Password validations
	failedAttempts       metric.Int64Counter     // Failed login attempts
}

// NewAuthMetrics creates a new Auth metrics provider
func NewAuthMetrics(cfg AuthMetricsConfig) *AuthMetrics {
	return &AuthMetrics{
		config: cfg,
	}
}

// MetricsName returns the metrics group name
func (m *AuthMetrics) MetricsName() string {
	return "auth"
}

// IsMetricsEnabled returns whether metrics collection is enabled
func (m *AuthMetrics) IsMetricsEnabled() bool {
	return m.config.Enabled
}

// RegisterMetrics registers all Auth metrics with the provided Meter
func (m *AuthMetrics) RegisterMetrics(meter metric.Meter) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.registered {
		return nil
	}

	m.meter = meter
	var err error

	// Counter: login attempts
	m.loginsTotal, err = meter.Int64Counter(
		"auth_logins_total",
		metric.WithDescription("Total number of login attempts"),
		metric.WithUnit("{login}"),
	)
	if err != nil {
		return err
	}

	// Histogram: login duration
	m.loginDuration, err = meter.Float64Histogram(
		"auth_login_duration_seconds",
		metric.WithDescription("Login duration distribution"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	// Counter: password validations
	m.passwordValidations, err = meter.Int64Counter(
		"auth_password_validations_total",
		metric.WithDescription("Total number of password validations"),
		metric.WithUnit("{validation}"),
	)
	if err != nil {
		return err
	}

	// Counter: failed attempts
	m.failedAttempts, err = meter.Int64Counter(
		"auth_failed_attempts_total",
		metric.WithDescription("Total number of failed authentication attempts"),
		metric.WithUnit("{attempt}"),
	)
	if err != nil {
		return err
	}

	m.registered = true
	return nil
}

// RecordLogin records a login attempt
func (m *AuthMetrics) RecordLogin(ctx context.Context, provider, result string, duration time.Duration) {
	if !m.registered {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("provider", provider),
		attribute.String("result", result),
	}

	m.loginsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.loginDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(
		attribute.String("provider", provider),
	))
}

// RecordPasswordValidation records a password validation
func (m *AuthMetrics) RecordPasswordValidation(ctx context.Context, result string) {
	if !m.registered {
		return
	}
	m.passwordValidations.Add(ctx, 1, metric.WithAttributes(
		attribute.String("result", result),
	))
}

// RecordFailedAttempt records a failed authentication attempt
func (m *AuthMetrics) RecordFailedAttempt(ctx context.Context, reason string) {
	if !m.registered {
		return
	}
	m.failedAttempts.Add(ctx, 1, metric.WithAttributes(
		attribute.String("reason", reason),
	))
}

// IsRegistered returns whether metrics have been registered
func (m *AuthMetrics) IsRegistered() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.registered
}
