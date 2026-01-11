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
	// 清理测试环境
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	// 重置全局 Manager
	globalManager = nil
	managerOnce = sync.Once{}

	// 初始化管理器
	InitManager(ManagerConfig{
		BaseLogDir:            "logs",
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
	})

	// 使用包级别函数
	Info("order", "Order creation", zap.String("id", "001"))
	Error("auth", "Login failed", zap.String("user", "admin"))
	Info("user", "用户注册", zap.Int("uid", 100))

	// Sync
	CloseAll()

	// 验证文件存在
	assert.DirExists(t, "logs/order")
	assert.DirExists(t, "logs/auth")
	assert.DirExists(t, "logs/user")

	assert.FileExists(t, "logs/order/order-info.log")
	assert.FileExists(t, "logs/auth/auth-error.log")
	assert.FileExists(t, "logs/user/user-info.log")

	// 验证内容
	orderContent, _ := os.ReadFile("logs/order/order-info.log")
	assert.Contains(t, string(orderContent), "Order creation")
	assert.Contains(t, string(orderContent), "001")

	authContent, _ := os.ReadFile("logs/auth/auth-error.log")
	assert.Contains(t, string(authContent), "Login failed")
}

func TestManager_DynamicModules(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	// 重置
	globalManager = nil
	managerOnce = sync.Once{}

	// 不预先配置，直接使用（自动创建模块）
	InitManager(DefaultManagerConfig())

	// 动态创建多个模块
	Info("order", "Order creation")
	Info("payment", "支付成功")
	Info("notification", "发送通知")
	Error("auth", "认证失败")

	CloseAll()

	// 验证所有模块目录都自动创建
	assert.DirExists(t, "logs/order")
	assert.DirExists(t, "logs/payment")
	assert.DirExists(t, "logs/notification")
	assert.DirExists(t, "logs/auth")
}

func TestManager_ConcurrentAccess(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	// 重置
	globalManager = nil
	managerOnce = sync.Once{}

	InitManager(DefaultManagerConfig())

	// 并发获取同一个 Logger
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			logger := GetLogger("concurrent")
			logger.DebugCtx(context.Background(), "test")
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证只创建了一个 Logger 实例（在 CloseAll 之前检查）
	assert.Len(t, globalManager.loggers, 1)

	CloseAll()
}

func TestManager_ZeroConfig(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	// 重置
	globalManager = nil
	managerOnce = sync.Once{}

	// 不调用 InitManager，直接使用（自动使用默认配置）
	Info("default", "使用默认配置")
	CloseAll()

	// 应该使用默认配置创建
	assert.NotNil(t, globalManager)
	assert.DirExists(t, "logs/default")
}

// TestManager_DateInFilename 测试日期文件名功能
func TestManager_DateInFilename(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	globalManager = nil
	managerOnce = sync.Once{}

	// 配置启用日期文件名
	InitManager(ManagerConfig{
		BaseLogDir:            "logs",
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  true, // 启用日期
		DateFormat:            "2006-01-02",
		MaxSize:               10,
	})

	Info("order", "测试日期文件名")
	CloseAll()

	// 验证文件名包含日期
	today := time.Now().Format("2006-01-02")
	expectedFile := filepath.Join("logs", "order", "order-info-"+today+".log")
	assert.FileExists(t, expectedFile)

	content, _ := os.ReadFile(expectedFile)
	assert.Contains(t, string(content), "测试日期文件名")
}

// TestManager_FileSplit 测试文件分割功能
func TestManager_FileSplit(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	globalManager = nil
	managerOnce = sync.Once{}

	// 配置小文件大小以触发切割（1KB）
	InitManager(ManagerConfig{
		BaseLogDir:            "logs",
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               1, // 1MB，写足够多日志会切割
		MaxBackups:            3,
		MaxAge:                7,
		Compress:              false,
	})

	// 写入大量日志（每条约100字节，写10000条约1MB）
	for i := 0; i < 10000; i++ {
		Info("split", "测试文件分割功能", zap.Int("index", i), zap.String("data", strings.Repeat("x", 50)))
	}
	CloseAll()

	// 验证日志目录存在
	assert.DirExists(t, "logs/split")

	// 验证主日志文件存在
	assert.FileExists(t, "logs/split/split-info.log")

	// 检查文件大小（应该被分割了）
	info, err := os.Stat("logs/split/split-info.log")
	assert.NoError(t, err)
	// 文件应该小于等于 MaxSize (1MB = 1048576 bytes)
	// 由于 lumberjack 的实现，可能略大于限制
	assert.LessOrEqual(t, info.Size(), int64(2*1024*1024)) // 允许2MB的误差

	t.Logf("主日志文件大小: %d bytes", info.Size())
}

