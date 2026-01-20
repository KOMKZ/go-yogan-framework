package httpclient

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// HTTP request encapsulation
type Request struct {
	Method  string
	URL     string
	Headers map[string]string
	Query   url.Values
	Body    io.Reader
	
	// Internal field
	bodyBytes []byte // Cache Body (for retry)
}

// Create new Request
func NewRequest(method, urlStr string) *Request {
	return &Request{
		Method:  method,
		URL:     urlStr,
		Headers: make(map[string]string),
		Query:   make(url.Values),
	}
}

// Create GET Request
func NewGetRequest(urlStr string) *Request {
	return NewRequest(http.MethodGet, urlStr)
}

// Create POST Request
func NewPostRequest(urlStr string) *Request {
	return NewRequest(http.MethodPost, urlStr)
}

// Create PUT Request
func NewPutRequest(urlStr string) *Request {
	return NewRequest(http.MethodPut, urlStr)
}

// Create DELETE Request
func NewDeleteRequest(urlStr string) *Request {
	return NewRequest(http.MethodDelete, urlStr)
}

// Set Header
func (r *Request) WithHeader(key, value string) *Request {
	r.Headers[key] = value
	return r
}

// WithQuery sets the Query parameters
func (r *Request) WithQuery(key, value string) *Request {
	r.Query.Set(key, value)
	return r
}

// WithBody sets the Body
func (r *Request) WithBody(body io.Reader) *Request {
	r.Body = body
	// Try to read and cache Body (for retry)
	if body != nil {
		if data, err := io.ReadAll(body); err == nil {
			r.bodyBytes = data
			r.Body = bytes.NewReader(data)
		}
	}
	return r
}

// WithJSON sets the JSON Body
func (r *Request) WithJSON(data interface{}) *Request {
	if data == nil {
		return r
	}
	
	jsonData, err := json.Marshal(data)
	if err != nil {
		return r
	}
	
	r.bodyBytes = jsonData
	r.Body = bytes.NewReader(jsonData)
	r.Headers["Content-Type"] = "application/json"
	
	return r
}

// WithForm sets Form Body
func (r *Request) WithForm(data map[string]string) *Request {
	if data == nil {
		return r
	}
	
	formData := make(url.Values)
	for k, v := range data {
		formData.Set(k, v)
	}
	
	formStr := formData.Encode()
	r.bodyBytes = []byte(formStr)
	r.Body = strings.NewReader(formStr)
	r.Headers["Content-Type"] = "application/x-www-form-urlencoded"
	
	return r
}

// buildHTTPRequest builds http.Request
func (r *Request) buildHTTPRequest() (*http.Request, error) {
	// Build URL (including query)
	fullURL := r.URL
	if len(r.Query) > 0 {
		if strings.Contains(fullURL, "?") {
			fullURL += "&" + r.Query.Encode()
		} else {
			fullURL += "?" + r.Query.Encode()
		}
	}
	
	// Reset Body (for retry)
	var body io.Reader
	if len(r.bodyBytes) > 0 {
		body = bytes.NewReader(r.bodyBytes)
	} else if r.Body != nil {
		body = r.Body
	}
	
	// Create http.Request
	req, err := http.NewRequest(r.Method, fullURL, body)
	if err != nil {
		return nil, err
	}
	
	// Set Headers
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}
	
	return req, nil
}

// Clone request (for retry)
func (r *Request) Clone() *Request {
	clone := &Request{
		Method:    r.Method,
		URL:       r.URL,
		Headers:   make(map[string]string),
		Query:     make(url.Values),
		bodyBytes: r.bodyBytes,
	}
	
	// Copy Headers
	for k, v := range r.Headers {
		clone.Headers[k] = v
	}
	
	// Copy Query
	for k, vs := range r.Query {
		for _, v := range vs {
			clone.Query.Add(k, v)
		}
	}
	
	// Reset Body
	if len(r.bodyBytes) > 0 {
		clone.Body = bytes.NewReader(r.bodyBytes)
	}
	
	return clone
}

