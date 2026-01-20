package httpclient

import (
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// Encapsulate HTTP response
type Response struct {
	StatusCode  int
	Status      string
	Headers     http.Header
	Body        []byte
	RawResponse *http.Response
	
	// Extend fields
	Duration time.Duration // Total request duration
	Attempts int           // Number of retries
}

// Checks if the response is successful (2xx)
func (r *Response) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// determines if it is a client error (4xx)
func (r *Response) IsClientError() bool {
	return r.StatusCode >= 400 && r.StatusCode < 500
}

// Check if it is a server error (5xx)
func (r *Response) IsServerError() bool {
	return r.StatusCode >= 500 && r.StatusCode < 600
}

// JSON deserialization of JSON response
func (r *Response) JSON(v interface{}) error {
	if v == nil {
		return nil
	}
	return json.Unmarshal(r.Body, v)
}

// String return response Body string
func (r *Response) String() string {
	return string(r.Body)
}

// Returns response body as byte array
func (r *Response) Bytes() []byte {
	return r.Body
}

// Close the response (if RawResponse exists)
func (r *Response) Close() error {
	if r.RawResponse != nil && r.RawResponse.Body != nil {
		return r.RawResponse.Body.Close()
	}
	return nil
}

// newResponse is created from http.Response
func newResponse(httpResp *http.Response, duration time.Duration, attempts int) (*Response, error) {
	if httpResp == nil {
		return nil, nil
	}
	
	// Read Body
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

