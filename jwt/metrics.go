package jwt

import (
	"context"
	"sync"
	"time"

	"github.com/KOMKZ/go-yogan-framework/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// JWTMetricsConfig holds configuration for JWT metrics
type JWTMetricsConfig struct {
	Enabled bool
}

// JWTMetrics implements component.MetricsProvider for JWT instrumentation.
// 使用 telemetry.TokenMetrics 模板减少样板代码
type JWTMetrics struct {
	config     JWTMetricsConfig
	meter      metric.Meter
	registered bool
	mu         sync.RWMutex

	// 使用预定义模板
	tokens *telemetry.TokenMetrics
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
// 使用 MetricsBuilder 模板，代码量从 50+ 行减少到 10 行
func (m *JWTMetrics) RegisterMetrics(meter metric.Meter) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.registered {
		return nil
	}

	m.meter = meter

	// 使用 MetricsBuilder 的 TokenMetrics 模板，一行创建 5 个指标
	builder := telemetry.NewMetricsBuilder(meter, "")
	tokens, err := builder.NewTokenMetrics("jwt")
	if err != nil {
		return err
	}
	m.tokens = tokens

	m.registered = true
	return nil
}

// RecordGenerated records a token generation
func (m *JWTMetrics) RecordGenerated(ctx context.Context, tokenType string) {
	if !m.registered || m.tokens == nil {
		return
	}
	m.tokens.RecordGenerated(ctx, attribute.String("type", tokenType))
}

// RecordVerified records a token verification
func (m *JWTMetrics) RecordVerified(ctx context.Context, result string, duration time.Duration) {
	if !m.registered || m.tokens == nil {
		return
	}
	m.tokens.RecordVerified(ctx, duration.Seconds(), result == "success",
		attribute.String("result", result))
}

// RecordRefreshed records a token refresh
func (m *JWTMetrics) RecordRefreshed(ctx context.Context, result string) {
	if !m.registered || m.tokens == nil {
		return
	}
	m.tokens.RecordRefreshed(ctx, attribute.String("result", result))
}

// RecordRevoked records a token revocation
func (m *JWTMetrics) RecordRevoked(ctx context.Context) {
	if !m.registered || m.tokens == nil {
		return
	}
	m.tokens.RecordRevoked(ctx)
}

// IsRegistered returns whether metrics have been registered
func (m *JWTMetrics) IsRegistered() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.registered
}
