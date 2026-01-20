package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// sliding window algorithm implementation
type slidingWindowAlgorithm struct{}

// Create new sliding window algorithm
func NewSlidingWindowAlgorithm() Algorithm {
	return &slidingWindowAlgorithm{}
}

// Name Returns the algorithm name
func (a *slidingWindowAlgorithm) Name() string {
	return string(AlgorithmSlidingWindow)
}

// Allow check if the request is permitted
func (a *slidingWindowAlgorithm) Allow(ctx context.Context, store Store, resource string, n int64, cfg ResourceConfig) (*Response, error) {
	if n <= 0 {
		n = 1
	}

	now := time.Now()
	key := a.windowKey(resource)

	// Delete data outside the window (delete data < windowStart)
	windowStart := now.Add(-cfg.WindowSize)
	minScore := float64(windowStart.UnixNano())
	// Delete data with scores less than minScore (excluding minScore itself)
	if err := store.ZRemRangeByScore(ctx, key, 0, minScore-1); err != nil {
		return nil, fmt.Errorf("remove old entries failed: %w", err)
	}

	// Count the number of requests within the current window (from windowStart to now, inclusive)
	maxScore := float64(now.UnixNano())
	count, err := store.ZCount(ctx, key, minScore, maxScore)
	if err != nil {
		return nil, fmt.Errorf("count requests failed: %w", err)
	}

	// Check if exceeded limit
	if count+n <= cfg.Limit {
		// Add new request
		for i := int64(0); i < n; i++ {
			// Slightly increase the score for each request to ensure uniqueness
			scoreTime := now.Add(time.Duration(i) * time.Nanosecond)
			score := float64(scoreTime.UnixNano())
			// Use UUID to ensure member uniqueness (avoid conflicts within the same nanosecond)
			member := uuid.New().String()
			if err := store.ZAdd(ctx, key, score, member); err != nil {
				return nil, fmt.Errorf("add request failed: %w", err)
			}
		}

		return &Response{
			Allowed:   true,
			Remaining: cfg.Limit - count - n,
			Limit:     cfg.Limit,
			ResetAt:   now.Add(cfg.WindowSize),
		}, nil
	}

	// Exceeding limit, calculating retry time
	retryAfter := cfg.WindowSize / time.Duration(cfg.Limit)

	return &Response{
		Allowed:    false,
		RetryAfter: retryAfter,
		Remaining:  maxInt64(0, cfg.Limit-count),
		Limit:      cfg.Limit,
		ResetAt:    now.Add(cfg.WindowSize),
	}, nil
}

// Wait for permission acquisition
func (a *slidingWindowAlgorithm) Wait(ctx context.Context, store Store, resource string, n int64, cfg ResourceConfig, timeout time.Duration) error {
	if n <= 0 {
		n = 1
	}

	deadline := time.Now().Add(timeout)

	for {
		resp, err := a.Allow(ctx, store, resource, n, cfg)
		if err != nil {
			return err
		}

		if resp.Allowed {
			return nil
		}

		if time.Now().After(deadline) {
			return ErrWaitTimeout
		}

		waitTime := min64Duration(resp.RetryAfter, time.Until(deadline))
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

// GetMetrics retrieves current metrics
func (a *slidingWindowAlgorithm) GetMetrics(ctx context.Context, store Store, resource string) (*AlgorithmMetrics, error) {
	now := time.Now()
	key := a.windowKey(resource)

	// Get the number of requests within the current window
	minScore := float64(now.Add(-1 * time.Second).UnixNano())
	maxScore := float64(now.UnixNano())

	count, err := store.ZCount(ctx, key, minScore, maxScore)
	if err != nil {
		return nil, fmt.Errorf("count requests failed: %w", err)
	}

	return &AlgorithmMetrics{
		Current:   count,
		Limit:     0,
		Remaining: 0,
		ResetAt:   now,
	}, nil
}

// Reset reset status
func (a *slidingWindowAlgorithm) Reset(ctx context.Context, store Store, resource string) error {
	key := a.windowKey(resource)
	return store.Del(ctx, key)
}

// windowKey returns the window storage key
func (a *slidingWindowAlgorithm) windowKey(resource string) string {
	return fmt.Sprintf("limiter:window:%s", resource)
}

