package logger

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestNewManager test creating independent Manager instance
func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "test")

	cfg := ManagerConfig{
		BaseLogDir: logDir,
		Level:      "info",
		Encoding:   "json",
	}

	manager := NewManager(cfg)
	assert.NotNil(t, manager)
	assert.Equal(t, logDir, manager.baseConfig.BaseLogDir)
	assert.Equal(t, "info", manager.baseConfig.Level)
	assert.NotNil(t, manager.loggers)
}

// TestManager_IndependentInstances test multiple independent instances
func TestManager_IndependentInstances(t *testing.T) {
	tmpDir := t.TempDir()
	appLogDir := filepath.Join(tmpDir, "app")
	auditLogDir := filepath.Join(tmpDir, "audit")

	// Create application log Manager
	appManager := NewManager(ManagerConfig{
		BaseLogDir:            appLogDir,
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
	})

	// Create audit log Manager
	auditManager := NewManager(ManagerConfig{
		BaseLogDir:            auditLogDir,
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
	})

	// Independent use
	appManager.InfoCtx(context.Background(), "order", "Order creation", zap.String("id", "001"))
	auditManager.InfoCtx(context.Background(), "security", "User login", zap.String("user", "admin"))

	// Close
	appManager.CloseAll()
	auditManager.CloseAll()

	// Verify file independence
	assert.DirExists(t, filepath.Join(appLogDir, "order"))
	assert.DirExists(t, filepath.Join(auditLogDir, "security"))
	assert.FileExists(t, filepath.Join(appLogDir, "order", "order-info.log"))
	assert.FileExists(t, filepath.Join(auditLogDir, "security", "security-info.log"))

	// Verify content
	appContent, _ := os.ReadFile(filepath.Join(appLogDir, "order", "order-info.log"))
	assert.Contains(t, string(appContent), "Order creation")
	assert.Contains(t, string(appContent), "001")

	auditContent, _ := os.ReadFile(filepath.Join(auditLogDir, "security", "security-info.log"))
	assert.Contains(t, string(auditContent), "User login")
	assert.Contains(t, string(auditContent), "admin")
}

// TestManager_InstanceMethods test instance method integrity
func TestManager_InstanceMethods(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "instance")

	manager := NewManager(ManagerConfig{
		BaseLogDir:            logDir,
		Level:                 "debug",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
	})

	// Test all levels
	manager.Debug("test", "Debug消息")
	manager.DebugCtx(context.Background(), "test", "Info消息")
	manager.Warn("test", "Warn消息")
	manager.Error("test", "Error消息")

	manager.CloseAll()

	// Validate info file
	infoContent, _ := os.ReadFile(filepath.Join(logDir, "test", "test-info.log"))
	infoStr := string(infoContent)
	assert.Contains(t, infoStr, "Info消息")
	assert.Contains(t, infoStr, "Warn消息")

	// Validate error file
	errorContent, _ := os.ReadFile(filepath.Join(logDir, "test", "test-error.log"))
	errorStr := string(errorContent)
	assert.Contains(t, errorStr, "Error消息")
}

// TestManager_InstanceWithFields test instance with fields
func TestManager_InstanceWithFields(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "fields")

	manager := NewManager(ManagerConfig{
		BaseLogDir:            logDir,
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
	})

	// Use WithFields
	orderLogger := manager.WithFields("order",
		zap.String("service", "order-service"),
		zap.String("version", "v1.0"),
	)

	orderLogger.InfoCtx(context.Background(), "Order creation", zap.String("order_id", "12345"))
	manager.CloseAll()

	content, _ := os.ReadFile(filepath.Join(logDir, "order", "order-info.log"))
	contentStr := string(content)

	assert.Contains(t, contentStr, "service")
	assert.Contains(t, contentStr, "order-service")
	assert.Contains(t, contentStr, "version")
	assert.Contains(t, contentStr, "v1.0")
	assert.Contains(t, contentStr, "order_id")
	assert.Contains(t, contentStr, "12345")
}

