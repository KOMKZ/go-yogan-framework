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

// TestCtxZapLogger_AllMethods tests all methods of CtxZapLogger
func TestCtxZapLogger_AllMethods(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "ctx_logger")

	// Reset global Manager
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

	// Test InfoCtx
	logger.InfoCtx(ctx, "Info 消息", zap.String("key", "value"))

	// Test Info (without ctx)
	logger.Info("Info 不带 ctx")

	// Test DebugCtx
	logger.DebugCtx(ctx, "Debug 消息", zap.Int("count", 10))

	// Test Debug (without ctx)
	logger.Debug("Debug 不带 ctx")

	// Test WarnCtx
	logger.WarnCtx(ctx, "Warn 消息", zap.Bool("flag", true))

	// Test Warn (without ctx)
	logger.Warn("Warn 不带 ctx")

	// Test ErrorCtx (stack is automatically added)
	logger.ErrorCtx(ctx, "Error 消息", zap.Error(nil))

	// Test Error (without ctx)
	logger.Error("Error 不带 ctx")

	CloseAll()

	// Verify that the log file exists
	assert.FileExists(t, filepath.Join(logDir, "test", "test-info.log"))
	assert.FileExists(t, filepath.Join(logDir, "test", "test-error.log"))

	// Verify the content of the info log
	infoContent, _ := os.ReadFile(filepath.Join(logDir, "test", "test-info.log"))
	infoStr := string(infoContent)
	assert.Contains(t, infoStr, "Info 消息")
	assert.Contains(t, infoStr, "trace_id")
	assert.Contains(t, infoStr, "test-trace-123")
	assert.Contains(t, infoStr, "Debug 消息")
	assert.Contains(t, infoStr, "Warn 消息")

	// Verify error log contents
	errorContent, _ := os.ReadFile(filepath.Join(logDir, "test", "test-error.log"))
	errorStr := string(errorContent)
	assert.Contains(t, errorStr, "Error 消息")
	assert.Contains(t, errorStr, "stack") // Should include stack
}

// TestCtxZapLogger_With test the With method
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

	// Use With to add preset fields
	orderLogger := logger.With(
		zap.String("service", "order-service"),
		zap.Int64("order_id", 12345),
	)

	orderLogger.InfoCtx(context.Background(), "订单创建")
	orderLogger.InfoCtx(context.Background(), "订单支付")

	CloseAll()

	// Validate preset fields exist
	content, _ := os.ReadFile(filepath.Join(logDir, "order", "order-info.log"))
	contentStr := string(content)
	assert.Contains(t, contentStr, "service")
	assert.Contains(t, contentStr, "order-service")
	assert.Contains(t, contentStr, "order_id")
	assert.Contains(t, contentStr, "12345")
	assert.Contains(t, contentStr, "订单创建")
	assert.Contains(t, contentStr, "订单支付")
}

// TestCtxZapLogger_GetZapLogger test the GetZapLogger method
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

	// Get the underlying zap.Logger
	zapLogger := logger.GetZapLogger()
	assert.NotNil(t, zapLogger)

	// Use underlying Logger for logging
	zapLogger.Info("直接使用 zap.Logger")

	CloseAll()

	// Verify log file
	content, _ := os.ReadFile(filepath.Join(logDir, "test", "test-info.log"))
	assert.Contains(t, string(content), "直接使用 zap.Logger")
}

// TestCtxZapLogger_TraceIDFromDifferentKeys Test different TraceID keys
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

	// Test custom key
	ctx1 := context.WithValue(context.Background(), "custom_trace", "custom-trace-456")
	logger.InfoCtx(ctx1, "自定义 key 测试")

	// Test for standard trace_id key (fallback)
	ctx2 := context.WithValue(context.Background(), "trace_id", "standard-trace-789")
	logger.InfoCtx(ctx2, "标准 key 测试")

	// Test traceId key (compatibility fallback)
	ctx3 := context.WithValue(context.Background(), "traceId", "camel-trace-000")
	logger.InfoCtx(ctx3, "驼峰 key 测试")

	CloseAll()

	content, _ := os.ReadFile(filepath.Join(logDir, "test", "test-info.log"))
	contentStr := string(content)
	assert.Contains(t, contentStr, "request_id") // Custom field name
	assert.Contains(t, contentStr, "custom-trace-456")
	assert.Contains(t, contentStr, "standard-trace-789")
	assert.Contains(t, contentStr, "camel-trace-000")
}

// TestCtxZapLogger_NoStacktraceWhenDisabled Tests do not include stack traces when disabled
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
		EnableStacktrace:      false, // Disable stack
		MaxSize:               10,
	})

	logger := GetLogger("test")
	logger.ErrorCtx(context.Background(), "Error 无堆栈")

	CloseAll()

	content, _ := os.ReadFile(filepath.Join(logDir, "test", "test-error.log"))
	contentStr := string(content)
	assert.Contains(t, contentStr, "Error 无堆栈")
	assert.NotContains(t, contentStr, "\"stack\"") // There should not be a stack field
}

// TestNewCtxZapLogger test the NewCtxZapLogger function
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

	// Use NewCtxZapLogger to create
	logger := NewCtxZapLogger("new_module")
	assert.NotNil(t, logger)

	logger.InfoCtx(context.Background(), "使用 NewCtxZapLogger 创建")

	CloseAll()

	content, _ := os.ReadFile(filepath.Join(logDir, "new_module", "new_module-info.log"))
	assert.Contains(t, string(content), "使用 NewCtxZapLogger 创建")
}
