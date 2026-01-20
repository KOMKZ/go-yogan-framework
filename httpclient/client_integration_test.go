package httpclient

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
	
	"github.com/KOMKZ/go-yogan-framework/retry"
)

// ============================================================
// Complete testing for Client.Post/Put/Delete
// ============================================================

func TestClient_Post_WithBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "test data") {
			t.Error("body should contain 'test data'")
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer ts.Close()
	
	client := NewClient()
	resp, err := client.Post(context.Background(), ts.URL,
		WithBody(strings.NewReader("test data")),
	)
	if err != nil {
		t.Fatalf("Post() failed: %v", err)
	}
	defer resp.Close()
}

func TestClient_Put_WithBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "updated data") {
			t.Error("body should contain 'updated data'")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	
	client := NewClient()
	resp, err := client.Put(context.Background(), ts.URL,
		WithBody(strings.NewReader("updated data")),
	)
	if err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	defer resp.Close()
}

// ============================================================
// Complete testing for DoWithData
// ============================================================

type TestProduct struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Price float64 `json:"price"`
}

func TestDoWithData_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(TestProduct{ID: 1, Name: "Product A", Price: 99.99})
	}))
	defer ts.Close()
	
	client := NewClient()
	req := NewGetRequest(ts.URL)
	
	product, err := DoWithData[TestProduct](client, context.Background(), req)
	if err != nil {
		t.Fatalf("DoWithData() failed: %v", err)
	}
	
	if product.ID != 1 {
		t.Errorf("expected ID=1, got %d", product.ID)
	}
	if product.Name != "Product A" {
		t.Errorf("expected Name='Product A', got %s", product.Name)
	}
}

func TestDoWithData_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()
	
	client := NewClient()
	req := NewGetRequest(ts.URL)
	
	_, err := DoWithData[TestProduct](client, context.Background(), req)
	if err == nil {
		t.Error("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention 404, got: %v", err)
	}
}

func TestDoWithData_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer ts.Close()
	
	client := NewClient()
	req := NewGetRequest(ts.URL)
	
	_, err := DoWithData[TestProduct](client, context.Background(), req)
	if err == nil {
		t.Error("expected unmarshal error")
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("error should mention unmarshal, got: %v", err)
	}
}

// ============================================================
// Generic Put test
// ============================================================

func TestPut_Generic(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		
		var product TestProduct
		json.NewDecoder(r.Body).Decode(&product)
		
		// Return the updated product
		product.Price = 89.99 // Update price
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(product)
	}))
	defer ts.Close()
	
	client := NewClient()
	inputProduct := TestProduct{ID: 1, Name: "Product A", Price: 99.99}
	
	resultProduct, err := Put[TestProduct](client, context.Background(), ts.URL, inputProduct)
	if err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	
	if resultProduct.Price != 89.99 {
		t.Errorf("expected Price=89.99, got %f", resultProduct.Price)
	}
}

func TestPut_Generic_InvalidJSON(t *testing.T) {
	client := NewClient()
	
	// Using a non-serializable type
	invalidData := make(chan int)
	
	_, err := Put[TestProduct](client, context.Background(), "http://example.com", invalidData)
	if err == nil {
		t.Error("expected marshal error")
	}
	if !strings.Contains(err.Error(), "marshal") {
		t.Errorf("error should mention marshal, got: %v", err)
	}
}

func TestPost_Generic_InvalidJSON(t *testing.T) {
	client := NewClient()
	
	// Using an unserializable type
	invalidData := make(chan int)
	
	_, err := Post[TestProduct](client, context.Background(), "http://example.com", invalidData)
	if err == nil {
		t.Error("expected marshal error")
	}
}

// ============================================================
// Options comprehensive coverage test
// ============================================================

func TestWithTransport(t *testing.T) {
	customTransport := &http.Transport{
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,
	}
	
	client := NewClient(WithTransport(customTransport))
	if client.config.transport != customTransport {
		t.Error("custom transport not set")
	}
}

func TestWithCookieJar(t *testing.T) {
	jar, _ := cookiejar.New(nil)
	
	client := NewClient(WithCookieJar(jar))
	if client.config.cookieJar != jar {
		t.Error("cookie jar not set")
	}
}

func TestWithContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), "key", "value")
	
	cfg := newConfig()
	WithContext(ctx)(cfg)
	
	if cfg.ctx != ctx {
		t.Error("context not set")
	}
}

func TestWithQueries(t *testing.T) {
	queries := url.Values{}
	queries.Set("page", "1")
	queries.Set("limit", "20")
	
	cfg := newConfig()
	WithQueries(queries)(cfg)
	
	if cfg.queries.Get("page") != "1" {
		t.Error("page query not set")
	}
	if cfg.queries.Get("limit") != "20" {
		t.Error("limit query not set")
	}
}

func TestWithJSON(t *testing.T) {
	cfg := newConfig()
	data := map[string]string{"name": "test"}
	
	WithJSON(data)(cfg)
	// WithJSON is just a marker, actual serialization happens at runtime
	// Here only tests what won't cause a panic
}

func TestWithForm(t *testing.T) {
	cfg := newConfig()
	data := map[string]string{"username": "alice"}
	
	WithForm(data)(cfg)
	// WithForm is just a marker, actual encoding occurs during execution
}

func TestWithBody(t *testing.T) {
	cfg := newConfig()
	body := strings.NewReader("test data")
	
	WithBody(body)(cfg)
	
	if cfg.body == nil {
		t.Error("body not set")
	}
}

func TestWithBodyString(t *testing.T) {
	cfg := newConfig()
	
	WithBodyString("test string")(cfg)
	// WithBodyString is just a marker
}

// ============================================================
// boundary and error scenario testing
// ============================================================

