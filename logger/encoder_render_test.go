package logger

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

// TestPrettyConsoleEncoder_KeyValueStyle 测试键值对渲染样式
func TestPrettyConsoleEncoder_KeyValueStyle(t *testing.T) {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:       "time",
		LevelKey:      "level",
		NameKey:       "logger",
		CallerKey:     "caller",
		MessageKey:    "message",
		StacktraceKey: "stacktrace",
		LineEnding:    zapcore.DefaultLineEnding,
		EncodeLevel:   zapcore.CapitalLevelEncoder,
		EncodeTime:    zapcore.ISO8601TimeEncoder,
		EncodeCaller:  zapcore.ShortCallerEncoder,
	}

	// 创建键值对样式编码器
	encoder := NewPrettyConsoleEncoderWithStyle(encoderConfig, RenderStyleKeyValue)

	entry := zapcore.Entry{
		Level:   zapcore.InfoLevel,
		Time:    time.Date(2025, 12, 23, 1, 10, 1, 165000000, time.FixedZone("CST", 8*3600)),
		Message: "[GIN-debug] GET / --> handler.Index (4 handlers)",
		Caller: zapcore.EntryCaller{
			Defined: true,
			File:    "logger/manager.go",
			Line:    316,
		},
	}

	fields := []zapcore.Field{
		{Key: "module", Type: zapcore.StringType, String: "gin-route"},
		{Key: "order_id", Type: zapcore.StringType, String: "001"},
		{Key: "amount", Type: zapcore.Int64Type, Integer: 99}, // 使用整数
	}

	buf, err := encoder.EncodeEntry(entry, fields)
	assert.NoError(t, err)

	output := buf.String()
	t.Logf("输出:\n%s", output)

	// 验证输出格式
	assert.Contains(t, output, "🔵 INFO | 2025-12-23 01:10:01.165")
	assert.Contains(t, output, "trace: -")
	assert.Contains(t, output, "module: gin-route")
	assert.Contains(t, output, "caller: logger/manager.go:316")
	assert.Contains(t, output, "message: [GIN-debug] GET / --> handler.Index (4 handlers)")
	assert.Contains(t, output, `fields: {"order_id":"001","amount":99}`)
}

// TestPrettyConsoleEncoder_KeyValueStyle_WithTraceID 测试带 TraceID 的键值对渲染
func TestPrettyConsoleEncoder_KeyValueStyle_WithTraceID(t *testing.T) {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:       "time",
		LevelKey:      "level",
		NameKey:       "logger",
		CallerKey:     "caller",
		MessageKey:    "message",
		StacktraceKey: "stacktrace",
		LineEnding:    zapcore.DefaultLineEnding,
		EncodeLevel:   zapcore.CapitalLevelEncoder,
		EncodeTime:    zapcore.ISO8601TimeEncoder,
		EncodeCaller:  zapcore.ShortCallerEncoder,
	}

	encoder := NewPrettyConsoleEncoderWithStyle(encoderConfig, RenderStyleKeyValue)

	entry := zapcore.Entry{
		Level:   zapcore.WarnLevel,
		Time:    time.Now(),
		Message: "用户登录失败",
		Caller: zapcore.EntryCaller{
			Defined: true,
			File:    "auth/service.go",
			Line:    89,
		},
	}

	fields := []zapcore.Field{
		{Key: "trace_id", Type: zapcore.StringType, String: "47dfd756-254f-4f"},
		{Key: "module", Type: zapcore.StringType, String: "auth"},
		{Key: "user_id", Type: zapcore.Int64Type, Integer: 123},
	}

	buf, err := encoder.EncodeEntry(entry, fields)
	assert.NoError(t, err)

	output := buf.String()
	t.Logf("输出:\n%s", output)

	// 验证 TraceID 被正确渲染
	assert.Contains(t, output, "trace: 47dfd756-254f-4f")
	assert.Contains(t, output, "module: auth")
	assert.Contains(t, output, `fields: {"user_id":123}`)
}

