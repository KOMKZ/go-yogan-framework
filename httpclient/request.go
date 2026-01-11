package httpclient

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Request HTTP 请求封装
type Request struct {
	Method  string
	URL     string
	Headers map[string]string
	Query   url.Values
	Body    io.Reader
	
	// 内部字段
	bodyBytes []byte // 缓存 Body（用于重试）
}

// NewRequest 创建新的 Request
func NewRequest(method, urlStr string) *Request {
	return &Request{
		Method:  method,
		URL:     urlStr,
		Headers: make(map[string]string),
		Query:   make(url.Values),
	}
}

// NewGetRequest 创建 GET Request
func NewGetRequest(urlStr string) *Request {
	return NewRequest(http.MethodGet, urlStr)
}

// NewPostRequest 创建 POST Request
func NewPostRequest(urlStr string) *Request {
	return NewRequest(http.MethodPost, urlStr)
}

// NewPutRequest 创建 PUT Request
func NewPutRequest(urlStr string) *Request {
	return NewRequest(http.MethodPut, urlStr)
}

// NewDeleteRequest 创建 DELETE Request
func NewDeleteRequest(urlStr string) *Request {
	return NewRequest(http.MethodDelete, urlStr)
}

// WithHeader 设置 Header
func (r *Request) WithHeader(key, value string) *Request {
	r.Headers[key] = value
	return r
}

// WithQuery 设置 Query 参数
func (r *Request) WithQuery(key, value string) *Request {
	r.Query.Set(key, value)
	return r
}

// WithBody 设置 Body
func (r *Request) WithBody(body io.Reader) *Request {
	r.Body = body
	// 尝试读取并缓存 Body（用于重试）
	if body != nil {
		if data, err := io.ReadAll(body); err == nil {
			r.bodyBytes = data
			r.Body = bytes.NewReader(data)
		}
	}
	return r
}

// WithJSON 设置 JSON Body
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

// WithForm 设置 Form Body
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

// buildHTTPRequest 构建 http.Request
func (r *Request) buildHTTPRequest() (*http.Request, error) {
	// 构建 URL（包含 Query）
	fullURL := r.URL
	if len(r.Query) > 0 {
		if strings.Contains(fullURL, "?") {
			fullURL += "&" + r.Query.Encode()
		} else {
			fullURL += "?" + r.Query.Encode()
		}
	}
	
	// 重置 Body（用于重试）
	var body io.Reader
	if len(r.bodyBytes) > 0 {
		body = bytes.NewReader(r.bodyBytes)
	} else if r.Body != nil {
		body = r.Body
	}
	
	// 创建 http.Request
	req, err := http.NewRequest(r.Method, fullURL, body)
	if err != nil {
		return nil, err
	}
	
	// 设置 Headers
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}
	
	return req, nil
}

// Clone 克隆 Request（用于重试）
func (r *Request) Clone() *Request {
	clone := &Request{
		Method:    r.Method,
		URL:       r.URL,
		Headers:   make(map[string]string),
		Query:     make(url.Values),
		bodyBytes: r.bodyBytes,
	}
	
	// 复制 Headers
	for k, v := range r.Headers {
		clone.Headers[k] = v
	}
	
	// 复制 Query
	for k, vs := range r.Query {
		for _, v := range vs {
			clone.Query.Add(k, v)
		}
	}
	
	// 重置 Body
	if len(r.bodyBytes) > 0 {
		clone.Body = bytes.NewReader(r.bodyBytes)
	}
	
	return clone
}

