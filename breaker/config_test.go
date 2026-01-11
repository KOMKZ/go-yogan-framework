package breaker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultConfig 测试默认配置
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	
	assert.False(t, cfg.Enabled, "默认不启用")
	assert.Equal(t, 500, cfg.EventBusBuffer)
	assert.NotNil(t, cfg.Default)
	assert.NotNil(t, cfg.Resources)
}

// TestDefaultResourceConfig 测试默认资源配置
func TestDefaultResourceConfig(t *testing.T) {
	cfg := DefaultResourceConfig()
	
	assert.Equal(t, "error_rate", cfg.Strategy)
	assert.Equal(t, 20, cfg.MinRequests)
	assert.Equal(t, 0.5, cfg.ErrorRateThreshold)
	assert.Equal(t, time.Second, cfg.SlowCallThreshold)
	assert.Equal(t, 0.5, cfg.SlowRateThreshold)
	assert.Equal(t, 5, cfg.ConsecutiveFailures)
	assert.Equal(t, 30*time.Second, cfg.Timeout)
	assert.Equal(t, 3, cfg.HalfOpenRequests)
	assert.Equal(t, 10*time.Second, cfg.WindowSize)
	assert.Equal(t, time.Second, cfg.BucketSize)
}

// TestConfig_Validate 测试配置验证
func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "未启用时不验证",
			config: Config{
				Enabled: false,
				Default: ResourceConfig{}, // 无效配置
			},
			wantErr: false,
		},
		{
			name: "启用时验证默认配置",
			config: Config{
				Enabled: true,
				Default: ResourceConfig{
					MinRequests: -1, // 无效
				},
			},
			wantErr: true,
			errMsg:  "MinRequests",
		},
		{
			name: "验证资源配置",
			config: Config{
				Enabled: true,
				Default: DefaultResourceConfig(),
				Resources: map[string]ResourceConfig{
					"test-service": {
						ErrorRateThreshold: 1.5, // 无效
					},
				},
			},
			wantErr: true,
			errMsg:  "test-service",
		},
		{
			name: "有效配置",
			config: Config{
				Enabled: true,
				Default: DefaultResourceConfig(),
			},
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestResourceConfig_Validate 测试资源配置验证
func TestResourceConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  ResourceConfig
		wantErr bool
		field   string
	}{
		{
			name:    "MinRequests为负数",
			config:  ResourceConfig{MinRequests: -1},
			wantErr: true,
			field:   "MinRequests",
		},
		{
			name: "ErrorRateThreshold超出范围",
			config: ResourceConfig{
				MinRequests:        0,
				ErrorRateThreshold: 1.5,
			},
			wantErr: true,
			field:   "ErrorRateThreshold",
		},
		{
			name: "SlowRateThreshold超出范围",
			config: ResourceConfig{
				MinRequests:        0,
				ErrorRateThreshold: 0.5,
				SlowRateThreshold:  -0.1,
			},
			wantErr: true,
			field:   "SlowRateThreshold",
		},
		{
			name: "ConsecutiveFailures为负数",
			config: ResourceConfig{
				MinRequests:         0,
				ErrorRateThreshold:  0.5,
				SlowRateThreshold:   0.5,
				ConsecutiveFailures: -1,
			},
			wantErr: true,
			field:   "ConsecutiveFailures",
		},
		{
			name: "Timeout为零",
			config: ResourceConfig{
				MinRequests:         0,
				ErrorRateThreshold:  0.5,
				SlowRateThreshold:   0.5,
				ConsecutiveFailures: 0,
				Timeout:             0,
			},
			wantErr: true,
			field:   "Timeout",
		},
		{
			name: "HalfOpenRequests为零",
			config: ResourceConfig{
				MinRequests:         0,
				ErrorRateThreshold:  0.5,
				SlowRateThreshold:   0.5,
				ConsecutiveFailures: 0,
				Timeout:             time.Second,
				HalfOpenRequests:    0,
			},
			wantErr: true,
			field:   "HalfOpenRequests",
		},
		{
			name: "WindowSize为零",
			config: ResourceConfig{
				MinRequests:         0,
				ErrorRateThreshold:  0.5,
				SlowRateThreshold:   0.5,
				ConsecutiveFailures: 0,
				Timeout:             time.Second,
				HalfOpenRequests:    1,
				WindowSize:          0,
			},
			wantErr: true,
			field:   "WindowSize",
		},
		{
			name: "BucketSize为零",
			config: ResourceConfig{
				MinRequests:         0,
				ErrorRateThreshold:  0.5,
				SlowRateThreshold:   0.5,
				ConsecutiveFailures: 0,
				Timeout:             time.Second,
				HalfOpenRequests:    1,
				WindowSize:          time.Second,
				BucketSize:          0,
			},
			wantErr: true,
			field:   "BucketSize",
		},
		{
			name: "WindowSize小于BucketSize",
			config: ResourceConfig{
				MinRequests:         0,
				ErrorRateThreshold:  0.5,
				SlowRateThreshold:   0.5,
				ConsecutiveFailures: 0,
				Timeout:             time.Second,
				HalfOpenRequests:    1,
				WindowSize:          time.Second,
				BucketSize:          2 * time.Second,
			},
			wantErr: true,
			field:   "WindowSize",
		},
		{
			name:    "有效配置",
			config:  DefaultResourceConfig(),
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.field != "" {
					assert.Contains(t, err.Error(), tt.field)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestConfig_GetResourceConfig 测试获取资源配置
func TestConfig_GetResourceConfig(t *testing.T) {
	defaultCfg := DefaultResourceConfig()
	defaultCfg.MinRequests = 10
	
	customCfg := DefaultResourceConfig()
	customCfg.MinRequests = 50
	
	cfg := Config{
		Default: defaultCfg,
		Resources: map[string]ResourceConfig{
			"custom-service": customCfg,
		},
	}
	
	t.Run("获取自定义配置", func(t *testing.T) {
		resCfg := cfg.GetResourceConfig("custom-service")
		assert.Equal(t, 50, resCfg.MinRequests)
	})
	
	t.Run("获取默认配置", func(t *testing.T) {
		resCfg := cfg.GetResourceConfig("unknown-service")
		assert.Equal(t, 10, resCfg.MinRequests)
	})
}

// TestState_String 测试状态字符串
func TestState_String(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateClosed, "Closed"},
		{StateOpen, "Open"},
		{StateHalfOpen, "HalfOpen"},
		{State(999), "Unknown"},
	}
	
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.state.String())
		})
	}
}

