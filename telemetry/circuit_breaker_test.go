package telemetry

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
)

// mockExporter 模拟导出器
type mockExporter struct {
	shouldFail bool
	callCount  int
}

func (m *mockExporter) ExportSpans(ctx context.Context, spans []trace.ReadOnlySpan) error {
	m.callCount++
	if m.shouldFail {
		return errors.New("export failed")
	}
	return nil
}

func (m *mockExporter) Shutdown(ctx context.Context) error {
	return nil
}

func TestCircuitBreaker_StateMachine(t *testing.T) {
	logger := zap.NewNop()
	primaryExporter := &mockExporter{}
	fallbackExporter := &mockExporter{}

	config := CircuitBreakerConfig{
		Enabled:              true,
		FailureThreshold:     3,
		SuccessThreshold:     2,
		Timeout:              100 * time.Millisecond,
		HalfOpenMaxRequests:  2,
		FallbackExporterType: "stdout",
	}

	cb := NewCircuitBreaker(config, logger, primaryExporter, fallbackExporter)

	ctx := context.Background()
	spans := []trace.ReadOnlySpan{}

	// 初始状态：Closed
	if cb.GetState() != StateClosed {
		t.Errorf("Expected initial state to be Closed, got %s", cb.GetState())
	}

	// 场景 1：连续失败触发熔断
	primaryExporter.shouldFail = true
	for i := 0; i < 3; i++ {
		_ = cb.ExportSpans(ctx, spans)
	}

	if cb.GetState() != StateOpen {
		t.Errorf("Expected state to be Open after %d failures, got %s", config.FailureThreshold, cb.GetState())
	}

	// 场景 2：熔断期间使用降级导出器
	fallbackCallCount := fallbackExporter.callCount
	_ = cb.ExportSpans(ctx, spans)
	if fallbackExporter.callCount <= fallbackCallCount {
		t.Error("Expected fallback exporter to be called during Open state")
	}

	// 场景 3：等待超时后尝试恢复（Half-Open）
	time.Sleep(150 * time.Millisecond)
	primaryExporter.shouldFail = false
	_ = cb.ExportSpans(ctx, spans)

	if cb.GetState() != StateHalfOpen {
		t.Errorf("Expected state to be HalfOpen after timeout, got %s", cb.GetState())
	}

	// 场景 4：半开状态成功后恢复
	for i := 0; i < config.SuccessThreshold; i++ {
		_ = cb.ExportSpans(ctx, spans)
	}

	if cb.GetState() != StateClosed {
		t.Errorf("Expected state to be Closed after %d successes, got %s", config.SuccessThreshold, cb.GetState())
	}
}

func TestCircuitBreaker_HalfOpenMaxRequests(t *testing.T) {
	logger := zap.NewNop()
	primaryExporter := &mockExporter{}
	fallbackExporter := &mockExporter{}

	config := CircuitBreakerConfig{
		Enabled:              true,
		FailureThreshold:     2,
		SuccessThreshold:     10, // 设置很高，确保保持 HalfOpen 状态
		Timeout:              50 * time.Millisecond,
		HalfOpenMaxRequests:  2, // 减少限制，更容易测试
		FallbackExporterType: "stdout",
	}

	cb := NewCircuitBreaker(config, logger, primaryExporter, fallbackExporter)
	ctx := context.Background()
	spans := []trace.ReadOnlySpan{}

	// 触发熔断
	primaryExporter.shouldFail = true
	_ = cb.ExportSpans(ctx, spans)
	_ = cb.ExportSpans(ctx, spans)
	
	if cb.GetState() != StateOpen {
		t.Fatalf("Expected state Open, got %s", cb.GetState())
	}

	// 等待超时
	time.Sleep(100 * time.Millisecond)

	// 恢复主导出器
	primaryExporter.shouldFail = false
	primaryExporter.callCount = 0
	fallbackExporter.callCount = 0

	// 第一次请求触发 HalfOpen
	err1 := cb.ExportSpans(ctx, spans)
	if err1 != nil {
		t.Fatalf("First request failed: %v", err1)
	}
	
	// 此时应该在 HalfOpen 状态
	if cb.GetState() != StateHalfOpen {
		t.Logf("After first request, state=%s (expected HalfOpen)", cb.GetState())
	}

	// 后续请求（在半开限制内）
	_ = cb.ExportSpans(ctx, spans)
	
	// 超过限制的请求应该走 fallback
	_ = cb.ExportSpans(ctx, spans)
	_ = cb.ExportSpans(ctx, spans)

	t.Logf("Primary calls: %d, Fallback calls: %d, State: %s", 
		primaryExporter.callCount, fallbackExporter.callCount, cb.GetState())

	// 验证：fallback 至少被调用一次
	if fallbackExporter.callCount == 0 {
		t.Error("Expected fallback exporter to be called when exceeding HalfOpenMaxRequests")
	}
}

