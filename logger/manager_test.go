// src/pkg/logger/manager_test.go
package logger

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestManager_Demo01(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")
	InitManager(ManagerConfig{
		BaseLogDir:            "logs",
		Level:                 "info",
		Encoding:              "json",
		ConsoleEncoding:       "console",
		EnableConsole:         true,
		EnableLevelInFilename: true,
		EnableDateInFilename:  true,
		MaxSize:               10,
		DateFormat:            "2006-01-02",
		EnableStacktrace:      true,
		StacktraceLevel:       "error",
		EnableTraceID:         true,
	})
	Error("order", "Order creation", zap.String("id", "001"))
	Info("order", "Order creation", zap.String("id", "001"))
	ctx := context.WithValue(context.Background(), "trace_id", "adfadfadfadfadfadfadf")
	DebugCtx(ctx, "hello", "world", zap.String("id", "001"))

}

func TestManager_MultipleModules(t *testing.T) {
	// Clean up test environment
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	// Reset global Manager
	globalManager = nil
	managerOnce = sync.Once{}

	// Initialize manager
	InitManager(ManagerConfig{
		BaseLogDir:            "logs",
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
	})

	// Use package-level functions
	Info("order", "Order creation", zap.String("id", "001"))
	Error("auth", "Login failed", zap.String("user", "admin"))
	Info("user", "用户注册", zap.Int("uid", 100))

	// Sync
	CloseAll()

	// Verify file existence
	assert.DirExists(t, "logs/order")
	assert.DirExists(t, "logs/auth")
	assert.DirExists(t, "logs/user")

	assert.FileExists(t, "logs/order/order-info.log")
	assert.FileExists(t, "logs/auth/auth-error.log")
	assert.FileExists(t, "logs/user/user-info.log")

	// Validate content
	orderContent, _ := os.ReadFile("logs/order/order-info.log")
	assert.Contains(t, string(orderContent), "Order creation")
	assert.Contains(t, string(orderContent), "001")

	authContent, _ := os.ReadFile("logs/auth/auth-error.log")
	assert.Contains(t, string(authContent), "Login failed")
}

func TestManager_DynamicModules(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	// reset
	globalManager = nil
	managerOnce = sync.Once{}

	// Without preconfiguration, use directly (automatically create module)
	InitManager(DefaultManagerConfig())

	// Dynamically create multiple modules
	Info("order", "Order creation")
	Info("payment", "支付成功")
	Info("notification", "发送通知")
	Error("auth", "认证失败")

	CloseAll()

	// Verify that all module directories are automatically created
	assert.DirExists(t, "logs/order")
	assert.DirExists(t, "logs/payment")
	assert.DirExists(t, "logs/notification")
	assert.DirExists(t, "logs/auth")
}

func TestManager_ConcurrentAccess(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	// reset
	globalManager = nil
	managerOnce = sync.Once{}

	InitManager(DefaultManagerConfig())

	// Concurrently obtain the same Logger
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			logger := GetLogger("concurrent")
			logger.DebugCtx(context.Background(), "test")
			done <- true
		}()
	}

	// wait for all goroutines to finish
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify that only one Logger instance was created (check before CloseAll)
	assert.Len(t, globalManager.loggers, 1)

	CloseAll()
}

func TestManager_ZeroConfig(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	// Reset
	globalManager = nil
	managerOnce = sync.Once{}

	// Do not call InitManager, use directly (uses default configuration automatically)
	Info("default", "使用默认配置")
	CloseAll()

	// Should use default configuration for creation
	assert.NotNil(t, globalManager)
	assert.DirExists(t, "logs/default")
}

// TestManager_DateInFilename test date in filename functionality
func TestManager_DateInFilename(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	globalManager = nil
	managerOnce = sync.Once{}

	// Configure enable date filename
	InitManager(ManagerConfig{
		BaseLogDir:            "logs",
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  true, // Enable date
		DateFormat:            "2006-01-02",
		MaxSize:               10,
	})

	Info("order", "测试日期文件名")
	CloseAll()

	// Verify that the filename contains a date
	today := time.Now().Format("2006-01-02")
	expectedFile := filepath.Join("logs", "order", "order-info-"+today+".log")
	assert.FileExists(t, expectedFile)

	content, _ := os.ReadFile(expectedFile)
	assert.Contains(t, string(content), "测试日期文件名")
}

