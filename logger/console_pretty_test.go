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

// TestConsolePretty_Integration test console_pretty encoding integration
func TestConsolePretty_Integration(t *testing.T) {
	tmpDir := t.TempDir()

	// Reset global Manager
	globalManager = nil
	managerOnce.Do(func() {})
	managerOnce = sync.Once{}

	// Initialize manager, use console_pretty encoding
	InitManager(ManagerConfig{
		BaseLogDir:            tmpDir,
		LogDirMode:            LogDirModeModule,
		Level:                 "info",
		Encoding:              "json",           // The file uses JSON
		ConsoleEncoding:       "console_pretty", // Use pretty for console output
		EnableConsole:         true,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
		EnableCaller:          true,
		EnableStacktrace:      true,
		StacktraceLevel:       "error",
		EnableTraceID:         true,
	})

	// Test various log levels
	Info("order", "Order creation", zap.String("order_id", "001"), zap.Float64("amount", 99.99))
	Warn("cache", "缓存未命中", zap.String("key", "user:100"))
	Error("auth", "Login failed", zap.String("user", "admin"), zap.String("reason", "密码错误"))

	// Test logs with TraceID
	ctx := context.WithValue(context.Background(), "trace_id", "trace-abc-123")
	DebugCtx(ctx, "payment", "支付成功", zap.String("order_id", "001"), zap.Float64("amount", 199.99))

	CloseAll()

	// Verify file existence (file should be in JSON format)
	assert.FileExists(t, filepath.Join(tmpDir, "order", "order-info.log"))
	assert.FileExists(t, filepath.Join(tmpDir, "auth", "auth-error.log"))

	// Verify file content (should be JSON)
	orderContent, _ := os.ReadFile(filepath.Join(tmpDir, "order", "order-info.log"))
	orderStr := string(orderContent)
	assert.Contains(t, orderStr, `"level":"info"`) // JSON format
	assert.Contains(t, orderStr, `"msg":"Order creation"`)
	assert.Contains(t, orderStr, `"order_id":"001"`)

	// Note: Console output is in console_pretty format but cannot be directly captured for testing
	// Need to run manually to check console output effects
	t.Log("✅ 控制台应该显示 console_pretty 格式（带 Emoji）")
}

// TestConsolePretty_Pure test pure console_pretty (files also use pretty)
func TestConsolePretty_Pure(t *testing.T) {
	tmpDir := t.TempDir()

	// Reset global Manager
	globalManager = nil
	managerOnce = sync.Once{}

	// Both file and console use console_pretty
	InitManager(ManagerConfig{
		BaseLogDir:            tmpDir,
		LogDirMode:            LogDirModeModule,
		Level:                 "debug",
		Encoding:              "console_pretty", // The file also uses pretty formatting
		EnableConsole:         false,            // Close console, view only files
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
		EnableCaller:          true,
		EnableStacktrace:      true,
		StacktraceLevel:       "error",
		EnableTraceID:         true,
	})

	// Log messages at all levels
	Debug("test", "调试日志", zap.String("key", "value"))
	Info("order", "Order creation", zap.String("order_id", "001"))
	Warn("cache", "缓存过期", zap.String("key", "user:100"))
	Error("auth", "认证失败", zap.String("error", "token expired"))

	// With TraceID
	ctx := context.WithValue(context.Background(), "trace_id", "trace-xyz-789")
	DebugCtx(ctx, "payment", "支付处理中", zap.String("order_id", "002"))

	CloseAll()

	// Verify file content (should be in console_pretty format)
	orderContent, _ := os.ReadFile(filepath.Join(tmpDir, "order", "order-info.log"))
	orderStr := string(orderContent)
	t.Logf("订单日志内容:\n%s", orderStr)

	// Verify pretty format features
	assert.Contains(t, orderStr, "[🔵INFO]") // Emoji + Level
	assert.Contains(t, orderStr, "[order]") // module name
	assert.Contains(t, orderStr, "Order creation")
	assert.Contains(t, orderStr, `"order_id":"001"`) // JSON field

	// Verify TraceID
	paymentContent, _ := os.ReadFile(filepath.Join(tmpDir, "payment", "payment-info.log"))
	paymentStr := string(paymentContent)
	t.Logf("支付日志内容:\n%s", paymentStr)
	assert.Contains(t, paymentStr, "trace-xyz-789")
	assert.Contains(t, paymentStr, "[payment]")
	assert.Contains(t, paymentStr, "支付处理中")

	// Verify Error level
	authContent, _ := os.ReadFile(filepath.Join(tmpDir, "auth", "auth-error.log"))
	authStr := string(authContent)
	t.Logf("认证错误日志:\n%s", authStr)
	assert.Contains(t, authStr, "[🔴ERRO]") // Error Emoji
	assert.Contains(t, authStr, "[auth]")
	assert.Contains(t, authStr, "认证失败")
}

// TestConfigValidation_Pretty	Console Pretty Configuration Validation
func TestConsolePretty_ConfigValidation(t *testing.T) {
	tmpDir := t.TempDir()

	// Test valid configuration
	validCfg := ManagerConfig{
		BaseLogDir:      tmpDir,
		LogDirMode:      LogDirModeModule,
		Level:           "info",
		Encoding:        "console_pretty",
		MaxSize:         100,
		MaxBackups:      3,
		MaxAge:          7,
		StacktraceLevel: "error",
	}
	err := validCfg.Validate()
	assert.NoError(t, err)

	// Test invalid encoding
	invalidCfg := ManagerConfig{
		BaseLogDir:      tmpDir,
		LogDirMode:      LogDirModeModule,
		Level:           "info",
		Encoding:        "invalid_encoding",
		MaxSize:         100,
		MaxBackups:      3,
		MaxAge:          7,
		StacktraceLevel: "error",
	}
	err = invalidCfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid log encoding")
}
