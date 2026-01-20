package logger

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	gormlogger "gorm.io/gorm/logger"
)

// GormLogger GORM 自定义 Logger（实现 gorm logger.Interface 接口）
// 统一使用 gorm_sql 模块记录所有数据库日志
type GormLogger struct {
	slowThreshold time.Duration       // 慢查询阈值
	logLevel      gormlogger.LogLevel // 日志级别
	enableAudit   bool                // 是否启用 SQL 审计
}

// GormLoggerConfig GORM Logger 配置
type GormLoggerConfig struct {
	SlowThreshold time.Duration       // 慢查询阈值，默认 200ms
	LogLevel      gormlogger.LogLevel // 日志级别
	EnableAudit   bool                // 是否启用 SQL 审计，默认 true
}

// DefaultGormLoggerConfig 默认配置
func DefaultGormLoggerConfig() GormLoggerConfig {
	return GormLoggerConfig{
		SlowThreshold: 200 * time.Millisecond,
		LogLevel:      gormlogger.Info,
		EnableAudit:   true,
	}
}

// NewGormLogger 创建自定义 GORM Logger
func NewGormLogger(cfg GormLoggerConfig) *GormLogger {
	return &GormLogger{
		slowThreshold: cfg.SlowThreshold,
		logLevel:      cfg.LogLevel,
		enableAudit:   cfg.EnableAudit,
	}
}

// LogMode 设置日志级别（实现 gorm logger.Interface）
func (l *GormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	newLogger := *l
	newLogger.logLevel = level
	return &newLogger
}

// Info 记录 Info 级别日志（实现 gorm logger.Interface）
func (l *GormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.logLevel >= gormlogger.Info {
		DebugCtx(ctx, "yogan_sql", fmt.Sprintf(msg, data...))
	}
}

// Warn 记录 Warn 级别日志（实现 gorm logger.Interface）
func (l *GormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.logLevel >= gormlogger.Warn {
		WarnCtx(ctx, "yogan_sql", fmt.Sprintf(msg, data...))
	}
}

// Error 记录 Error 级别日志（实现 gorm logger.Interface）
func (l *GormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.logLevel >= gormlogger.Error {
		ErrorCtx(ctx, "yogan_sql", fmt.Sprintf(msg, data...))
	}
}

// Trace 记录 SQL 执行日志（实现 gorm logger.Interface）
// 这是最重要的方法，用于记录所有 SQL 执行情况
func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	// 如果日志级别为 Silent，不记录任何日志
	if l.logLevel <= gormlogger.Silent {
		return
	}

	// 计算执行时间
	elapsed := time.Since(begin)

	// 获取 SQL 语句和影响行数
	sql, rows := fc()

	// 对 SQL 进行脱敏处理
	sanitizedSQL := sanitizeSQL(sql)

	// 构建基础日志字段
	fields := []zap.Field{
		zap.String("sql", sanitizedSQL),
		zap.Duration("elapsed", elapsed),
		zap.Int64("rows", rows),
	}

	// ✅ 使用 Context API，自动关联 TraceID（无需手动提取）

	// 根据不同情况记录日志（统一使用 gorm_sql 模块）
	switch {
	case err != nil && l.logLevel >= gormlogger.Error:
		// SQL 执行错误
		if !errors.Is(err, gormlogger.ErrRecordNotFound) {
			// 忽略 RecordNotFound 错误（这是正常业务逻辑）
			fields = append(fields, zap.Error(err))
			ErrorCtx(ctx, "yogan_sql", "SQL 执行错误", fields...)
		} else if l.enableAudit {
			DebugCtx(ctx, "yogan_sql", "SQL 执行", fields...)
		}

	case elapsed > l.slowThreshold && l.slowThreshold != 0 && l.logLevel >= gormlogger.Warn:
		// 慢查询检测
		fields = append(fields, zap.Duration("threshold", l.slowThreshold))

		// 根据慢查询严重程度选择日志级别
		if elapsed > l.slowThreshold*2 {
			// 严重慢查询（超过阈值2倍）
			ErrorCtx(ctx, "yogan_sql", "严重慢查询", fields...)
		} else {
			// 一般慢查询
			WarnCtx(ctx, "yogan_sql", "慢查询检测", fields...)
		}

	case l.logLevel >= gormlogger.Info:
		// 正常 SQL 执行
		if l.enableAudit {
			// 审计日志：记录所有 SQL 执行
			DebugCtx(ctx, "yogan_sql", "SQL 执行", fields...)
		}
	}
}