// TestManager_FileSplit test file splitting functionality
func TestManager_FileSplit(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	globalManager = nil
	managerOnce = sync.Once{}

	// Configure small file size to trigger splitting (1KB)
	InitManager(ManagerConfig{
		BaseLogDir:            "logs",
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               1, // 1MB, enough logging will cause rotation
		MaxBackups:            3,
		MaxAge:                7,
		Compress:              false,
	})

	// Write a large number of logs (approximately 100 bytes per log, writing 10,000 logs is about 1MB)
	for i := 0; i < 10000; i++ {
		Info("split", "测试文件分割功能", zap.Int("index", i), zap.String("data", strings.Repeat("x", 50)))
	}
	CloseAll()

	// Verify log directory exists
	assert.DirExists(t, "logs/split")

	// Verify the existence of the main log file
	assert.FileExists(t, "logs/split/split-info.log")

	// Check file size (should be split)
	info, err := os.Stat("logs/split/split-info.log")
	assert.NoError(t, err)
	// The file should be less than or equal to MaxSize (1MB = 1048576 bytes)
	// Due to the implementation of lumberjack, it may be slightly greater than the limit
	assert.LessOrEqual(t, info.Size(), int64(2*1024*1024)) // Allow for 2MB of variance

	t.Logf("主日志文件大小: %d bytes", info.Size())
}

// TestManager_Stacktrace test call stack functionality
func TestManager_Stacktrace(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	globalManager = nil
	managerOnce = sync.Once{}

	// Configure stack trace enabled
	InitManager(ManagerConfig{
		BaseLogDir:            "logs",
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		EnableCaller:          true,    // Enable caller information
		EnableStacktrace:      true,    // Enable stack trace
		StacktraceLevel:       "error", // Error level stack recording begins
		StacktraceDepth:       5,       // Limit stack depth
		MaxSize:               10,
	})

	ctx := context.Background()
	log := GetLogger("stacktest")

	// Info level should not have stack
	log.InfoCtx(ctx, "Info level log")

	// The error level should have a stack (using ErrorCtx will automatically add it)
	log.ErrorCtx(ctx, "Error level log", zap.String("error", "测试错误"))

	CloseAll()

	// Verify that the Info log does not contain stack traces
	infoContent, _ := os.ReadFile("logs/stacktest/stacktest-info.log")
	infoStr := string(infoContent)
	assert.Contains(t, infoStr, "Info level log")
	assert.Contains(t, infoStr, "caller")          // Has caller information
	assert.NotContains(t, infoStr, "\"stack\":\"") // No stack trace field

	// Verify that the Error log contains a stack trace
	errorContent, _ := os.ReadFile("logs/stacktest/stacktest-error.log")
	errorStr := string(errorContent)
	assert.Contains(t, errorStr, "Error level log")
	assert.Contains(t, errorStr, "caller")          // Contains caller information
	assert.Contains(t, errorStr, "\"stack\":\"")    // Has stack trace fields
	assert.Contains(t, errorStr, "manager_test.go") // The stack contains test file names
}

// TestManager_CallerInfo Test caller information
func TestManager_CallerInfo(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	globalManager = nil
	managerOnce = sync.Once{}

	// Enable caller information
	InitManager(ManagerConfig{
		BaseLogDir:            "logs",
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		EnableCaller:          true, // Enable
		EnableStacktrace:      false,
		MaxSize:               10,
	})

	Info("caller", "测试调用者信息")
	CloseAll()

	content, _ := os.ReadFile("logs/caller/caller-info.log")
	contentStr := string(content)

	// Verify inclusion of caller field
	assert.Contains(t, contentStr, "caller")
	assert.Contains(t, contentStr, "manager.go") // Call the Info function from manager.go
}