// TestPrettyConsoleEncoder_KeyValueStyle_NoFields 测试无字段的键值对渲染
func TestPrettyConsoleEncoder_KeyValueStyle_NoFields(t *testing.T) {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:       "time",
		LevelKey:      "level",
		NameKey:       "logger",
		CallerKey:     "caller",
		MessageKey:    "message",
		StacktraceKey: "stacktrace",
		LineEnding:    zapcore.DefaultLineEnding,
		EncodeLevel:   zapcore.CapitalLevelEncoder,
		EncodeTime:    zapcore.ISO8601TimeEncoder,
		EncodeCaller:  zapcore.ShortCallerEncoder,
	}

	encoder := NewPrettyConsoleEncoderWithStyle(encoderConfig, RenderStyleKeyValue)

	entry := zapcore.Entry{
		Level:   zapcore.DebugLevel,
		Time:    time.Now(),
		Message: "简单的调试信息",
		Caller: zapcore.EntryCaller{
			Defined: true,
			File:    "main.go",
			Line:    10,
		},
	}

	fields := []zapcore.Field{
		{Key: "module", Type: zapcore.StringType, String: "core"},
	}

	buf, err := encoder.EncodeEntry(entry, fields)
	assert.NoError(t, err)

	output := buf.String()
	t.Logf("输出:\n%s", output)

	// 验证无字段时不显示 fields 行
	assert.Contains(t, output, "🟢 DEBU")
	assert.Contains(t, output, "module: core")
	assert.Contains(t, output, "message: 简单的调试信息")
	assert.NotContains(t, output, "fields:")
}

// TestPrettyConsoleEncoder_KeyValueStyle_WithStack 测试带栈追踪的键值对渲染
func TestPrettyConsoleEncoder_KeyValueStyle_WithStack(t *testing.T) {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:       "time",
		LevelKey:      "level",
		NameKey:       "logger",
		CallerKey:     "caller",
		MessageKey:    "message",
		StacktraceKey: "stacktrace",
		LineEnding:    zapcore.DefaultLineEnding,
		EncodeLevel:   zapcore.CapitalLevelEncoder,
		EncodeTime:    zapcore.ISO8601TimeEncoder,
		EncodeCaller:  zapcore.ShortCallerEncoder,
	}

	encoder := NewPrettyConsoleEncoderWithStyle(encoderConfig, RenderStyleKeyValue)

	entry := zapcore.Entry{
		Level:   zapcore.ErrorLevel,
		Time:    time.Now(),
		Message: "数据库连接失败",
		Caller: zapcore.EntryCaller{
			Defined: true,
			File:    "db/connection.go",
			Line:    45,
		},
		Stack: "goroutine 1 [running]:\nmain.main()\n\t/app/main.go:10",
	}

	fields := []zapcore.Field{
		{Key: "module", Type: zapcore.StringType, String: "database"},
		{Key: "error", Type: zapcore.StringType, String: "connection timeout"},
	}

	buf, err := encoder.EncodeEntry(entry, fields)
	assert.NoError(t, err)

	output := buf.String()
	t.Logf("输出:\n%s", output)

	// 验证栈追踪被正确渲染
	assert.Contains(t, output, "🔴 ERRO")
	assert.Contains(t, output, "module: database")
	assert.Contains(t, output, "message: 数据库连接失败")
	assert.Contains(t, output, "stack:")
	assert.Contains(t, output, "goroutine 1 [running]:")
}

// TestParseRenderStyle 测试渲染样式解析
func TestParseRenderStyle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected RenderStyle
	}{
		{"空字符串应返回默认值", "", RenderStyleSingleLine},
		{"single_line", "single_line", RenderStyleSingleLine},
		{"key_value", "key_value", RenderStyleKeyValue},
		{"未知值应返回默认值", "unknown", RenderStyleSingleLine},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseRenderStyle(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
