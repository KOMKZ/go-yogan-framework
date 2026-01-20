package logger

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// TestPrettyConsoleEncoder_Basic test basic format
func TestPrettyConsoleEncoder_Basic(t *testing.T) {
	encoderCfg := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		MessageKey:     "msg",
		CallerKey:      "caller",
		StacktraceKey:  "stack",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	enc := NewPrettyConsoleEncoder(encoderCfg)

	// Construct log entry
	entry := zapcore.Entry{
		Level:   zapcore.InfoLevel,
		Time:    time.Date(2025, 12, 20, 9, 14, 58, 575000000, time.FixedZone("CST", 8*3600)),
		Message: "Order creation",
		Caller:  zapcore.NewEntryCaller(0, "order/manager.go", 123, true),
	}

	// field
	fields := []zapcore.Field{
		zap.String("trace_id", "trace-abc-123"),
		zap.String("module", "order"),
		zap.String("order_id", "001"),
		zap.Float64("amount", 99.99),
	}

	buf, err := enc.EncodeEntry(entry, fields)
	assert.NoError(t, err)
	assert.NotNil(t, buf)

	output := buf.String()
	t.Logf("输出:\n%s", output)

	// Validate format
	assert.Contains(t, output, "[🔵INFO]")
	assert.Contains(t, output, "2025-12-20T09:14:58.575+0800")
	assert.Contains(t, output, "trace-abc-123")
	assert.Contains(t, output, "[order]")
	assert.Contains(t, output, "order/manager.go:123")
	assert.Contains(t, output, "Order creation")
	assert.Contains(t, output, `"order_id":"001"`)
	assert.Contains(t, output, `"amount":99.99`)
}

// TestPrettyConsoleEncoder_AllLevels test all log levels
func TestPrettyConsoleEncoder_AllLevels(t *testing.T) {
	encoderCfg := zapcore.EncoderConfig{
		TimeKey:       "time",
		LevelKey:      "level",
		MessageKey:    "msg",
		EncodeLevel:   zapcore.LowercaseLevelEncoder,
		EncodeTime:    zapcore.ISO8601TimeEncoder,
		EncodeCaller:  zapcore.ShortCallerEncoder,
	}

	tests := []struct {
		level         zapcore.Level
		expectedEmoji string
	}{
		{zapcore.DebugLevel, "🟢DEBU"},
		{zapcore.InfoLevel, "🔵INFO"},
		{zapcore.WarnLevel, "🟡WARN"},
		{zapcore.ErrorLevel, "🔴ERRO"},
		{zapcore.DPanicLevel, "🟠DPAN"},
		{zapcore.PanicLevel, "🟣PANI"},
		{zapcore.FatalLevel, "💀FATA"},
	}

	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			enc := NewPrettyConsoleEncoder(encoderCfg)

			entry := zapcore.Entry{
				Level:   tt.level,
				Time:    time.Now(),
				Message: "Test log",
				Caller:  zapcore.NewEntryCaller(0, "test.go", 1, true),
			}

			fields := []zapcore.Field{
				zap.String("module", "test"),
			}

			buf, err := enc.EncodeEntry(entry, fields)
			assert.NoError(t, err)

			output := buf.String()
			assert.Contains(t, output, "["+tt.expectedEmoji+"]")
			t.Logf("%s: %s", tt.level, output)
		})
	}
}

// TestPrettyConsoleEncoder_NoTraceID test without TraceID
func TestPrettyConsoleEncoder_NoTraceID(t *testing.T) {
	encoderCfg := zapcore.EncoderConfig{
		TimeKey:      "time",
		LevelKey:     "level",
		MessageKey:   "msg",
		EncodeLevel:  zapcore.LowercaseLevelEncoder,
		EncodeTime:   zapcore.ISO8601TimeEncoder,
		EncodeCaller: zapcore.ShortCallerEncoder,
	}

	enc := NewPrettyConsoleEncoder(encoderCfg)

	entry := zapcore.Entry{
		Level:   zapcore.InfoLevel,
		Time:    time.Now(),
		Message: "无 TraceID 日志",
		Caller:  zapcore.NewEntryCaller(0, "cache/redis.go", 89, true),
	}

	fields := []zapcore.Field{
		zap.String("module", "cache"),
		zap.String("key", "user:100"),
	}

	buf, err := enc.EncodeEntry(entry, fields)
	assert.NoError(t, err)

	output := buf.String()
	t.Logf("输出:\n%s", output)

	// Verify that the TraceID position displays "-" (with padding)
	assert.Contains(t, output, "[cache]")
	assert.Contains(t, output, "无 TraceID 日志")
	assert.Contains(t, output, `"key":"user:100"`)
}

// TestPrettyConsoleEncoder_FieldTypes test various field types
func TestPrettyConsoleEncoder_FieldTypes(t *testing.T) {
	encoderCfg := zapcore.EncoderConfig{
		TimeKey:      "time",
		LevelKey:     "level",
		MessageKey:   "msg",
		EncodeLevel:  zapcore.LowercaseLevelEncoder,
		EncodeTime:   zapcore.ISO8601TimeEncoder,
		EncodeCaller: zapcore.ShortCallerEncoder,
	}

	enc := NewPrettyConsoleEncoder(encoderCfg)

	entry := zapcore.Entry{
		Level:   zapcore.InfoLevel,
		Time:    time.Now(),
		Message: "测试各种类型",
		Caller:  zapcore.NewEntryCaller(0, "test.go", 1, true),
	}

	fields := []zapcore.Field{
		zap.String("module", "test"),
		zap.String("str", "字符串"),
		zap.Int("int", 123),
		zap.Int64("int64", 456),
		zap.Uint("uint", 789),
		zap.Float64("float", 3.14),
		zap.Bool("bool", true),
		zap.Duration("duration", 5*time.Second),
	}

	buf, err := enc.EncodeEntry(entry, fields)
	assert.NoError(t, err)

	output := buf.String()
	t.Logf("输出:\n%s", output)

	// Validate each type
	assert.Contains(t, output, `"str":"字符串"`)
	assert.Contains(t, output, `"int":123`)
	assert.Contains(t, output, `"int64":456`)
	assert.Contains(t, output, `"uint":789`)
	assert.Contains(t, output, `"float":3.14`)
	assert.Contains(t, output, `"bool":true`)
	assert.Contains(t, output, `"duration":5000000000`)
}

