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

// Client HTTP 客户端
type Client struct {
	httpClient *http.Client
	config     *config
}

// NewClient 创建新的 HTTP Client
func NewClient(opts ...Option) *Client {
	cfg := newConfig()
	applyOptions(cfg, opts)
	
	// 设置默认 Transport
	if cfg.transport == nil {
		cfg.transport = http.DefaultTransport.(*http.Transport).Clone()
	}
	
	// 创建 http.Client
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

// Do 执行 HTTP 请求
func (c *Client) Do(ctx context.Context, req *Request, opts ...Option) (*Response, error) {
	// 合并配置
	reqCfg := newConfig()
	applyOptions(reqCfg, opts)
	finalCfg := c.config.merge(reqCfg)
	
	// 设置 Context
	if ctx == nil {
		ctx = context.Background()
	}
	if finalCfg.ctx != nil {
		ctx = finalCfg.ctx
	}
	
	// 构建 URL（拼接 baseURL）
	fullURL := req.URL
	if finalCfg.baseURL != "" && !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
		fullURL = strings.TrimRight(finalCfg.baseURL, "/") + "/" + strings.TrimLeft(req.URL, "/")
	}
	req.URL = fullURL
	
	// 合并 Query 参数
	for k, vs := range finalCfg.queries {
		for _, v := range vs {
			req.Query.Add(k, v)
		}
	}
	
	// 合并 Headers
	for k, v := range finalCfg.headers {
		if _, exists := req.Headers[k]; !exists {
			req.Headers[k] = v
		}
	}
	
	// 执行请求（带熔断器 + 重试）
	var resp *Response
	var err error
	startTime := time.Now()
	attempts := 1
	
	// 判断是否使用熔断器
	useBreaker := finalCfg.breakerManager != nil && 
		!finalCfg.breakerDisabled && 
		finalCfg.breakerManager.IsEnabled()
	
	if finalCfg.retryEnabled && len(finalCfg.retryOpts) > 0 {
		// 使用重试
		err = retry.Do(ctx, func() error {
			if useBreaker {
				// 使用熔断器保护
				resp, err = c.executeWithBreaker(ctx, req, finalCfg)
			} else {
				// 直接执行
				resp, err = c.doRequest(ctx, req, finalCfg)
			}
			
			if err != nil {
				return err
			}
			
			// 检查 HTTP 状态码是否需要重试
			if resp.IsServerError() || resp.StatusCode == 429 {
				return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
			}
			
			return nil
		}, finalCfg.retryOpts...)
		
		// 记录重试次数（从 error 中提取）
		if multiErr, ok := err.(*retry.MultiError); ok {
			attempts = len(multiErr.Errors) + 1
		}
	} else {
		// 不使用重试
		if useBreaker {
			// 使用熔断器保护
			resp, err = c.executeWithBreaker(ctx, req, finalCfg)
		} else {
			// 直接执行
			resp, err = c.doRequest(ctx, req, finalCfg)
		}
	}
	
	if err != nil {
		return nil, err
	}
	
	// 设置扩展字段
	resp.Duration = time.Since(startTime)
	resp.Attempts = attempts
	
	// 执行响应后钩子
	if finalCfg.afterResponse != nil {
		if err := finalCfg.afterResponse(resp); err != nil {
			return resp, err
		}
	}
	
	return resp, nil
}

// doRequest 执行单次 HTTP 请求（内部方法）
func (c *Client) doRequest(ctx context.Context, req *Request, cfg *config) (*Response, error) {
	// 构建 http.Request
	httpReq, err := req.buildHTTPRequest()
	if err != nil {
		return nil, fmt.Errorf("build http request failed: %w", err)
	}
	
	// 设置 Context（带超时）
	if cfg.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.timeout)
		defer cancel()
	}
	httpReq = httpReq.WithContext(ctx)
	
	// 执行请求前钩子
	if cfg.beforeRequest != nil {
		if err := cfg.beforeRequest(httpReq); err != nil {
			return nil, fmt.Errorf("before request hook failed: %w", err)
		}
	}
	
	// 执行 HTTP 请求
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	
	// 构建 Response
	resp, err := newResponse(httpResp, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("build response failed: %w", err)
	}
	
	return resp, nil
}

// Get 执行 GET 请求
func (c *Client) Get(ctx context.Context, url string, opts ...Option) (*Response, error) {
	req := NewGetRequest(url)
	return c.Do(ctx, req, opts...)
}

// Post 执行 POST 请求
func (c *Client) Post(ctx context.Context, url string, opts ...Option) (*Response, error) {
	req := NewPostRequest(url)
	
	// 解析 opts 中的 Body 配置
	reqCfg := newConfig()
	applyOptions(reqCfg, opts)
	
	// 应用 Body 配置到 Request
	if reqCfg.body != nil {
		req.WithBody(reqCfg.body)
	}
	
	return c.Do(ctx, req, opts...)
}

// Put 执行 PUT 请求
func (c *Client) Put(ctx context.Context, url string, opts ...Option) (*Response, error) {
	req := NewPutRequest(url)
	
	// 解析 opts 中的 Body 配置
	reqCfg := newConfig()
	applyOptions(reqCfg, opts)
	
	// 应用 Body 配置到 Request
	if reqCfg.body != nil {
		req.WithBody(reqCfg.body)
	}
	
	return c.Do(ctx, req, opts...)
}

// Delete 执行 DELETE 请求
func (c *Client) Delete(ctx context.Context, url string, opts ...Option) (*Response, error) {
	req := NewDeleteRequest(url)
	return c.Do(ctx, req, opts...)
}

// ============================================================
// 泛型方法（自动反序列化）
// ============================================================

// DoWithData 执行请求并自动反序列化（泛型）
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

// Get 泛型版本
func Get[T any](client *Client, ctx context.Context, url string, opts ...Option) (*T, error) {
	req := NewGetRequest(url)
	return DoWithData[T](client, ctx, req, opts...)
}

// Post 泛型版本
func Post[T any](client *Client, ctx context.Context, url string, data interface{}, opts ...Option) (*T, error) {
	req := NewPostRequest(url)
	
	// 序列化 data
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

// Put 泛型版本
func Put[T any](client *Client, ctx context.Context, url string, data interface{}, opts ...Option) (*T, error) {
	req := NewPutRequest(url)
	
	// 序列化 data
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

