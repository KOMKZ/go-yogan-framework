package logger

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	gormlogger "gorm.io/gorm/logger"
)

// GormLogger custom GORM Logger (implements the gorm logger.Interface interface)
// Use the gorm_sql module uniformly to log all database logs
type GormLogger struct {
	slowThreshold time.Duration       // slow query threshold
	logLevel      gormlogger.LogLevel // Log level
	enableAudit   bool                // Whether to enable SQL audit
}

// GormLoggerConfig GORM Logger configuration
type GormLoggerConfig struct {
	SlowThreshold time.Duration       // Slow query threshold, default 200ms
	LogLevel      gormlogger.LogLevel // Log level
	EnableAudit   bool                // Whether to enable SQL auditing, default is true
}

// DefaultGormLoggerConfig default configuration
func DefaultGormLoggerConfig() GormLoggerConfig {
	return GormLoggerConfig{
		SlowThreshold: 200 * time.Millisecond,
		LogLevel:      gormlogger.Info,
		EnableAudit:   true,
	}
}

// Create custom GORM Logger
func NewGormLogger(cfg GormLoggerConfig) *GormLogger {
	return &GormLogger{
		slowThreshold: cfg.SlowThreshold,
		logLevel:      cfg.LogLevel,
		enableAudit:   cfg.EnableAudit,
	}
}

// LogMode sets the log level (implements gorm logger.Interface)
func (l *GormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	newLogger := *l
	newLogger.logLevel = level
	return &newLogger
}

// Info level log recording (implements gorm logger.Interface)
func (l *GormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.logLevel >= gormlogger.Info {
		DebugCtx(ctx, "yogan_sql", fmt.Sprintf(msg, data...))
	}
}

// Warn record Warn level log (implement gorm Logger Interface)
func (l *GormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.logLevel >= gormlogger.Warn {
		WarnCtx(ctx, "yogan_sql", fmt.Sprintf(msg, data...))
	}
}

// Error level logging (implements gorm Logger Interface)
func (l *GormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.logLevel >= gormlogger.Error {
		ErrorCtx(ctx, "yogan_sql", fmt.Sprintf(msg, data...))
	}
}

// Record SQL execution log (implement gorm Logger Interface)
// This is the most important method for logging all SQL executions
func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	// If the log level is Silent, do not record any logs
	if l.logLevel <= gormlogger.Silent {
		return
	}

	// Calculate execution time
	elapsed := time.Since(begin)

	// Get the SQL statement and affected row count
	sql, rows := fc()

	// Sanitize the SQL
	sanitizedSQL := sanitizeSQL(sql)

	// Build basic log fields
	fields := []zap.Field{
		zap.String("sql", sanitizedSQL),
		zap.Duration("elapsed", elapsed),
		zap.Int64("rows", rows),
	}

	// ✅ Uses Context API to automatically associate TraceID (no need for manual extraction)

	// Log according to different situations (uniformly use the gorm_sql module)
	switch {
	case err != nil && l.logLevel >= gormlogger.Error:
		// SQL execution error
		if !errors.Is(err, gormlogger.ErrRecordNotFound) {
			// Ignore RecordNotFound error (this is normal business logic)
			fields = append(fields, zap.Error(err))
			ErrorCtx(ctx, "yogan_sql", "SQL 执行错误", fields...)
		} else if l.enableAudit {
			DebugCtx(ctx, "yogan_sql", "SQL 执行", fields...)
		}

	case elapsed > l.slowThreshold && l.slowThreshold != 0 && l.logLevel >= gormlogger.Warn:
		// Slow query detection
		fields = append(fields, zap.Duration("threshold", l.slowThreshold))

		// Select log level based on severity of slow queries
		if elapsed > l.slowThreshold*2 {
			// Severe slow query (above twice the threshold)
			ErrorCtx(ctx, "yogan_sql", "严重慢查询", fields...)
		} else {
			// general slow query
			WarnCtx(ctx, "yogan_sql", "慢查询检测", fields...)
		}

	case l.logLevel >= gormlogger.Info:
		// Normal SQL execution
		if l.enableAudit {
			// Audit log: record all SQL executions
			DebugCtx(ctx, "yogan_sql", "SQL 执行", fields...)
		}
	}
}
