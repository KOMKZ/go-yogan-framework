package limiter

import (
	"context"
	"time"
)

// Rate limiting algorithm interface (strategy pattern)
type Algorithm interface {
	// Allow check if the request is permitted
	Allow(ctx context.Context, store Store, resource string, n int64, cfg ResourceConfig) (*Response, error)

	// Wait for permission (blocking until acquired or timed out)
	Wait(ctx context.Context, store Store, resource string, n int64, cfg ResourceConfig, timeout time.Duration) error

	// GetMetrics retrieves current metrics
	GetMetrics(ctx context.Context, store Store, resource string) (*AlgorithmMetrics, error)

	// Reset reset status
	Reset(ctx context.Context, store Store, resource string) error

	// Name Returns algorithm name
	Name() string
}

// AlgorithmMetrics algorithm metrics
type AlgorithmMetrics struct {
	Current   int64     // Current value (concurrency count/token usage/request count)
	Limit     int64     // Limit value
	Remaining int64     // remaining quota
	ResetAt   time.Time // Reset time
}

// AlgorithmType algorithm type
type AlgorithmType string

const (
	// AlgorithmTokenBucket token bucket algorithm
	AlgorithmTokenBucket AlgorithmType = "token_bucket"

	// AlgorithmSlidingWindow Sliding Window Algorithm
	AlgorithmSlidingWindow AlgorithmType = "sliding_window"

	// AlgorithmConcurrency concurrency rate limiting algorithm
	AlgorithmConcurrency AlgorithmType = "concurrency"

	// Algorithm Adaptive for rate limiting
	AlgorithmAdaptive AlgorithmType = "adaptive"
)

// GetAlgorithm obtains an algorithm instance according to the configuration
func GetAlgorithm(cfg ResourceConfig, provider AdaptiveProvider) Algorithm {
	switch AlgorithmType(cfg.Algorithm) {
	case AlgorithmTokenBucket:
		return NewTokenBucketAlgorithm()
	case AlgorithmSlidingWindow:
		return NewSlidingWindowAlgorithm()
	case AlgorithmConcurrency:
		return NewConcurrencyAlgorithm()
	case AlgorithmAdaptive:
		return NewAdaptiveAlgorithm(provider)
	default:
		// Use token bucket by default
		return NewTokenBucketAlgorithm()
	}
}
