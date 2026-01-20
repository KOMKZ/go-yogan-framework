package logger

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	gormlogger "gorm.io/gorm/logger"
)

// TestGormLogger_Basic 测试 GormLogger 基本功能
func TestGormLogger_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "gorm")

	globalManager = nil
	managerOnce = sync.Once{}

	InitManager(ManagerConfig{
		BaseLogDir:            logDir,
		Level:                 "debug",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
	})

	// 创建 GormLogger
	gormLog := NewGormLogger(GormLoggerConfig{
		SlowThreshold: 200 * time.Millisecond,
		LogLevel:      gormlogger.Info,
		EnableAudit:   true,
	})

	assert.NotNil(t, gormLog)

	ctx := context.Background()

	// 测试 Info
	gormLog.Info(ctx, "GORM Info 消息: %s", "测试")

	// 测试 Warn
	gormLog.Warn(ctx, "GORM Warn 消息: %s", "警告")

	// 测试 Error
	gormLog.Error(ctx, "GORM Error 消息: %s", "错误")

	// 测试 Trace - 正常查询（启用审计）
	gormLog.Trace(ctx, time.Now().Add(-100*time.Millisecond), func() (string, int64) {
		return "SELECT * FROM users WHERE id = 1", 1
	}, nil)

	// 测试 Trace - 慢查询（超过阈值）
	gormLog.Trace(ctx, time.Now().Add(-500*time.Millisecond), func() (string, int64) {
		return "SELECT * FROM orders", 100
	}, nil)

	// 测试 Trace - 严重慢查询（超过阈值2倍）
	gormLog.Trace(ctx, time.Now().Add(-1*time.Second), func() (string, int64) {
		return "SELECT * FROM big_table", 1000
	}, nil)

	// 测试 Trace - 错误（非 RecordNotFound）
	gormLog.Trace(ctx, time.Now().Add(-50*time.Millisecond), func() (string, int64) {
		return "INSERT INTO users VALUES (1)", 0
	}, errors.New("duplicate key"))

	// 测试 Trace - RecordNotFound 错误（应该被忽略或审计）
	gormLog.Trace(ctx, time.Now().Add(-50*time.Millisecond), func() (string, int64) {
		return "SELECT * FROM users WHERE id = 999", 0
	}, gormlogger.ErrRecordNotFound)

	CloseAll()

	// 验证日志文件
	assert.DirExists(t, filepath.Join(logDir, "yogan_sql"))

	infoContent, _ := os.ReadFile(filepath.Join(logDir, "yogan_sql", "yogan_sql-info.log"))
	infoStr := string(infoContent)
	assert.Contains(t, infoStr, "GORM Info 消息")
	assert.Contains(t, infoStr, "GORM Warn 消息")
	assert.Contains(t, infoStr, "SELECT")
}

// TestGormLogger_LogMode 测试 LogMode
func TestGormLogger_LogMode(t *testing.T) {
	gormLog := NewGormLogger(DefaultGormLoggerConfig())

	// 测试 LogMode - 返回新的 Logger
	silentLog := gormLog.LogMode(gormlogger.Silent)
	assert.NotNil(t, silentLog)

	warnLog := gormLog.LogMode(gormlogger.Warn)
	assert.NotNil(t, warnLog)

	errorLog := gormLog.LogMode(gormlogger.Error)
	assert.NotNil(t, errorLog)
}

// TestDefaultGormLoggerConfig 测试默认配置
func TestDefaultGormLoggerConfig(t *testing.T) {
	cfg := DefaultGormLoggerConfig()

	assert.Equal(t, 200*time.Millisecond, cfg.SlowThreshold)
	assert.True(t, cfg.EnableAudit)
	assert.Equal(t, gormlogger.Info, cfg.LogLevel)
}

// TestGormLogger_SilentMode 测试 Silent 模式
func TestGormLogger_SilentMode(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "gorm_silent")

	globalManager = nil
	managerOnce = sync.Once{}

	InitManager(ManagerConfig{
		BaseLogDir:            logDir,
		Level:                 "debug",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
	})

	// 创建 Silent 模式的 Logger
	gormLog := NewGormLogger(GormLoggerConfig{
		SlowThreshold: 200 * time.Millisecond,
		LogLevel:      gormlogger.Silent, // Silent 模式
		EnableAudit:   true,
	})

	ctx := context.Background()

	// Silent 模式下不应该记录任何日志
	gormLog.Trace(ctx, time.Now().Add(-100*time.Millisecond), func() (string, int64) {
		return "SELECT * FROM users", 1
	}, nil)

	CloseAll()
}
