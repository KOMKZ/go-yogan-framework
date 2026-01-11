package breaker

import (
	"time"
)

// Config 熔断器配置
type Config struct {
	// Enabled 是否启用熔断器（false 时直接透传，不进行熔断判断）
	Enabled bool `mapstructure:"enabled"`

	// EventBusBuffer 事件总线缓冲区大小
	EventBusBuffer int `mapstructure:"event_bus_buffer"`

	// Default 默认资源配置
	Default ResourceConfig `mapstructure:"default"`

	// Resources 资源级配置（覆盖 Default）
	Resources map[string]ResourceConfig `mapstructure:"resources"`
}

// ResourceConfig 资源级配置
type ResourceConfig struct {
	// Strategy 熔断策略名称: error_rate, slow_call_rate, consecutive, adaptive
	Strategy string `mapstructure:"strategy"`

	// MinRequests 最小请求数（避免小流量误判）
	MinRequests int `mapstructure:"min_requests"`

	// ErrorRateThreshold 错误率阈值 (0.0-1.0)
	ErrorRateThreshold float64 `mapstructure:"error_rate_threshold"`

	// SlowCallThreshold 慢调用阈值
	SlowCallThreshold time.Duration `mapstructure:"slow_call_threshold"`

	// SlowRateThreshold 慢调用比例阈值 (0.0-1.0)
	SlowRateThreshold float64 `mapstructure:"slow_rate_threshold"`

	// ConsecutiveFailures 连续失败次数阈值
	ConsecutiveFailures int `mapstructure:"consecutive_failures"`

	// Timeout 熔断超时时间（Open 状态持续时间）
	Timeout time.Duration `mapstructure:"timeout"`

	// HalfOpenRequests 半开状态允许的请求数
	HalfOpenRequests int `mapstructure:"half_open_requests"`

	// WindowSize 滑动窗口大小
	WindowSize time.Duration `mapstructure:"window_size"`

	// BucketSize 时间桶大小
	BucketSize time.Duration `mapstructure:"bucket_size"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		Enabled:        false, // 默认不启用
		EventBusBuffer: 500,
		Default:        DefaultResourceConfig(),
		Resources:      make(map[string]ResourceConfig),
	}
}

// DefaultResourceConfig 返回默认资源配置
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

// Validate 验证配置
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil // 未启用，不需要验证
	}

	if c.EventBusBuffer <= 0 {
		c.EventBusBuffer = 500
	}

	// 验证默认配置
	if err := c.Default.Validate(); err != nil {
		return err
	}

	// 合并并验证资源配置
	for name, cfg := range c.Resources {
		// 合并默认配置（资源配置中未设置的字段使用默认值）
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

// Merge 合并配置（override 覆盖默认值）
func (rc ResourceConfig) Merge(override ResourceConfig) ResourceConfig {
	result := rc // 从默认配置开始

	// 只覆盖非零值
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

// Validate 验证资源配置
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

// GetResourceConfig 获取资源配置（优先使用资源级，否则使用默认）
func (c *Config) GetResourceConfig(resource string) ResourceConfig {
	if cfg, ok := c.Resources[resource]; ok {
		return cfg
	}
	return c.Default
}

// ValidationError 配置验证错误
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
