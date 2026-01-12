package limiter

import (
	"time"
)

// Config 限流器配置
type Config struct {
	// Enabled 是否启用限流器（false时直接透传）
	Enabled bool `mapstructure:"enabled"`

	// StoreType 存储类型: memory, redis
	StoreType string `mapstructure:"store_type"`

	// Redis 配置（StoreType=redis时必需）
	Redis RedisInstanceConfig `mapstructure:"redis"`

	// EventBusBuffer 事件总线缓冲区大小
	EventBusBuffer int `mapstructure:"event_bus_buffer"`

	// KeyFunc 资源键生成方式（用于中间件）
	// 可选值：path, ip, user, path_ip, api_key（默认 path）
	KeyFunc string `mapstructure:"key_func"`

	// SkipPaths 跳过限流的路径列表（用于中间件）
	SkipPaths []string `mapstructure:"skip_paths"`

	// Default 默认资源配置（如果配置了有效的 default，自动应用到未配置资源）
	Default ResourceConfig `mapstructure:"default"`

	// Resources 资源级配置（覆盖Default）
	Resources map[string]ResourceConfig `mapstructure:"resources"`
}

// ResourceConfig 资源级配置
type ResourceConfig struct {
	// Algorithm 限流算法: token_bucket, sliding_window, concurrency, adaptive
	Algorithm string `mapstructure:"algorithm"`

	// 令牌桶配置
	Rate       int64 `mapstructure:"rate"`        // 令牌生成速率（个/秒）
	Capacity   int64 `mapstructure:"capacity"`    // 桶容量
	InitTokens int64 `mapstructure:"init_tokens"` // 初始令牌数

	// 滑动窗口配置
	Limit      int64         `mapstructure:"limit"`       // 窗口内最大请求数
	WindowSize time.Duration `mapstructure:"window_size"` // 窗口大小
	BucketSize time.Duration `mapstructure:"bucket_size"` // 时间桶大小

	// 并发限流配置
	MaxConcurrency int64         `mapstructure:"max_concurrency"` // 最大并发数
	Timeout        time.Duration `mapstructure:"timeout"`         // 等待超时

	// 自适应限流配置
	MinLimit       int64         `mapstructure:"min_limit"`       // 最小限流值
	MaxLimit       int64         `mapstructure:"max_limit"`       // 最大限流值
	TargetCPU      float64       `mapstructure:"target_cpu"`      // 目标CPU使用率
	TargetMemory   float64       `mapstructure:"target_memory"`   // 目标内存使用率
	TargetLoad     float64       `mapstructure:"target_load"`     // 目标系统负载
	AdjustInterval time.Duration `mapstructure:"adjust_interval"` // 调整间隔
}

// RedisInstanceConfig Redis实例引用配置（复用内核redis组件）
type RedisInstanceConfig struct {
	Instance  string `mapstructure:"instance"`   // Redis 实例名称（在 redis.instances 中配置）
	KeyPrefix string `mapstructure:"key_prefix"` // Redis key 前缀（默认 "limiter:"）
}

// DefaultConfig 返回默认配置
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

// DefaultResourceConfig 返回默认资源配置
func DefaultResourceConfig() ResourceConfig {
	return ResourceConfig{
		Algorithm:  string(AlgorithmTokenBucket),
		Rate:       100,             // 100 QPS
		Capacity:   200,             // 允许200突发
		InitTokens: 200,             // 初始满桶
		WindowSize: 1 * time.Second, // 1秒窗口
		Timeout:    1 * time.Second, // 1秒超时
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

	// 验证存储类型
	if c.StoreType != string(StoreTypeMemory) && c.StoreType != string(StoreTypeRedis) {
		return &ValidationError{Field: "store_type", Message: "must be 'memory' or 'redis'"}
	}

	// 验证Redis配置
	if c.StoreType == string(StoreTypeRedis) {
		if c.Redis.Instance == "" {
			return &ValidationError{Field: "redis.instance", Message: "redis instance name is required"}
		}
		// 设置默认 key 前缀
		if c.Redis.KeyPrefix == "" {
			c.Redis.KeyPrefix = "limiter:"
		}
	}

	// 验证默认配置（如果有配置的话）
	// 如果 default 配置了有效值，则验证；如果是空配置则跳过
	if !c.Default.isEmpty() {
		if err := c.Default.Validate(); err != nil {
			return err
		}
	}

	// 合并并验证资源配置
	for name, cfg := range c.Resources {
		// 如果 default 有效，则合并默认配置
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

// Merge 合并配置（override覆盖默认值）
func (rc ResourceConfig) Merge(override ResourceConfig) ResourceConfig {
	result := rc // 从默认配置开始

	// 只覆盖非零值
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

// isEmpty 判断 ResourceConfig 是否为空配置
func (rc ResourceConfig) isEmpty() bool {
	return rc.Algorithm == "" &&
		rc.Rate == 0 &&
		rc.Capacity == 0 &&
		rc.Limit == 0 &&
		rc.MaxConcurrency == 0 &&
		rc.MinLimit == 0 &&
		rc.MaxLimit == 0
}

// Validate 验证资源配置
func (rc *ResourceConfig) Validate() error {
	// 验证算法类型
	algo := AlgorithmType(rc.Algorithm)
	if algo != AlgorithmTokenBucket && algo != AlgorithmSlidingWindow &&
		algo != AlgorithmConcurrency && algo != AlgorithmAdaptive {
		return &ValidationError{Field: "algorithm", Message: "invalid algorithm type"}
	}

	// 根据算法类型验证配置
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
			rc.BucketSize = 100 * time.Millisecond // 默认100ms
		}
		if rc.WindowSize < rc.BucketSize {
			return &ValidationError{Field: "window_size", Message: "must be >= bucket_size"}
		}

	case AlgorithmConcurrency:
		if rc.MaxConcurrency <= 0 {
			return &ValidationError{Field: "max_concurrency", Message: "must be > 0"}
		}
		if rc.Timeout <= 0 {
			rc.Timeout = 1 * time.Second // 默认1秒
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
			rc.AdjustInterval = 10 * time.Second // 默认10秒
		}
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
