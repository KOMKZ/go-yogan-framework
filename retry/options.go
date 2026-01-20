package retry

import (
	"time"
)

// Retry configuration
type Config struct {
	maxAttempts int              // Maximum number of attempts (default 3)
	backoff     BackoffStrategy  // backoff strategy (default exponential backoff)
	condition   RetryCondition   // Retry conditions (by default, all errors are retried)
	onRetry     func(attempt int, err error) // retry callback
	timeout     time.Duration    // Timeout for single operation (0 indicates no limit)
	budget      *BudgetManager   // Retry budget (optional)
}

// default configuration
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

// Option configuration function
type Option func(*Config)

// Set the maximum number of retry attempts
func MaxAttempts(n int) Option {
	return func(c *Config) {
		if n > 0 {
			c.maxAttempts = n
		}
	}
}

// Set backoff strategy
func Backoff(b BackoffStrategy) Option {
	return func(c *Config) {
		if b != nil {
			c.backoff = b
		}
	}
}

// Set retry conditions
func Condition(cond RetryCondition) Option {
	return func(c *Config) {
		if cond != nil {
			c.condition = cond
		}
	}
}

// Sets the retry callback
func OnRetry(f func(attempt int, err error)) Option {
	return func(c *Config) {
		c.onRetry = f
	}
}

// Set timeout for individual operation
func Timeout(d time.Duration) Option {
	return func(c *Config) {
		if d > 0 {
			c.timeout = d
		}
	}
}

// Budget sets retry budget
func Budget(b *BudgetManager) Option {
	return func(c *Config) {
		c.budget = b
	}
}

