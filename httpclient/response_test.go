package httpclient

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// ============================================================
// Response 基础测试
// ============================================================

func TestResponse_IsSuccess(t *testing.T) {
	tests := []struct {
		statusCode int
		expected   bool
	}{
		{200, true},
		{201, true},
		{299, true},
		{300, false},
		{400, false},
		{500, false},
	}
	
	for _, tt := range tests {
		resp := &Response{StatusCode: tt.statusCode}
		if resp.IsSuccess() != tt.expected {
			t.Errorf("IsSuccess(%d) = %v, want %v", tt.statusCode, resp.IsSuccess(), tt.expected)
		}
	}
}

func TestResponse_IsClientError(t *testing.T) {
	tests := []struct {
		statusCode int
		expected   bool
	}{
		{400, true},
		{404, true},
		{499, true},
		{200, false},
		{500, false},
	}
	
	for _, tt := range tests {
		resp := &Response{StatusCode: tt.statusCode}
		if resp.IsClientError() != tt.expected {
			t.Errorf("IsClientError(%d) = %v, want %v", tt.statusCode, resp.IsClientError(), tt.expected)
		}
	}
}

func TestResponse_IsServerError(t *testing.T) {
	tests := []struct {
		statusCode int
		expected   bool
	}{
		{500, true},
		{503, true},
		{599, true},
		{200, false},
		{400, false},
	}
	
	for _, tt := range tests {
		resp := &Response{StatusCode: tt.statusCode}
		if resp.IsServerError() != tt.expected {
			t.Errorf("IsServerError(%d) = %v, want %v", tt.statusCode, resp.IsServerError(), tt.expected)
		}
	}
}

// ============================================================
// Response JSON 测试
// ============================================================

func TestResponse_JSON(t *testing.T) {
	jsonBody := `{"name": "Alice", "age": 30}`
	resp := &Response{
		Body: []byte(jsonBody),
	}
	
	var result map[string]interface{}
	err := resp.JSON(&result)
	if err != nil {
		t.Fatalf("JSON() failed: %v", err)
	}
	
	if result["name"] != "Alice" {
		t.Errorf("expected name=Alice, got %v", result["name"])
	}
	
	if result["age"].(float64) != 30 {
		t.Errorf("expected age=30, got %v", result["age"])
	}
}

func TestResponse_JSON_InvalidJSON(t *testing.T) {
	resp := &Response{
		Body: []byte("invalid json"),
	}
	
	var result map[string]interface{}
	err := resp.JSON(&result)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestResponse_JSON_Nil(t *testing.T) {
	resp := &Response{
		Body: []byte(`{"name": "Alice"}`),
	}
	
	err := resp.JSON(nil)
	if err != nil {
		t.Errorf("JSON(nil) should not error: %v", err)
	}
}

// ============================================================
// Response String/Bytes 测试
// ============================================================

func TestResponse_String(t *testing.T) {
	resp := &Response{
		Body: []byte("test response body"),
	}
	
	if resp.String() != "test response body" {
		t.Errorf("expected 'test response body', got %s", resp.String())
	}
}

func TestResponse_Bytes(t *testing.T) {
	resp := &Response{
		Body: []byte("test response body"),
	}
	
	if string(resp.Bytes()) != "test response body" {
		t.Errorf("expected 'test response body', got %s", string(resp.Bytes()))
	}
}

// ============================================================
// Response Close 测试
// ============================================================

func TestResponse_Close_WithRawResponse(t *testing.T) {
	// 创建一个 mock http.Response
	httpResp := &http.Response{
		Body: io.NopCloser(strings.NewReader("test")),
	}
	
	resp := &Response{
		RawResponse: httpResp,
	}
	
	err := resp.Close()
	if err != nil {
		t.Errorf("Close() failed: %v", err)
	}
}

func TestResponse_Close_NoRawResponse(t *testing.T) {
	resp := &Response{}
	
	err := resp.Close()
	if err != nil {
		t.Errorf("Close() should not error when RawResponse is nil: %v", err)
	}
}

// ============================================================
// newResponse 测试
// ============================================================

func TestNewResponse(t *testing.T) {
	// 创建一个 mock http.Response
	bodyStr := "test response body"
	httpResp := &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(bodyStr)),
	}
	
	duration := 100 * time.Millisecond
	attempts := 2
	
	resp, err := newResponse(httpResp, duration, attempts)
	if err != nil {
		t.Fatalf("newResponse() failed: %v", err)
	}
	
	if resp.StatusCode != 200 {
		t.Errorf("expected StatusCode=200, got %d", resp.StatusCode)
	}
	
	if resp.Status != "200 OK" {
		t.Errorf("expected Status='200 OK', got %s", resp.Status)
	}
	
	if resp.Headers.Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type=application/json, got %s", resp.Headers.Get("Content-Type"))
	}
	
	if string(resp.Body) != bodyStr {
		t.Errorf("expected Body=%s, got %s", bodyStr, string(resp.Body))
	}
	
	if resp.Duration != duration {
		t.Errorf("expected Duration=%v, got %v", duration, resp.Duration)
	}
	
	if resp.Attempts != attempts {
		t.Errorf("expected Attempts=%d, got %d", attempts, resp.Attempts)
	}
	
	if resp.RawResponse != httpResp {
		t.Error("RawResponse should be set")
	}
}

func TestNewResponse_Nil(t *testing.T) {
	resp, err := newResponse(nil, 0, 0)
	if err != nil {
		t.Errorf("newResponse(nil) should not error: %v", err)
	}
	
	if resp != nil {
		t.Error("expected nil response")
	}
}

func TestNewResponse_ReadBodyError(t *testing.T) {
	// 创建一个会失败的 Reader
	httpResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(&errorReader{}),
	}
	
	_, err := newResponse(httpResp, 0, 0)
	if err == nil {
		t.Error("expected error when reading body fails")
	}
}

// ============================================================
// Helper: errorReader
// ============================================================

type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}

// ============================================================
// Benchmark
// ============================================================

func BenchmarkResponse_JSON(b *testing.B) {
	jsonBody := `{"name": "Alice", "age": 30, "email": "alice@example.com"}`
	resp := &Response{
		Body: []byte(jsonBody),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result map[string]interface{}
		resp.JSON(&result)
	}
}

func BenchmarkNewResponse(b *testing.B) {
	bodyStr := strings.Repeat("test data ", 100)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		httpResp := &http.Response{
			StatusCode: 200,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader([]byte(bodyStr))),
		}
		
		newResponse(httpResp, 100*time.Millisecond, 1)
	}
}

