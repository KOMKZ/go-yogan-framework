package httpclient

import (
	"context"
	"fmt"
	
	"github.com/KOMKZ/go-yogan-framework/breaker"
)

// BreakerManager circuit breaker management interface (for decoupling)
type BreakerManager interface {
	// Execute the protected operation
	Execute(ctx context.Context, req *breaker.Request) (interface{}, error)
	
	// Check if the circuit breaker is enabled
	IsEnabled() bool
	
	// GetState Retrieve the current state of the resource
	GetState(resource string) breaker.State
}

// Set circuit breaker manager
func WithBreaker(manager BreakerManager) Option {
	return func(c *config) {
		c.breakerManager = manager
	}
}

// WithBreakerResource sets the circuit breaker resource name (defaults to URL)
func WithBreakerResource(resource string) Option {
	return func(c *config) {
		c.breakerResource = resource
	}
}

// WithBreakerFallback set circuit breaker fallback logic
func WithBreakerFallback(fallback func(ctx context.Context, err error) (*Response, error)) Option {
	return func(c *config) {
		c.breakerFallback = fallback
	}
}

// Disable breaker (per-request level)
func DisableBreaker() Option {
	return func(c *config) {
		c.breakerDisabled = true
	}
}

// executeWithBreaker executes an HTTP request with circuit breaker protection
func (c *Client) executeWithBreaker(ctx context.Context, req *Request, cfg *config) (*Response, error) {
	// Check if the circuit breaker is disabled
	if cfg.breakerDisabled || cfg.breakerManager == nil || !cfg.breakerManager.IsEnabled() {
		// Circuit breaker not enabled, execute directly
		return c.doRequest(ctx, req, cfg)
	}
	
	// Determine resource name
	resource := cfg.breakerResource
	if resource == "" {
		// Use the URL as the resource name by default
		resource = req.URL
		// If it is a relative path and there is a baseURL, use the full URL
		if cfg.baseURL != "" {
			resource = cfg.baseURL + req.URL
		}
	}
	
	// Build circuit breaker request
	breakerReq := &breaker.Request{
		Resource: resource,
		Execute: func(ctx context.Context) (interface{}, error) {
			// Execute the actual HTTP request
			resp, err := c.doRequest(ctx, req, cfg)
			if err != nil {
				return nil, err
			}
			
			// Check the HTTP status code; 5xx errors should trigger circuit breaking
			if resp.IsServerError() {
				return resp, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
			}
			
			return resp, nil
		},
	}
	
	// Set fallback logic
	if cfg.breakerFallback != nil {
		breakerReq.Fallback = func(ctx context.Context, err error) (interface{}, error) {
			return cfg.breakerFallback(ctx, err)
		}
	}
	
	// Execute circuit breaker protection request
	result, err := cfg.breakerManager.Execute(ctx, breakerReq)
	if err != nil {
		// If it is a circuit breaker error and there is a fallback, the error has already been handled in the breaker.
		return nil, err
	}
	
	// Type assertion returns result
	resp, ok := result.(*Response)
	if !ok {
		return nil, fmt.Errorf("invalid response type from breaker")
	}
	
	return resp, nil
}

