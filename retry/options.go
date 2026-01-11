package retry

import (
	"time"
)

// Config 重试配置
type Config struct {
	maxAttempts int              // 最大尝试次数（默认 3）
	backoff     BackoffStrategy  // 退避策略（默认指数退避）
	condition   RetryCondition   // 重试条件（默认所有错误都重试）
	onRetry     func(attempt int, err error) // 重试回调
	timeout     time.Duration    // 单次操作超时（0 表示无限制）
	budget      *BudgetManager   // 重试预算（可选）
}

// defaultConfig 默认配置
func defaultConfig() *Config {
	return &Config{
		maxAttempts: 3,
		backoff:     ExponentialBackoff(time.Second),
		condition:   AlwaysRetry(),
		onRetry:     nil,
		timeout:     0,
		budget:      nil,
	}
}

// Option 配置选项函数
type Option func(*Config)

// MaxAttempts 设置最大尝试次数
func MaxAttempts(n int) Option {
	return func(c *Config) {
		if n > 0 {
			c.maxAttempts = n
		}
	}
}

// Backoff 设置退避策略
func Backoff(b BackoffStrategy) Option {
	return func(c *Config) {
		if b != nil {
			c.backoff = b
		}
	}
}

// Condition 设置重试条件
func Condition(cond RetryCondition) Option {
	return func(c *Config) {
		if cond != nil {
			c.condition = cond
		}
	}
}

// OnRetry 设置重试回调
func OnRetry(f func(attempt int, err error)) Option {
	return func(c *Config) {
		c.onRetry = f
	}
}

// Timeout 设置单次操作超时
func Timeout(d time.Duration) Option {
	return func(c *Config) {
		if d > 0 {
			c.timeout = d
		}
	}
}

// Budget 设置重试预算
func Budget(b *BudgetManager) Option {
	return func(c *Config) {
		c.budget = b
	}
}

