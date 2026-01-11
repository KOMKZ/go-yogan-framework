package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	
	"github.com/KOMKZ/go-yogan-framework/retry"
)

// ============================================================
// Client 基础测试
// ============================================================

func TestNewClient(t *testing.T) {
	client := NewClient()
	
	if client == nil {
		t.Fatal("NewClient() should not return nil")
	}
	
	if client.httpClient == nil {
		t.Error("httpClient should be initialized")
	}
	
	if client.config == nil {
		t.Error("config should be initialized")
	}
}

func TestNewClient_WithOptions(t *testing.T) {
	client := NewClient(
		WithBaseURL("https://api.example.com"),
		WithTimeout(5*time.Second),
		WithHeader("User-Agent", "Test/1.0"),
	)
	
	if client.config.baseURL != "https://api.example.com" {
		t.Error("baseURL not set")
	}
	
	if client.config.timeout != 5*time.Second {
		t.Error("timeout not set")
	}
	
	if client.config.headers["User-Agent"] != "Test/1.0" {
		t.Error("header not set")
	}
}

// ============================================================
// Client.Do 测试
// ============================================================

func TestClient_Do_Success(t *testing.T) {
	// 创建测试服务器
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success"}`))
	}))
	defer ts.Close()
	
	client := NewClient()
	req := NewGetRequest(ts.URL)
	
	resp, err := client.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("Do() failed: %v", err)
	}
	defer resp.Close()
	
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	
	if !resp.IsSuccess() {
		t.Error("IsSuccess() should be true")
	}
}

func TestClient_Do_WithBaseURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/users" {
			t.Errorf("expected /api/users, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	
	client := NewClient(WithBaseURL(ts.URL))
	req := NewGetRequest("/api/users")
	
	resp, err := client.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("Do() failed: %v", err)
	}
	defer resp.Close()
	
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestClient_Do_WithQuery(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") != "1" {
			t.Errorf("expected page=1, got %s", r.URL.Query().Get("page"))
		}
		if r.URL.Query().Get("limit") != "20" {
			t.Errorf("expected limit=20, got %s", r.URL.Query().Get("limit"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	
	client := NewClient()
	req := NewGetRequest(ts.URL)
	
	resp, err := client.Do(context.Background(), req,
		WithQuery("page", "1"),
		WithQuery("limit", "20"),
	)
	if err != nil {
		t.Fatalf("Do() failed: %v", err)
	}
	defer resp.Close()
}

func TestClient_Do_WithHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Error("Authorization header not set")
		}
		if r.Header.Get("X-Custom") != "value" {
			t.Error("X-Custom header not set")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	
	client := NewClient()
	req := NewGetRequest(ts.URL)
	req.WithHeader("Authorization", "Bearer token")
	
	resp, err := client.Do(context.Background(), req,
		WithHeader("X-Custom", "value"),
	)
	if err != nil {
		t.Fatalf("Do() failed: %v", err)
	}
	defer resp.Close()
}

func TestClient_Do_WithTimeout(t *testing.T) {
	// 创建慢速服务器
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	
	client := NewClient(WithTimeout(50 * time.Millisecond))
	req := NewGetRequest(ts.URL)
	
	_, err := client.Do(context.Background(), req)
	if err == nil {
		t.Error("expected timeout error")
	}
	
	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("expected timeout error, got: %v", err)
	}
}

func TestClient_Do_WithBeforeRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Injected") != "true" {
			t.Error("X-Injected header not set")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	
	client := NewClient()
	req := NewGetRequest(ts.URL)
	
	resp, err := client.Do(context.Background(), req,
		WithBeforeRequest(func(r *http.Request) error {
			r.Header.Set("X-Injected", "true")
			return nil
		}),
	)
	if err != nil {
		t.Fatalf("Do() failed: %v", err)
	}
	defer resp.Close()
}

func TestClient_Do_WithAfterResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	
	afterResponseCalled := false
	
	client := NewClient()
	req := NewGetRequest(ts.URL)
	
	resp, err := client.Do(context.Background(), req,
		WithAfterResponse(func(r *Response) error {
			afterResponseCalled = true
			return nil
		}),
	)
	if err != nil {
		t.Fatalf("Do() failed: %v", err)
	}
	defer resp.Close()
	
	if !afterResponseCalled {
		t.Error("afterResponse hook not called")
	}
}

// ============================================================
// Client.Get 测试
// ============================================================

func TestClient_Get(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success"}`))
	}))
	defer ts.Close()
	
	client := NewClient()
	resp, err := client.Get(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	defer resp.Close()
	
	if !resp.IsSuccess() {
		t.Error("IsSuccess() should be true")
	}
}

// ============================================================
// Client.Post 测试
// ============================================================

func TestClient_Post(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "Alice") {
			t.Error("body should contain 'Alice'")
		}
		
		w.WriteHeader(http.StatusCreated)
	}))
	defer ts.Close()
	
	client := NewClient()
	
	req := NewPostRequest(ts.URL)
	req.WithJSON(map[string]string{"name": "Alice"})
	
	resp, err := client.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("Post() failed: %v", err)
	}
	defer resp.Close()
	
	if resp.StatusCode != 201 {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}
}

