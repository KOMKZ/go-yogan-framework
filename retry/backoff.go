package retry

import (
	"math"
	"math/rand"
	"time"
)

// BackoffStrategy 退避策略接口
type BackoffStrategy interface {
	// Next 返回第 N 次重试的延迟时间（attempt 从 1 开始）
	Next(attempt int) time.Duration
}

// BackoffOption 退避策略选项
type BackoffOption func(*backoffConfig)

// backoffConfig 退避策略配置
type backoffConfig struct {
	multiplier float64       // 指数倍数（默认 2.0）
	maxDelay   time.Duration // 最大延迟（默认 30s）
	jitter     float64       // 抖动比例（默认 0.2，即 20%）
}

// defaultBackoffConfig 默认退避配置
func defaultBackoffConfig() *backoffConfig {
	return &backoffConfig{
		multiplier: 2.0,
		maxDelay:   30 * time.Second,
		jitter:     0.2,
	}
}

// WithMultiplier 设置指数倍数
func WithMultiplier(m float64) BackoffOption {
	return func(c *backoffConfig) {
		if m > 0 {
			c.multiplier = m
		}
	}
}

// WithMaxDelay 设置最大延迟
func WithMaxDelay(d time.Duration) BackoffOption {
	return func(c *backoffConfig) {
		if d > 0 {
			c.maxDelay = d
		}
	}
}

// WithJitter 设置抖动比例（0.0 - 1.0）
func WithJitter(ratio float64) BackoffOption {
	return func(c *backoffConfig) {
		if ratio >= 0 && ratio <= 1.0 {
			c.jitter = ratio
		}
	}
}

// ============================================================
// 指数退避策略
// ============================================================

// exponentialBackoff 指数退避策略
type exponentialBackoff struct {
	base   time.Duration
	config *backoffConfig
}

// ExponentialBackoff 创建指数退避策略
// delay = base * (multiplier ^ (attempt - 1))
// 例如：base=1s, multiplier=2.0
//   attempt 1: 1s
//   attempt 2: 2s
//   attempt 3: 4s
//   attempt 4: 8s
func ExponentialBackoff(base time.Duration, opts ...BackoffOption) BackoffStrategy {
	config := defaultBackoffConfig()
	for _, opt := range opts {
		opt(config)
	}
	
	return &exponentialBackoff{
		base:   base,
		config: config,
	}
}

// Next 实现 BackoffStrategy 接口
func (b *exponentialBackoff) Next(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	
	// 计算基础延迟：base * (multiplier ^ (attempt - 1))
	delay := float64(b.base) * math.Pow(b.config.multiplier, float64(attempt-1))
	
	// 限制最大延迟
	if delay > float64(b.config.maxDelay) {
		delay = float64(b.config.maxDelay)
	}
	
	// 应用抖动（随机 ±jitter%）
	if b.config.jitter > 0 {
		delay = applyJitter(delay, b.config.jitter)
	}
	
	return time.Duration(delay)
}

// ============================================================
// 线性退避策略
// ============================================================

// linearBackoff 线性退避策略
type linearBackoff struct {
	base   time.Duration
	config *backoffConfig
}

// LinearBackoff 创建线性退避策略
// delay = base * attempt
// 例如：base=1s
//   attempt 1: 1s
//   attempt 2: 2s
//   attempt 3: 3s
//   attempt 4: 4s
func LinearBackoff(base time.Duration, opts ...BackoffOption) BackoffStrategy {
	config := defaultBackoffConfig()
	for _, opt := range opts {
		opt(config)
	}
	
	return &linearBackoff{
		base:   base,
		config: config,
	}
}

// Next 实现 BackoffStrategy 接口
func (b *linearBackoff) Next(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	
	// 计算基础延迟：base * attempt
	delay := float64(b.base) * float64(attempt)
	
	// 限制最大延迟
	if delay > float64(b.config.maxDelay) {
		delay = float64(b.config.maxDelay)
	}
	
	// 应用抖动
	if b.config.jitter > 0 {
		delay = applyJitter(delay, b.config.jitter)
	}
	
	return time.Duration(delay)
}

// ============================================================
// 固定退避策略
// ============================================================

// constantBackoff 固定退避策略
type constantBackoff struct {
	delay  time.Duration
	config *backoffConfig
}

// ConstantBackoff 创建固定退避策略
// delay = 固定值
// 例如：delay=2s
//   attempt 1: 2s
//   attempt 2: 2s
//   attempt 3: 2s
func ConstantBackoff(delay time.Duration, opts ...BackoffOption) BackoffStrategy {
	config := defaultBackoffConfig()
	for _, opt := range opts {
		opt(config)
	}
	
	return &constantBackoff{
		delay:  delay,
		config: config,
	}
}

// Next 实现 BackoffStrategy 接口
func (b *constantBackoff) Next(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	
	delay := float64(b.delay)
	
	// 应用抖动
	if b.config.jitter > 0 {
		delay = applyJitter(delay, b.config.jitter)
	}
	
	return time.Duration(delay)
}

// ============================================================
// 无退避策略
// ============================================================

// noBackoff 无退避策略
type noBackoff struct{}

// NoBackoff 创建无退避策略（立即重试）
func NoBackoff() BackoffStrategy {
	return &noBackoff{}
}

// Next 实现 BackoffStrategy 接口
func (b *noBackoff) Next(attempt int) time.Duration {
	return 0
}

// ============================================================
// 工具函数
// ============================================================

// applyJitter 应用抖动
// 在 [delay * (1 - jitter), delay * (1 + jitter)] 范围内随机
func applyJitter(delay float64, jitter float64) float64 {
	// 计算抖动范围
	delta := delay * jitter
	
	// 随机偏移：[-delta, +delta]
	offset := (rand.Float64()*2 - 1) * delta
	
	result := delay + offset
	
	// 确保不为负数
	if result < 0 {
		return 0
	}
	
	return result
}

