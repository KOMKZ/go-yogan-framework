package limiter

import (
	"context"
	"sync"
	"time"
)

// adaptiveAlgorithm implementation of adaptive rate limiting algorithm
type adaptiveAlgorithm struct {
	provider       AdaptiveProvider
	currentLimit   int64
	lastAdjustTime time.Time
	mu             sync.RWMutex
}

// Create new adaptive rate limiting algorithm
func NewAdaptiveAlgorithm(provider AdaptiveProvider) Algorithm {
	return &adaptiveAlgorithm{
		provider:       provider,
		currentLimit:   0,
		lastAdjustTime: time.Now(),
	}
}

// Name Returns algorithm name
func (a *adaptiveAlgorithm) Name() string {
	return string(AlgorithmAdaptive)
}

// Allow check if the request is permitted
func (a *adaptiveAlgorithm) Allow(ctx context.Context, store Store, resource string, n int64, cfg ResourceConfig) (*Response, error) {
	// Adjust rate limiting threshold
	a.adjustLimit(cfg)

	// Get current rate limit value
	a.mu.RLock()
	limit := a.currentLimit
	a.mu.RUnlock()

	// If there is no provider or the rate limit value is invalid, use the maximum rate limit value
	if limit <= 0 {
		limit = cfg.MaxLimit
	}

	// Implement using token bucket algorithm
	tokenBucket := NewTokenBucketAlgorithm()

	// Temporarily modify the configuration to use adaptive rate limiting values
	tempCfg := cfg
	tempCfg.Rate = limit
	tempCfg.Capacity = limit * 2

	return tokenBucket.Allow(ctx, store, resource, n, tempCfg)
}

// Wait for permission to be acquired
func (a *adaptiveAlgorithm) Wait(ctx context.Context, store Store, resource string, n int64, cfg ResourceConfig, timeout time.Duration) error {
	// Adjust rate limiting threshold
	a.adjustLimit(cfg)

	// Get current rate limit value
	a.mu.RLock()
	limit := a.currentLimit
	a.mu.RUnlock()

	if limit <= 0 {
		limit = cfg.MaxLimit
	}

	// Implement using token bucket algorithm
	tokenBucket := NewTokenBucketAlgorithm()

	tempCfg := cfg
	tempCfg.Rate = limit
	tempCfg.Capacity = limit * 2

	return tokenBucket.Wait(ctx, store, resource, n, tempCfg, timeout)
}

// GetMetrics获取当前指标
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

// Reset reset status
func (a *adaptiveAlgorithm) Reset(ctx context.Context, store Store, resource string) error {
	// Reset using token bucket algorithm
	tokenBucket := NewTokenBucketAlgorithm()
	return tokenBucket.Reset(ctx, store, resource)
}

// adjustLimit Adjusts the rate limiting threshold based on system load
func (a *adaptiveAlgorithm) adjustLimit(cfg ResourceConfig) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Check if adjustment is needed
	if time.Since(a.lastAdjustTime) < cfg.AdjustInterval {
		return
	}

	a.lastAdjustTime = time.Now()

	// If there is no provider, use the maximum rate limit value
	if a.provider == nil {
		a.currentLimit = cfg.MaxLimit
		return
	}

	// Initialize current rate limit value
	if a.currentLimit == 0 {
		a.currentLimit = (cfg.MinLimit + cfg.MaxLimit) / 2
	}

	// Get system load data
	cpuUsage := a.provider.GetCPUUsage()
	memoryUsage := a.provider.GetMemoryUsage()
	systemLoad := a.provider.GetSystemLoad()

	// Calculate average load (priority: CPU > Memory > Load)
	var avgLoad float64
	if cfg.TargetCPU > 0 {
		avgLoad = cpuUsage / cfg.TargetCPU
	} else if cfg.TargetMemory > 0 {
		avgLoad = memoryUsage / cfg.TargetMemory
	} else if cfg.TargetLoad > 0 {
		avgLoad = systemLoad / cfg.TargetLoad
	} else {
		// No target value configured, using maximum rate limiting
		a.currentLimit = cfg.MaxLimit
		return
	}

	// Adjust throttling values based on load
	oldLimit := a.currentLimit

	if avgLoad > 1.2 {
		// High load, reduce rate limiting value by 10%
		a.currentLimit = maxInt64(cfg.MinLimit, int64(float64(a.currentLimit)*0.9))
	} else if avgLoad < 0.8 {
		// Low load, increase rate limit by 10%
		a.currentLimit = minInt64(cfg.MaxLimit, int64(float64(a.currentLimit)*1.1))
	}

	// Limit within configuration range
	if a.currentLimit < cfg.MinLimit {
		a.currentLimit = cfg.MinLimit
	}
	if a.currentLimit > cfg.MaxLimit {
		a.currentLimit = cfg.MaxLimit
	}

	// If the rate limit value changes, it can be notified via events
	_ = oldLimit // To be used for event notifications later
}

// returns the minimum int64 value
func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