// ============================================================
// Client.Put 测试
// ============================================================

func TestClient_Put(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	
	client := NewClient()
	
	req := NewPutRequest(ts.URL)
	resp, err := client.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	defer resp.Close()
}

// ============================================================
// Client.Delete 测试
// ============================================================

func TestClient_Delete(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()
	
	client := NewClient()
	resp, err := client.Delete(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}
	defer resp.Close()
	
	if resp.StatusCode != 204 {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}
}

// ============================================================
// Retry 测试
// ============================================================

func TestClient_Do_WithRetry_Success(t *testing.T) {
	attempts := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer ts.Close()
	
	client := NewClient()
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
	
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestClient_Do_DisableRetry(t *testing.T) {
	attempts := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()
	
	client := NewClient(
		WithRetry(retry.MaxAttempts(3)),
	)
	req := NewGetRequest(ts.URL)
	
	resp, err := client.Do(context.Background(), req, DisableRetry())
	if err != nil {
		t.Fatalf("Do() failed: %v", err)
	}
	defer resp.Close()
	
	if attempts != 1 {
		t.Errorf("expected 1 attempt (no retry), got %d", attempts)
	}
}

// ============================================================
// 泛型方法测试
// ============================================================

type TestUser struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func TestGet_Generic(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(TestUser{Name: "Alice", Email: "alice@example.com"})
	}))
	defer ts.Close()
	
	client := NewClient()
	user, err := Get[TestUser](client, context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	
	if user.Name != "Alice" {
		t.Errorf("expected Name=Alice, got %s", user.Name)
	}
	
	if user.Email != "alice@example.com" {
		t.Errorf("expected Email=alice@example.com, got %s", user.Email)
	}
}

func TestPost_Generic(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var user TestUser
		json.NewDecoder(r.Body).Decode(&user)
		
		// 返回相同的用户
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(user)
	}))
	defer ts.Close()
	
	client := NewClient()
	inputUser := TestUser{Name: "Bob", Email: "bob@example.com"}
	
	resultUser, err := Post[TestUser](client, context.Background(), ts.URL, inputUser)
	if err != nil {
		t.Fatalf("Post() failed: %v", err)
	}
	
	if resultUser.Name != "Bob" {
		t.Errorf("expected Name=Bob, got %s", resultUser.Name)
	}
}

// ============================================================
// 错误场景测试
// ============================================================

func TestClient_Do_InvalidURL(t *testing.T) {
	client := NewClient()
	req := NewGetRequest("http://[invalid url")
	
	_, err := client.Do(context.Background(), req)
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestClient_Do_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	
	client := NewClient()
	req := NewGetRequest(ts.URL)
	
	resp, err := client.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("Do() failed: %v", err)
	}
	defer resp.Close()
	
	if !resp.IsServerError() {
		t.Error("IsServerError() should be true")
	}
}

func TestClient_Do_ContextCanceled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	
	client := NewClient()
	req := NewGetRequest(ts.URL)
	
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消
	
	_, err := client.Do(ctx, req)
	if err == nil {
		t.Error("expected context canceled error")
	}
}

// ============================================================
// 边界测试
// ============================================================

func TestClient_Do_NilContext(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	
	client := NewClient()
	req := NewGetRequest(ts.URL)
	
	// nil context 应该被替换为 context.Background()
	resp, err := client.Do(nil, req)
	if err != nil {
		t.Fatalf("Do() failed: %v", err)
	}
	defer resp.Close()
}

func TestClient_Do_EmptyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
		// 不写任何 body
	}))
	defer ts.Close()
	
	client := NewClient()
	req := NewGetRequest(ts.URL)
	
	resp, err := client.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("Do() failed: %v", err)
	}
	defer resp.Close()
	
	if len(resp.Body) != 0 {
		t.Errorf("expected empty body, got %d bytes", len(resp.Body))
	}
}

// ============================================================
// Benchmark
// ============================================================

func BenchmarkClient_Get(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success"}`))
	}))
	defer ts.Close()
	
	client := NewClient()
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, _ := client.Get(ctx, ts.URL)
		if resp != nil {
			resp.Close()
		}
	}
}

func BenchmarkClient_Post_JSON(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer ts.Close()
	
	client := NewClient()
	ctx := context.Background()
	data := map[string]string{"name": "Alice", "email": "alice@example.com"}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := NewPostRequest(ts.URL)
		req.WithJSON(data)
		resp, _ := client.Do(ctx, req)
		if resp != nil {
			resp.Close()
		}
	}
}

func Benchmark_Get_Generic(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"name": "Alice", "email": "alice@example.com"}`)
	}))
	defer ts.Close()
	
	client := NewClient()
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Get[TestUser](client, ctx, ts.URL)
	}
}