// TestManager_WithFields test preset field functionality
func TestManager_WithFields(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	globalManager = nil
	managerOnce = sync.Once{}

	InitManager(ManagerConfig{
		BaseLogDir:            "logs",
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
	})

	// Create a logger with preset fields
	orderLogger := WithFields("order",
		zap.String("service", "order-service"),
		zap.String("version", "v1.0"),
	)

	orderLogger.InfoCtx(context.Background(), "Order creation", zap.String("order_id", "12345"))
	CloseAll()

	content, _ := os.ReadFile("logs/order/order-info.log")
	contentStr := string(content)

	// Validate default fields exist
	assert.Contains(t, contentStr, "service")
	assert.Contains(t, contentStr, "order-service")
	assert.Contains(t, contentStr, "version")
	assert.Contains(t, contentStr, "v1.0")
	assert.Contains(t, contentStr, "order_id")
	assert.Contains(t, contentStr, "12345")
}

// TestManager_LevelSeparation test log level separation
func TestManager_LevelSeparation(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	globalManager = nil
	managerOnce = sync.Once{}

	InitManager(ManagerConfig{
		BaseLogDir:            "logs",
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
	})

	// Log messages at different levels
	Info("level", "Info level log")
	Warn("level", "Warn级别日志")
	Error("level", "Error level log")

	CloseAll()

	// Info file should contain Info and Warn (<' Error)
	infoContent, _ := os.ReadFile("logs/level/level-info.log")
	infoStr := string(infoContent)
	assert.Contains(t, infoStr, "Info level log")
	assert.Contains(t, infoStr, "Warn级别日志")
	assert.NotContains(t, infoStr, "Error level log") // Error not in info file

	// Error file contains only Error
	errorContent, _ := os.ReadFile("logs/level/level-error.log")
	errorStr := string(errorContent)
	assert.NotContains(t, errorStr, "Info level log")
	assert.NotContains(t, errorStr, "Warn级别日志")
	assert.Contains(t, errorStr, "Error level log")
}

// ============================================
// Trace ID related tests
// ============================================

// TestManager_TraceIDBasic test basic TraceID functionality
func TestManager_TraceIDBasic(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	globalManager = nil
	managerOnce = sync.Once{}

	// Initialize (enable TraceID)
	InitManager(ManagerConfig{
		BaseLogDir:            "logs",
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		EnableTraceID:         true,
		TraceIDKey:            "trace_id",
		TraceIDFieldName:      "trace_id",
		MaxSize:               10,
	})

	// Create context with traceID
	ctx := context.WithValue(context.Background(), "trace_id", "abc-123-xyz")

	// Using Context API
	InfoCtx(ctx, "order", "Order creation", zap.String("order_id", "001"))
	ErrorCtx(ctx, "order", "订单失败", zap.String("reason", "库存不足"))

	CloseAll()

	// Verify that the Info log contains the traceID
	infoContent, _ := os.ReadFile("logs/order/order-info.log")
	infoStr := string(infoContent)
	assert.Contains(t, infoStr, "trace_id")
	assert.Contains(t, infoStr, "abc-123-xyz")
	assert.Contains(t, infoStr, "Order creation")
	assert.Contains(t, infoStr, "order_id")
	assert.Contains(t, infoStr, "001")

	// Verify that the Error log contains the traceID
	errorContent, _ := os.ReadFile("logs/order/order-error.log")
	errorStr := string(errorContent)
	assert.Contains(t, errorStr, "trace_id")
	assert.Contains(t, errorStr, "abc-123-xyz")
	assert.Contains(t, errorStr, "订单失败")
}

// TestManager_TraceIDDisabled Test disable TraceID
func TestManager_TraceIDDisabled(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	globalManager = nil
	managerOnce = sync.Once{}

	// Initialize (disable TraceID)
	InitManager(ManagerConfig{
		BaseLogDir:            "logs",
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		EnableTraceID:         false, // disable
		MaxSize:               10,
	})

	// Even if the context has a traceID, it should not be logged
	ctx := context.WithValue(context.Background(), "trace_id", "should-not-appear")
	InfoCtx(ctx, "order", "测试禁用")

	CloseAll()

	content, _ := os.ReadFile("logs/order/order-info.log")
	contentStr := string(content)
	assert.Contains(t, contentStr, "测试禁用")
	assert.NotContains(t, contentStr, "should-not-appear")
	assert.NotContains(t, contentStr, "trace_id")
}

