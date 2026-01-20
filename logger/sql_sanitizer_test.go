package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSanitizeSQL test SQL sanitization
func TestSanitizeSQL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "脱敏密码（带等号格式）",
			input:    "UPDATE users SET password = 'secret123' WHERE id = 1",
			expected: "UPDATE users SET password = '***' WHERE id = 1",
		},
		{
			name:     "脱敏手机号",
			input:    "SELECT * FROM users WHERE phone = '13812345678'",
			expected: "SELECT * FROM users WHERE phone = '138****5678'",
		},
		{
			name:     "无敏感信息",
			input:    "SELECT * FROM products WHERE price > 100",
			expected: "SELECT * FROM products WHERE price > 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeSQL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestSanitizePassword test password sanitization
func TestSanitizePassword(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "单引号密码",
			input:    "password = 'secret123'",
			expected: "password = '***'",
		},
		{
			name:     "双引号密码",
			input:    `password = "secret123"`,
			expected: `password = "***"`,
		},
		{
			name:     "无空格",
			input:    "password='mypassword'",
			expected: "password='***'",
		},
		{
			name:     "大写 PASSWORD",
			input:    "PASSWORD = 'Secret'",
			expected: "PASSWORD = '***'",
		},
		{
			name:     "混合大小写",
			input:    "PassWord = 'test'",
			expected: "PassWord = '***'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizePassword(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestSanitizePhone test phone number desensitization
func TestSanitizePhone(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "标准手机号",
			input:    "13812345678",
			expected: "138****5678",
		},
		{
			name:     "SQL 中的手机号",
			input:    "phone = '13912345678'",
			expected: "phone = '139****5678'",
		},
		{
			name:     "多个手机号",
			input:    "13812345678,13987654321",
			expected: "138****5678,139****4321",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizePhone(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestSanitizeIDCard test ID card desensitization
func TestSanitizeIDCard(t *testing.T) {
	// Note: The current regex will match any consecutive 14 digits, keeping the first 6 and last 4 digits
	// This is effective for pure ID card scenarios, but may conflict when used with phone numbers
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "纯数字身份证",
			input:    "ID:110101199001011234",
			expected: "ID:110101********1234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeIDCard(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