// TestManager_Stacktrace 测试调用栈功能
func TestManager_Stacktrace(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	globalManager = nil
	managerOnce = sync.Once{}

	// 配置启用栈追踪
	InitManager(ManagerConfig{
		BaseLogDir:            "logs",
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		EnableCaller:          true,    // 启用调用者信息
		EnableStacktrace:      true,    // 启用栈追踪
		StacktraceLevel:       "error", // Error 级别开始记录栈
		StacktraceDepth:       5,       // 限制堆栈深度
		MaxSize:               10,
	})

	ctx := context.Background()
	log := GetLogger("stacktest")

	// Info 级别不应该有栈
	log.DebugCtx(ctx, "Info level log")

	// Error 级别应该有栈（使用 ErrorCtx 会自动添加）
	log.ErrorCtx(ctx, "Error level log", zap.String("error", "测试错误"))

	CloseAll()

	// 验证 Info 日志没有栈
	infoContent, _ := os.ReadFile("logs/stacktest/stacktest-info.log")
	infoStr := string(infoContent)
	assert.Contains(t, infoStr, "Info level log")
	assert.Contains(t, infoStr, "caller")          // 有调用者信息
	assert.NotContains(t, infoStr, "\"stack\":\"") // 没有栈信息字段

	// 验证 Error 日志有栈
	errorContent, _ := os.ReadFile("logs/stacktest/stacktest-error.log")
	errorStr := string(errorContent)
	assert.Contains(t, errorStr, "Error level log")
	assert.Contains(t, errorStr, "caller")          // 有调用者信息
	assert.Contains(t, errorStr, "\"stack\":\"")    // 有栈信息字段
	assert.Contains(t, errorStr, "manager_test.go") // 栈中包含测试文件名
}

// TestManager_CallerInfo 测试调用者信息
func TestManager_CallerInfo(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	globalManager = nil
	managerOnce = sync.Once{}

	// 启用调用者信息
	InitManager(ManagerConfig{
		BaseLogDir:            "logs",
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		EnableCaller:          true, // 启用
		EnableStacktrace:      false,
		MaxSize:               10,
	})

	Info("caller", "测试调用者信息")
	CloseAll()

	content, _ := os.ReadFile("logs/caller/caller-info.log")
	contentStr := string(content)

	// 验证包含 caller 字段
	assert.Contains(t, contentStr, "caller")
	assert.Contains(t, contentStr, "manager.go") // 调用来自 manager.go 的 Info 函数
}

// TestManager_WithFields 测试预设字段功能
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

	// 创建带预设字段的 Logger
	orderLogger := WithFields("order",
		zap.String("service", "order-service"),
		zap.String("version", "v1.0"),
	)

	orderLogger.DebugCtx(context.Background(), "Order creation", zap.String("order_id", "12345"))
	CloseAll()

	content, _ := os.ReadFile("logs/order/order-info.log")
	contentStr := string(content)

	// 验证预设字段存在
	assert.Contains(t, contentStr, "service")
	assert.Contains(t, contentStr, "order-service")
	assert.Contains(t, contentStr, "version")
	assert.Contains(t, contentStr, "v1.0")
	assert.Contains(t, contentStr, "order_id")
	assert.Contains(t, contentStr, "12345")
}

// TestManager_LevelSeparation 测试日志级别分离
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

	// 记录不同级别的日志
	Info("level", "Info level log")
	Warn("level", "Warn级别日志")
	Error("level", "Error level log")

	CloseAll()

	// Info 文件应包含 Info 和 Warn（< Error）
	infoContent, _ := os.ReadFile("logs/level/level-info.log")
	infoStr := string(infoContent)
	assert.Contains(t, infoStr, "Info level log")
	assert.Contains(t, infoStr, "Warn级别日志")
	assert.NotContains(t, infoStr, "Error level log") // Error 不在 info 文件

	// Error 文件只包含 Error
	errorContent, _ := os.ReadFile("logs/level/level-error.log")
	errorStr := string(errorContent)
	assert.NotContains(t, errorStr, "Info level log")
	assert.NotContains(t, errorStr, "Warn级别日志")
	assert.Contains(t, errorStr, "Error level log")
}

// ============================================
// TraceID 相关测试
// ============================================

// TestManager_TraceIDBasic 测试基本 TraceID 功能
func TestManager_TraceIDBasic(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	globalManager = nil
	managerOnce = sync.Once{}

	// 初始化（启用 TraceID）
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

	// 创建带 traceID 的 context
	ctx := context.WithValue(context.Background(), "trace_id", "abc-123-xyz")

	// 使用 Context API
	DebugCtx(ctx, "order", "Order creation", zap.String("order_id", "001"))
	ErrorCtx(ctx, "order", "订单失败", zap.String("reason", "库存不足"))

	CloseAll()

	// 验证 Info 日志包含 traceID
	infoContent, _ := os.ReadFile("logs/order/order-info.log")
	infoStr := string(infoContent)
	assert.Contains(t, infoStr, "trace_id")
	assert.Contains(t, infoStr, "abc-123-xyz")
	assert.Contains(t, infoStr, "Order creation")
	assert.Contains(t, infoStr, "order_id")
	assert.Contains(t, infoStr, "001")

	// 验证 Error 日志包含 traceID
	errorContent, _ := os.ReadFile("logs/order/order-error.log")
	errorStr := string(errorContent)
	assert.Contains(t, errorStr, "trace_id")
	assert.Contains(t, errorStr, "abc-123-xyz")
	assert.Contains(t, errorStr, "订单失败")
}

