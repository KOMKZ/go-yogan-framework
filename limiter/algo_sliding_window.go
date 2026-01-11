package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// slidingWindowAlgorithm 滑动窗口算法实现
type slidingWindowAlgorithm struct{}

// NewSlidingWindowAlgorithm 创建滑动窗口算法
func NewSlidingWindowAlgorithm() Algorithm {
	return &slidingWindowAlgorithm{}
}

// Name 返回算法名称
func (a *slidingWindowAlgorithm) Name() string {
	return string(AlgorithmSlidingWindow)
}

// Allow 检查是否允许请求
func (a *slidingWindowAlgorithm) Allow(ctx context.Context, store Store, resource string, n int64, cfg ResourceConfig) (*Response, error) {
	if n <= 0 {
		n = 1
	}

	now := time.Now()
	key := a.windowKey(resource)

	// 删除窗口外的数据（删除 < windowStart 的数据）
	windowStart := now.Add(-cfg.WindowSize)
	minScore := float64(windowStart.UnixNano())
	// 删除小于minScore的数据（不包括minScore本身）
	if err := store.ZRemRangeByScore(ctx, key, 0, minScore-1); err != nil {
		return nil, fmt.Errorf("remove old entries failed: %w", err)
	}

	// 统计当前窗口内的请求数（从windowStart到now，都包括）
	maxScore := float64(now.UnixNano())
	count, err := store.ZCount(ctx, key, minScore, maxScore)
	if err != nil {
		return nil, fmt.Errorf("count requests failed: %w", err)
	}

	// 检查是否超过限制
	if count+n <= cfg.Limit {
		// 添加新请求
		for i := int64(0); i < n; i++ {
			// 每个请求的score稍微递增，确保唯一
			scoreTime := now.Add(time.Duration(i) * time.Nanosecond)
			score := float64(scoreTime.UnixNano())
			// 使用UUID确保member唯一（避免同一纳秒内的冲突）
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

	// 超过限制，计算重试时间
	retryAfter := cfg.WindowSize / time.Duration(cfg.Limit)

	return &Response{
		Allowed:    false,
		RetryAfter: retryAfter,
		Remaining:  maxInt64(0, cfg.Limit-count),
		Limit:      cfg.Limit,
		ResetAt:    now.Add(cfg.WindowSize),
	}, nil
}

// Wait 等待获取许可
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
			// 继续重试
		}
	}
}

// GetMetrics 获取当前指标
func (a *slidingWindowAlgorithm) GetMetrics(ctx context.Context, store Store, resource string) (*AlgorithmMetrics, error) {
	now := time.Now()
	key := a.windowKey(resource)

	// 获取当前窗口内的请求数
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

// Reset 重置状态
func (a *slidingWindowAlgorithm) Reset(ctx context.Context, store Store, resource string) error {
	key := a.windowKey(resource)
	return store.Del(ctx, key)
}

// windowKey 返回窗口存储键
func (a *slidingWindowAlgorithm) windowKey(resource string) string {
	return fmt.Sprintf("limiter:window:%s", resource)
}

