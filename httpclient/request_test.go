package httpclient

import (
	"io"
	"strings"
	"testing"
)

// ============================================================
// NewRequest test
// ============================================================

func TestNewRequest(t *testing.T) {
	req := NewRequest("GET", "https://example.com")
	
	if req.Method != "GET" {
		t.Errorf("expected GET, got %s", req.Method)
	}
	
	if req.URL != "https://example.com" {
		t.Errorf("expected https://example.com, got %s", req.URL)
	}
	
	if req.Headers == nil {
		t.Error("Headers should be initialized")
	}
	
	if req.Query == nil {
		t.Error("Query should be initialized")
	}
}

func TestNewGetRequest(t *testing.T) {
	req := NewGetRequest("https://example.com")
	if req.Method != "GET" {
		t.Errorf("expected GET, got %s", req.Method)
	}
}

func TestNewPostRequest(t *testing.T) {
	req := NewPostRequest("https://example.com")
	if req.Method != "POST" {
		t.Errorf("expected POST, got %s", req.Method)
	}
}

func TestNewPutRequest(t *testing.T) {
	req := NewPutRequest("https://example.com")
	if req.Method != "PUT" {
		t.Errorf("expected PUT, got %s", req.Method)
	}
}

func TestNewDeleteRequest(t *testing.T) {
	req := NewDeleteRequest("https://example.com")
	if req.Method != "DELETE" {
		t.Errorf("expected DELETE, got %s", req.Method)
	}
}

// ============================================================
// WithHeader test
// ============================================================

func TestRequest_WithHeader(t *testing.T) {
	req := NewRequest("GET", "https://example.com")
	req.WithHeader("Authorization", "Bearer token")
	req.WithHeader("Content-Type", "application/json")
	
	if req.Headers["Authorization"] != "Bearer token" {
		t.Error("Authorization header not set")
	}
	
	if req.Headers["Content-Type"] != "application/json" {
		t.Error("Content-Type header not set")
	}
}

func TestRequest_WithHeader_Chaining(t *testing.T) {
	req := NewRequest("GET", "https://example.com").
		WithHeader("X-Test", "value1").
		WithHeader("X-Test2", "value2")
	
	if req.Headers["X-Test"] != "value1" {
		t.Error("chaining failed")
	}
}

// ============================================================
// WithQuery test
// ============================================================

func TestRequest_WithQuery(t *testing.T) {
	req := NewRequest("GET", "https://example.com")
	req.WithQuery("page", "1")
	req.WithQuery("limit", "20")
	
	if req.Query.Get("page") != "1" {
		t.Error("page query not set")
	}
	
	if req.Query.Get("limit") != "20" {
		t.Error("limit query not set")
	}
}

func TestRequest_WithQuery_Chaining(t *testing.T) {
	req := NewRequest("GET", "https://example.com").
		WithQuery("a", "1").
		WithQuery("b", "2")
	
	if req.Query.Get("a") != "1" || req.Query.Get("b") != "2" {
		t.Error("chaining failed")
	}
}

// ============================================================
// WithBody test
// ============================================================

func TestRequest_WithBody(t *testing.T) {
	body := strings.NewReader("test body")
	req := NewRequest("POST", "https://example.com")
	req.WithBody(body)
	
	if req.Body == nil {
		t.Error("Body should be set")
	}
	
	if len(req.bodyBytes) == 0 {
		t.Error("bodyBytes should be cached")
	}
}

func TestRequest_WithBody_Nil(t *testing.T) {
	req := NewRequest("POST", "https://example.com")
	req.WithBody(nil)
	
	if req.Body != nil {
		t.Error("Body should be nil")
	}
}

// ============================================================
// WithJSON test
// ============================================================

func TestRequest_WithJSON(t *testing.T) {
	data := map[string]interface{}{
		"name": "Alice",
		"age":  30,
	}
	
	req := NewRequest("POST", "https://example.com")
	req.WithJSON(data)
	
	if req.Headers["Content-Type"] != "application/json" {
		t.Error("Content-Type should be application/json")
	}
	
	if len(req.bodyBytes) == 0 {
		t.Error("bodyBytes should be set")
	}
	
	// Validate JSON content
	bodyStr := string(req.bodyBytes)
	if !strings.Contains(bodyStr, "Alice") {
		t.Error("JSON body should contain 'Alice'")
	}
}

func TestRequest_WithJSON_Nil(t *testing.T) {
	req := NewRequest("POST", "https://example.com")
	req.WithJSON(nil)
	
	if req.Body != nil {
		t.Error("Body should be nil")
	}
}

// ============================================================
// WithForm test
// ============================================================

func TestRequest_WithForm(t *testing.T) {
	data := map[string]string{
		"username": "alice",
		"password": "secret",
	}
	
	req := NewRequest("POST", "https://example.com")
	req.WithForm(data)
	
	if req.Headers["Content-Type"] != "application/x-www-form-urlencoded" {
		t.Error("Content-Type should be application/x-www-form-urlencoded")
	}
	
	if len(req.bodyBytes) == 0 {
		t.Error("bodyBytes should be set")
	}
	
	// Validate form content
	bodyStr := string(req.bodyBytes)
	if !strings.Contains(bodyStr, "username=alice") {
		t.Error("Form body should contain 'username=alice'")
	}
}

