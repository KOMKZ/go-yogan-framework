package testutil

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
)

// RequestBuilder HTTP 请求构建器
type RequestBuilder struct {
	method  string
	path    string
	body    interface{}
	headers map[string]string
	query   map[string]string
}

// NewRequest 创建请求构建器
func NewRequest(method, path string) *RequestBuilder {
	return &RequestBuilder{
		method:  method,
		path:    path,
		headers: make(map[string]string),
		query:   make(map[string]string),
	}
}

// WithJSON 设置 JSON Body
func (rb *RequestBuilder) WithJSON(body interface{}) *RequestBuilder {
	rb.body = body
	return rb
}

// WithHeader 设置 Header
func (rb *RequestBuilder) WithHeader(key, value string) *RequestBuilder {
	rb.headers[key] = value
	return rb
}

// WithQuery 设置 Query 参数
func (rb *RequestBuilder) WithQuery(key, value string) *RequestBuilder {
	rb.query[key] = value
	return rb
}

// WithTraceID 设置 TraceID
func (rb *RequestBuilder) WithTraceID(traceID string) *RequestBuilder {
	return rb.WithHeader("X-Trace-ID", traceID)
}

// Do 执行请求
func (rb *RequestBuilder) Do(engine *gin.Engine) *ResponseHelper {
	// 构建请求 URL
	url := rb.path
	if len(rb.query) > 0 {
		url += "?"
		first := true
		for k, v := range rb.query {
			if !first {
				url += "&"
			}
			url += k + "=" + v
			first = false
		}
	}

	// 构建请求 Body
	var bodyReader *bytes.Reader
	if rb.body != nil {
		bodyBytes, _ := json.Marshal(rb.body)
		bodyReader = bytes.NewReader(bodyBytes)
	} else {
		bodyReader = bytes.NewReader([]byte{})
	}

	// 创建请求
	req := httptest.NewRequest(rb.method, url, bodyReader)

	// 设置 Headers
	for k, v := range rb.headers {
		req.Header.Set(k, v)
	}

	// 默认 Content-Type
	if rb.body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// 执行请求
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	return &ResponseHelper{
		Recorder: w,
	}
}

// ResponseHelper 响应辅助工具
type ResponseHelper struct {
	Recorder *httptest.ResponseRecorder
}

// Status 获取状态码
func (rh *ResponseHelper) Status() int {
	return rh.Recorder.Code
}

// Body 获取响应 Body
func (rh *ResponseHelper) Body() string {
	return rh.Recorder.Body.String()
}

// JSON 解析 JSON 响应
func (rh *ResponseHelper) JSON(v interface{}) error {
	return json.Unmarshal(rh.Recorder.Body.Bytes(), v)
}

// Header 获取响应 Header
func (rh *ResponseHelper) Header(key string) string {
	return rh.Recorder.Header().Get(key)
}

// ============================================
// 便捷方法
// ============================================

// GET 创建 GET 请求
func GET(path string) *RequestBuilder {
	return NewRequest("GET", path)
}

// POST 创建 POST 请求
func POST(path string) *RequestBuilder {
	return NewRequest("POST", path)
}

// PUT 创建 PUT 请求
func PUT(path string) *RequestBuilder {
	return NewRequest("PUT", path)
}

// DELETE 创建 DELETE 请求
func DELETE(path string) *RequestBuilder {
	return NewRequest("DELETE", path)
}

// PATCH 创建 PATCH 请求
func PATCH(path string) *RequestBuilder {
	return NewRequest("PATCH", path)
}