// TestManager_InstanceTraceID instance trace ID functionality testing
func TestManager_InstanceTraceID(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "trace")

	manager := NewManager(ManagerConfig{
		BaseLogDir:            logDir,
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

	ctx := context.WithValue(context.Background(), "trace_id", "test-trace-123")

	manager.InfoCtx(ctx, "order", "Order creation", zap.String("order_id", "001"))
	manager.ErrorCtx(ctx, "order", "订单失败", zap.String("reason", "库存不足"))

	manager.CloseAll()

	// Verify Info log
	infoContent, _ := os.ReadFile(filepath.Join(logDir, "order", "order-info.log"))
	infoStr := string(infoContent)
	assert.Contains(t, infoStr, "trace_id")
	assert.Contains(t, infoStr, "test-trace-123")
	assert.Contains(t, infoStr, "Order creation")

	// Verify Error log
	errorContent, _ := os.ReadFile(filepath.Join(logDir, "order", "order-error.log"))
	errorStr := string(errorContent)
	assert.Contains(t, errorStr, "trace_id")
	assert.Contains(t, errorStr, "test-trace-123")
	assert.Contains(t, errorStr, "订单失败")
}

// TestManager_InstanceReloadConfig Hot reload configuration for test instance
func TestManager_InstanceReloadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "reload")

	manager := NewManager(ManagerConfig{
		BaseLogDir:            logDir,
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
	})

	// Record initial log
	manager.InfoCtx(context.Background(), "test", "初始配置")

	// Override configuration (change to debug level)
	newCfg := ManagerConfig{
		BaseLogDir:            logDir,
		Level:                 "debug",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		EnableStacktrace:      false,
		StacktraceLevel:       "error", // Must provide valid values
		MaxSize:               10,
	}

	err := manager.ReloadConfig(newCfg)
	assert.NoError(t, err)

	// The debug log should be able to be recorded after overriding.
	manager.Debug("test", "重载后的Debug")
	manager.DebugCtx(context.Background(), "test", "重载后的Info")

	manager.CloseAll()

	// Validate log
	infoContent, _ := os.ReadFile(filepath.Join(logDir, "test", "test-info.log"))
	infoStr := string(infoContent)
	assert.Contains(t, infoStr, "初始配置")
	assert.Contains(t, infoStr, "重载后的Info")
}

// TestManager_GlobalAndInstanceCoexist test global and instance coexistence
func TestManager_GlobalAndInstanceCoexist(t *testing.T) {
	tmpDir := t.TempDir()
	globalLogDir := filepath.Join(tmpDir, "global")
	customLogDir := filepath.Join(tmpDir, "custom")

	// Reset global
	globalManager = nil
	managerOnce.Do(func() {})
	managerOnce = sync.Once{}

	// Initialize global Manager
	InitManager(ManagerConfig{
		BaseLogDir:            globalLogDir,
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
	})

	// Create custom Manager
	customManager := NewManager(ManagerConfig{
		BaseLogDir:            customLogDir,
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
	})

	// Global usage
	Info("order", "全局订单创建")

	// Custom usage
	customManager.InfoCtx(context.Background(), "order", "自定义订单创建")

	CloseAll()
	customManager.CloseAll()

	// Verify two independent log files
	assert.FileExists(t, filepath.Join(globalLogDir, "order", "order-info.log"))
	assert.FileExists(t, filepath.Join(customLogDir, "order", "order-info.log"))

	globalContent, _ := os.ReadFile(filepath.Join(globalLogDir, "order", "order-info.log"))
	assert.Contains(t, string(globalContent), "全局订单创建")

	customContent, _ := os.ReadFile(filepath.Join(customLogDir, "order", "order-info.log"))
	assert.Contains(t, string(customContent), "自定义订单创建")
}

// TestManager_IsolatedTesting isolated unit test scenarios
func TestManager_IsolatedTesting(t *testing.T) {
	// Each sub-test uses an independent Manager,不影响其他测试
	t.Run("Test1", func(t *testing.T) {
		tmpDir := t.TempDir()
		logDir := filepath.Join(tmpDir, "test1")

		m := NewManager(DefaultManagerConfig())
		m.baseConfig.BaseLogDir = logDir
		m.InfoCtx(context.Background(), "module1", "测试1")
		m.CloseAll()

		assert.FileExists(t, filepath.Join(logDir, "module1", "module1-info-"+time.Now().Format("2006-01-02")+".log"))
	})

	t.Run("Test2", func(t *testing.T) {
		tmpDir := t.TempDir()
		logDir := filepath.Join(tmpDir, "test2")

		m := NewManager(DefaultManagerConfig())
		m.baseConfig.BaseLogDir = logDir
		m.InfoCtx(context.Background(), "module2", "测试2")
		m.CloseAll()

		assert.FileExists(t, filepath.Join(logDir, "module2", "module2-info-"+time.Now().Format("2006-01-02")+".log"))
	})
}
