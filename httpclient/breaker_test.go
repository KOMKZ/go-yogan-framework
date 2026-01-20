package httpclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	
	"github.com/KOMKZ/go-yogan-framework/breaker"
	"github.com/KOMKZ/go-yogan-framework/retry"
)

// ============================================================
// Mock BreakerManager
// ============================================================

type mockBreakerManager struct {
	enabled       bool
	executeFunc   func(ctx context.Context, req *breaker.Request) (interface{}, error)
	getStateFunc  func(resource string) breaker.State
}

func (m *mockBreakerManager) Execute(ctx context.Context, req *breaker.Request) (interface{}, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, req)
	}
	// Execute by default
	return req.Execute(ctx)
}

func (m *mockBreakerManager) IsEnabled() bool {
	return m.enabled
}

func (m *mockBreakerManager) GetState(resource string) breaker.State {
	if m.getStateFunc != nil {
		return m.getStateFunc(resource)
	}
	return breaker.StateClosed
}

// ============================================================
// Breaker basic test
// ============================================================

func TestWithBreaker(t *testing.T) {
	manager := &mockBreakerManager{enabled: true}
	
	client := NewClient(WithBreaker(manager))
	
	if client.config.breakerManager != manager {
		t.Error("breaker manager not set")
	}
}

func TestWithBreakerResource(t *testing.T) {
	client := NewClient(WithBreakerResource("test-service"))
	
	if client.config.breakerResource != "test-service" {
		t.Error("breaker resource not set")
	}
}

func TestWithBreakerFallback(t *testing.T) {
	fallback := func(ctx context.Context, err error) (*Response, error) {
		return &Response{StatusCode: 200}, nil
	}
	
	client := NewClient(WithBreakerFallback(fallback))
	
	if client.config.breakerFallback == nil {
		t.Error("breaker fallback not set")
	}
}

func TestDisableBreaker(t *testing.T) {
	client := NewClient(DisableBreaker())
	
	if !client.config.breakerDisabled {
		t.Error("breaker should be disabled")
	}
}

// ============================================================
// Circuit breaker integration test
// ============================================================

func TestClient_Do_WithBreaker_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer ts.Close()
	
	executeCalled := false
	manager := &mockBreakerManager{
		enabled: true,
		executeFunc: func(ctx context.Context, req *breaker.Request) (interface{}, error) {
			executeCalled = true
			// Validate resource name
			if !strings.Contains(req.Resource, ts.URL) {
				t.Errorf("unexpected resource: %s", req.Resource)
			}
			// Execute the actual request
			return req.Execute(ctx)
		},
	}
	
	client := NewClient(WithBreaker(manager))
	req := NewGetRequest(ts.URL)
	
	resp, err := client.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("Do() failed: %v", err)
	}
	defer resp.Close()
	
	if !executeCalled {
		t.Error("breaker Execute not called")
	}
	
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestClient_Do_WithBreaker_CircuitOpen(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	
	// simulate circuit breaker open
	manager := &mockBreakerManager{
		enabled: true,
		executeFunc: func(ctx context.Context, req *breaker.Request) (interface{}, error) {
			// Return circuit breaker error
			return nil, errors.New("circuit breaker is open")
		},
	}
	
	client := NewClient(WithBreaker(manager))
	req := NewGetRequest(ts.URL)
	
	_, err := client.Do(context.Background(), req)
	if err == nil {
		t.Error("expected circuit breaker error")
	}
	
	if !strings.Contains(err.Error(), "circuit breaker") {
		t.Errorf("error should mention circuit breaker, got: %v", err)
	}
}

