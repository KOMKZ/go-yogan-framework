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
	// 默认直接执行
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
// Breaker 基础测试
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
// 熔断器集成测试
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
			// 验证资源名称
			if !strings.Contains(req.Resource, ts.URL) {
				t.Errorf("unexpected resource: %s", req.Resource)
			}
			// 执行实际请求
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
	
	// 模拟熔断器打开
	manager := &mockBreakerManager{
		enabled: true,
		executeFunc: func(ctx context.Context, req *breaker.Request) (interface{}, error) {
			// 返回熔断错误
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
	
	// 模拟熔断器执行降级
	manager := &mockBreakerManager{
		enabled: true,
		executeFunc: func(ctx context.Context, req *breaker.Request) (interface{}, error) {
			// 先执行原请求（会失败）
			_, err := req.Execute(ctx)
			if err != nil && req.Fallback != nil {
				// 执行降级
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
	
	// 请求级禁用熔断器
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
		enabled: false, // 熔断器未启用
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
			// 执行请求
			result, err := req.Execute(ctx)
			capturedError = err
			return result, err
		},
	}
	
	client := NewClient(WithBreaker(manager))
	req := NewGetRequest(ts.URL)
	
	// 5xx 错误会被转换为 error 传递给熔断器
	_, err := client.Do(context.Background(), req)
	if err == nil {
		t.Error("expected error for 5xx response")
	}
	
	// 熔断器应该收到错误
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
	
	// 资源名称应该包含完整 URL
	if !strings.Contains(capturedResource, ts.URL) {
		t.Errorf("resource should contain base URL, got: %s", capturedResource)
	}
	if !strings.Contains(capturedResource, "/api/users") {
		t.Errorf("resource should contain path, got: %s", capturedResource)
	}
}

// ============================================================
// Breaker + Retry 组合测试
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
	
	// 重试应该在熔断器外层
	if attempts != 2 {
		t.Errorf("expected 2 HTTP attempts, got %d", attempts)
	}
	
	// 熔断器应该被调用 2 次（与重试次数一致）
	if breakerExecuteCount != 2 {
		t.Errorf("expected 2 breaker executions, got %d", breakerExecuteCount)
	}
}

// ============================================================
// Config merge 测试
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
	
	// 测试实际调用
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