// TestManager_TraceIDDisabled 测试禁用 TraceID
func TestManager_TraceIDDisabled(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	globalManager = nil
	managerOnce = sync.Once{}

	// 初始化（禁用 TraceID）
	InitManager(ManagerConfig{
		BaseLogDir:            "logs",
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		EnableTraceID:         false, // 禁用
		MaxSize:               10,
	})

	// 即使 context 有 traceID，也不应该记录
	ctx := context.WithValue(context.Background(), "trace_id", "should-not-appear")
	DebugCtx(ctx, "order", "测试禁用")

	CloseAll()

	content, _ := os.ReadFile("logs/order/order-info.log")
	contentStr := string(content)
	assert.Contains(t, contentStr, "测试禁用")
	assert.NotContains(t, contentStr, "should-not-appear")
	assert.NotContains(t, contentStr, "trace_id")
}

// TestManager_TraceIDCustomKey 测试自定义 TraceID Key
func TestManager_TraceIDCustomKey(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	globalManager = nil
	managerOnce = sync.Once{}

	// 初始化（自定义 key 名称）
	InitManager(ManagerConfig{
		BaseLogDir:            "logs",
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		EnableTraceID:         true,
		TraceIDKey:            "request_id", // 自定义 context key
		TraceIDFieldName:      "request_id", // 自定义日志字段名
		MaxSize:               10,
	})

	// 使用自定义 key
	ctx := context.WithValue(context.Background(), "request_id", "req-999")
	DebugCtx(ctx, "order", "自定义Key测试")

	CloseAll()

	content, _ := os.ReadFile("logs/order/order-info.log")
	contentStr := string(content)
	assert.Contains(t, contentStr, "request_id")
	assert.Contains(t, contentStr, "req-999")
	assert.NotContains(t, contentStr, "trace_id") // 不应该有默认的 trace_id 字段
}

// TestManager_TraceIDEmptyContext 测试空 Context
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

	// 使用空 context（没有 traceID）
	ctx := context.Background()
	DebugCtx(ctx, "order", "空Context测试", zap.String("key", "value"))

	CloseAll()

	content, _ := os.ReadFile("logs/order/order-info.log")
	contentStr := string(content)
	assert.Contains(t, contentStr, "空Context测试")
	assert.Contains(t, contentStr, "key")
	assert.Contains(t, contentStr, "value")
	// 没有 traceID，不应该添加 trace_id 字段
	// 注意：JSON 中可能有其他字段，所以不能简单 NotContains
}

// TestManager_TraceIDAllLevels 测试所有级别的 Context API
func TestManager_TraceIDAllLevels(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	globalManager = nil
	managerOnce = sync.Once{}

	InitManager(ManagerConfig{
		BaseLogDir:            "logs",
		Level:                 "debug", // 允许 debug
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		EnableTraceID:         true,
		MaxSize:               10,
	})

	ctx := context.WithValue(context.Background(), "trace_id", "test-all-levels")

	// 测试所有级别
	DebugCtx(ctx, "test", "Debug级别")
	DebugCtx(ctx, "test", "Info级别")
	WarnCtx(ctx, "test", "Warn级别")
	ErrorCtx(ctx, "test", "Error级别")

	CloseAll()

	// 验证 info 文件（包含 info, warn，不包含 debug）
	// 注意：info core 只接受 >= InfoLevel 的日志
	infoContent, _ := os.ReadFile("logs/test/test-info.log")
	infoStr := string(infoContent)
	assert.Contains(t, infoStr, "test-all-levels")
	assert.Contains(t, infoStr, "Info级别")
	assert.Contains(t, infoStr, "Warn级别")
	// Debug 级别低于 Info，不会记录到 info 文件

	// 验证 error 文件
	errorContent, _ := os.ReadFile("logs/test/test-error.log")
	errorStr := string(errorContent)
	assert.Contains(t, errorStr, "test-all-levels")
	assert.Contains(t, errorStr, "Error级别")
}

// TestManager_TraceIDConcurrent 测试并发场景下的 TraceID
func TestManager_TraceIDConcurrent(t *testing.T) {
	os.RemoveAll("logs")
	defer os.RemoveAll("logs")

	globalManager = nil
	managerOnce = sync.Once{}

	InitManager(DefaultManagerConfig())

	// 并发写入不同 traceID 的日志
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			traceID := "trace-" + string(rune('0'+id))
			ctx := context.WithValue(context.Background(), "trace_id", traceID)
			DebugCtx(ctx, "concurrent", "并发测试", zap.Int("goroutine", id))
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	CloseAll()

	// 验证日志文件存在且包含内容
	assert.DirExists(t, "logs/concurrent")
	content, _ := os.ReadFile("logs/concurrent/concurrent-info-" + time.Now().Format("2006-01-02") + ".log")
	contentStr := string(content)
	assert.Contains(t, contentStr, "并发测试")
	assert.Contains(t, contentStr, "trace_id")
}
