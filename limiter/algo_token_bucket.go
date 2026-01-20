package limiter

import (
	"context"
	"fmt"
	"math"
	"time"
)

// tokenBucketAlgorithm implementation of the token bucket algorithm
type tokenBucketAlgorithm struct{}

// NewTokenBucketAlgorithm Creates the token bucket algorithm
func NewTokenBucketAlgorithm() Algorithm {
	return &tokenBucketAlgorithm{}
}

// Name Returns the algorithm name
func (a *tokenBucketAlgorithm) Name() string {
	return string(AlgorithmTokenBucket)
}

// Allow check if the request is permitted
func (a *tokenBucketAlgorithm) Allow(ctx context.Context, store Store, resource string, n int64, cfg ResourceConfig) (*Response, error) {
	if n <= 0 {
		n = 1
	}

	now := time.Now()

	// Generate storage key
	tokensKey := a.tokensKey(resource)
	lastRefillKey := a.lastRefillKey(resource)

	// Get current token count and last refill time
	tokens, err := store.GetInt64(ctx, tokensKey)
	if err != nil && err != ErrKeyNotFound {
		return nil, fmt.Errorf("get tokens failed: %w", err)
	}

	lastRefillNano, err2 := store.GetInt64(ctx, lastRefillKey)
	if err2 != nil && err2 != ErrKeyNotFound {
		return nil, fmt.Errorf("get last refill time failed: %w", err2)
	}

	// If it is the first visit (both keys do not exist), initialize
	if err == ErrKeyNotFound && err2 == ErrKeyNotFound {
		// Use configured InitTokens, if not set use Capacity
		tokens = cfg.InitTokens
		if cfg.InitTokens == 0 {
			tokens = cfg.Capacity // Default full bucket
		}
		lastRefillNano = now.UnixNano()

		// Initialize storage
		if err := store.SetInt64(ctx, tokensKey, tokens, 0); err != nil {
			return nil, fmt.Errorf("init tokens failed: %w", err)
		}
		if err := store.SetInt64(ctx, lastRefillKey, lastRefillNano, 0); err != nil {
			return nil, fmt.Errorf("init last refill failed: %w", err)
		}
	} else {
		// Calculate the number of new tokens
		lastRefill := time.Unix(0, lastRefillNano)
		elapsed := now.Sub(lastRefill)
		newTokens := int64(float64(cfg.Rate) * elapsed.Seconds())
		tokens = min(tokens+newTokens, cfg.Capacity)
	}

	// Check if there are enough tokens
	if tokens >= n {
		// deduct token
		tokens -= n

		// Update storage
		if err := store.SetInt64(ctx, tokensKey, tokens, 0); err != nil {
			return nil, fmt.Errorf("set tokens failed: %w", err)
		}
		if err := store.SetInt64(ctx, lastRefillKey, now.UnixNano(), 0); err != nil {
			return nil, fmt.Errorf("set last refill failed: %w", err)
		}

		return &Response{
			Allowed:   true,
			Remaining: tokens,
			Limit:     cfg.Capacity,
			ResetAt:   now.Add(time.Duration(float64(cfg.Capacity-tokens) / float64(cfg.Rate) * float64(time.Second))),
		}, nil
	}

	// Token insufficient, update lastRefill time (to avoid recalculation next time)
	if err := store.SetInt64(ctx, lastRefillKey, now.UnixNano(), 0); err != nil {
		return nil, fmt.Errorf("set last refill failed: %w", err)
	}

	// Calculate retry time
	tokensNeeded := n - tokens
	retryAfter := time.Duration(float64(tokensNeeded) / float64(cfg.Rate) * float64(time.Second))

	return &Response{
		Allowed:    false,
		RetryAfter: retryAfter,
		Remaining:  tokens,
		Limit:      cfg.Capacity,
		ResetAt:    now.Add(time.Duration(float64(cfg.Capacity-tokens) / float64(cfg.Rate) * float64(time.Second))),
	}, nil
}

// Wait for permission acquisition
func (a *tokenBucketAlgorithm) Wait(ctx context.Context, store Store, resource string, n int64, cfg ResourceConfig, timeout time.Duration) error {
	if n <= 0 {
		n = 1
	}

	deadline := time.Now().Add(timeout)

	for {
		// Check if timeout has occurred
		if time.Now().After(deadline) {
			return ErrWaitTimeout
		}

		// Try to get permission
		resp, err := a.Allow(ctx, store, resource, n, cfg)
		if err != nil {
			return err
		}

		if resp.Allowed {
			return nil
		}

		// Calculate the waiting time (take the smaller value of retry time and remaining time)
		remaining := time.Until(deadline)
		waitTime := resp.RetryAfter
		if waitTime > remaining {
			waitTime = remaining
		}

		// If the wait time is too short, return timeout directly
		if waitTime <= 0 {
			return ErrWaitTimeout
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Continue retrying
		}
	}
}

// GetMetrics获取当前指标
func (a *tokenBucketAlgorithm) GetMetrics(ctx context.Context, store Store, resource string) (*AlgorithmMetrics, error) {
	tokensKey := a.tokensKey(resource)
	lastRefillKey := a.lastRefillKey(resource)

	// Get current token count
	tokens, err := store.GetInt64(ctx, tokensKey)
	if err != nil && err != ErrKeyNotFound {
		return nil, fmt.Errorf("get tokens failed: %w", err)
	}

	if err == ErrKeyNotFound {
		tokens = 0
	}

	// Get last fill time
	lastRefillNano, err := store.GetInt64(ctx, lastRefillKey)
	if err != nil && err != ErrKeyNotFound {
		return nil, fmt.Errorf("get last refill failed: %w", err)
	}

	var resetAt time.Time
	if err != ErrKeyNotFound {
		resetAt = time.Unix(0, lastRefillNano)
	}

	return &AlgorithmMetrics{
		Current:   tokens,
		Limit:     0, // The token bucket does not have a clear limit concept
		Remaining: tokens,
		ResetAt:   resetAt,
	}, nil
}

// Reset reset status
func (a *tokenBucketAlgorithm) Reset(ctx context.Context, store Store, resource string) error {
	tokensKey := a.tokensKey(resource)
	lastRefillKey := a.lastRefillKey(resource)

	return store.Del(ctx, tokensKey, lastRefillKey)
}

// tokensKey returns the token storage key
func (a *tokenBucketAlgorithm) tokensKey(resource string) string {
	return fmt.Sprintf("limiter:token:%s:tokens", resource)
}

// lastRefillKey returns the storage key for the last refill time
func (a *tokenBucketAlgorithm) lastRefillKey(resource string) string {
	return fmt.Sprintf("limiter:token:%s:last_refill", resource)
}

// min returns the smaller of two int64 values
func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// min64Duration returns the minimum value of time.Duration
func min64Duration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// returns the maximum int64 value
func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// returns the minimum float64 value
func minFloat64(a, b float64) float64 {
	return math.Min(a, b)
}

// returns the maximum float64 value
func maxFloat64(a, b float64) float64 {
	return math.Max(a, b)
}

