package limiter

import (
	"time"
)

// Rate limiter configuration
type Config struct {
	// Enabled whether to enable rate limiting (false means direct passthrough)
	Enabled bool `mapstructure:"enabled"`

	// StoreType storage type: memory, redis
	StoreType string `mapstructure:"store_type"`

	// Redis configuration (required when StoreType is redis)
	Redis RedisInstanceConfig `mapstructure:"redis"`

	// EventBusBuffer event bus buffer size
	EventBusBuffer int `mapstructure:"event_bus_buffer"`

	// KeyFunc resource key generation method (for middleware)
	// Optional values: path, ip, user, path_ip, api_key (default is path)
	KeyFunc string `mapstructure:"key_func"`

	// SkipPaths list of paths to bypass rate limiting (for middleware)
	SkipPaths []string `mapstructure:"skip_paths"`

	// Default resource configuration (if a valid default is set, it will be automatically applied to unconfigured resources)
	Default ResourceConfig `mapstructure:"default"`

	// Resources configuration level (overrides Default)
	Resources map[string]ResourceConfig `mapstructure:"resources"`
}

// ResourceConfig resource-level configuration
type ResourceConfig struct {
	// Algorithm rate_limiting: token_bucket, sliding_window, concurrency, adaptive
	Algorithm string `mapstructure:"algorithm"`

	// Token bucket configuration
	Rate       int64 `mapstructure:"rate"`        // token generation rate (per second)
	Capacity   int64 `mapstructure:"capacity"`    // bucket capacity
	InitTokens int64 `mapstructure:"init_tokens"` // Initial token count

	// Sliding window configuration
	Limit      int64         `mapstructure:"limit"`       // maximum request count within window
	WindowSize time.Duration `mapstructure:"window_size"` // window size
	BucketSize time.Duration `mapstructure:"bucket_size"` // bucket size

	// Concurrent rate limiting configuration
	MaxConcurrency int64         `mapstructure:"max_concurrency"` // maximum concurrency limit
	Timeout        time.Duration `mapstructure:"timeout"`         // timeout waiting

	// Adaptive rate limiting configuration
	MinLimit       int64         `mapstructure:"min_limit"`       // minimum rate limiting value
	MaxLimit       int64         `mapstructure:"max_limit"`       // maximum rate limiting value
	TargetCPU      float64       `mapstructure:"target_cpu"`      // Target CPU utilization rate
	TargetMemory   float64       `mapstructure:"target_memory"`   // Target memory utilization rate
	TargetLoad     float64       `mapstructure:"target_load"`     // target system load
	AdjustInterval time.Duration `mapstructure:"adjust_interval"` // Adjust interval
}

// RedisInstanceConfig Redis instance reference configuration (reusing kernel redis component)
type RedisInstanceConfig struct {
	Instance  string `mapstructure:"instance"`   // Redis instance name (configured in redis.instances)
	KeyPrefix string `mapstructure:"key_prefix"` // Redis key prefix (default "limiter:")
}

// Return default configuration
func DefaultConfig() Config {
	return Config{
		Enabled:        false,
		StoreType:      "memory",
		EventBusBuffer: 500,
		KeyFunc:        "path",
		SkipPaths:      []string{},
		Default:        DefaultResourceConfig(),
		Resources:      make(map[string]ResourceConfig),
	}
}

// Returns default resource configuration
func DefaultResourceConfig() ResourceConfig {
	return ResourceConfig{
		Algorithm:  string(AlgorithmTokenBucket),
		Rate:       100,             // 100 QPS
		Capacity:   200,             // Allow 200 burst requests
		InitTokens: 200,             // Initial full bucket
		WindowSize: 1 * time.Second, // 1-second window
		Timeout:    1 * time.Second, // timeout of 1 second
	}
}

// Validate configuration
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil // not enabled, verification not required
	}

	if c.EventBusBuffer <= 0 {
		c.EventBusBuffer = 500
	}

	// Validate storage type
	if c.StoreType != string(StoreTypeMemory) && c.StoreType != string(StoreTypeRedis) {
		return &ValidationError{Field: "store_type", Message: "must be 'memory' or 'redis'"}
	}

	// Verify Redis configuration
	if c.StoreType == string(StoreTypeRedis) {
		if c.Redis.Instance == "" {
			return &ValidationError{Field: "redis.instance", Message: "redis instance name is required"}
		}
		// Set default key prefix
		if c.Redis.KeyPrefix == "" {
			c.Redis.KeyPrefix = "limiter:"
		}
	}

	// Verify default configuration if any configuration is present
	// If the default configuration has valid values, verify it; if it is an empty configuration, skip verification
	if !c.Default.isEmpty() {
		if err := c.Default.Validate(); err != nil {
			return err
		}
	}

	// Merge and validate resource configurations
	for name, cfg := range c.Resources {
		// If default is valid, merge the default configuration
		var merged ResourceConfig
		if !c.Default.isEmpty() {
			merged = c.Default.Merge(cfg)
		} else {
			merged = cfg
		}
		c.Resources[name] = merged

		if err := merged.Validate(); err != nil {
			return &ValidationError{
				Resource: name,
				Err:      err,
			}
		}
	}

	return nil
}

