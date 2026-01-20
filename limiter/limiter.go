// Package limiter provides rate limiting functionality
//
// Design philosophy:
// - Standalone package, depends only on the logger component of yogan
// - Event-driven, the application layer can subscribe to all events
// - Metrics exposed, application layer can access real-time data
// - Optional enablement, does not take effect if not configured
// - Supports multiple algorithms: token bucket, sliding window, concurrent rate limiting, adaptive
// - Support multiple storages: memory, Redis
package limiter

import (
	"context"
	"time"
)

// Limiter core interface
type Limiter interface {
	// Allow check if the request is permitted (quick check)
	Allow(ctx context.Context, resource string) (bool, error)

	// AllowN checks if N requests are permitted
	AllowN(ctx context.Context, resource string, n int64) (bool, error)

	// Wait for permission acquisition (blocking wait, supports timeout)
	Wait(ctx context.Context, resource string) error

	// Wait for N licenses to be available
	WaitN(ctx context.Context, resource string, n int64) error

	// GetMetrics获取指标快照
	GetMetrics(resource string) *MetricsSnapshot

	// GetEventBus Obtain the event bus (for subscribing to events)
	GetEventBus() EventBus

	// Reset rate limiter state
	Reset(resource string)

	// Close the rate limiter (clean up resources)
	Close() error

	// Check if the rate limiter is enabled
	IsEnabled() bool
}

// Rate limiting response
type Response struct {
	// Allowed 是否允许: Is allowed
	Allowed bool

	// RetryAfter suggests retry time (valid when Allowed=false)
	RetryAfter time.Duration

	// Remaining quota (token bucket/sliding window)
	Remaining int64

	// Limit total quota
	Limit int64

	// ResetTime quota reset time
	ResetAt time.Time
}

