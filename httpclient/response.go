package httpclient

import (
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// Response HTTP 响应封装
type Response struct {
	StatusCode  int
	Status      string
	Headers     http.Header
	Body        []byte
	RawResponse *http.Response
	
	// 扩展字段
	Duration time.Duration // 请求总耗时
	Attempts int           // 重试次数
}

// IsSuccess 判断响应是否成功（2xx）
func (r *Response) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// IsClientError 判断是否客户端错误（4xx）
func (r *Response) IsClientError() bool {
	return r.StatusCode >= 400 && r.StatusCode < 500
}

// IsServerError 判断是否服务端错误（5xx）
func (r *Response) IsServerError() bool {
	return r.StatusCode >= 500 && r.StatusCode < 600
}

// JSON 反序列化 JSON 响应
func (r *Response) JSON(v interface{}) error {
	if v == nil {
		return nil
	}
	return json.Unmarshal(r.Body, v)
}

// String 返回响应 Body 字符串
func (r *Response) String() string {
	return string(r.Body)
}

// Bytes 返回响应 Body 字节数组
func (r *Response) Bytes() []byte {
	return r.Body
}

// Close 关闭响应（如果 RawResponse 存在）
func (r *Response) Close() error {
	if r.RawResponse != nil && r.RawResponse.Body != nil {
		return r.RawResponse.Body.Close()
	}
	return nil
}

// newResponse 从 http.Response 创建 Response
func newResponse(httpResp *http.Response, duration time.Duration, attempts int) (*Response, error) {
	if httpResp == nil {
		return nil, nil
	}
	
	// 读取 Body
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}
	
	resp := &Response{
		StatusCode:  httpResp.StatusCode,
		Status:      httpResp.Status,
		Headers:     httpResp.Header,
		Body:        body,
		RawResponse: httpResp,
		Duration:    duration,
		Attempts:    attempts,
	}
	
	return resp, nil
}

