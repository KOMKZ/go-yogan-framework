package limiter

import (
	"context"
	"sync"
	"time"
)

// adaptiveAlgorithm 自适应限流算法实现
type adaptiveAlgorithm struct {
	provider       AdaptiveProvider
	currentLimit   int64
	lastAdjustTime time.Time
	mu             sync.RWMutex
}

// NewAdaptiveAlgorithm 创建自适应限流算法
func NewAdaptiveAlgorithm(provider AdaptiveProvider) Algorithm {
	return &adaptiveAlgorithm{
		provider:       provider,
		currentLimit:   0,
		lastAdjustTime: time.Now(),
	}
}

// Name 返回算法名称
func (a *adaptiveAlgorithm) Name() string {
	return string(AlgorithmAdaptive)
}

// Allow 检查是否允许请求
func (a *adaptiveAlgorithm) Allow(ctx context.Context, store Store, resource string, n int64, cfg ResourceConfig) (*Response, error) {
	// 调整限流阈值
	a.adjustLimit(cfg)

	// 获取当前限流值
	a.mu.RLock()
	limit := a.currentLimit
	a.mu.RUnlock()

	// 如果没有provider或限流值无效，使用最大限流值
	if limit <= 0 {
		limit = cfg.MaxLimit
	}

	// 使用令牌桶算法实现
	tokenBucket := NewTokenBucketAlgorithm()

	// 临时修改配置使用自适应限流值
	tempCfg := cfg
	tempCfg.Rate = limit
	tempCfg.Capacity = limit * 2

	return tokenBucket.Allow(ctx, store, resource, n, tempCfg)
}

// Wait 等待获取许可
func (a *adaptiveAlgorithm) Wait(ctx context.Context, store Store, resource string, n int64, cfg ResourceConfig, timeout time.Duration) error {
	// 调整限流阈值
	a.adjustLimit(cfg)

	// 获取当前限流值
	a.mu.RLock()
	limit := a.currentLimit
	a.mu.RUnlock()

	if limit <= 0 {
		limit = cfg.MaxLimit
	}

	// 使用令牌桶算法实现
	tokenBucket := NewTokenBucketAlgorithm()

	tempCfg := cfg
	tempCfg.Rate = limit
	tempCfg.Capacity = limit * 2

	return tokenBucket.Wait(ctx, store, resource, n, tempCfg, timeout)
}

// GetMetrics 获取当前指标
func (a *adaptiveAlgorithm) GetMetrics(ctx context.Context, store Store, resource string) (*AlgorithmMetrics, error) {
	a.mu.RLock()
	limit := a.currentLimit
	a.mu.RUnlock()

	return &AlgorithmMetrics{
		Current:   0,
		Limit:     limit,
		Remaining: limit,
		ResetAt:   time.Now(),
	}, nil
}

// Reset 重置状态
func (a *adaptiveAlgorithm) Reset(ctx context.Context, store Store, resource string) error {
	// 使用令牌桶算法重置
	tokenBucket := NewTokenBucketAlgorithm()
	return tokenBucket.Reset(ctx, store, resource)
}

// adjustLimit 根据系统负载调整限流阈值
func (a *adaptiveAlgorithm) adjustLimit(cfg ResourceConfig) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 检查是否需要调整
	if time.Since(a.lastAdjustTime) < cfg.AdjustInterval {
		return
	}

	a.lastAdjustTime = time.Now()

	// 如果没有provider，使用最大限流值
	if a.provider == nil {
		a.currentLimit = cfg.MaxLimit
		return
	}

	// 初始化当前限流值
	if a.currentLimit == 0 {
		a.currentLimit = (cfg.MinLimit + cfg.MaxLimit) / 2
	}

	// 获取系统负载数据
	cpuUsage := a.provider.GetCPUUsage()
	memoryUsage := a.provider.GetMemoryUsage()
	systemLoad := a.provider.GetSystemLoad()

	// 计算平均负载（优先级：CPU > Memory > Load）
	var avgLoad float64
	if cfg.TargetCPU > 0 {
		avgLoad = cpuUsage / cfg.TargetCPU
	} else if cfg.TargetMemory > 0 {
		avgLoad = memoryUsage / cfg.TargetMemory
	} else if cfg.TargetLoad > 0 {
		avgLoad = systemLoad / cfg.TargetLoad
	} else {
		// 没有配置目标值，使用最大限流
		a.currentLimit = cfg.MaxLimit
		return
	}

	// 根据负载调整限流值
	oldLimit := a.currentLimit

	if avgLoad > 1.2 {
		// 负载过高，降低限流值10%
		a.currentLimit = maxInt64(cfg.MinLimit, int64(float64(a.currentLimit)*0.9))
	} else if avgLoad < 0.8 {
		// 负载较低，提高限流值10%
		a.currentLimit = minInt64(cfg.MaxLimit, int64(float64(a.currentLimit)*1.1))
	}

	// 限制在配置范围内
	if a.currentLimit < cfg.MinLimit {
		a.currentLimit = cfg.MinLimit
	}
	if a.currentLimit > cfg.MaxLimit {
		a.currentLimit = cfg.MaxLimit
	}

	// 如果限流值有变化，可以通过事件通知
	_ = oldLimit // 后续可用于事件通知
}

// minInt64 返回最小的int64值
func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

