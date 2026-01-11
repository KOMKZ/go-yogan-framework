package logger

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// TestPrettyConsoleEncoder_ErrorField 测试 error 字段显示
func TestPrettyConsoleEncoder_ErrorField(t *testing.T) {
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
		Level:   zapcore.ErrorLevel,
		Time:    time.Now(),
		Message: "数据库错误",
		Caller:  zapcore.NewEntryCaller(0, "db/connection.go", 45, true),
	}

	// 测试 zap.Error 字段
	testErr := errors.New("连接超时")
	fields := []zapcore.Field{
		zap.String("module", "database"),
		zap.Error(testErr),
		zap.String("host", "localhost"),
	}

	buf, err := enc.EncodeEntry(entry, fields)
	assert.NoError(t, err)

	output := buf.String()
	t.Logf("输出:\n%s", output)

	// 验证 error 字段正确显示
	assert.Contains(t, output, "[🔴ERRO]")
	assert.Contains(t, output, "[database]")
	assert.Contains(t, output, "数据库错误")
	assert.Contains(t, output, `"error":"连接超时"`) // ✅ 关键验证
	assert.Contains(t, output, `"host":"localhost"`)
	assert.NotContains(t, output, `"error":null`) // ❌ 不应该是 null
}

