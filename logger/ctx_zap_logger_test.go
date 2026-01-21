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
	logger.InfoCtx(ctx, "Info English: Info Message", zap.String("key", "value"))

	// Test Info (without ctx)
	logger.Info("Info English: Info without ctx ctx")

	// Test DebugCtx
	logger.DebugCtx(ctx, "Debug English: Debug message", zap.Int("count", 10))

	// Test Debug (without ctx)
	logger.Debug("Debug English: Debug without ctx ctx")

	// Test WarnCtx
	logger.WarnCtx(ctx, "Warn English: Warning Message", zap.Bool("flag", true))

	// Test Warn (without ctx)
	logger.Warn("Warn English: Warn No ctx provided ctx")

	// Test ErrorCtx (stack is automatically added)
	logger.ErrorCtx(ctx, "Error English: Error message", zap.Error(nil))

	// Test Error (without ctx)
	logger.Error("Error English: Error without ctx ctx")

	CloseAll()

	// Verify that the log file exists
	assert.FileExists(t, filepath.Join(logDir, "test", "test-info.log"))
	assert.FileExists(t, filepath.Join(logDir, "test", "test-error.log"))

	// Verify the content of the info log
	infoContent, _ := os.ReadFile(filepath.Join(logDir, "test", "test-info.log"))
	infoStr := string(infoContent)
	assert.Contains(t, infoStr, "Info English: Info Message")
	assert.Contains(t, infoStr, "trace_id")
	assert.Contains(t, infoStr, "test-trace-123")
	assert.Contains(t, infoStr, "Debug English: Debug message")
	assert.Contains(t, infoStr, "Warn English: Warning Message")

	// Verify error log contents
	errorContent, _ := os.ReadFile(filepath.Join(logDir, "test", "test-error.log"))
	errorStr := string(errorContent)
	assert.Contains(t, errorStr, "Error English: Error message")
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

	orderLogger.InfoCtx(context.Background(), "Order creation")
	orderLogger.InfoCtx(context.Background(), "Order payment")

	CloseAll()

	// Validate preset fields exist
	content, _ := os.ReadFile(filepath.Join(logDir, "order", "order-info.log"))
	contentStr := string(content)
	assert.Contains(t, contentStr, "service")
	assert.Contains(t, contentStr, "order-service")
	assert.Contains(t, contentStr, "order_id")
	assert.Contains(t, contentStr, "12345")
	assert.Contains(t, contentStr, "Order creation")
	assert.Contains(t, contentStr, "Order payment")
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
	zapLogger.Info("English: Use zap.Logger directly zap.Logger")

	CloseAll()

	// Verify log file
	content, _ := os.ReadFile(filepath.Join(logDir, "test", "test-info.log"))
	assert.Contains(t, string(content), "English: Use zap.Logger directly zap.Logger")
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
	logger.InfoCtx(ctx1, "English: Custom key test key English: Custom key test")

	// Test for standard trace_id key (fallback)
	ctx2 := context.WithValue(context.Background(), "trace_id", "standard-trace-789")
	logger.InfoCtx(ctx2, "English: Standard key test key English: Standard key test")

	// Test traceId key (compatibility fallback)
	ctx3 := context.WithValue(context.Background(), "traceId", "camel-trace-000")
	logger.InfoCtx(ctx3, "English: Camel case key test key English: Camel case key test")

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
	logger.ErrorCtx(context.Background(), "Error English: Error no stack trace")

	CloseAll()

	content, _ := os.ReadFile(filepath.Join(logDir, "test", "test-error.log"))
	contentStr := string(content)
	assert.Contains(t, contentStr, "Error English: Error no stack trace")
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

	logger.InfoCtx(context.Background(), "English: Created using NewCtxZapLogger NewCtxZapLogger English: Created using NewCtxZapLogger")

	CloseAll()

	content, _ := os.ReadFile(filepath.Join(logDir, "new_module", "new_module-info.log"))
	assert.Contains(t, string(content), "English: Created using NewCtxZapLogger")
}
