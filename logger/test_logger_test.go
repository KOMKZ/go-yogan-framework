package logger

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestTestCtxLogger 测试 TestCtxLogger 的所有方法
func TestTestCtxLogger(t *testing.T) {
	// 创建测试 Logger
	logger := NewTestCtxLogger()
	assert.NotNil(t, logger)

	ctx := context.Background()
	ctxWithTrace := context.WithValue(ctx, "trace_id", "test-trace-123")

	// 测试 InfoCtx
	logger.InfoCtx(ctx, "Info 消息", zap.String("key", "value"))

	// 测试 DebugCtx
	logger.DebugCtx(ctx, "Debug 消息", zap.Int("count", 10))

	// 测试 WarnCtx
	logger.WarnCtx(ctx, "Warn 消息", zap.Bool("flag", true))

	// 测试 ErrorCtx
	logger.ErrorCtx(ctx, "Error 消息", zap.Error(nil))

	// 测试带 TraceID 的日志
	logger.InfoCtx(ctxWithTrace, "带 TraceID 的消息")

	// 测试 HasLog（使用大写级别，与 TestCtxLogger 的存储格式一致）
	assert.True(t, logger.HasLog("INFO", "Info 消息"))
	assert.True(t, logger.HasLog("DEBUG", "Debug 消息"))
	assert.True(t, logger.HasLog("WARN", "Warn 消息"))
	assert.True(t, logger.HasLog("ERROR", "Error 消息"))
	assert.False(t, logger.HasLog("INFO", "不存在的消息"))

	// 测试 HasLogWithTraceID
	assert.True(t, logger.HasLogWithTraceID("INFO", "带 TraceID 的消息", "test-trace-123"))
	assert.False(t, logger.HasLogWithTraceID("INFO", "带 TraceID 的消息", "wrong-trace"))

	// 测试 HasLogWithField
	assert.True(t, logger.HasLogWithField("INFO", "Info 消息", "key", "value"))
	assert.True(t, logger.HasLogWithField("DEBUG", "Debug 消息", "count", int64(10))) // zap.Int 会被编码为 int64
	assert.False(t, logger.HasLogWithField("INFO", "Info 消息", "key", "wrong"))

	// 测试 CountLogs
	assert.Equal(t, 2, logger.CountLogs("INFO")) // Info 消息 + 带 TraceID 的消息
	assert.Equal(t, 1, logger.CountLogs("DEBUG"))
	assert.Equal(t, 1, logger.CountLogs("WARN"))
	assert.Equal(t, 1, logger.CountLogs("ERROR"))

	// 测试 Logs
	allLogs := logger.Logs()
	assert.GreaterOrEqual(t, len(allLogs), 5) // 至少有 5 条日志

	// 测试 Clear
	logger.Clear()
	assert.Equal(t, 0, logger.CountLogs("info"))
	assert.Equal(t, 0, logger.CountLogs("error"))
}

// TestTestCtxLogger_With 测试 With 方法
func TestTestCtxLogger_With(t *testing.T) {
	logger := NewTestCtxLogger()

	// 使用 With 创建新 Logger
	orderLogger := logger.With(
		zap.String("service", "order-service"),
		zap.Int64("order_id", 12345),
	)

	// 新 Logger 应该存在且不为 nil
	assert.NotNil(t, orderLogger)

	// 新 Logger 可以记录日志
	orderLogger.InfoCtx(context.Background(), "订单创建")

	// 新 Logger 自己能看到记录
	assert.True(t, orderLogger.HasLog("INFO", "订单创建"))
}
