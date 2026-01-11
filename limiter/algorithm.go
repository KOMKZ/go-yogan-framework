package limiter

import (
	"context"
	"time"
)

// Algorithm 限流算法接口（策略模式）
type Algorithm interface {
	// Allow 检查是否允许请求
	Allow(ctx context.Context, store Store, resource string, n int64, cfg ResourceConfig) (*Response, error)

	// Wait 等待获取许可（阻塞直到获取或超时）
	Wait(ctx context.Context, store Store, resource string, n int64, cfg ResourceConfig, timeout time.Duration) error

	// GetMetrics 获取当前指标
	GetMetrics(ctx context.Context, store Store, resource string) (*AlgorithmMetrics, error)

	// Reset 重置状态
	Reset(ctx context.Context, store Store, resource string) error

	// Name 返回算法名称
	Name() string
}

// AlgorithmMetrics 算法指标
type AlgorithmMetrics struct {
	Current   int64     // 当前值（并发数/已用令牌/请求数）
	Limit     int64     // 限制值
	Remaining int64     // 剩余配额
	ResetAt   time.Time // 重置时间
}

// AlgorithmType 算法类型
type AlgorithmType string

const (
	// AlgorithmTokenBucket 令牌桶算法
	AlgorithmTokenBucket AlgorithmType = "token_bucket"

	// AlgorithmSlidingWindow 滑动窗口算法
	AlgorithmSlidingWindow AlgorithmType = "sliding_window"

	// AlgorithmConcurrency 并发限流算法
	AlgorithmConcurrency AlgorithmType = "concurrency"

	// AlgorithmAdaptive 自适应限流算法
	AlgorithmAdaptive AlgorithmType = "adaptive"
)

// GetAlgorithm 根据配置获取算法实例
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
		// 默认使用令牌桶
		return NewTokenBucketAlgorithm()
	}
}
