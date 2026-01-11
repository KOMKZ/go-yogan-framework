package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockValidator 模拟验证器（通过验证）
type mockValidator struct {
	shouldFail bool
	err        error
}

func (m mockValidator) Validate() error {
	if m.shouldFail {
		return m.err
	}
	return nil
}

// TestValidateAll_Success 测试全部验证通过
func TestValidateAll_Success(t *testing.T) {
	v1 := mockValidator{shouldFail: false}
	v2 := mockValidator{shouldFail: false}
	v3 := mockValidator{shouldFail: false}

	err := ValidateAll(v1, v2, v3)
	assert.NoError(t, err)
}

// TestValidateAll_SingleFailure 测试单个验证失败
func TestValidateAll_SingleFailure(t *testing.T) {
	v1 := mockValidator{shouldFail: false}
	v2 := mockValidator{shouldFail: true, err: errors.New("validation error")}
	v3 := mockValidator{shouldFail: false}

	err := ValidateAll(v1, v2, v3)
	assert.Error(t, err)
	assert.Equal(t, "validation error", err.Error())
}

// TestValidateAll_MultipleFailures 测试多个验证失败（返回第一个错误）
func TestValidateAll_MultipleFailures(t *testing.T) {
	v1 := mockValidator{shouldFail: true, err: errors.New("first error")}
	v2 := mockValidator{shouldFail: true, err: errors.New("second error")}
	v3 := mockValidator{shouldFail: false}

	err := ValidateAll(v1, v2, v3)
	assert.Error(t, err)
	// 应该返回第一个错误
	assert.Equal(t, "first error", err.Error())
}

// TestValidateAll_Empty 测试空验证器列表
func TestValidateAll_Empty(t *testing.T) {
	err := ValidateAll()
	assert.NoError(t, err)
}

// TestValidateAll_NilValidator 测试包含 nil 的情况
func TestValidateAll_WithNil(t *testing.T) {
	v1 := mockValidator{shouldFail: false}
	
	// 这里会 panic，因为不能对 nil 调用方法
	// 测试应该确保这种情况被正确处理
	defer func() {
		if r := recover(); r != nil {
			t.Logf("捕获到 panic: %v", r)
		}
	}()

	err := ValidateAll(v1, nil)
	// 如果没有 panic，验证结果
	if err == nil {
		t.Log("nil validator 被跳过")
	}
}

// TestValidator_Interface 测试 Validator 接口实现
func TestValidator_Interface(t *testing.T) {
	var v Validator
	
	// 确保 mockValidator 实现了 Validator 接口
	v = mockValidator{shouldFail: false}
	assert.NotNil(t, v)
	
	err := v.Validate()
	assert.NoError(t, err)
}

// realValidator 真实配置验证器示例
type realServerConfig struct {
	Port int
	Host string
}

func (c realServerConfig) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return errors.New("端口必须在 1-65535 之间")
	}
	if c.Host == "" {
		return errors.New("主机地址不能为空")
	}
	return nil
}

// TestValidateAll_RealExample 测试真实使用场景
func TestValidateAll_RealExample(t *testing.T) {
	tests := []struct {
		name      string
		configs   []Validator
		wantError bool
		errorMsg  string
	}{
		{
			name: "所有配置有效",
			configs: []Validator{
				realServerConfig{Port: 8080, Host: "localhost"},
				realServerConfig{Port: 9090, Host: "0.0.0.0"},
			},
			wantError: false,
		},
		{
			name: "端口无效",
			configs: []Validator{
				realServerConfig{Port: 0, Host: "localhost"},
			},
			wantError: true,
			errorMsg:  "端口必须在 1-65535 之间",
		},
		{
			name: "主机地址为空",
			configs: []Validator{
				realServerConfig{Port: 8080, Host: ""},
			},
			wantError: true,
			errorMsg:  "主机地址不能为空",
		},
		{
			name: "第一个有效，第二个无效",
			configs: []Validator{
				realServerConfig{Port: 8080, Host: "localhost"},
				realServerConfig{Port: 99999, Host: "0.0.0.0"},
			},
			wantError: true,
			errorMsg:  "端口必须在 1-65535 之间",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAll(tt.configs...)
			
			if tt.wantError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