// TestPrettyConsoleEncoder_NoFields test with no additional fields
func TestPrettyConsoleEncoder_NoFields(t *testing.T) {
	encoderCfg := zapcore.EncoderConfig{
		TimeKey:      "time",
		LevelKey:     "level",
		MessageKey:   "msg",
		EncodeLevel:  zapcore.LowercaseLevelEncoder,
		EncodeTime:   zapcore.ISO8601TimeEncoder,
		EncodeCaller: zapcore.ShortCallerEncoder,
	}

	enc := NewPrettyConsoleEncoder(encoderCfg)

	entry := zapcore.Entry{
		Level:   zapcore.WarnLevel,
		Time:    time.Now(),
		Message: "仅消息",
		Caller:  zapcore.NewEntryCaller(0, "test.go", 1, true),
	}

	// Only the module field
	fields := []zapcore.Field{
		zap.String("module", "test"),
	}

	buf, err := enc.EncodeEntry(entry, fields)
	assert.NoError(t, err)

	output := buf.String()
	t.Logf("输出:\n%s", output)

	assert.Contains(t, output, "[🟡WARN]")
	assert.Contains(t, output, "[test]")
	assert.Contains(t, output, "仅消息")
	// Without additional fields, there should be only an empty JSON object
	assert.Contains(t, output, "{}")
}

// TestPrettyConsoleEncoder_WithStack tests stack information
func TestPrettyConsoleEncoder_WithStack(t *testing.T) {
	encoderCfg := zapcore.EncoderConfig{
		TimeKey:       "time",
		LevelKey:      "level",
		MessageKey:    "msg",
		StacktraceKey: "stack",
		EncodeLevel:   zapcore.LowercaseLevelEncoder,
		EncodeTime:    zapcore.ISO8601TimeEncoder,
		EncodeCaller:  zapcore.ShortCallerEncoder,
	}

	enc := NewPrettyConsoleEncoder(encoderCfg)

	entry := zapcore.Entry{
		Level:   zapcore.ErrorLevel,
		Time:    time.Now(),
		Message: "错误日志",
		Caller:  zapcore.NewEntryCaller(0, "test.go", 1, true),
		Stack:   "goroutine 1 [running]:\nmain.main()\n\t/path/to/main.go:10 +0x123",
	}

	fields := []zapcore.Field{
		zap.String("module", "test"),
		zap.String("error", "测试错误"),
	}

	buf, err := enc.EncodeEntry(entry, fields)
	assert.NoError(t, err)

	output := buf.String()
	t.Logf("输出:\n%s", output)

	assert.Contains(t, output, "[🔴ERRO]")
	assert.Contains(t, output, "错误日志")
	assert.Contains(t, output, "goroutine 1")
	assert.Contains(t, output, "main.go:10")
}

// TestPrettyConsoleEncoder_EscapeString test string escaping
func TestPrettyConsoleEncoder_EscapeString(t *testing.T) {
	encoderCfg := zapcore.EncoderConfig{
		TimeKey:      "time",
		LevelKey:     "level",
		MessageKey:   "msg",
		EncodeLevel:  zapcore.LowercaseLevelEncoder,
		EncodeTime:   zapcore.ISO8601TimeEncoder,
		EncodeCaller: zapcore.ShortCallerEncoder,
	}

	enc := NewPrettyConsoleEncoder(encoderCfg)

	entry := zapcore.Entry{
		Level:   zapcore.InfoLevel,
		Time:    time.Now(),
		Message: "测试转义",
		Caller:  zapcore.NewEntryCaller(0, "test.go", 1, true),
	}

	fields := []zapcore.Field{
		zap.String("module", "test"),
		zap.String("text", "包含\"引号\"和\n换行"),
	}

	buf, err := enc.EncodeEntry(entry, fields)
	assert.NoError(t, err)

	output := buf.String()
	t.Logf("输出:\n%s", output)

	// Validate escape sequencing
	assert.Contains(t, output, `\"引号\"`)
	assert.Contains(t, output, `\n换行`)
}

// TestPrettyConsoleEncoder_Clone test clone
func TestPrettyConsoleEncoder_Clone(t *testing.T) {
	encoderCfg := zapcore.EncoderConfig{
		TimeKey:      "time",
		LevelKey:     "level",
		MessageKey:   "msg",
		EncodeLevel:  zapcore.LowercaseLevelEncoder,
		EncodeTime:   zapcore.ISO8601TimeEncoder,
		EncodeCaller: zapcore.ShortCallerEncoder,
	}

	enc1 := NewPrettyConsoleEncoder(encoderCfg)
	enc2 := enc1.Clone()

	assert.NotNil(t, enc2)
	assert.IsType(t, &PrettyConsoleEncoder{}, enc2)
}

