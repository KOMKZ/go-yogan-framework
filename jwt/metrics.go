package jwt

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// JWTMetricsConfig holds configuration for JWT metrics
type JWTMetricsConfig struct {
	Enabled bool
}

// JWTMetrics implements component.MetricsProvider for JWT instrumentation.
type JWTMetrics struct {
	config     JWTMetricsConfig
	meter      metric.Meter
	registered bool
	mu         sync.RWMutex

	// Metrics instruments
	tokensGenerated     metric.Int64Counter     // Tokens generated
	tokensVerified      metric.Int64Counter     // Tokens verified
	tokensRefreshed     metric.Int64Counter     // Tokens refreshed
	tokensRevoked       metric.Int64Counter     // Tokens revoked
	verificationDuration metric.Float64Histogram // Verification duration
}

// NewJWTMetrics creates a new JWT metrics provider
func NewJWTMetrics(cfg JWTMetricsConfig) *JWTMetrics {
	return &JWTMetrics{
		config: cfg,
	}
}

// MetricsName returns the metrics group name
func (m *JWTMetrics) MetricsName() string {
	return "jwt"
}

// IsMetricsEnabled returns whether metrics collection is enabled
func (m *JWTMetrics) IsMetricsEnabled() bool {
	return m.config.Enabled
}

// RegisterMetrics registers all JWT metrics with the provided Meter
func (m *JWTMetrics) RegisterMetrics(meter metric.Meter) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.registered {
		return nil
	}

	m.meter = meter
	var err error

	// Counter: tokens generated
	m.tokensGenerated, err = meter.Int64Counter(
		"jwt_tokens_generated_total",
		metric.WithDescription("Total number of JWT tokens generated"),
		metric.WithUnit("{token}"),
	)
	if err != nil {
		return err
	}

	// Counter: tokens verified
	m.tokensVerified, err = meter.Int64Counter(
		"jwt_tokens_verified_total",
		metric.WithDescription("Total number of JWT tokens verified"),
		metric.WithUnit("{token}"),
	)
	if err != nil {
		return err
	}

	// Counter: tokens refreshed
	m.tokensRefreshed, err = meter.Int64Counter(
		"jwt_tokens_refreshed_total",
		metric.WithDescription("Total number of JWT tokens refreshed"),
		metric.WithUnit("{token}"),
	)
	if err != nil {
		return err
	}

	// Counter: tokens revoked
	m.tokensRevoked, err = meter.Int64Counter(
		"jwt_tokens_revoked_total",
		metric.WithDescription("Total number of JWT tokens revoked"),
		metric.WithUnit("{token}"),
	)
	if err != nil {
		return err
	}

	// Histogram: verification duration
	m.verificationDuration, err = meter.Float64Histogram(
		"jwt_verification_duration_seconds",
		metric.WithDescription("JWT verification duration distribution"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	m.registered = true
	return nil
}

// RecordGenerated records a token generation
func (m *JWTMetrics) RecordGenerated(ctx context.Context, tokenType string) {
	if !m.registered {
		return
	}
	m.tokensGenerated.Add(ctx, 1, metric.WithAttributes(
		attribute.String("type", tokenType),
	))
}

// RecordVerified records a token verification
func (m *JWTMetrics) RecordVerified(ctx context.Context, result string, duration time.Duration) {
	if !m.registered {
		return
	}
	m.tokensVerified.Add(ctx, 1, metric.WithAttributes(
		attribute.String("result", result),
	))
	m.verificationDuration.Record(ctx, duration.Seconds())
}

// RecordRefreshed records a token refresh
func (m *JWTMetrics) RecordRefreshed(ctx context.Context, result string) {
	if !m.registered {
		return
	}
	m.tokensRefreshed.Add(ctx, 1, metric.WithAttributes(
		attribute.String("result", result),
	))
}

// RecordRevoked records a token revocation
func (m *JWTMetrics) RecordRevoked(ctx context.Context) {
	if !m.registered {
		return
	}
	m.tokensRevoked.Add(ctx, 1)
}

// IsRegistered returns whether metrics have been registered
func (m *JWTMetrics) IsRegistered() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.registered
}