// Merge merge configuration (override default values)
func (rc ResourceConfig) Merge(override ResourceConfig) ResourceConfig {
	result := rc // Start with default configuration

	// Only cover non-zero values
	if override.Algorithm != "" {
		result.Algorithm = override.Algorithm
	}
	if override.Rate > 0 {
		result.Rate = override.Rate
	}
	if override.Capacity > 0 {
		result.Capacity = override.Capacity
	}
	if override.InitTokens > 0 {
		result.InitTokens = override.InitTokens
	}
	if override.Limit > 0 {
		result.Limit = override.Limit
	}
	if override.WindowSize > 0 {
		result.WindowSize = override.WindowSize
	}
	if override.BucketSize > 0 {
		result.BucketSize = override.BucketSize
	}
	if override.MaxConcurrency > 0 {
		result.MaxConcurrency = override.MaxConcurrency
	}
	if override.Timeout > 0 {
		result.Timeout = override.Timeout
	}
	if override.MinLimit > 0 {
		result.MinLimit = override.MinLimit
	}
	if override.MaxLimit > 0 {
		result.MaxLimit = override.MaxLimit
	}
	if override.TargetCPU > 0 {
		result.TargetCPU = override.TargetCPU
	}
	if override.TargetMemory > 0 {
		result.TargetMemory = override.TargetMemory
	}
	if override.TargetLoad > 0 {
		result.TargetLoad = override.TargetLoad
	}
	if override.AdjustInterval > 0 {
		result.AdjustInterval = override.AdjustInterval
	}

	return result
}

// checks if ResourceConfig is an empty configuration
func (rc ResourceConfig) isEmpty() bool {
	return rc.Algorithm == "" &&
		rc.Rate == 0 &&
		rc.Capacity == 0 &&
		rc.Limit == 0 &&
		rc.MaxConcurrency == 0 &&
		rc.MinLimit == 0 &&
		rc.MaxLimit == 0
}

// Validate resource configuration
func (rc *ResourceConfig) Validate() error {
	// Validate algorithm type
	algo := AlgorithmType(rc.Algorithm)
	if algo != AlgorithmTokenBucket && algo != AlgorithmSlidingWindow &&
		algo != AlgorithmConcurrency && algo != AlgorithmAdaptive {
		return &ValidationError{Field: "algorithm", Message: "invalid algorithm type"}
	}

	// Validate configuration based on algorithm type
	switch algo {
	case AlgorithmTokenBucket:
		if rc.Rate <= 0 {
			return &ValidationError{Field: "rate", Message: "must be > 0"}
		}
		if rc.Capacity <= 0 {
			return &ValidationError{Field: "capacity", Message: "must be > 0"}
		}
		if rc.InitTokens < 0 {
			return &ValidationError{Field: "init_tokens", Message: "must be >= 0"}
		}
		if rc.InitTokens > rc.Capacity {
			return &ValidationError{Field: "init_tokens", Message: "must be <= capacity"}
		}

	case AlgorithmSlidingWindow:
		if rc.Limit <= 0 {
			return &ValidationError{Field: "limit", Message: "must be > 0"}
		}
		if rc.WindowSize <= 0 {
			return &ValidationError{Field: "window_size", Message: "must be > 0"}
		}
		if rc.BucketSize <= 0 {
			rc.BucketSize = 100 * time.Millisecond // Default 100ms
		}
		if rc.WindowSize < rc.BucketSize {
			return &ValidationError{Field: "window_size", Message: "must be >= bucket_size"}
		}

	case AlgorithmConcurrency:
		if rc.MaxConcurrency <= 0 {
			return &ValidationError{Field: "max_concurrency", Message: "must be > 0"}
		}
		if rc.Timeout <= 0 {
			rc.Timeout = 1 * time.Second // Default 1 second
		}

	case AlgorithmAdaptive:
		if rc.MinLimit <= 0 {
			return &ValidationError{Field: "min_limit", Message: "must be > 0"}
		}
		if rc.MaxLimit <= 0 {
			return &ValidationError{Field: "max_limit", Message: "must be > 0"}
		}
		if rc.MinLimit > rc.MaxLimit {
			return &ValidationError{Field: "min_limit", Message: "must be <= max_limit"}
		}
		if rc.TargetCPU > 0 && (rc.TargetCPU < 0 || rc.TargetCPU > 1) {
			return &ValidationError{Field: "target_cpu", Message: "must be between 0.0 and 1.0"}
		}
		if rc.TargetMemory > 0 && (rc.TargetMemory < 0 || rc.TargetMemory > 1) {
			return &ValidationError{Field: "target_memory", Message: "must be between 0.0 and 1.0"}
		}
		if rc.TargetLoad > 0 && rc.TargetLoad < 0 {
			return &ValidationError{Field: "target_load", Message: "must be >= 0"}
		}
		if rc.AdjustInterval <= 0 {
			rc.AdjustInterval = 10 * time.Second // Default 10 seconds
		}
	}

	return nil
}

// GetResourceConfig Retrieve resource configuration (prioritize resource-level configuration, fallback to default)
func (c *Config) GetResourceConfig(resource string) ResourceConfig {
	if cfg, ok := c.Resources[resource]; ok {
		return cfg
	}
	return c.Default
}
