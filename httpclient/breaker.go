package httpclient

import (
	"context"
	"fmt"
	
	"github.com/KOMKZ/go-yogan-framework/breaker"
)

// BreakerManager 熔断器管理器接口（用于解耦）
type BreakerManager interface {
	// Execute 执行受保护的操作
	Execute(ctx context.Context, req *breaker.Request) (interface{}, error)
	
	// IsEnabled 检查熔断器是否启用
	IsEnabled() bool
	
	// GetState 获取资源的当前状态
	GetState(resource string) breaker.State
}

// WithBreaker 设置熔断器管理器
func WithBreaker(manager BreakerManager) Option {
	return func(c *config) {
		c.breakerManager = manager
	}
}

// WithBreakerResource 设置熔断器资源名称（默认使用 URL）
func WithBreakerResource(resource string) Option {
	return func(c *config) {
		c.breakerResource = resource
	}
}

// WithBreakerFallback 设置熔断降级逻辑
func WithBreakerFallback(fallback func(ctx context.Context, err error) (*Response, error)) Option {
	return func(c *config) {
		c.breakerFallback = fallback
	}
}

// DisableBreaker 禁用熔断器（单次请求级别）
func DisableBreaker() Option {
	return func(c *config) {
		c.breakerDisabled = true
	}
}

// executeWithBreaker 执行带熔断保护的 HTTP 请求
func (c *Client) executeWithBreaker(ctx context.Context, req *Request, cfg *config) (*Response, error) {
	// 检查是否禁用熔断器
	if cfg.breakerDisabled || cfg.breakerManager == nil || !cfg.breakerManager.IsEnabled() {
		// 熔断器未启用，直接执行
		return c.doRequest(ctx, req, cfg)
	}
	
	// 确定资源名称
	resource := cfg.breakerResource
	if resource == "" {
		// 默认使用 URL 作为资源名称
		resource = req.URL
		// 如果是相对路径且有 baseURL，使用完整 URL
		if cfg.baseURL != "" {
			resource = cfg.baseURL + req.URL
		}
	}
	
	// 构建熔断器请求
	breakerReq := &breaker.Request{
		Resource: resource,
		Execute: func(ctx context.Context) (interface{}, error) {
			// 执行实际的 HTTP 请求
			resp, err := c.doRequest(ctx, req, cfg)
			if err != nil {
				return nil, err
			}
			
			// 检查 HTTP 状态码，5xx 错误应该触发熔断
			if resp.IsServerError() {
				return resp, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
			}
			
			return resp, nil
		},
	}
	
	// 设置降级逻辑
	if cfg.breakerFallback != nil {
		breakerReq.Fallback = func(ctx context.Context, err error) (interface{}, error) {
			return cfg.breakerFallback(ctx, err)
		}
	}
	
	// 执行熔断保护的请求
	result, err := cfg.breakerManager.Execute(ctx, breakerReq)
	if err != nil {
		// 如果是熔断错误且有降级，错误已在 breaker 中处理
		return nil, err
	}
	
	// 类型断言返回结果
	resp, ok := result.(*Response)
	if !ok {
		return nil, fmt.Errorf("invalid response type from breaker")
	}
	
	return resp, nil
}