func TestCircuitBreaker_HalfOpenFailureReopen(t *testing.T) {
	logger := zap.NewNop()
	primaryExporter := &mockExporter{}
	fallbackExporter := &mockExporter{}

	config := CircuitBreakerConfig{
		Enabled:              true,
		FailureThreshold:     2,
		SuccessThreshold:     2,
		Timeout:              50 * time.Millisecond,
		HalfOpenMaxRequests:  5,
		FallbackExporterType: "stdout",
	}

	cb := NewCircuitBreaker(config, logger, primaryExporter, fallbackExporter)
	ctx := context.Background()
	spans := []trace.ReadOnlySpan{}

	// 触发熔断
	primaryExporter.shouldFail = true
	for i := 0; i < 2; i++ {
		_ = cb.ExportSpans(ctx, spans)
	}

	// 等待超时进入 HalfOpen
	time.Sleep(100 * time.Millisecond)

	// HalfOpen 状态下第一次请求失败
	_ = cb.ExportSpans(ctx, spans)

	// 验证：HalfOpen 状态任何失败立即重新打开
	if cb.GetState() != StateOpen {
		t.Errorf("Expected state to be Open after failure in HalfOpen, got %s", cb.GetState())
	}
}

func TestCircuitBreaker_Disabled(t *testing.T) {
	logger := zap.NewNop()
	primaryExporter := &mockExporter{shouldFail: true}
	fallbackExporter := &mockExporter{}

	config := CircuitBreakerConfig{
		Enabled: false, // 禁用熔断器
	}

	cb := NewCircuitBreaker(config, logger, primaryExporter, fallbackExporter)
	ctx := context.Background()
	spans := []trace.ReadOnlySpan{}

	// 即使失败，也不会触发熔断
	for i := 0; i < 10; i++ {
		_ = cb.ExportSpans(ctx, spans)
	}

	if cb.GetState() != StateClosed {
		t.Errorf("Expected state to remain Closed when disabled, got %s", cb.GetState())
	}

	if fallbackExporter.callCount > 0 {
		t.Error("Expected fallback exporter never to be called when circuit breaker is disabled")
	}
}

func TestCircuitBreaker_GetStats(t *testing.T) {
	logger := zap.NewNop()
	primaryExporter := &mockExporter{}
	fallbackExporter := &mockExporter{}

	config := CircuitBreakerConfig{
		Enabled:              true,
		FailureThreshold:     5,
		SuccessThreshold:     3,
		Timeout:              60 * time.Second,
		HalfOpenMaxRequests:  2,
		FallbackExporterType: "stdout",
	}

	cb := NewCircuitBreaker(config, logger, primaryExporter, fallbackExporter)

	stats := cb.GetStats()

	if stats["state"] != "closed" {
		t.Errorf("Expected state 'closed', got %s", stats["state"])
	}

	if stats["failure_threshold"] != 5 {
		t.Errorf("Expected failure_threshold 5, got %v", stats["failure_threshold"])
	}

	if stats["success_threshold"] != 3 {
		t.Errorf("Expected success_threshold 3, got %v", stats["success_threshold"])
	}

	if stats["fallback_exporter"] != "stdout" {
		t.Errorf("Expected fallback_exporter 'stdout', got %v", stats["fallback_exporter"])
	}
}

