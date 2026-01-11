package limiter

import (
	"context"
	"fmt"
	"time"
)

// concurrencyAlgorithm 并发限流算法实现
type concurrencyAlgorithm struct{}

// NewConcurrencyAlgorithm 创建并发限流算法
func NewConcurrencyAlgorithm() Algorithm {
	return &concurrencyAlgorithm{}
}

// Name 返回算法名称
func (a *concurrencyAlgorithm) Name() string {
	return string(AlgorithmConcurrency)
}

// Allow 检查是否允许请求
func (a *concurrencyAlgorithm) Allow(ctx context.Context, store Store, resource string, n int64, cfg ResourceConfig) (*Response, error) {
	if n <= 0 {
		n = 1
	}

	key := a.concurrencyKey(resource)

	// 获取当前并发数
	current, err := store.GetInt64(ctx, key)
	if err != nil && err != ErrKeyNotFound {
		return nil, fmt.Errorf("get current concurrency failed: %w", err)
	}

	if err == ErrKeyNotFound {
		current = 0
	}

	// 检查是否超过限制
	if current+n <= cfg.MaxConcurrency {
		// 增加并发数
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

// Wait 等待获取许可
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

		// 短暂等待后重试
		waitTime := min64Duration(100*time.Millisecond, time.Until(deadline))
		if waitTime <= 0 {
			return ErrWaitTimeout
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// 继续重试
		}
	}
}

// GetMetrics 获取当前指标
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

// Reset 重置状态
func (a *concurrencyAlgorithm) Reset(ctx context.Context, store Store, resource string) error {
	key := a.concurrencyKey(resource)
	return store.Del(ctx, key)
}

// Release 释放并发数（需要在请求完成后调用）
func (a *concurrencyAlgorithm) Release(ctx context.Context, store Store, resource string, n int64) error {
	if n <= 0 {
		n = 1
	}

	key := a.concurrencyKey(resource)
	_, err := store.DecrBy(ctx, key, n)
	return err
}

// concurrencyKey 返回并发数存储键
func (a *concurrencyAlgorithm) concurrencyKey(resource string) string {
	return fmt.Sprintf("limiter:concurrency:%s:count", resource)
}

