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

// TestNewManager 测试创建独立 Manager 实例
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

// TestManager_IndependentInstances 测试多个独立实例
func TestManager_IndependentInstances(t *testing.T) {
	tmpDir := t.TempDir()
	appLogDir := filepath.Join(tmpDir, "app")
	auditLogDir := filepath.Join(tmpDir, "audit")

	// 创建应用日志 Manager
	appManager := NewManager(ManagerConfig{
		BaseLogDir:            appLogDir,
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
	})

	// 创建审计日志 Manager
	auditManager := NewManager(ManagerConfig{
		BaseLogDir:            auditLogDir,
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
	})

	// 独立使用
	appManager.InfoCtx(context.Background(), "order", "Order creation", zap.String("id", "001"))
	auditManager.InfoCtx(context.Background(), "security", "User login", zap.String("user", "admin"))

	// 关闭
	appManager.CloseAll()
	auditManager.CloseAll()

	// 验证文件独立
	assert.DirExists(t, filepath.Join(appLogDir, "order"))
	assert.DirExists(t, filepath.Join(auditLogDir, "security"))
	assert.FileExists(t, filepath.Join(appLogDir, "order", "order-info.log"))
	assert.FileExists(t, filepath.Join(auditLogDir, "security", "security-info.log"))

	// 验证内容
	appContent, _ := os.ReadFile(filepath.Join(appLogDir, "order", "order-info.log"))
	assert.Contains(t, string(appContent), "Order creation")
	assert.Contains(t, string(appContent), "001")

	auditContent, _ := os.ReadFile(filepath.Join(auditLogDir, "security", "security-info.log"))
	assert.Contains(t, string(auditContent), "User login")
	assert.Contains(t, string(auditContent), "admin")
}

// TestManager_InstanceMethods 测试实例方法完整性
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

	// 测试所有级别
	manager.Debug("test", "Debug消息")
	manager.DebugCtx(context.Background(), "test", "Info消息")
	manager.Warn("test", "Warn消息")
	manager.Error("test", "Error消息")

	manager.CloseAll()

	// 验证 info 文件
	infoContent, _ := os.ReadFile(filepath.Join(logDir, "test", "test-info.log"))
	infoStr := string(infoContent)
	assert.Contains(t, infoStr, "Info消息")
	assert.Contains(t, infoStr, "Warn消息")

	// 验证 error 文件
	errorContent, _ := os.ReadFile(filepath.Join(logDir, "test", "test-error.log"))
	errorStr := string(errorContent)
	assert.Contains(t, errorStr, "Error消息")
}

// TestManager_InstanceWithFields 测试实例的 WithFields
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

	// 使用 WithFields
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

// TestManager_InstanceTraceID 测试实例的 TraceID 功能
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

	// 验证 Info 日志
	infoContent, _ := os.ReadFile(filepath.Join(logDir, "order", "order-info.log"))
	infoStr := string(infoContent)
	assert.Contains(t, infoStr, "trace_id")
	assert.Contains(t, infoStr, "test-trace-123")
	assert.Contains(t, infoStr, "Order creation")

	// 验证 Error 日志
	errorContent, _ := os.ReadFile(filepath.Join(logDir, "order", "order-error.log"))
	errorStr := string(errorContent)
	assert.Contains(t, errorStr, "trace_id")
	assert.Contains(t, errorStr, "test-trace-123")
	assert.Contains(t, errorStr, "订单失败")
}

// TestManager_InstanceReloadConfig 测试实例的配置热重载
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

	// 记录初始日志
	manager.InfoCtx(context.Background(), "test", "初始配置")

	// 重载配置（改为 debug 级别）
	newCfg := ManagerConfig{
		BaseLogDir:            logDir,
		Level:                 "debug",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		EnableStacktrace:      false,
		StacktraceLevel:       "error", // 必须提供有效值
		MaxSize:               10,
	}

	err := manager.ReloadConfig(newCfg)
	assert.NoError(t, err)

	// 重载后应该能记录 debug 日志
	manager.Debug("test", "重载后的Debug")
	manager.DebugCtx(context.Background(), "test", "重载后的Info")

	manager.CloseAll()

	// 验证日志
	infoContent, _ := os.ReadFile(filepath.Join(logDir, "test", "test-info.log"))
	infoStr := string(infoContent)
	assert.Contains(t, infoStr, "初始配置")
	assert.Contains(t, infoStr, "重载后的Info")
}

// TestManager_GlobalAndInstanceCoexist 测试全局和实例共存
func TestManager_GlobalAndInstanceCoexist(t *testing.T) {
	tmpDir := t.TempDir()
	globalLogDir := filepath.Join(tmpDir, "global")
	customLogDir := filepath.Join(tmpDir, "custom")

	// 重置全局
	globalManager = nil
	managerOnce.Do(func() {})
	managerOnce = sync.Once{}

	// 初始化全局 Manager
	InitManager(ManagerConfig{
		BaseLogDir:            globalLogDir,
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
	})

	// 创建自定义 Manager
	customManager := NewManager(ManagerConfig{
		BaseLogDir:            customLogDir,
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
	})

	// 全局使用
	Info("order", "全局订单创建")

	// 自定义使用
	customManager.InfoCtx(context.Background(), "order", "自定义订单创建")

	CloseAll()
	customManager.CloseAll()

	// 验证两个独立的日志文件
	assert.FileExists(t, filepath.Join(globalLogDir, "order", "order-info.log"))
	assert.FileExists(t, filepath.Join(customLogDir, "order", "order-info.log"))

	globalContent, _ := os.ReadFile(filepath.Join(globalLogDir, "order", "order-info.log"))
	assert.Contains(t, string(globalContent), "全局订单创建")

	customContent, _ := os.ReadFile(filepath.Join(customLogDir, "order", "order-info.log"))
	assert.Contains(t, string(customContent), "自定义订单创建")
}

// TestManager_IsolatedTesting 测试隔离的单元测试场景
func TestManager_IsolatedTesting(t *testing.T) {
	// 每个子测试使用独立的 Manager，互不干扰
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
