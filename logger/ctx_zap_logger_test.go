package logger

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestCtxZapLogger_AllMethods 测试 CtxZapLogger 的所有方法
func TestCtxZapLogger_AllMethods(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "ctx_logger")

	// 重置全局 Manager
	globalManager = nil
	managerOnce = sync.Once{}

	InitManager(ManagerConfig{
		BaseLogDir:            logDir,
		Level:                 "debug",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		EnableTraceID:         true,
		TraceIDKey:            "trace_id",
		TraceIDFieldName:      "trace_id",
		EnableStacktrace:      true,
		StacktraceLevel:       "error",
		StacktraceDepth:       5,
		MaxSize:               10,
	})

	logger := GetLogger("test")
	ctx := context.WithValue(context.Background(), "trace_id", "test-trace-123")

	// 测试 InfoCtx
	logger.InfoCtx(ctx, "Info 消息", zap.String("key", "value"))

	// 测试 Info（不带 ctx）
	logger.Info("Info 不带 ctx")

	// 测试 DebugCtx
	logger.DebugCtx(ctx, "Debug 消息", zap.Int("count", 10))

	// 测试 Debug（不带 ctx）
	logger.Debug("Debug 不带 ctx")

	// 测试 WarnCtx
	logger.WarnCtx(ctx, "Warn 消息", zap.Bool("flag", true))

	// 测试 Warn（不带 ctx）
	logger.Warn("Warn 不带 ctx")

	// 测试 ErrorCtx（会自动添加堆栈）
	logger.ErrorCtx(ctx, "Error 消息", zap.Error(nil))

	// 测试 Error（不带 ctx）
	logger.Error("Error 不带 ctx")

	CloseAll()

	// 验证日志文件存在
	assert.FileExists(t, filepath.Join(logDir, "test", "test-info.log"))
	assert.FileExists(t, filepath.Join(logDir, "test", "test-error.log"))

	// 验证 info 日志内容
	infoContent, _ := os.ReadFile(filepath.Join(logDir, "test", "test-info.log"))
	infoStr := string(infoContent)
	assert.Contains(t, infoStr, "Info 消息")
	assert.Contains(t, infoStr, "trace_id")
	assert.Contains(t, infoStr, "test-trace-123")
	assert.Contains(t, infoStr, "Debug 消息")
	assert.Contains(t, infoStr, "Warn 消息")

	// 验证 error 日志内容
	errorContent, _ := os.ReadFile(filepath.Join(logDir, "test", "test-error.log"))
	errorStr := string(errorContent)
	assert.Contains(t, errorStr, "Error 消息")
	assert.Contains(t, errorStr, "stack") // 应该包含堆栈
}

// TestCtxZapLogger_With 测试 With 方法
func TestCtxZapLogger_With(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "with_logger")

	globalManager = nil
	managerOnce = sync.Once{}

	InitManager(ManagerConfig{
		BaseLogDir:            logDir,
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
	})

	logger := GetLogger("order")

	// 使用 With 添加预设字段
	orderLogger := logger.With(
		zap.String("service", "order-service"),
		zap.Int64("order_id", 12345),
	)

	orderLogger.InfoCtx(context.Background(), "订单创建")
	orderLogger.InfoCtx(context.Background(), "订单支付")

	CloseAll()

	// 验证预设字段存在
	content, _ := os.ReadFile(filepath.Join(logDir, "order", "order-info.log"))
	contentStr := string(content)
	assert.Contains(t, contentStr, "service")
	assert.Contains(t, contentStr, "order-service")
	assert.Contains(t, contentStr, "order_id")
	assert.Contains(t, contentStr, "12345")
	assert.Contains(t, contentStr, "订单创建")
	assert.Contains(t, contentStr, "订单支付")
}

// TestCtxZapLogger_GetZapLogger 测试 GetZapLogger 方法
func TestCtxZapLogger_GetZapLogger(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "zap_logger")

	globalManager = nil
	managerOnce = sync.Once{}

	InitManager(ManagerConfig{
		BaseLogDir:            logDir,
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
	})

	logger := GetLogger("test")

	// 获取底层 zap.Logger
	zapLogger := logger.GetZapLogger()
	assert.NotNil(t, zapLogger)

	// 使用底层 Logger 记录
	zapLogger.Info("直接使用 zap.Logger")

	CloseAll()

	// 验证日志文件
	content, _ := os.ReadFile(filepath.Join(logDir, "test", "test-info.log"))
	assert.Contains(t, string(content), "直接使用 zap.Logger")
}

// TestCtxZapLogger_TraceIDFromDifferentKeys 测试不同的 TraceID key
func TestCtxZapLogger_TraceIDFromDifferentKeys(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "trace_keys")

	globalManager = nil
	managerOnce = sync.Once{}

	InitManager(ManagerConfig{
		BaseLogDir:            logDir,
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		EnableTraceID:         true,
		TraceIDKey:            "custom_trace",
		TraceIDFieldName:      "request_id",
		MaxSize:               10,
	})

	logger := GetLogger("test")

	// 测试自定义 key
	ctx1 := context.WithValue(context.Background(), "custom_trace", "custom-trace-456")
	logger.InfoCtx(ctx1, "自定义 key 测试")

	// 测试标准 trace_id key（回退）
	ctx2 := context.WithValue(context.Background(), "trace_id", "standard-trace-789")
	logger.InfoCtx(ctx2, "标准 key 测试")

	// 测试 traceId key（兼容性回退）
	ctx3 := context.WithValue(context.Background(), "traceId", "camel-trace-000")
	logger.InfoCtx(ctx3, "驼峰 key 测试")

	CloseAll()

	content, _ := os.ReadFile(filepath.Join(logDir, "test", "test-info.log"))
	contentStr := string(content)
	assert.Contains(t, contentStr, "request_id") // 自定义字段名
	assert.Contains(t, contentStr, "custom-trace-456")
	assert.Contains(t, contentStr, "standard-trace-789")
	assert.Contains(t, contentStr, "camel-trace-000")
}

// TestCtxZapLogger_NoStacktraceWhenDisabled 测试禁用堆栈时不添加堆栈
func TestCtxZapLogger_NoStacktraceWhenDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "no_stack")

	globalManager = nil
	managerOnce = sync.Once{}

	InitManager(ManagerConfig{
		BaseLogDir:            logDir,
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		EnableStacktrace:      false, // 禁用堆栈
		MaxSize:               10,
	})

	logger := GetLogger("test")
	logger.ErrorCtx(context.Background(), "Error 无堆栈")

	CloseAll()

	content, _ := os.ReadFile(filepath.Join(logDir, "test", "test-error.log"))
	contentStr := string(content)
	assert.Contains(t, contentStr, "Error 无堆栈")
	assert.NotContains(t, contentStr, "\"stack\"") // 不应该有 stack 字段
}

// TestNewCtxZapLogger 测试 NewCtxZapLogger 函数
func TestNewCtxZapLogger(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "new_ctx")

	globalManager = nil
	managerOnce = sync.Once{}

	InitManager(ManagerConfig{
		BaseLogDir:            logDir,
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
	})

	// 使用 NewCtxZapLogger 创建
	logger := NewCtxZapLogger("new_module")
	assert.NotNil(t, logger)

	logger.InfoCtx(context.Background(), "使用 NewCtxZapLogger 创建")

	CloseAll()

	content, _ := os.ReadFile(filepath.Join(logDir, "new_module", "new_module-info.log"))
	assert.Contains(t, string(content), "使用 NewCtxZapLogger 创建")
}