func TestRequest_WithForm_Nil(t *testing.T) {
	req := NewRequest("POST", "https://example.com")
	req.WithForm(nil)
	
	if req.Body != nil {
		t.Error("Body should be nil")
	}
}

// ============================================================
// buildHTTPRequest test
// ============================================================

func TestRequest_buildHTTPRequest_Basic(t *testing.T) {
	req := NewRequest("GET", "https://example.com")
	
	httpReq, err := req.buildHTTPRequest()
	if err != nil {
		t.Fatalf("buildHTTPRequest failed: %v", err)
	}
	
	if httpReq.Method != "GET" {
		t.Errorf("expected GET, got %s", httpReq.Method)
	}
	
	if httpReq.URL.String() != "https://example.com" {
		t.Errorf("expected https://example.com, got %s", httpReq.URL.String())
	}
}

func TestRequest_buildHTTPRequest_WithQuery(t *testing.T) {
	req := NewRequest("GET", "https://example.com")
	req.WithQuery("page", "1")
	req.WithQuery("limit", "20")
	
	httpReq, err := req.buildHTTPRequest()
	if err != nil {
		t.Fatalf("buildHTTPRequest failed: %v", err)
	}
	
	urlStr := httpReq.URL.String()
	if !strings.Contains(urlStr, "page=1") {
		t.Error("URL should contain page=1")
	}
	if !strings.Contains(urlStr, "limit=20") {
		t.Error("URL should contain limit=20")
	}
}

func TestRequest_buildHTTPRequest_WithExistingQuery(t *testing.T) {
	req := NewRequest("GET", "https://example.com?existing=true")
	req.WithQuery("page", "1")
	
	httpReq, err := req.buildHTTPRequest()
	if err != nil {
		t.Fatalf("buildHTTPRequest failed: %v", err)
	}
	
	urlStr := httpReq.URL.String()
	if !strings.Contains(urlStr, "existing=true") {
		t.Error("URL should contain existing=true")
	}
	if !strings.Contains(urlStr, "page=1") {
		t.Error("URL should contain page=1")
	}
}

func TestRequest_buildHTTPRequest_WithHeaders(t *testing.T) {
	req := NewRequest("GET", "https://example.com")
	req.WithHeader("Authorization", "Bearer token")
	req.WithHeader("X-Custom", "value")
	
	httpReq, err := req.buildHTTPRequest()
	if err != nil {
		t.Fatalf("buildHTTPRequest failed: %v", err)
	}
	
	if httpReq.Header.Get("Authorization") != "Bearer token" {
		t.Error("Authorization header not set")
	}
	
	if httpReq.Header.Get("X-Custom") != "value" {
		t.Error("X-Custom header not set")
	}
}

func TestRequest_buildHTTPRequest_WithBody(t *testing.T) {
	req := NewRequest("POST", "https://example.com")
	req.WithJSON(map[string]string{"name": "Alice"})
	
	httpReq, err := req.buildHTTPRequest()
	if err != nil {
		t.Fatalf("buildHTTPRequest failed: %v", err)
	}
	
	if httpReq.Body == nil {
		t.Error("Body should be set")
	}
	
	body, _ := io.ReadAll(httpReq.Body)
	if !strings.Contains(string(body), "Alice") {
		t.Error("Body should contain 'Alice'")
	}
}

// ============================================================
// Clone test
// ============================================================

func TestRequest_Clone(t *testing.T) {
	req := NewRequest("POST", "https://example.com")
	req.WithHeader("Authorization", "Bearer token")
	req.WithQuery("page", "1")
	req.WithJSON(map[string]string{"name": "Alice"})
	
	clone := req.Clone()
	
	// Validate clone
	if clone.Method != req.Method {
		t.Error("Method not cloned")
	}
	
	if clone.URL != req.URL {
		t.Error("URL not cloned")
	}
	
	if clone.Headers["Authorization"] != "Bearer token" {
		t.Error("Headers not cloned")
	}
	
	if clone.Query.Get("page") != "1" {
		t.Error("Query not cloned")
	}
	
	if len(clone.bodyBytes) != len(req.bodyBytes) {
		t.Error("bodyBytes not cloned")
	}
	
	// Verify independence (modifying the original object does not affect the clone)
	req.WithHeader("X-Test", "value")
	if clone.Headers["X-Test"] == "value" {
		t.Error("Clone should be independent")
	}
}

func TestRequest_Clone_EmptyBody(t *testing.T) {
	req := NewRequest("GET", "https://example.com")
	clone := req.Clone()
	
	if clone.Body != nil {
		t.Error("Body should be nil")
	}
}

// ============================================================
// Benchmark
// ============================================================

func BenchmarkNewRequest(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewRequest("GET", "https://example.com")
	}
}

func BenchmarkRequest_WithJSON(b *testing.B) {
	data := map[string]string{"name": "Alice"}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := NewRequest("POST", "https://example.com")
		req.WithJSON(data)
	}
}

func BenchmarkRequest_Clone(b *testing.B) {
	req := NewRequest("POST", "https://example.com")
	req.WithHeader("Authorization", "Bearer token")
	req.WithJSON(map[string]string{"name": "Alice"})
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req.Clone()
	}
}