// TestState_Methods 测试状态判断方法
func TestState_Methods(t *testing.T) {
	t.Run("StateClosed", func(t *testing.T) {
		state := StateClosed
		assert.True(t, state.IsClosed())
		assert.False(t, state.IsOpen())
		assert.False(t, state.IsHalfOpen())
	})
	
	t.Run("StateOpen", func(t *testing.T) {
		state := StateOpen
		assert.False(t, state.IsClosed())
		assert.True(t, state.IsOpen())
		assert.False(t, state.IsHalfOpen())
	})
	
	t.Run("StateHalfOpen", func(t *testing.T) {
		state := StateHalfOpen
		assert.False(t, state.IsClosed())
		assert.False(t, state.IsOpen())
		assert.True(t, state.IsHalfOpen())
	})
}

// TestValidationError 测试验证错误
func TestValidationError(t *testing.T) {
	tests := []struct {
		name string
		err  *ValidationError
		want string
	}{
		{
			name: "资源+子错误",
			err: &ValidationError{
				Resource: "test-service",
				Err:      assert.AnError,
			},
			want: "breaker config validation failed for resource 'test-service'",
		},
		{
			name: "资源+字段",
			err: &ValidationError{
				Resource: "test-service",
				Field:    "MinRequests",
				Message:  "must be >= 0",
			},
			want: "breaker config validation failed for resource 'test-service.MinRequests': must be >= 0",
		},
		{
			name: "字段",
			err: &ValidationError{
				Field:   "ErrorRateThreshold",
				Message: "must be between 0.0 and 1.0",
			},
			want: "breaker config validation failed for field 'ErrorRateThreshold': must be between 0.0 and 1.0",
		},
		{
			name: "子错误",
			err: &ValidationError{
				Err: assert.AnError,
			},
			want: "breaker config validation failed",
		},
		{
			name: "空错误",
			err:  &ValidationError{},
			want: "breaker config validation failed",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			assert.Contains(t, got, tt.want)
		})
	}
}