func TestClient_Do_WithBreaker_Fallback(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()
	
	fallbackCalled := false
	fallback := func(ctx context.Context, err error) (*Response, error) {
		fallbackCalled = true
		return &Response{
			StatusCode: 200,
			Body:       []byte("fallback response"),
		}, nil
	}
	
	// Simulate circuit breaker executing fallback
	manager := &mockBreakerManager{
		enabled: true,
		executeFunc: func(ctx context.Context, req *breaker.Request) (interface{}, error) {
			// Execute the original request first (it will fail)
			_, err := req.Execute(ctx)
			if err != nil && req.Fallback != nil {
				// Execute degradation plan
				return req.Fallback(ctx, err)
			}
			return nil, err
		},
	}
	
	client := NewClient(
		WithBreaker(manager),
		WithBreakerFallback(fallback),
	)
	req := NewGetRequest(ts.URL)
	
	resp, err := client.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("Do() failed: %v", err)
	}
	defer resp.Close()
	
	if !fallbackCalled {
		t.Error("fallback not called")
	}
	
	if string(resp.Body) != "fallback response" {
		t.Errorf("expected fallback response, got: %s", string(resp.Body))
	}
}

func TestClient_Do_WithBreaker_CustomResource(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	
	capturedResource := ""
	manager := &mockBreakerManager{
		enabled: true,
		executeFunc: func(ctx context.Context, req *breaker.Request) (interface{}, error) {
			capturedResource = req.Resource
			return req.Execute(ctx)
		},
	}
	
	client := NewClient(WithBreaker(manager))
	req := NewGetRequest(ts.URL)
	
	_, err := client.Do(context.Background(), req,
		WithBreakerResource("custom-service"),
	)
	if err != nil {
		t.Fatalf("Do() failed: %v", err)
	}
	
	if capturedResource != "custom-service" {
		t.Errorf("expected resource 'custom-service', got: %s", capturedResource)
	}
}

func TestClient_Do_BreakerDisabled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	
	executeCalled := false
	manager := &mockBreakerManager{
		enabled: true,
		executeFunc: func(ctx context.Context, req *breaker.Request) (interface{}, error) {
			executeCalled = true
			return req.Execute(ctx)
		},
	}
	
	client := NewClient(WithBreaker(manager))
	req := NewGetRequest(ts.URL)
	
	// Disable circuit breaker at request level
	resp, err := client.Do(context.Background(), req, DisableBreaker())
	if err != nil {
		t.Fatalf("Do() failed: %v", err)
	}
	defer resp.Close()
	
	if executeCalled {
		t.Error("breaker should not be called when disabled")
	}
}

func TestClient_Do_BreakerNotEnabled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	
	executeCalled := false
	manager := &mockBreakerManager{
		enabled: false, // Circuit breaker not enabled
		executeFunc: func(ctx context.Context, req *breaker.Request) (interface{}, error) {
			executeCalled = true
			return req.Execute(ctx)
		},
	}
	
	client := NewClient(WithBreaker(manager))
	req := NewGetRequest(ts.URL)
	
	resp, err := client.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("Do() failed: %v", err)
	}
	defer resp.Close()
	
	if executeCalled {
		t.Error("breaker should not be called when not enabled")
	}
}

func TestClient_Do_WithBreaker_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	
	var capturedError error
	manager := &mockBreakerManager{
		enabled: true,
		executeFunc: func(ctx context.Context, req *breaker.Request) (interface{}, error) {
			// Execute request
			result, err := req.Execute(ctx)
			capturedError = err
			return result, err
		},
	}
	
	client := NewClient(WithBreaker(manager))
	req := NewGetRequest(ts.URL)
	
	// 5xx errors will be converted to error and passed to the circuit breaker
	_, err := client.Do(context.Background(), req)
	if err == nil {
		t.Error("expected error for 5xx response")
	}
	
	// The circuit breaker should receive an error
	if capturedError == nil {
		t.Error("breaker should receive error for 5xx response")
	}
	
	if !strings.Contains(capturedError.Error(), "500") {
		t.Errorf("error should mention 500, got: %v", capturedError)
	}
}

