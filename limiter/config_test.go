package limiter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "未启用配置",
			config: Config{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name: "默认配置",
			config: Config{
				Enabled:   true,
				StoreType: "memory",
				Default:   DefaultResourceConfig(),
				Resources: make(map[string]ResourceConfig),
			},
			wantErr: false,
		},
		{
			name: "无效的存储类型",
			config: Config{
				Enabled:   true,
				StoreType: "invalid",
			},
			wantErr: true,
		},
		{
			name: "Redis配置缺少实例名",
			config: Config{
				Enabled:   true,
				StoreType: "redis",
				Redis:     RedisInstanceConfig{},
			},
			wantErr: true,
		},
		{
			name: "合法的Redis配置",
			config: Config{
				Enabled:   true,
				StoreType: "redis",
				Redis: RedisInstanceConfig{
					Instance:  "main",
					KeyPrefix: "limiter:",
				},
				Default: DefaultResourceConfig(),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestResourceConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  ResourceConfig
		wantErr bool
	}{
		{
			name: "令牌桶合法配置",
			config: ResourceConfig{
				Algorithm:  "token_bucket",
				Rate:       100,
				Capacity:   200,
				InitTokens: 200,
			},
			wantErr: false,
		},
		{
			name: "令牌桶Rate为0",
			config: ResourceConfig{
				Algorithm: "token_bucket",
				Rate:      0,
				Capacity:  100,
			},
			wantErr: true,
		},
		{
			name: "令牌桶Capacity为0",
			config: ResourceConfig{
				Algorithm: "token_bucket",
				Rate:      100,
				Capacity:  0,
			},
			wantErr: true,
		},
		{
			name: "令牌桶InitTokens超过Capacity",
			config: ResourceConfig{
				Algorithm:  "token_bucket",
				Rate:       100,
				Capacity:   200,
				InitTokens: 300,
			},
			wantErr: true,
		},
		{
			name: "滑动窗口合法配置",
			config: ResourceConfig{
				Algorithm:  "sliding_window",
				Limit:      1000,
				WindowSize: 1 * time.Second,
				BucketSize: 100 * time.Millisecond,
			},
			wantErr: false,
		},
		{
			name: "滑动窗口Limit为0",
			config: ResourceConfig{
				Algorithm:  "sliding_window",
				Limit:      0,
				WindowSize: 1 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "滑动窗口WindowSize为0",
			config: ResourceConfig{
				Algorithm: "sliding_window",
				Limit:     1000,
			},
			wantErr: true,
		},
		{
			name: "并发限流合法配置",
			config: ResourceConfig{
				Algorithm:      "concurrency",
				MaxConcurrency: 50,
				Timeout:        5 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "并发限流MaxConcurrency为0",
			config: ResourceConfig{
				Algorithm:      "concurrency",
				MaxConcurrency: 0,
			},
			wantErr: true,
		},
		{
			name: "自适应限流合法配置",
			config: ResourceConfig{
				Algorithm:      "adaptive",
				MinLimit:       100,
				MaxLimit:       1000,
				TargetCPU:      0.7,
				TargetMemory:   0.8,
				TargetLoad:     0.75,
				AdjustInterval: 10 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "自适应限流MinLimit为0",
			config: ResourceConfig{
				Algorithm: "adaptive",
				MinLimit:  0,
				MaxLimit:  1000,
			},
			wantErr: true,
		},
		{
			name: "自适应限流MaxLimit为0",
			config: ResourceConfig{
				Algorithm: "adaptive",
				MinLimit:  100,
				MaxLimit:  0,
			},
			wantErr: true,
		},
		{
			name: "自适应限流MinLimit大于MaxLimit",
			config: ResourceConfig{
				Algorithm: "adaptive",
				MinLimit:  1000,
				MaxLimit:  100,
			},
			wantErr: true,
		},
		{
			name: "无效的算法类型",
			config: ResourceConfig{
				Algorithm: "invalid_algo",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestResourceConfig_Merge(t *testing.T) {
	defaultCfg := ResourceConfig{
		Algorithm:  "token_bucket",
		Rate:       100,
		Capacity:   200,
		InitTokens: 200,
		Timeout:    1 * time.Second,
	}

	overrideCfg := ResourceConfig{
		Rate:     50,
		Capacity: 100,
	}

	merged := defaultCfg.Merge(overrideCfg)

	// The covered fields should use the override values
	assert.Equal(t, int64(50), merged.Rate)
	assert.Equal(t, int64(100), merged.Capacity)

	// Fields not covered should use default values
	assert.Equal(t, "token_bucket", merged.Algorithm)
	assert.Equal(t, int64(200), merged.InitTokens)
	assert.Equal(t, 1*time.Second, merged.Timeout)
}

func TestConfig_GetResourceConfig(t *testing.T) {
	cfg := Config{
		Default: ResourceConfig{
			Algorithm: "token_bucket",
			Rate:      100,
			Capacity:  200,
		},
		Resources: map[string]ResourceConfig{
			"api1": {
				Algorithm: "sliding_window",
				Limit:     1000,
			},
		},
	}

	// Get configured resources
	resCfg := cfg.GetResourceConfig("api1")
	assert.Equal(t, "sliding_window", resCfg.Algorithm)
	assert.Equal(t, int64(1000), resCfg.Limit)

	// Get resources that are not configured, should return default configuration
	resCfg = cfg.GetResourceConfig("api2")
	assert.Equal(t, "token_bucket", resCfg.Algorithm)
	assert.Equal(t, int64(100), resCfg.Rate)
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.False(t, cfg.Enabled)
	assert.Equal(t, "memory", cfg.StoreType)
	assert.Equal(t, 500, cfg.EventBusBuffer)
	assert.NotNil(t, cfg.Resources)
}

func TestDefaultResourceConfig(t *testing.T) {
	cfg := DefaultResourceConfig()

	assert.Equal(t, "token_bucket", cfg.Algorithm)
	assert.Equal(t, int64(100), cfg.Rate)
	assert.Equal(t, int64(200), cfg.Capacity)
	assert.Equal(t, int64(200), cfg.InitTokens)
	assert.Equal(t, 1*time.Second, cfg.WindowSize)
	assert.Equal(t, 1*time.Second, cfg.Timeout)
}

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  ValidationError
		want string
	}{
		{
			name: "资源错误",
			err: ValidationError{
				Resource: "api1",
				Err:      assert.AnError,
			},
			want: "limiter config validation failed for resource 'api1': assert.AnError general error for testing",
		},
		{
			name: "字段错误",
			err: ValidationError{
				Field:   "rate",
				Message: "must be > 0",
			},
			want: "limiter config validation failed for field 'rate': must be > 0",
		},
		{
			name: "通用错误",
			err: ValidationError{
				Err: assert.AnError,
			},
			want: "limiter config validation failed: assert.AnError general error for testing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			assert.Contains(t, got, "limiter config validation failed")
		})
	}
}

func TestConfig_Validate_Merge(t *testing.T) {
	cfg := Config{
		Enabled:   true,
		StoreType: "memory",
		Default: ResourceConfig{
			Algorithm: "token_bucket",
			Rate:      100,
			Capacity:  200,
		},
		Resources: map[string]ResourceConfig{
			"api1": {
				Rate:     50, // Only cover Rate
				Capacity: 100, // Only cover Capacity
			},
		},
	}

	err := cfg.Validate()
	require.NoError(t, err)

	// Validate the merged configuration
	api1Cfg := cfg.Resources["api1"]
	assert.Equal(t, "token_bucket", api1Cfg.Algorithm) // Inherit default configuration
	assert.Equal(t, int64(50), api1Cfg.Rate)           // coverage value
	assert.Equal(t, int64(100), api1Cfg.Capacity)      // coverage value
}

