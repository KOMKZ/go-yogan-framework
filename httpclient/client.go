package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	
	"github.com/KOMKZ/go-yogan-framework/retry"
)

// Client HTTP client
type Client struct {
	httpClient *http.Client
	config     *config
}

// Create new HTTP client
func NewClient(opts ...Option) *Client {
	cfg := newConfig()
	applyOptions(cfg, opts)
	
	// Set default transport
	if cfg.transport == nil {
		cfg.transport = http.DefaultTransport.(*http.Transport).Clone()
	}
	
	// Create HTTP client
	httpClient := &http.Client{
		Timeout:   cfg.timeout,
		Transport: cfg.transport,
		Jar:       cfg.cookieJar,
	}
	
	return &Client{
		httpClient: httpClient,
		config:     cfg,
	}
}

// Perform HTTP request
func (c *Client) Do(ctx context.Context, req *Request, opts ...Option) (*Response, error) {
	// Merge configuration
	reqCfg := newConfig()
	applyOptions(reqCfg, opts)
	finalCfg := c.config.merge(reqCfg)
	
	// Set Context
	if ctx == nil {
		ctx = context.Background()
	}
	if finalCfg.ctx != nil {
		ctx = finalCfg.ctx
	}
	
	// Build URL (concatenate baseURL)
	fullURL := req.URL
	if finalCfg.baseURL != "" && !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
		fullURL = strings.TrimRight(finalCfg.baseURL, "/") + "/" + strings.TrimLeft(req.URL, "/")
	}
	req.URL = fullURL
	
	// Merge query parameters
	for k, vs := range finalCfg.queries {
		for _, v := range vs {
			req.Query.Add(k, v)
		}
	}
	
	// Merge Headers
	for k, v := range finalCfg.headers {
		if _, exists := req.Headers[k]; !exists {
			req.Headers[k] = v
		}
	}
	
	// Execute request (with circuit breaker + retry)
	var resp *Response
	var err error
	startTime := time.Now()
	attempts := 1
	
	// determine if circuit breaker is used
	useBreaker := finalCfg.breakerManager != nil && 
		!finalCfg.breakerDisabled && 
		finalCfg.breakerManager.IsEnabled()
	
	if finalCfg.retryEnabled && len(finalCfg.retryOpts) > 0 {
		// Use retry
		err = retry.Do(ctx, func() error {
			if useBreaker {
				// Use circuit breaker for protection
				resp, err = c.executeWithBreaker(ctx, req, finalCfg)
			} else {
				// Execute directly
				resp, err = c.doRequest(ctx, req, finalCfg)
			}
			
			if err != nil {
				return err
			}
			
			// Check if the HTTP status code requires a retry
			if resp.IsServerError() || resp.StatusCode == 429 {
				return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
			}
			
			return nil
		}, finalCfg.retryOpts...)
		
		// Record the number of retry attempts (extracted from the error)
		if multiErr, ok := err.(*retry.MultiError); ok {
			attempts = len(multiErr.Errors) + 1
		}
	} else {
		// Do not use retries
		if useBreaker {
			// Use circuit breaker for protection
			resp, err = c.executeWithBreaker(ctx, req, finalCfg)
		} else {
			// Execute directly
			resp, err = c.doRequest(ctx, req, finalCfg)
		}
	}
	
	if err != nil {
		return nil, err
	}
	
	// Set extended fields
	resp.Duration = time.Since(startTime)
	resp.Attempts = attempts
	
	// Execute response post-hook
	if finalCfg.afterResponse != nil {
		if err := finalCfg.afterResponse(resp); err != nil {
			return resp, err
		}
	}
	
	return resp, nil
}

// execute single HTTP request (internal method)
func (c *Client) doRequest(ctx context.Context, req *Request, cfg *config) (*Response, error) {
	// Build http.Request
	httpReq, err := req.buildHTTPRequest()
	if err != nil {
		return nil, fmt.Errorf("build http request failed: %w", err)
	}
	
	// Set context (with timeout)
	if cfg.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.timeout)
		defer cancel()
	}
	httpReq = httpReq.WithContext(ctx)
	
	// Execute request pre-hook
	if cfg.beforeRequest != nil {
		if err := cfg.beforeRequest(httpReq); err != nil {
			return nil, fmt.Errorf("before request hook failed: %w", err)
		}
	}
	
	// Execute HTTP request
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	
	// Build Response
	resp, err := newResponse(httpResp, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("build response failed: %w", err)
	}
	
	return resp, nil
}

// Send GET request
func (c *Client) Get(ctx context.Context, url string, opts ...Option) (*Response, error) {
	req := NewGetRequest(url)
	return c.Do(ctx, req, opts...)
}

// Execute POST request
func (c *Client) Post(ctx context.Context, url string, opts ...Option) (*Response, error) {
	req := NewPostRequest(url)
	
	// Parse the Body configuration in opts
	reqCfg := newConfig()
	applyOptions(reqCfg, opts)
	
	// Apply Body configuration to Request
	if reqCfg.body != nil {
		req.WithBody(reqCfg.body)
	}
	
	return c.Do(ctx, req, opts...)
}

// Execute PUT request
func (c *Client) Put(ctx context.Context, url string, opts ...Option) (*Response, error) {
	req := NewPutRequest(url)
	
	// Parse the Body configuration in opts
	reqCfg := newConfig()
	applyOptions(reqCfg, opts)
	
	// Apply Body configuration to Request
	if reqCfg.body != nil {
		req.WithBody(reqCfg.body)
	}
	
	return c.Do(ctx, req, opts...)
}

// Delete Execute DELETE request
func (c *Client) Delete(ctx context.Context, url string, opts ...Option) (*Response, error) {
	req := NewDeleteRequest(url)
	return c.Do(ctx, req, opts...)
}

// ============================================================
// Generic method (auto deserialization)
// ============================================================

// DoWithData executes the request and automatically deserializes (generic)
func DoWithData[T any](client *Client, ctx context.Context, req *Request, opts ...Option) (*T, error) {
	resp, err := client.Do(ctx, req, opts...)
	if err != nil {
		return nil, err
	}
	defer resp.Close()
	
	if !resp.IsSuccess() {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}
	
	var result T
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w", err)
	}
	
	return &result, nil
}

// Get generic version
func Get[T any](client *Client, ctx context.Context, url string, opts ...Option) (*T, error) {
	req := NewGetRequest(url)
	return DoWithData[T](client, ctx, req, opts...)
}

// Post generic version
func Post[T any](client *Client, ctx context.Context, url string, data interface{}, opts ...Option) (*T, error) {
	req := NewPostRequest(url)
	
	// serialize data
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("marshal request data failed: %w", err)
		}
		req.WithBody(bytes.NewReader(jsonData))
		req.WithHeader("Content-Type", "application/json")
	}
	
	return DoWithData[T](client, ctx, req, opts...)
}

// Put generic version
func Put[T any](client *Client, ctx context.Context, url string, data interface{}, opts ...Option) (*T, error) {
	req := NewPutRequest(url)
	
	// serialize data
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("marshal request data failed: %w", err)
		}
		req.WithBody(bytes.NewReader(jsonData))
		req.WithHeader("Content-Type", "application/json")
	}
	
	return DoWithData[T](client, ctx, req, opts...)
}

