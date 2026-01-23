package auth

import (
	"context"
	"sync"
	"time"

	"github.com/KOMKZ/go-yogan-framework/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// AuthMetricsConfig holds configuration for Auth metrics
type AuthMetricsConfig struct {
	Enabled bool
}

// AuthMetrics implements component.MetricsProvider for Auth instrumentation.
// 使用 telemetry.RequestMetrics 模板减少样板代码
type AuthMetrics struct {
	config     AuthMetricsConfig
	meter      metric.Meter
	registered bool
	mu         sync.RWMutex

	// 使用预定义模板
	login *telemetry.RequestMetrics // 登录指标

	// 额外指标
	passwordValidations metric.Int64Counter
	failedAttempts      metric.Int64Counter
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
// 使用 MetricsBuilder 模板，代码量从 50+ 行减少到 20 行
func (m *AuthMetrics) RegisterMetrics(meter metric.Meter) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.registered {
		return nil
	}

	m.meter = meter
	builder := telemetry.NewMetricsBuilder(meter, "auth")

	// 使用 RequestMetrics 模板创建登录指标（total, duration, errors）
	login, err := builder.NewRequestMetrics("login")
	if err != nil {
		return err
	}
	m.login = login

	// 额外指标：密码验证
	m.passwordValidations, err = builder.Counter("password_validations_total", "Total number of password validations")
	if err != nil {
		return err
	}

	// 额外指标：失败尝试
	m.failedAttempts, err = builder.Counter("failed_attempts_total", "Total number of failed authentication attempts")
	if err != nil {
		return err
	}

	m.registered = true
	return nil
}

// RecordLogin records a login attempt
func (m *AuthMetrics) RecordLogin(ctx context.Context, provider, result string, duration time.Duration) {
	if !m.registered || m.login == nil {
		return
	}

	var err error
	if result != "success" {
		err = &loginError{result: result}
	}

	m.login.Record(ctx, duration.Seconds(), err,
		attribute.String("provider", provider),
		attribute.String("result", result),
	)
}

// loginError 用于标识登录失败
type loginError struct {
	result string
}

func (e *loginError) Error() string { return e.result }

// RecordPasswordValidation records a password validation
func (m *AuthMetrics) RecordPasswordValidation(ctx context.Context, result string) {
	if !m.registered || m.passwordValidations == nil {
		return
	}
	m.passwordValidations.Add(ctx, 1, metric.WithAttributes(
		attribute.String("result", result),
	))
}

// RecordFailedAttempt records a failed authentication attempt
func (m *AuthMetrics) RecordFailedAttempt(ctx context.Context, reason string) {
	if !m.registered || m.failedAttempts == nil {
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
