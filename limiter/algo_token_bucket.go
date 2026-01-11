package limiter

import (
	"context"
	"fmt"
	"math"
	"time"
)

// tokenBucketAlgorithm 令牌桶算法实现
type tokenBucketAlgorithm struct{}

// NewTokenBucketAlgorithm 创建令牌桶算法
func NewTokenBucketAlgorithm() Algorithm {
	return &tokenBucketAlgorithm{}
}

// Name 返回算法名称
func (a *tokenBucketAlgorithm) Name() string {
	return string(AlgorithmTokenBucket)
}

// Allow 检查是否允许请求
func (a *tokenBucketAlgorithm) Allow(ctx context.Context, store Store, resource string, n int64, cfg ResourceConfig) (*Response, error) {
	if n <= 0 {
		n = 1
	}

	now := time.Now()

	// 生成存储键
	tokensKey := a.tokensKey(resource)
	lastRefillKey := a.lastRefillKey(resource)

	// 获取当前令牌数和上次填充时间
	tokens, err := store.GetInt64(ctx, tokensKey)
	if err != nil && err != ErrKeyNotFound {
		return nil, fmt.Errorf("get tokens failed: %w", err)
	}

	lastRefillNano, err2 := store.GetInt64(ctx, lastRefillKey)
	if err2 != nil && err2 != ErrKeyNotFound {
		return nil, fmt.Errorf("get last refill time failed: %w", err2)
	}

	// 如果是首次访问（两个键都不存在），初始化
	if err == ErrKeyNotFound && err2 == ErrKeyNotFound {
		// 使用配置的InitTokens，如果未设置则使用Capacity
		tokens = cfg.InitTokens
		if cfg.InitTokens == 0 {
			tokens = cfg.Capacity // 默认满桶
		}
		lastRefillNano = now.UnixNano()

		// 初始化存储
		if err := store.SetInt64(ctx, tokensKey, tokens, 0); err != nil {
			return nil, fmt.Errorf("init tokens failed: %w", err)
		}
		if err := store.SetInt64(ctx, lastRefillKey, lastRefillNano, 0); err != nil {
			return nil, fmt.Errorf("init last refill failed: %w", err)
		}
	} else {
		// 计算新增令牌数
		lastRefill := time.Unix(0, lastRefillNano)
		elapsed := now.Sub(lastRefill)
		newTokens := int64(float64(cfg.Rate) * elapsed.Seconds())
		tokens = min(tokens+newTokens, cfg.Capacity)
	}

	// 检查是否有足够的令牌
	if tokens >= n {
		// 扣除令牌
		tokens -= n

		// 更新存储
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

	// 令牌不足，更新lastRefill时间（避免下次重复计算）
	if err := store.SetInt64(ctx, lastRefillKey, now.UnixNano(), 0); err != nil {
		return nil, fmt.Errorf("set last refill failed: %w", err)
	}

	// 计算重试时间
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

// Wait 等待获取许可
func (a *tokenBucketAlgorithm) Wait(ctx context.Context, store Store, resource string, n int64, cfg ResourceConfig, timeout time.Duration) error {
	if n <= 0 {
		n = 1
	}

	deadline := time.Now().Add(timeout)

	for {
		// 检查是否已超时
		if time.Now().After(deadline) {
			return ErrWaitTimeout
		}

		// 尝试获取许可
		resp, err := a.Allow(ctx, store, resource, n, cfg)
		if err != nil {
			return err
		}

		if resp.Allowed {
			return nil
		}

		// 计算等待时间（取重试时间和剩余时间的较小值）
		remaining := time.Until(deadline)
		waitTime := resp.RetryAfter
		if waitTime > remaining {
			waitTime = remaining
		}

		// 如果等待时间过小，直接返回超时
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
func (a *tokenBucketAlgorithm) GetMetrics(ctx context.Context, store Store, resource string) (*AlgorithmMetrics, error) {
	tokensKey := a.tokensKey(resource)
	lastRefillKey := a.lastRefillKey(resource)

	// 获取当前令牌数
	tokens, err := store.GetInt64(ctx, tokensKey)
	if err != nil && err != ErrKeyNotFound {
		return nil, fmt.Errorf("get tokens failed: %w", err)
	}

	if err == ErrKeyNotFound {
		tokens = 0
	}

	// 获取上次填充时间
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
		Limit:     0, // 令牌桶没有明确的limit概念
		Remaining: tokens,
		ResetAt:   resetAt,
	}, nil
}

// Reset 重置状态
func (a *tokenBucketAlgorithm) Reset(ctx context.Context, store Store, resource string) error {
	tokensKey := a.tokensKey(resource)
	lastRefillKey := a.lastRefillKey(resource)

	return store.Del(ctx, tokensKey, lastRefillKey)
}

// tokensKey 返回令牌存储键
func (a *tokenBucketAlgorithm) tokensKey(resource string) string {
	return fmt.Sprintf("limiter:token:%s:tokens", resource)
}

// lastRefillKey 返回上次填充时间存储键
func (a *tokenBucketAlgorithm) lastRefillKey(resource string) string {
	return fmt.Sprintf("limiter:token:%s:last_refill", resource)
}

// min 返回两个int64的最小值
func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// min64Duration 返回time.Duration的最小值
func min64Duration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// maxInt64 返回最大的int64值
func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// minFloat64 返回最小的float64值
func minFloat64(a, b float64) float64 {
	return math.Min(a, b)
}

// maxFloat64 返回最大的float64值
func maxFloat64(a, b float64) float64 {
	return math.Max(a, b)
}

