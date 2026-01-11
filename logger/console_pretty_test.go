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

// TestConsolePretty_Integration 测试 console_pretty 编码集成
func TestConsolePretty_Integration(t *testing.T) {
	tmpDir := t.TempDir()

	// 重置全局 Manager
	globalManager = nil
	managerOnce.Do(func() {})
	managerOnce = sync.Once{}

	// 初始化管理器，使用 console_pretty 编码
	InitManager(ManagerConfig{
		BaseLogDir:            tmpDir,
		Level:                 "info",
		Encoding:              "json",           // 文件使用 json
		ConsoleEncoding:       "console_pretty", // 控制台使用 pretty
		EnableConsole:         true,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
		EnableCaller:          true,
		EnableStacktrace:      true,
		StacktraceLevel:       "error",
		EnableTraceID:         true,
	})

	// 测试各种日志级别
	Info("order", "Order creation", zap.String("order_id", "001"), zap.Float64("amount", 99.99))
	Warn("cache", "缓存未命中", zap.String("key", "user:100"))
	Error("auth", "Login failed", zap.String("user", "admin"), zap.String("reason", "密码错误"))

	// 测试带 TraceID 的日志
	ctx := context.WithValue(context.Background(), "trace_id", "trace-abc-123")
	DebugCtx(ctx, "payment", "支付成功", zap.String("order_id", "001"), zap.Float64("amount", 199.99))

	CloseAll()

	// 验证文件存在（文件应该是 JSON 格式）
	assert.FileExists(t, filepath.Join(tmpDir, "order", "order-info.log"))
	assert.FileExists(t, filepath.Join(tmpDir, "auth", "auth-error.log"))

	// 验证文件内容（应该是 JSON）
	orderContent, _ := os.ReadFile(filepath.Join(tmpDir, "order", "order-info.log"))
	orderStr := string(orderContent)
	assert.Contains(t, orderStr, `"level":"info"`) // JSON 格式
	assert.Contains(t, orderStr, `"msg":"Order creation"`)
	assert.Contains(t, orderStr, `"order_id":"001"`)

	// 注意：控制台输出是 console_pretty 格式，但无法直接捕获测试
	// 需要手动运行查看控制台输出效果
	t.Log("✅ 控制台应该显示 console_pretty 格式（带 Emoji）")
}

// TestConsolePretty_Pure 测试纯 console_pretty（文件也用 pretty）
func TestConsolePretty_Pure(t *testing.T) {
	tmpDir := t.TempDir()

	// 重置全局 Manager
	globalManager = nil
	managerOnce = sync.Once{}

	// 文件和控制台都使用 console_pretty
	InitManager(ManagerConfig{
		BaseLogDir:            tmpDir,
		Level:                 "debug",
		Encoding:              "console_pretty", // 文件也用 pretty
		EnableConsole:         false,            // 关闭控制台，只看文件
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
		EnableCaller:          true,
		EnableStacktrace:      true,
		StacktraceLevel:       "error",
		EnableTraceID:         true,
	})

	// 记录各级别日志
	Debug("test", "调试日志", zap.String("key", "value"))
	Info("order", "Order creation", zap.String("order_id", "001"))
	Warn("cache", "缓存过期", zap.String("key", "user:100"))
	Error("auth", "认证失败", zap.String("error", "token expired"))

	// 带 TraceID
	ctx := context.WithValue(context.Background(), "trace_id", "trace-xyz-789")
	DebugCtx(ctx, "payment", "支付处理中", zap.String("order_id", "002"))

	CloseAll()

	// 验证文件内容（应该是 console_pretty 格式）
	orderContent, _ := os.ReadFile(filepath.Join(tmpDir, "order", "order-info.log"))
	orderStr := string(orderContent)
	t.Logf("订单日志内容:\n%s", orderStr)

	// 验证 pretty 格式特征
	assert.Contains(t, orderStr, "[🔵INFO]") // Emoji + 级别
	assert.Contains(t, orderStr, "[order]")  // 模块名
	assert.Contains(t, orderStr, "Order creation")
	assert.Contains(t, orderStr, `"order_id":"001"`) // JSON 字段

	// 验证 TraceID
	paymentContent, _ := os.ReadFile(filepath.Join(tmpDir, "payment", "payment-info.log"))
	paymentStr := string(paymentContent)
	t.Logf("支付日志内容:\n%s", paymentStr)
	assert.Contains(t, paymentStr, "trace-xyz-789")
	assert.Contains(t, paymentStr, "[payment]")
	assert.Contains(t, paymentStr, "支付处理中")

	// 验证 Error 级别
	authContent, _ := os.ReadFile(filepath.Join(tmpDir, "auth", "auth-error.log"))
	authStr := string(authContent)
	t.Logf("认证错误日志:\n%s", authStr)
	assert.Contains(t, authStr, "[🔴ERRO]") // Error Emoji
	assert.Contains(t, authStr, "[auth]")
	assert.Contains(t, authStr, "认证失败")
}

// TestConsolePretty_ConfigValidation 测试配置验证
func TestConsolePretty_ConfigValidation(t *testing.T) {
	tmpDir := t.TempDir()

	// 测试有效配置
	validCfg := ManagerConfig{
		BaseLogDir:      tmpDir,
		Level:           "info",
		Encoding:        "console_pretty",
		MaxSize:         100,
		MaxBackups:      3,
		MaxAge:          7,
		StacktraceLevel: "error",
	}
	err := validCfg.Validate()
	assert.NoError(t, err)

	// 测试无效编码
	invalidCfg := ManagerConfig{
		BaseLogDir:      tmpDir,
		Level:           "info",
		Encoding:        "invalid_encoding",
		MaxSize:         100,
		MaxBackups:      3,
		MaxAge:          7,
		StacktraceLevel: "error",
	}
	err = invalidCfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无效的日志编码")
}
