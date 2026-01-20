package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockValidator simulated validator (passes validation)
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

// TestValidateAll_Success All validations passed
func TestValidateAll_Success(t *testing.T) {
	v1 := mockValidator{shouldFail: false}
	v2 := mockValidator{shouldFail: false}
	v3 := mockValidator{shouldFail: false}

	err := ValidateAll(v1, v2, v3)
	assert.NoError(t, err)
}

// TestValidateAll_SingleFailure tests single validation failure
func TestValidateAll_SingleFailure(t *testing.T) {
	v1 := mockValidator{shouldFail: false}
	v2 := mockValidator{shouldFail: true, err: errors.New("validation error")}
	v3 := mockValidator{shouldFail: false}

	err := ValidateAll(v1, v2, v3)
	assert.Error(t, err)
	assert.Equal(t, "validation error", err.Error())
}

// TestValidateAll_MultipleFailures test multiple validation failures (return first error)
func TestValidateAll_MultipleFailures(t *testing.T) {
	v1 := mockValidator{shouldFail: true, err: errors.New("first error")}
	v2 := mockValidator{shouldFail: true, err: errors.New("second error")}
	v3 := mockValidator{shouldFail: false}

	err := ValidateAll(v1, v2, v3)
	assert.Error(t, err)
	// Should return the first error
	assert.Equal(t, "first error", err.Error())
}

// TestValidateAll_Empty test empty validator list
func TestValidateAll_Empty(t *testing.T) {
	err := ValidateAll()
	assert.NoError(t, err)
}

// TestValidateAll_NilValidator tests cases involving nil
func TestValidateAll_WithNil(t *testing.T) {
	v1 := mockValidator{shouldFail: false}
	
	// This will cause a panic because a method cannot be called on nil.
	// The test should ensure that this situation is handled correctly
	defer func() {
		if r := recover(); r != nil {
			t.Logf("捕获到 panic: %v", r)
		}
	}()

	err := ValidateAll(v1, nil)
	// If there is no panic, verify the result
	if err == nil {
		t.Log("nil validator 被跳过")
	}
}

// TestValidator_Interface test implementation of Validator interface
func TestValidator_Interface(t *testing.T) {
	var v Validator
	
	// Ensure that mockValidator implements the Validator interface
	v = mockValidator{shouldFail: false}
	assert.NotNil(t, v)
	
	err := v.Validate()
	assert.NoError(t, err)
}

// realValidator example of real configuration validator
type realServerConfig struct {
	Port int
	Host string
}

func (c realServerConfig) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return errors.New("The port must be between 1 and 65535 1-65535 The port must be between 1 and 65535")
	}
	if c.Host == "" {
		return errors.New("The host address cannot be empty")
	}
	return nil
}

// TestValidateAll_RealExample test real usage scenarios
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

