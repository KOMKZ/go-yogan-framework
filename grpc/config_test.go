package grpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestClientConfig_GetTimeout test default timeout configuration
func TestClientConfig_GetTimeout(t *testing.T) {
	tests := []struct {
		name     string
		config   ClientConfig
		expected int
	}{
		{
			name: "配置了超时时间",
			config: ClientConfig{
				Timeout: 10,
			},
			expected: 10,
		},
		{
			name: "未配置超时时间（默认5秒）",
			config: ClientConfig{
				Timeout: 0,
			},
			expected: 5,
		},
		{
			name: "配置了负数超时时间（默认5秒）",
			config: ClientConfig{
				Timeout: -1,
			},
			expected: 5,
		},
		{
			name: "配置了1秒超时",
			config: ClientConfig{
				Timeout: 1,
			},
			expected: 1,
		},
		{
			name: "配置了30秒超时",
			config: ClientConfig{
				Timeout: 30,
			},
			expected: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.config.GetTimeout()
			assert.Equal(t, tt.expected, actual, "超时时间应该匹配")
		})
	}
}

// TestClientConfig_GetMode test connection mode determination
func TestClientConfig_GetMode(t *testing.T) {
	tests := []struct {
		name     string
		config   ClientConfig
		expected string
	}{
		{
			name: "显式指定 etcd 模式",
			config: ClientConfig{
				DiscoveryMode: "etcd",
				ServiceName:   "test-service",
			},
			expected: "etcd",
		},
		{
			name: "显式指定 direct 模式",
			config: ClientConfig{
				DiscoveryMode: "direct",
				Target:        "127.0.0.1:9000",
			},
			expected: "direct",
		},
		{
			name: "未指定模式但配置了 Target（推断为 direct）",
			config: ClientConfig{
				Target: "127.0.0.1:9000",
			},
			expected: "direct",
		},
		{
			name: "未指定模式但配置了 ServiceName（推断为 etcd）",
			config: ClientConfig{
				ServiceName: "test-service",
			},
			expected: "etcd",
		},
		{
			name: "未指定任何配置（默认 direct）",
			config: ClientConfig{},
			expected: "direct",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.config.GetMode()
			assert.Equal(t, tt.expected, actual, "连接模式应该匹配")
		})
	}
}

// TestClientConfig_Validate test configuration validation
func TestClientConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  ClientConfig
		wantErr bool
	}{
		{
			name: "direct 模式配置正确",
			config: ClientConfig{
				Target: "127.0.0.1:9000",
			},
			wantErr: false,
		},
		{
			name: "direct 模式缺少 target",
			config: ClientConfig{
				DiscoveryMode: "direct",
			},
			wantErr: true,
		},
		{
			name: "etcd 模式配置正确",
			config: ClientConfig{
				DiscoveryMode: "etcd",
				ServiceName:   "test-service",
			},
			wantErr: false,
		},
		{
			name: "etcd 模式缺少 service_name",
			config: ClientConfig{
				DiscoveryMode: "etcd",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err, "应该返回错误")
			} else {
				assert.NoError(t, err, "不应该返回错误")
			}
		})
	}
}

// TestClientConfig_IsLogEnabled test log configuration
func TestClientConfig_IsLogEnabled(t *testing.T) {
	tests := []struct {
		name     string
		config   ClientConfig
		expected bool
	}{
		{
			name:     "未配置（默认启用）",
			config:   ClientConfig{},
			expected: true,
		},
		{
			name: "显式启用",
			config: ClientConfig{
				EnableLog: boolPtr(true),
			},
			expected: true,
		},
		{
			name: "显式禁用",
			config: ClientConfig{
				EnableLog: boolPtr(false),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.config.IsLogEnabled()
			assert.Equal(t, tt.expected, actual, "日志启用状态应该匹配")
		})
	}
}

// boolPtr helper function: returns a bool pointer
func boolPtr(b bool) *bool {
	return &b
}

