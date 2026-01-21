package logger

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// TestPrettyConsoleEncoder_ErrorField test error field display
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

	// Test the zap.Error field
	testErr := errors.New("Connection timeout")
	fields := []zapcore.Field{
		zap.String("module", "database"),
		zap.Error(testErr),
		zap.String("host", "localhost"),
	}

	buf, err := enc.EncodeEntry(entry, fields)
	assert.NoError(t, err)

	output := buf.String()
	t.Logf("输出:\n%s", output)

	// Verify that the error field is displayed correctly
	assert.Contains(t, output, "[🔴ERRO]")
	assert.Contains(t, output, "[database]")
	assert.Contains(t, output, "数据库错误")
	assert.Contains(t, output, `"error":"Connection timeout"`) // Validating key correctness
	assert.Contains(t, output, `"host":"localhost"`)
	assert.NotContains(t, output, `"error":null`) // Should not be null
}

