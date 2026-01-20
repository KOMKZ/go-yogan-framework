package breaker

import (
	"time"
)

// Circuit breaker configuration
type Config struct {
	// Enabled whether to enable circuit breaker (when false, bypass directly without circuit breaker judgment)
	Enabled bool `mapstructure:"enabled"`

	// EventBusBuffer event bus buffer size
	EventBusBuffer int `mapstructure:"event_bus_buffer"`

	// Default resource configuration settings
	Default ResourceConfig `mapstructure:"default"`

	// Resources Configuration at the resource level (overrides Default)
	Resources map[string]ResourceConfig `mapstructure:"resources"`
}

// ResourceConfig resource-level configuration
type ResourceConfig struct {
	// Strategy Circuit breaker strategy name: error_rate, slow_call_rate, consecutive, adaptive
	Strategy string `mapstructure:"strategy"`

	// Minimum request threshold (to avoid misjudgment in low traffic scenarios)
	MinRequests int `mapstructure:"min_requests"`

	// ErrorRateThreshold Error rate threshold (0.0-1.0)
	ErrorRateThreshold float64 `mapstructure:"error_rate_threshold"`

	// SlowCallThreshold slow call threshold
	SlowCallThreshold time.Duration `mapstructure:"slow_call_threshold"`

	// SlowRateThreshold slow call ratio threshold (0.0-1.0)
	SlowRateThreshold float64 `mapstructure:"slow_rate_threshold"`

	// ConsecutiveFailures consecutive failure threshold
	ConsecutiveFailures int `mapstructure:"consecutive_failures"`

	// Timeout circuit breaker open state duration
	Timeout time.Duration `mapstructure:"timeout"`

	// HalfOpenRequests Number of requests allowed in half-open state
	HalfOpenRequests int `mapstructure:"half_open_requests"`

	// Window size of sliding window
	WindowSize time.Duration `mapstructure:"window_size"`

	// BucketSize bucket size
	BucketSize time.Duration `mapstructure:"bucket_size"`
}

// Return default configuration
func DefaultConfig() Config {
	return Config{
		Enabled:        false, // Default disabled
		EventBusBuffer: 500,
		Default:        DefaultResourceConfig(),
		Resources:      make(map[string]ResourceConfig),
	}
}

// Returns default resource configuration
func DefaultResourceConfig() ResourceConfig {
	return ResourceConfig{
		Strategy:            "error_rate",
		MinRequests:         20,
		ErrorRateThreshold:  0.5,
		SlowCallThreshold:   time.Second,
		SlowRateThreshold:   0.5,
		ConsecutiveFailures: 5,
		Timeout:             30 * time.Second,
		HalfOpenRequests:    3,
		WindowSize:          10 * time.Second,
		BucketSize:          time.Second,
	}
}

// Validate configuration
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil // Not enabled, verification not required
	}

	if c.EventBusBuffer <= 0 {
		c.EventBusBuffer = 500
	}

	// Verify default configuration
	if err := c.Default.Validate(); err != nil {
		return err
	}

	// Merge and validate resource configurations
	for name, cfg := range c.Resources {
		// Merge default configuration (use default values for fields not set in resource configuration)
		merged := c.Default.Merge(cfg)
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

// Merge merge configuration (override override default values)
func (rc ResourceConfig) Merge(override ResourceConfig) ResourceConfig {
	result := rc // Start with default configuration

	// Only cover non-zero values
	if override.Strategy != "" {
		result.Strategy = override.Strategy
	}
	if override.MinRequests > 0 {
		result.MinRequests = override.MinRequests
	}
	if override.ErrorRateThreshold > 0 {
		result.ErrorRateThreshold = override.ErrorRateThreshold
	}
	if override.SlowCallThreshold > 0 {
		result.SlowCallThreshold = override.SlowCallThreshold
	}
	if override.SlowRateThreshold > 0 {
		result.SlowRateThreshold = override.SlowRateThreshold
	}
	if override.ConsecutiveFailures > 0 {
		result.ConsecutiveFailures = override.ConsecutiveFailures
	}
	if override.Timeout > 0 {
		result.Timeout = override.Timeout
	}
	if override.HalfOpenRequests > 0 {
		result.HalfOpenRequests = override.HalfOpenRequests
	}
	if override.WindowSize > 0 {
		result.WindowSize = override.WindowSize
	}
	if override.BucketSize > 0 {
		result.BucketSize = override.BucketSize
	}

	return result
}

// Validate resource configuration
func (rc *ResourceConfig) Validate() error {
	if rc.MinRequests < 0 {
		return &ValidationError{Field: "MinRequests", Message: "must be >= 0"}
	}

	if rc.ErrorRateThreshold < 0 || rc.ErrorRateThreshold > 1 {
		return &ValidationError{Field: "ErrorRateThreshold", Message: "must be between 0.0 and 1.0"}
	}

	if rc.SlowRateThreshold < 0 || rc.SlowRateThreshold > 1 {
		return &ValidationError{Field: "SlowRateThreshold", Message: "must be between 0.0 and 1.0"}
	}

	if rc.ConsecutiveFailures < 0 {
		return &ValidationError{Field: "ConsecutiveFailures", Message: "must be >= 0"}
	}

	if rc.Timeout <= 0 {
		return &ValidationError{Field: "Timeout", Message: "must be > 0"}
	}

	if rc.HalfOpenRequests <= 0 {
		return &ValidationError{Field: "HalfOpenRequests", Message: "must be > 0"}
	}

	if rc.WindowSize <= 0 {
		return &ValidationError{Field: "WindowSize", Message: "must be > 0"}
	}

	if rc.BucketSize <= 0 {
		return &ValidationError{Field: "BucketSize", Message: "must be > 0"}
	}

	if rc.WindowSize < rc.BucketSize {
		return &ValidationError{Field: "WindowSize", Message: "must be >= BucketSize"}
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

// ValidationError configuration validation error
type ValidationError struct {
	Resource string
	Field    string
	Message  string
	Err      error
}

func (e *ValidationError) Error() string {
	if e.Resource != "" {
		if e.Err != nil {
			return "breaker config validation failed for resource '" + e.Resource + "': " + e.Err.Error()
		}
		return "breaker config validation failed for resource '" + e.Resource + "." + e.Field + "': " + e.Message
	}

	if e.Field != "" {
		return "breaker config validation failed for field '" + e.Field + "': " + e.Message
	}

	if e.Err != nil {
		return "breaker config validation failed: " + e.Err.Error()
	}

	return "breaker config validation failed"
}