func TestClient_Do_WithBreaker_BaseURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	
	capturedResource := ""
	manager := &mockBreakerManager{
		enabled: true,
		executeFunc: func(ctx context.Context, req *breaker.Request) (interface{}, error) {
			capturedResource = req.Resource
			return req.Execute(ctx)
		},
	}
	
	client := NewClient(
		WithBaseURL(ts.URL),
		WithBreaker(manager),
	)
	req := NewGetRequest("/api/users")
	
	_, err := client.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("Do() failed: %v", err)
	}
	
	// The resource name should include the full URL
	if !strings.Contains(capturedResource, ts.URL) {
		t.Errorf("resource should contain base URL, got: %s", capturedResource)
	}
	if !strings.Contains(capturedResource, "/api/users") {
		t.Errorf("resource should contain path, got: %s", capturedResource)
	}
}

// ============================================================
// Breaker + Retry Combination Test
// ============================================================

func TestClient_Do_WithBreakerAndRetry(t *testing.T) {
	attempts := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer ts.Close()
	
	breakerExecuteCount := 0
	manager := &mockBreakerManager{
		enabled: true,
		executeFunc: func(ctx context.Context, req *breaker.Request) (interface{}, error) {
			breakerExecuteCount++
			return req.Execute(ctx)
		},
	}
	
	client := NewClient(WithBreaker(manager))
	req := NewGetRequest(ts.URL)
	
	resp, err := client.Do(context.Background(), req,
		WithRetry(
			retry.MaxAttempts(3),
			retry.Backoff(retry.ConstantBackoff(10*time.Millisecond)),
		),
	)
	if err != nil {
		t.Fatalf("Do() failed: %v", err)
	}
	defer resp.Close()
	
	// Retries should be outside the circuit breaker
	if attempts != 2 {
		t.Errorf("expected 2 HTTP attempts, got %d", attempts)
	}
	
	// The circuit breaker should be called twice (consistent with the number of retries)
	if breakerExecuteCount != 2 {
		t.Errorf("expected 2 breaker executions, got %d", breakerExecuteCount)
	}
}

// ============================================================
// Config merge test
// ============================================================

func TestConfig_merge_Breaker(t *testing.T) {
	manager1 := &mockBreakerManager{enabled: true}
	manager2 := &mockBreakerManager{enabled: false}
	
	base := newConfig()
	base.breakerManager = manager1
	base.breakerResource = "base-resource"
	
	other := newConfig()
	other.breakerManager = manager2
	other.breakerResource = "override-resource"
	
	merged := base.merge(other)
	
	if merged.breakerManager != manager2 {
		t.Error("breaker manager should be overridden")
	}
	
	if merged.breakerResource != "override-resource" {
		t.Error("breaker resource should be overridden")
	}
}

func TestConfig_merge_BreakerFallback(t *testing.T) {
	fallback1 := func(ctx context.Context, err error) (*Response, error) {
		return &Response{StatusCode: 503}, nil
	}
	fallback2 := func(ctx context.Context, err error) (*Response, error) {
		return &Response{StatusCode: 200}, nil
	}
	
	base := newConfig()
	base.breakerFallback = fallback1
	
	other := newConfig()
	other.breakerFallback = fallback2
	
	merged := base.merge(other)
	
	if merged.breakerFallback == nil {
		t.Error("breaker fallback should be set")
	}
	
	// Test actual invocation
	resp, _ := merged.breakerFallback(context.Background(), errors.New("test"))
	if resp.StatusCode != 200 {
		t.Error("fallback should be overridden")
	}
}

// ============================================================
// Benchmark
// ============================================================

func BenchmarkClient_Do_WithBreaker(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	
	manager := &mockBreakerManager{
		enabled: true,
		executeFunc: func(ctx context.Context, req *breaker.Request) (interface{}, error) {
			return req.Execute(ctx)
		},
	}
	
	client := NewClient(WithBreaker(manager))
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := NewGetRequest(ts.URL)
		resp, _ := client.Do(ctx, req)
		if resp != nil {
			resp.Close()
		}
	}
}

func BenchmarkClient_Do_WithoutBreaker(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	
	client := NewClient()
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := NewGetRequest(ts.URL)
		resp, _ := client.Do(ctx, req)
		if resp != nil {
			resp.Close()
		}
	}
}

