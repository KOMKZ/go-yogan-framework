package logger

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestTestCtxLogger test all methods of TestCtxLogger
func TestTestCtxLogger(t *testing.T) {
	// Create test Logger
	logger := NewTestCtxLogger()
	assert.NotNil(t, logger)

	ctx := context.Background()
	ctxWithTrace := context.WithValue(ctx, "trace_id", "test-trace-123")

	// Test InfoCtx
	logger.InfoCtx(ctx, "Info 消息", zap.String("key", "value"))

	// Test DebugCtx
	logger.DebugCtx(ctx, "Debug 消息", zap.Int("count", 10))

	// Test WarnCtx
	logger.WarnCtx(ctx, "Warn 消息", zap.Bool("flag", true))

	// Test ErrorCtx
	logger.ErrorCtx(ctx, "Error 消息", zap.Error(nil))

	// Test logs with TraceID
	logger.InfoCtx(ctxWithTrace, "带 TraceID 的消息")

	// Test HasLog (using uppercase level, consistent with TestCtxLogger storage format)
	assert.True(t, logger.HasLog("INFO", "Info 消息"))
	assert.True(t, logger.HasLog("DEBUG", "Debug 消息"))
	assert.True(t, logger.HasLog("WARN", "Warn 消息"))
	assert.True(t, logger.HasLog("ERROR", "Error 消息"))
	assert.False(t, logger.HasLog("INFO", "不存在的消息"))

	// Test HasLogWithTraceID
	assert.True(t, logger.HasLogWithTraceID("INFO", "带 TraceID 的消息", "test-trace-123"))
	assert.False(t, logger.HasLogWithTraceID("INFO", "带 TraceID 的消息", "wrong-trace"))

	// Test HasLogWithField
	assert.True(t, logger.HasLogWithField("INFO", "Info 消息", "key", "value"))
	assert.True(t, logger.HasLogWithField("DEBUG", "Debug 消息", "count", int64(10))) // zap.Int will be encoded as int64
	assert.False(t, logger.HasLogWithField("INFO", "Info 消息", "key", "wrong"))

	// Test CountLogs
	assert.Equal(t, 2, logger.CountLogs("INFO")) // Info message + message with TraceID
	assert.Equal(t, 1, logger.CountLogs("DEBUG"))
	assert.Equal(t, 1, logger.CountLogs("WARN"))
	assert.Equal(t, 1, logger.CountLogs("ERROR"))

	// Test Logs
	allLogs := logger.Logs()
	assert.GreaterOrEqual(t, len(allLogs), 5) // There are at least 5 logs

	// Test Clear
	logger.Clear()
	assert.Equal(t, 0, logger.CountLogs("info"))
	assert.Equal(t, 0, logger.CountLogs("error"))
}

// TestTestCtxLogger_With test the With method
func TestTestCtxLogger_With(t *testing.T) {
	logger := NewTestCtxLogger()

	// Use With to create new Logger
	orderLogger := logger.With(
		zap.String("service", "order-service"),
		zap.Int64("order_id", 12345),
	)

	// The new Logger should exist and not be nil
	assert.NotNil(t, orderLogger)

	// The new Logger can record logs
	orderLogger.InfoCtx(context.Background(), "订单创建")

	// The new Logger can see its own records
	assert.True(t, orderLogger.HasLog("INFO", "订单创建"))
}
