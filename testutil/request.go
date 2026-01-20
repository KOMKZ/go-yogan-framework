package testutil

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
)

// RequestBuilder HTTP request builder
type RequestBuilder struct {
	method  string
	path    string
	body    interface{}
	headers map[string]string
	query   map[string]string
}

// NewRequest creates request builder
func NewRequest(method, path string) *RequestBuilder {
	return &RequestBuilder{
		method:  method,
		path:    path,
		headers: make(map[string]string),
		query:   make(map[string]string),
	}
}

// WithJSON sets the JSON Body
func (rb *RequestBuilder) WithJSON(body interface{}) *RequestBuilder {
	rb.body = body
	return rb
}

// WithHeader set Header
func (rb *RequestBuilder) WithHeader(key, value string) *RequestBuilder {
	rb.headers[key] = value
	return rb
}

// WithQuery sets the Query parameters
func (rb *RequestBuilder) WithQuery(key, value string) *RequestBuilder {
	rb.query[key] = value
	return rb
}

// Set TraceID
func (rb *RequestBuilder) WithTraceID(traceID string) *RequestBuilder {
	return rb.WithHeader("X-Trace-ID", traceID)
}

// Execute request
func (rb *RequestBuilder) Do(engine *gin.Engine) *ResponseHelper {
	// Build request URL
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

	// Build request body
	var bodyReader *bytes.Reader
	if rb.body != nil {
		bodyBytes, _ := json.Marshal(rb.body)
		bodyReader = bytes.NewReader(bodyBytes)
	} else {
		bodyReader = bytes.NewReader([]byte{})
	}

	// Create request
	req := httptest.NewRequest(rb.method, url, bodyReader)

	// Set Headers
	for k, v := range rb.headers {
		req.Header.Set(k, v)
	}

	// Default Content-Type
	if rb.body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Execute request
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	return &ResponseHelper{
		Recorder: w,
	}
}

// ResponseHelper Response utility
type ResponseHelper struct {
	Recorder *httptest.ResponseRecorder
}

// Get status code
func (rh *ResponseHelper) Status() int {
	return rh.Recorder.Code
}

// Body Retrieve response body
func (rh *ResponseHelper) Body() string {
	return rh.Recorder.Body.String()
}

// Parse the JSON response
func (rh *ResponseHelper) JSON(v interface{}) error {
	return json.Unmarshal(rh.Recorder.Body.Bytes(), v)
}

// Get response headers
func (rh *ResponseHelper) Header(key string) string {
	return rh.Recorder.Header().Get(key)
}

// ============================================
// convenience method
// ============================================

// Create GET request
func GET(path string) *RequestBuilder {
	return NewRequest("GET", path)
}

// Create POST request
func POST(path string) *RequestBuilder {
	return NewRequest("POST", path)
}

// English: PUT create PUT request
func PUT(path string) *RequestBuilder {
	return NewRequest("PUT", path)
}

// DELETE create DELETE request
func DELETE(path string) *RequestBuilder {
	return NewRequest("DELETE", path)
}

// Create a PATCH request
func PATCH(path string) *RequestBuilder {
	return NewRequest("PATCH", path)
}

