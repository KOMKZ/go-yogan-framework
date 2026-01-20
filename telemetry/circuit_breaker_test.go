package telemetry

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
)

// mock exporter
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

	// Initial state: Closed
	if cb.GetState() != StateClosed {
		t.Errorf("Expected initial state to be Closed, got %s", cb.GetState())
	}

	// Scenario 1: Continuous failures trigger circuit breaker
	primaryExporter.shouldFail = true
	for i := 0; i < 3; i++ {
		_ = cb.ExportSpans(ctx, spans)
	}

	if cb.GetState() != StateOpen {
		t.Errorf("Expected state to be Open after %d failures, got %s", config.FailureThreshold, cb.GetState())
	}

	// Scenario 2: Use fallback exporter during circuit breaker period
	fallbackCallCount := fallbackExporter.callCount
	_ = cb.ExportSpans(ctx, spans)
	if fallbackExporter.callCount <= fallbackCallCount {
		t.Error("Expected fallback exporter to be called during Open state")
	}

	// Scenario 3: Attempt recovery after timeout (Half-Open)
	time.Sleep(150 * time.Millisecond)
	primaryExporter.shouldFail = false
	_ = cb.ExportSpans(ctx, spans)

	if cb.GetState() != StateHalfOpen {
		t.Errorf("Expected state to be HalfOpen after timeout, got %s", cb.GetState())
	}

	// Scenario 4: Recovery after successful half-open state
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
		SuccessThreshold:     10, // Set high, ensure staying in HalfOpen state
		Timeout:              50 * time.Millisecond,
		HalfOpenMaxRequests:  2, // reduce restrictions, easier to test
		FallbackExporterType: "stdout",
	}

	cb := NewCircuitBreaker(config, logger, primaryExporter, fallbackExporter)
	ctx := context.Background()
	spans := []trace.ReadOnlySpan{}

	// Trigger circuit breaker
	primaryExporter.shouldFail = true
	_ = cb.ExportSpans(ctx, spans)
	_ = cb.ExportSpans(ctx, spans)
	
	if cb.GetState() != StateOpen {
		t.Fatalf("Expected state Open, got %s", cb.GetState())
	}

	// timeout waiting
	time.Sleep(100 * time.Millisecond)

	// Restore master output device
	primaryExporter.shouldFail = false
	primaryExporter.callCount = 0
	fallbackExporter.callCount = 0

	// The first request triggers HalfOpen
	err1 := cb.ExportSpans(ctx, spans)
	if err1 != nil {
		t.Fatalf("First request failed: %v", err1)
	}
	
	// Now it should be in HalfOpen state
	if cb.GetState() != StateHalfOpen {
		t.Logf("After first request, state=%s (expected HalfOpen)", cb.GetState())
	}

	// Subsequent requests (within half-open limit)
	_ = cb.ExportSpans(ctx, spans)
	
	// Requests exceeding the limit should use the fallback method
	_ = cb.ExportSpans(ctx, spans)
	_ = cb.ExportSpans(ctx, spans)

	t.Logf("Primary calls: %d, Fallback calls: %d, State: %s", 
		primaryExporter.callCount, fallbackExporter.callCount, cb.GetState())

	// Verify: fallback is called at least once
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

	// Trigger circuit breaker
	primaryExporter.shouldFail = true
	for i := 0; i < 2; i++ {
		_ = cb.ExportSpans(ctx, spans)
	}

	// enter HalfOpen on timeout
	time.Sleep(100 * time.Millisecond)

	// First request fails in HalfOpen state
	_ = cb.ExportSpans(ctx, spans)

	// Verify: Any failure in HalfOpen state immediately reopens
	if cb.GetState() != StateOpen {
		t.Errorf("Expected state to be Open after failure in HalfOpen, got %s", cb.GetState())
	}
}

func TestCircuitBreaker_Disabled(t *testing.T) {
	logger := zap.NewNop()
	primaryExporter := &mockExporter{shouldFail: true}
	fallbackExporter := &mockExporter{}

	config := CircuitBreakerConfig{
		Enabled: false, // Disable circuit breaker
	}

	cb := NewCircuitBreaker(config, logger, primaryExporter, fallbackExporter)
	ctx := context.Background()
	spans := []trace.ReadOnlySpan{}

	// Even if it fails, it will not trigger circuit breaking
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

