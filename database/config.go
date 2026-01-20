// Provides database management and Repository infrastructure
package database

import (
	"time"
)

// Configuration database configuration
type Config struct {
	Driver          string        `mapstructure:"driver"`            // Driver types: mysql, postgres, sqlite
	DSN             string        `mapstructure:"dsn"`               // data source name
	MaxOpenConns    int           `mapstructure:"max_open_conns"`    // Maximum number of open connections
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`    // Maximum number of idle connections
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"` // Connection maximum lifetime (seconds)
	EnableLog       bool          `mapstructure:"enable_log"`        // Whether logging is enabled
	SlowThreshold   time.Duration `mapstructure:"slow_threshold"`    // slow query threshold (milliseconds)
	EnableAudit     bool          `mapstructure:"enable_audit"`      // Whether to enable SQL auditing

	// OpenTelemetry tracing configuration
	TraceSQL       bool `mapstructure:"trace_sql"`         // Whether to log SQL statements in an OTel Span (default false)
	TraceSQLMaxLen int  `mapstructure:"trace_sql_max_len"` // SQL statement maximum length (default 1000)
}

// Return the default configuration
func DefaultConfig() Config {
	return Config{
		Driver:          "mysql",
		MaxOpenConns:    100,
		MaxIdleConns:    10,
		ConnMaxLifetime: 3600 * time.Second,
		EnableLog:       true,
		SlowThreshold:   200 * time.Millisecond, // Default 200ms
		EnableAudit:     true,                   // Enable audit by default
		TraceSQL:        false,                  // By default, do not record SQL to Span (performance consideration)
		TraceSQLMaxLen:  1000,                   // SQL maximum length 1000 characters
	}
}

// Validate configuration
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
		c.TraceSQLMaxLen = 1000 // Default 1000 characters
	}
	return nil
}
