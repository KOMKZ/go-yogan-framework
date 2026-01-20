package limiter

import (
	"context"
	"fmt"
	"time"
)

// concurrencyAlgorithm concurrent rate limiting algorithm implementation
type concurrencyAlgorithm struct{}

// NewConcurrencyAlgorithm creates concurrency rate limiting algorithm
func NewConcurrencyAlgorithm() Algorithm {
	return &concurrencyAlgorithm{}
}

// Name Returns algorithm name
func (a *concurrencyAlgorithm) Name() string {
	return string(AlgorithmConcurrency)
}

// Allow check if the request is permitted
func (a *concurrencyAlgorithm) Allow(ctx context.Context, store Store, resource string, n int64, cfg ResourceConfig) (*Response, error) {
	if n <= 0 {
		n = 1
	}

	key := a.concurrencyKey(resource)

	// Get current concurrency count
	current, err := store.GetInt64(ctx, key)
	if err != nil && err != ErrKeyNotFound {
		return nil, fmt.Errorf("get current concurrency failed: %w", err)
	}

	if err == ErrKeyNotFound {
		current = 0
	}

	// Check if exceeded limit
	if current+n <= cfg.MaxConcurrency {
		// Increase concurrency number
		newCurrent, err := store.IncrBy(ctx, key, n)
		if err != nil {
			return nil, fmt.Errorf("increment concurrency failed: %w", err)
		}

		return &Response{
			Allowed:   true,
			Remaining: cfg.MaxConcurrency - newCurrent,
			Limit:     cfg.MaxConcurrency,
		}, nil
	}

	return &Response{
		Allowed:   false,
		Remaining: maxInt64(0, cfg.MaxConcurrency-current),
		Limit:     cfg.MaxConcurrency,
	}, nil
}

// Wait for permission to be acquired
func (a *concurrencyAlgorithm) Wait(ctx context.Context, store Store, resource string, n int64, cfg ResourceConfig, timeout time.Duration) error {
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

		// 短暂 wait 后重试
		waitTime := min64Duration(100*time.Millisecond, time.Until(deadline))
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
func (a *concurrencyAlgorithm) GetMetrics(ctx context.Context, store Store, resource string) (*AlgorithmMetrics, error) {
	key := a.concurrencyKey(resource)

	current, err := store.GetInt64(ctx, key)
	if err != nil && err != ErrKeyNotFound {
		return nil, fmt.Errorf("get current concurrency failed: %w", err)
	}

	if err == ErrKeyNotFound {
		current = 0
	}

	return &AlgorithmMetrics{
		Current:   current,
		Limit:     0,
		Remaining: 0,
	}, nil
}

// Reset reset status
func (a *concurrencyAlgorithm) Reset(ctx context.Context, store Store, resource string) error {
	key := a.concurrencyKey(resource)
	return store.Del(ctx, key)
}

// Release Decrease concurrency count (must be called after request completion)
func (a *concurrencyAlgorithm) Release(ctx context.Context, store Store, resource string, n int64) error {
	if n <= 0 {
		n = 1
	}

	key := a.concurrencyKey(resource)
	_, err := store.DecrBy(ctx, key, n)
	return err
}

// concurrencyKey returns the concurrency count storage key
func (a *concurrencyAlgorithm) concurrencyKey(resource string) string {
	return fmt.Sprintf("limiter:concurrency:%s:count", resource)
}

