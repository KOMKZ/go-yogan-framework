package logger

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

// TestPrettyConsoleEncoder_KeyValueStyle test key-value style rendering
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

	// Create key-value pair style encoder
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
		{Key: "amount", Type: zapcore.Int64Type, Integer: 99}, // Use integer
	}

	buf, err := encoder.EncodeEntry(entry, fields)
	assert.NoError(t, err)

	output := buf.String()
	t.Logf("输出:\n%s", output)

	// Verify output format
	assert.Contains(t, output, "🔵 INFO | 2025-12-23 01:10:01.165")
	assert.Contains(t, output, "trace: -")
	assert.Contains(t, output, "module: gin-route")
	assert.Contains(t, output, "caller: logger/manager.go:316")
	assert.Contains(t, output, "message: [GIN-debug] GET / --> handler.Index (4 handlers)")
	assert.Contains(t, output, `fields: {"order_id":"001","amount":99}`)
}

// TestPrettyConsoleEncoder_KeyValueStyle_WithTraceID_Test key-value rendering with TraceID
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

	// Verify that the TraceID is rendered correctly
	assert.Contains(t, output, "trace: 47dfd756-254f-4f")
	assert.Contains(t, output, "module: auth")
	assert.Contains(t, output, `fields: {"user_id":123}`)
}

// TestPrettyConsoleEncoder_KeyValueStyle_NoFields_TestKeyValueRenderingWithNoFields
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

	// Do not display fields row when there are no fields
	assert.Contains(t, output, "🟢 DEBU")
	assert.Contains(t, output, "module: core")
	assert.Contains(t, output, "message: 简单的调试信息")
	assert.NotContains(t, output, "fields:")
}

// TestPrettyConsoleEncoder_KeyValueStyle_WithStack_TestKeyValueRenderingWithStackTrace
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

	// Verify that the stack trace is rendered correctly
	assert.Contains(t, output, "🔴 ERRO")
	assert.Contains(t, output, "module: database")
	assert.Contains(t, output, "message: 数据库连接失败")
	assert.Contains(t, output, "stack:")
	assert.Contains(t, output, "goroutine 1 [running]:")
}