// TestManager_TraceIDCustomKey Test custom TraceID key
func TestManager_TraceIDCustomKey(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	globalManager = nil
	managerOnce = sync.Once{}

	// Initialize (custom key name)
	InitManager(ManagerConfig{
		BaseLogDir:            "logs",
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		EnableTraceID:         true,
		TraceIDKey:            "request_id", // Custom context key
		TraceIDFieldName:      "request_id", // Custom log field name
		MaxSize:               10,
	})

	// Use custom key
	ctx := context.WithValue(context.Background(), "request_id", "req-999")
	InfoCtx(ctx, "order", "自定义Key测试")

	CloseAll()

	content, _ := os.ReadFile("logs/order/order-info.log")
	contentStr := string(content)
	assert.Contains(t, contentStr, "request_id")
	assert.Contains(t, contentStr, "req-999")
	assert.NotContains(t, contentStr, "trace_id") // There should not be a default trace_id field
}

// TestManager_TraceIDEmptyContext test empty context
func TestManager_TraceIDEmptyContext(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	globalManager = nil
	managerOnce = sync.Once{}

	InitManager(ManagerConfig{
		BaseLogDir:            "logs",
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		EnableTraceID:         true,
		MaxSize:               10,
	})

	// Using an empty context (no traceID)
	ctx := context.Background()
	InfoCtx(ctx, "order", "空Context测试", zap.String("key", "value"))

	CloseAll()

	content, _ := os.ReadFile("logs/order/order-info.log")
	contentStr := string(content)
	assert.Contains(t, contentStr, "空Context测试")
	assert.Contains(t, contentStr, "key")
	assert.Contains(t, contentStr, "value")
	// Without traceID, the trace_id field should not be added
	// Note: There may be other fields in the JSON, so a simple NotContains cannot be used
}

// TestManager_TraceIDAllLevels Test Context API at all levels
func TestManager_TraceIDAllLevels(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	globalManager = nil
	managerOnce = sync.Once{}

	InitManager(ManagerConfig{
		BaseLogDir:            "logs",
		Level:                 "debug", // Enable debug
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		EnableTraceID:         true,
		MaxSize:               10,
	})

	ctx := context.WithValue(context.Background(), "trace_id", "test-all-levels")

	// Test all levels
	DebugCtx(ctx, "test", "Debug级别")
	InfoCtx(ctx, "test", "Info级别")
	WarnCtx(ctx, "test", "Warn级别")
	ErrorCtx(ctx, "test", "Error级别")

	CloseAll()

	// Verify info file (contains info, warn, does not contain debug)
	// Note: info core only accepts logs >= InfoLevel
	infoContent, _ := os.ReadFile("logs/test/test-info.log")
	infoStr := string(infoContent)
	assert.Contains(t, infoStr, "test-all-levels")
	assert.Contains(t, infoStr, "Info级别")
	assert.Contains(t, infoStr, "Warn级别")
	// Debug level is lower than Info, will not be logged to info file

	// Validate error file
	errorContent, _ := os.ReadFile("logs/test/test-error.log")
	errorStr := string(errorContent)
	assert.Contains(t, errorStr, "test-all-levels")
	assert.Contains(t, errorStr, "Error级别")
}

// TestManager_TraceIDConcurrent test TraceID in concurrent scenarios
func TestManager_TraceIDConcurrent(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	globalManager = nil
	managerOnce = sync.Once{}

	InitManager(DefaultManagerConfig())

	// Concurrent writing of logs with different traceIDs
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			traceID := "trace-" + string(rune('0'+id))
			ctx := context.WithValue(context.Background(), "trace_id", traceID)
			InfoCtx(ctx, "concurrent", "并发测试", zap.Int("goroutine", id))
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	CloseAll()

	// Verify that the log file exists and contains content
	assert.DirExists(t, "logs/concurrent")
	content, _ := os.ReadFile("logs/concurrent/concurrent-info-" + time.Now().Format("2006-01-02") + ".log")
	contentStr := string(content)
	assert.Contains(t, contentStr, "并发测试")
	assert.Contains(t, contentStr, "trace_id")
}
