package retry

import (
	"math"
	"math/rand"
	"time"
)

// BackoffStrategy retreat strategy interface
type BackoffStrategy interface {
	// Returns the delay time for the Nth retry (with attempt starting at 1)
	Next(attempt int) time.Duration
}

// BackoffOption back-off strategy option
type BackoffOption func(*backoffConfig)

// backoffConfig backoff strategy configuration
type backoffConfig struct {
	multiplier float64       // Exponential factor (default 2.0)
	maxDelay   time.Duration // Maximum delay (default 30s)
	jitter     float64       // Jitter ratio (default 0.2, i.e., 20%)
}

// default backoff configuration
func defaultBackoffConfig() *backoffConfig {
	return &backoffConfig{
		multiplier: 2.0,
		maxDelay:   30 * time.Second,
		jitter:     0.2,
	}
}

// WithMultiplier sets the exponential multiplier
func WithMultiplier(m float64) BackoffOption {
	return func(c *backoffConfig) {
		if m > 0 {
			c.multiplier = m
		}
	}
}

// SetMaximumDelay
func WithMaxDelay(d time.Duration) BackoffOption {
	return func(c *backoffConfig) {
		if d > 0 {
			c.maxDelay = d
		}
	}
}

// WithJitter sets the jitter ratio (0.0 - 1.0)
func WithJitter(ratio float64) BackoffOption {
	return func(c *backoffConfig) {
		if ratio >= 0 && ratio <= 1.0 {
			c.jitter = ratio
		}
	}
}

// ============================================================
// Exponential backoff strategy
// ============================================================

// exponential backoff strategy
type exponentialBackoff struct {
	base   time.Duration
	config *backoffConfig
}

// Create an exponential backoff strategy
// delay = base * (multiplier ^ (attempt - 1))
// For example: base=1s, multiplier=2.0
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

// Next implement the BackoffStrategy interface
func (b *exponentialBackoff) Next(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	
	// Calculate base latency: base * (multiplier ^ (attempt - 1))
	delay := float64(b.base) * math.Pow(b.config.multiplier, float64(attempt-1))
	
	// Limit maximum delay
	if delay > float64(b.config.maxDelay) {
		delay = float64(b.config.maxDelay)
	}
	
	// Apply jitter (random ±jitter%)
	if b.config.jitter > 0 {
		delay = applyJitter(delay, b.config.jitter)
	}
	
	return time.Duration(delay)
}

// ============================================================
// linear backoff strategy
// ============================================================

// linear backoff strategy
type linearBackoff struct {
	base   time.Duration
	config *backoffConfig
}

// Create linear backoff strategy
// delay = base * attempt
// For example: base=1s
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

// Next implement the BackoffStrategy interface
func (b *linearBackoff) Next(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	
	// Calculate base latency: base * attempt
	delay := float64(b.base) * float64(attempt)
	
	// Limit maximum latency
	if delay > float64(b.config.maxDelay) {
		delay = float64(b.config.maxDelay)
	}
	
	// Apply jitter
	if b.config.jitter > 0 {
		delay = applyJitter(delay, b.config.jitter)
	}
	
	return time.Duration(delay)
}

// ============================================================
// Fixed backoff strategy
// ============================================================

// constantBackoff fixed backoff strategy
type constantBackoff struct {
	delay  time.Duration
	config *backoffConfig
}

// Create fixed backoff strategy
// delay = fixed value
// For example: delay=2s
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

// Next implement the BackoffStrategy interface
func (b *constantBackoff) Next(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	
	delay := float64(b.delay)
	
	// Apply jitter
	if b.config.jitter > 0 {
		delay = applyJitter(delay, b.config.jitter)
	}
	
	return time.Duration(delay)
}

// ============================================================
// No backoff strategy
// ============================================================

// noBackoff no backoff strategy
type noBackoff struct{}

// NoBackoff creates a no-backoff policy (immediate retry)
func NoBackoff() BackoffStrategy {
	return &noBackoff{}
}

// Next implement the BackoffStrategy interface
func (b *noBackoff) Next(attempt int) time.Duration {
	return 0
}

// ============================================================
// utility function
// ============================================================

// Apply jitter
// Randomly within the range [delay * (1 - jitter), delay * (1 + jitter)]
func applyJitter(delay float64, jitter float64) float64 {
	// Calculate jitter range
	delta := delay * jitter
	
	// Random offset: [-delta, +delta]
	offset := (rand.Float64()*2 - 1) * delta
	
	result := delay + offset
	
	// Ensure not negative
	if result < 0 {
		return 0
	}
	
	return result
}