func TestClient_Do_BeforeRequestError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	
	client := NewClient()
	req := NewGetRequest(ts.URL)
	
	_, err := client.Do(context.Background(), req,
		WithBeforeRequest(func(r *http.Request) error {
			return fmt.Errorf("before request error")
		}),
	)
	
	if err == nil {
		t.Error("expected before request error")
	}
	if !strings.Contains(err.Error(), "before request") {
		t.Errorf("error should mention 'before request', got: %v", err)
	}
}

func TestClient_Do_AfterResponseError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	
	client := NewClient()
	req := NewGetRequest(ts.URL)
	
	_, err := client.Do(context.Background(), req,
		WithAfterResponse(func(r *Response) error {
			return fmt.Errorf("after response error")
		}),
	)
	
	if err == nil {
		t.Error("expected after response error")
	}
	if !strings.Contains(err.Error(), "after response") {
		t.Errorf("error should mention 'after response', got: %v", err)
	}
}

func TestClient_Do_WithRetry_AllFailed(t *testing.T) {
	attempts := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()
	
	client := NewClient()
	req := NewGetRequest(ts.URL)
	
	_, err := client.Do(context.Background(), req,
		WithRetry(
			retry.MaxAttempts(3),
			retry.Backoff(retry.ConstantBackoff(10*time.Millisecond)),
		),
	)
	
	if err == nil {
		t.Error("expected error after all retries failed")
	}
	
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestClient_Do_WithRetry_429(t *testing.T) {
	attempts := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusTooManyRequests) // 429
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
	
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestWithHeader_NilMap(t *testing.T) {
	cfg := newConfig()
	cfg.headers = nil
	
	WithHeader("X-Test", "value")(cfg)
	
	if cfg.headers["X-Test"] != "value" {
		t.Error("header should be set even if map was nil")
	}
}

func TestWithQuery_NilMap(t *testing.T) {
	cfg := newConfig()
	cfg.queries = nil
	
	WithQuery("page", "1")(cfg)
	
	if cfg.queries.Get("page") != "1" {
		t.Error("query should be set even if map was nil")
	}
}

func TestWithHeaders_NilMap(t *testing.T) {
	cfg := newConfig()
	cfg.headers = nil
	
	headers := map[string]string{"X-Test": "value"}
	WithHeaders(headers)(cfg)
	
	if cfg.headers["X-Test"] != "value" {
		t.Error("headers should be set even if map was nil")
	}
}

func TestWithInsecureSkipVerify_ExistingTransport(t *testing.T) {
	cfg := newConfig()
	cfg.transport = &http.Transport{}
	
	WithInsecureSkipVerify()(cfg)
	
	if cfg.transport.TLSClientConfig == nil {
		t.Error("TLS config should be created")
	}
	if !cfg.transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be true")
	}
}

func TestWithInsecureSkipVerify_ExistingTLSConfig(t *testing.T) {
	cfg := newConfig()
	cfg.transport = &http.Transport{
		TLSClientConfig: &tls.Config{},
	}
	
	WithInsecureSkipVerify()(cfg)
	
	if !cfg.transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be true")
	}
}

func TestConfig_merge_ContextNil(t *testing.T) {
	base := newConfig()
	other := newConfig()
	other.ctx = nil
	
	merged := base.merge(other)
	if merged.ctx != nil {
		t.Error("context should remain nil")
	}
}

func TestConfig_merge_BodyNil(t *testing.T) {
	base := newConfig()
	other := newConfig()
	other.body = nil
	
	merged := base.merge(other)
	if merged.body != nil {
		t.Error("body should remain nil")
	}
}

func TestConfig_merge_TimeoutZero(t *testing.T) {
	base := newConfig()
	base.timeout = 10 * time.Second
	
	other := newConfig()
	other.timeout = 0
	
	merged := base.merge(other)
	if merged.timeout != 10*time.Second {
		t.Error("timeout should not be overridden by zero value")
	}
}

func TestConfig_merge_BeforeRequestNil(t *testing.T) {
	base := newConfig()
	base.beforeRequest = func(r *http.Request) error { return nil }
	
	other := newConfig()
	other.beforeRequest = nil
	
	merged := base.merge(other)
	if merged.beforeRequest == nil {
		t.Error("beforeRequest should be inherited")
	}
}

func TestConfig_merge_AfterResponseOverride(t *testing.T) {
	base := newConfig()
	base.afterResponse = func(r *Response) error { return fmt.Errorf("base") }
	
	other := newConfig()
	other.afterResponse = func(r *Response) error { return fmt.Errorf("override") }
	
	merged := base.merge(other)
	if merged.afterResponse == nil {
		t.Error("afterResponse should be set")
	}
	
	// Test actual invocation
	err := merged.afterResponse(&Response{})
	if err == nil || !strings.Contains(err.Error(), "override") {
		t.Error("afterResponse should be overridden")
	}
}

// ============================================================
// Request boundary test for buildHTTPRequest
// ============================================================

func TestRequest_buildHTTPRequest_InvalidURL(t *testing.T) {
	req := NewRequest("GET", "http://[invalid url")
	
	_, err := req.buildHTTPRequest()
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

// ============================================================
// Performance benchmark test supplement
// ============================================================

func BenchmarkClient_Do_NoRetry(b *testing.B) {
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

func BenchmarkClient_Do_WithRetry(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	
	client := NewClient(
		WithRetry(retry.MaxAttempts(3)),
	)
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

func BenchmarkDoWithData(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"id": 1, "name": "Product A", "price": 99.99}`)
	}))
	defer ts.Close()
	
	client := NewClient()
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := NewGetRequest(ts.URL)
		DoWithData[TestProduct](client, ctx, req)
	}
}