// TestParseRenderStyle test render style parsing
func TestParseRenderStyle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected RenderStyle
	}{
		{"空字符串应返回默认值", "", RenderStyleSingleLine},
		{"single_line", "single_line", RenderStyleSingleLine},
		{"key_value", "key_value", RenderStyleKeyValue},
		{"modern_compact", "modern_compact", RenderStyleModernCompact},
		{"未知值应返回默认值", "unknown", RenderStyleSingleLine},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseRenderStyle(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestPrettyConsoleEncoder_ModernCompactStyle test modern compact style rendering
func TestPrettyConsoleEncoder_ModernCompactStyle(t *testing.T) {
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

	encoder := NewPrettyConsoleEncoderWithStyle(encoderConfig, RenderStyleModernCompact)

	entry := zapcore.Entry{
		Level:   zapcore.InfoLevel,
		Time:    time.Date(2025, 1, 13, 14, 30, 45, 0, time.FixedZone("CST", 8*3600)),
		Message: "HTTP server started",
		Caller: zapcore.EntryCaller{
			Defined: true,
			File:    "http_app.go",
			Line:    104,
		},
	}

	fields := []zapcore.Field{
		{Key: "module", Type: zapcore.StringType, String: "yogan"},
		{Key: "port", Type: zapcore.Int64Type, Integer: 8080},
	}

	buf, err := encoder.EncodeEntry(entry, fields)
	assert.NoError(t, err)

	output := buf.String()
	t.Logf("输出:\n%s", output)

	// Validate format: 14:30:45 │ INFO │ HTTP server started │ yogan {"port":8080}
	assert.Contains(t, output, "14:30:45")
	assert.Contains(t, output, "│")
	assert.Contains(t, output, "INFO")
	assert.Contains(t, output, "HTTP server started")
	assert.Contains(t, output, "yogan")
	assert.Contains(t, output, `"port":8080`)
}

// TestPrettyConsoleEncoder_ModernCompactStyle_AllLevels_test_all_log_levels
func TestPrettyConsoleEncoder_ModernCompactStyle_AllLevels(t *testing.T) {
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

	encoder := NewPrettyConsoleEncoderWithStyle(encoderConfig, RenderStyleModernCompact)

	levels := []struct {
		level    zapcore.Level
		expected string
	}{
		{zapcore.DebugLevel, "DEBUG"},
		{zapcore.InfoLevel, "INFO"},
		{zapcore.WarnLevel, "WARN"},
		{zapcore.ErrorLevel, "ERROR"},
	}

	for _, tt := range levels {
		t.Run(tt.expected, func(t *testing.T) {
			entry := zapcore.Entry{
				Level:   tt.level,
				Time:    time.Now(),
				Message: "Test message",
			}

			fields := []zapcore.Field{
				{Key: "module", Type: zapcore.StringType, String: "test"},
			}

			buf, err := encoder.EncodeEntry(entry, fields)
			assert.NoError(t, err)

			output := buf.String()
			t.Logf("输出: %s", output)

			assert.Contains(t, output, tt.expected)
			assert.Contains(t, output, "│")
		})
	}
}

// TestPrettyConsoleEncoder_ModernCompactStyle_LongMessage test long message truncation
func TestPrettyConsoleEncoder_ModernCompactStyle_LongMessage(t *testing.T) {
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

	encoder := NewPrettyConsoleEncoderWithStyle(encoderConfig, RenderStyleModernCompact)

	// Create a long message
	longMessage := "This is a very long message that should be truncated because it exceeds the maximum width"

	entry := zapcore.Entry{
		Level:   zapcore.WarnLevel,
		Time:    time.Now(),
		Message: longMessage,
	}

	fields := []zapcore.Field{
		{Key: "module", Type: zapcore.StringType, String: "database"},
	}

	buf, err := encoder.EncodeEntry(entry, fields)
	assert.NoError(t, err)

	output := buf.String()
	t.Logf("输出: %s", output)

	// Verify that the message is truncated and an ellipsis is added
	assert.Contains(t, output, "...")
	assert.Contains(t, output, "database")
}

// TestPrettyConsoleEncoder_ModernCompactStyle_ChineseMessage test Chinese message alignment
func TestPrettyConsoleEncoder_ModernCompactStyle_ChineseMessage(t *testing.T) {
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

	encoder := NewPrettyConsoleEncoderWithStyle(encoderConfig, RenderStyleModernCompact)

	// Test multiple mixed Chinese-English logs
	entries := []struct {
		level   zapcore.Level
		message string
		module  string
	}{
		{zapcore.InfoLevel, "服务器启动成功", "yogan"},
		{zapcore.DebugLevel, "Route registered: GET /api", "router"},
		{zapcore.WarnLevel, "数据库连接超时", "database"},
		{zapcore.ErrorLevel, "用户认证失败", "auth"},
		{zapcore.InfoLevel, "订单创建成功，订单号：12345", "order"},
	}

	t.Log("中文对齐测试输出:")
	for _, e := range entries {
		entry := zapcore.Entry{
			Level:   e.level,
			Time:    time.Date(2025, 1, 13, 14, 30, 45, 0, time.FixedZone("CST", 8*3600)),
			Message: e.message,
		}

		fields := []zapcore.Field{
			{Key: "module", Type: zapcore.StringType, String: e.module},
		}

		buf, err := encoder.EncodeEntry(entry, fields)
		assert.NoError(t, err)

		output := buf.String()
		t.Logf("%s", output)

		// Verify that the output contains key elements
		assert.Contains(t, output, "│")
		assert.Contains(t, output, e.module)
	}
}

// TestPrettyConsoleEncoder_ModernCompactStyle_LongChineseMessage Test long Chinese message truncation
func TestPrettyConsoleEncoder_ModernCompactStyle_LongChineseMessage(t *testing.T) {
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

	encoder := NewPrettyConsoleEncoderWithStyle(encoderConfig, RenderStyleModernCompact)

	// Create a very long Chinese message
	longMessage := "这是一条非常长的中文日志消息，应该被正确截断，不会破坏表格对齐"

	entry := zapcore.Entry{
		Level:   zapcore.WarnLevel,
		Time:    time.Now(),
		Message: longMessage,
	}

	fields := []zapcore.Field{
		{Key: "module", Type: zapcore.StringType, String: "database"},
	}

	buf, err := encoder.EncodeEntry(entry, fields)
	assert.NoError(t, err)

	output := buf.String()
	t.Logf("输出: %s", output)

	// Verify that the message is truncated and an ellipsis is added
	assert.Contains(t, output, "...")
	assert.Contains(t, output, "database")
}

// TestPrettyConsoleEncoder_ModernCompactStyle_NoFields test with no additional fields
func TestPrettyConsoleEncoder_ModernCompactStyle_NoFields(t *testing.T) {
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

	encoder := NewPrettyConsoleEncoderWithStyle(encoderConfig, RenderStyleModernCompact)

	entry := zapcore.Entry{
		Level:   zapcore.DebugLevel,
		Time:    time.Now(),
		Message: "Simple debug message",
	}

	// Only the module field is present, no other business fields.
	fields := []zapcore.Field{
		{Key: "module", Type: zapcore.StringType, String: "core"},
	}

	buf, err := encoder.EncodeEntry(entry, fields)
	assert.NoError(t, err)

	output := buf.String()
	t.Logf("输出: %s", output)

	// Verify that there are no JSON fields outputted
	assert.Contains(t, output, "DEBUG")
	assert.Contains(t, output, "Simple debug message")
	assert.Contains(t, output, "core")
	assert.NotContains(t, output, "{}")
}
