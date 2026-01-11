// Package database 提供数据库管理和 Repository 基础设施
package database

import (
	"time"
)

// Config 数据库配置
type Config struct {
	Driver          string        `mapstructure:"driver"`            // 驱动类型: mysql, postgres, sqlite
	DSN             string        `mapstructure:"dsn"`               // 数据源名称
	MaxOpenConns    int           `mapstructure:"max_open_conns"`    // 最大打开连接数
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`    // 最大空闲连接数
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"` // 连接最大生存时间（秒）
	EnableLog       bool          `mapstructure:"enable_log"`        // 是否启用日志
	SlowThreshold   time.Duration `mapstructure:"slow_threshold"`    // 慢查询阈值（毫秒）
	EnableAudit     bool          `mapstructure:"enable_audit"`      // 是否启用 SQL 审计

	// OpenTelemetry 追踪配置
	TraceSQL       bool `mapstructure:"trace_sql"`         // 是否在 OTel Span 中记录 SQL 语句（默认 false）
	TraceSQLMaxLen int  `mapstructure:"trace_sql_max_len"` // SQL 语句最大长度（默认 1000）
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		Driver:          "mysql",
		MaxOpenConns:    100,
		MaxIdleConns:    10,
		ConnMaxLifetime: 3600 * time.Second,
		EnableLog:       true,
		SlowThreshold:   200 * time.Millisecond, // 默认 200ms
		EnableAudit:     true,                   // 默认启用审计
		TraceSQL:        false,                  // 默认不记录 SQL 到 Span（性能考虑）
		TraceSQLMaxLen:  1000,                   // SQL 最大长度 1000 字符
	}
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Driver == "" {
		c.Driver = "mysql"
	}
	if c.DSN == "" {
		return ErrInvalidConfig
	}
	if c.MaxOpenConns <= 0 {
		c.MaxOpenConns = 100
	}
	if c.MaxIdleConns <= 0 {
		c.MaxIdleConns = 10
	}
	if c.ConnMaxLifetime <= 0 {
		c.ConnMaxLifetime = 3600 * time.Second
	}
	if c.SlowThreshold <= 0 {
		c.SlowThreshold = 200 * time.Millisecond
	}
	if c.TraceSQLMaxLen <= 0 {
		c.TraceSQLMaxLen = 1000 // 默认 1000 字符
	}
	return nil
}
