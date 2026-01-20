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

// TestGormLogger_Basic tests the basic functionality of GormLogger
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

	// Create GormLogger
	gormLog := NewGormLogger(GormLoggerConfig{
		SlowThreshold: 200 * time.Millisecond,
		LogLevel:      gormlogger.Info,
		EnableAudit:   true,
	})

	assert.NotNil(t, gormLog)

	ctx := context.Background()

	// Test Info
	gormLog.Info(ctx, "GORM Info English: GORM Info Message: %s: %s", "English: GORM Info Message: %s")

	// Test Warn
	gormLog.Warn(ctx, "GORM Warn English: GORM Warning Message: %s: %s", "English: GORM Warning Message: %s")

	// Test Error
	gormLog.Error(ctx, "GORM Error GORM Error Message: %s: %s", "GORM Error Message: %s")

	// Test Trace - Normal Query (Audit Enabled)
	gormLog.Trace(ctx, time.Now().Add(-100*time.Millisecond), func() (string, int64) {
		return "SELECT * FROM users WHERE id = 1", 1
	}, nil)

	// Test Trace - Slow queries (exceeding threshold)
	gormLog.Trace(ctx, time.Now().Add(-500*time.Millisecond), func() (string, int64) {
		return "SELECT * FROM orders", 100
	}, nil)

	// Test Trace - Severe slow queries (exceeding threshold by 2 times)
	gormLog.Trace(ctx, time.Now().Add(-1*time.Second), func() (string, int64) {
		return "SELECT * FROM big_table", 1000
	}, nil)

	// Test Trace - Error (not RecordNotFound)
	gormLog.Trace(ctx, time.Now().Add(-50*time.Millisecond), func() (string, int64) {
		return "INSERT INTO users VALUES (1)", 0
	}, errors.New("duplicate key"))

	// Test Trace - RecordNotFound error (should be ignored or audited)
	gormLog.Trace(ctx, time.Now().Add(-50*time.Millisecond), func() (string, int64) {
		return "SELECT * FROM users WHERE id = 999", 0
	}, gormlogger.ErrRecordNotFound)

	CloseAll()

	// Validate log file
	assert.DirExists(t, filepath.Join(logDir, "yogan_sql"))

	infoContent, _ := os.ReadFile(filepath.Join(logDir, "yogan_sql", "yogan_sql-info.log"))
	infoStr := string(infoContent)
	assert.Contains(t, infoStr, "GORM Info 消息")
	assert.Contains(t, infoStr, "GORM Warn 消息")
	assert.Contains(t, infoStr, "SELECT")
}

// TestGormLogger_LogMode test LogMode
func TestGormLogger_LogMode(t *testing.T) {
	gormLog := NewGormLogger(DefaultGormLoggerConfig())

	// Test LogMode - return new logger
	silentLog := gormLog.LogMode(gormlogger.Silent)
	assert.NotNil(t, silentLog)

	warnLog := gormLog.LogMode(gormlogger.Warn)
	assert.NotNil(t, warnLog)

	errorLog := gormLog.LogMode(gormlogger.Error)
	assert.NotNil(t, errorLog)
}

// TestDefaultGormLoggerConfig test default configuration
func TestDefaultGormLoggerConfig(t *testing.T) {
	cfg := DefaultGormLoggerConfig()

	assert.Equal(t, 200*time.Millisecond, cfg.SlowThreshold)
	assert.True(t, cfg.EnableAudit)
	assert.Equal(t, gormlogger.Info, cfg.LogLevel)
}

// TestGormLogger_SilentMode test Silent mode
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

	// Create Silent mode Logger
	gormLog := NewGormLogger(GormLoggerConfig{
		SlowThreshold: 200 * time.Millisecond,
		LogLevel:      gormlogger.Silent, // Silent Mode
		EnableAudit:   true,
	})

	ctx := context.Background()

	// In silent mode, no logs should be recorded
	gormLog.Trace(ctx, time.Now().Add(-100*time.Millisecond), func() (string, int64) {
		return "SELECT * FROM users", 1
	}, nil)

	CloseAll()
}
