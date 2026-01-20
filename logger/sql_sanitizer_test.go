package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSanitizeSQL 测试 SQL 脱敏
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

// TestSanitizePassword 测试密码脱敏
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

// TestSanitizePhone 测试手机号脱敏
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

// TestSanitizeIDCard 测试身份证脱敏
func TestSanitizeIDCard(t *testing.T) {
	// 注意：当前正则会匹配任何 14 位连续数字，保留前6位后4位
	// 这对于纯身份证场景有效，但与手机号混用时可能有冲突
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
