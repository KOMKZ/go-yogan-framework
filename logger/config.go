// src/pkg/logger/config.go
package logger

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap/zapcore"
)

// Config module log configuration (for internal use)
type Config struct {
	Level           string
	Development     bool
	Encoding        string // json, console or console_pretty
	ConsoleEncoding string

	// Internal fields (set automatically by Manager, no user action required)
	moduleName string // Business module name (e.g., order, auth, user)
	logDir     string // Log root directory (default logs/)

	EnableFile    bool
	EnableConsole bool

	// file name format configuration
	EnableLevelInFilename    bool   // Whether it includes level (info/error)
	EnableSequenceInFilename bool   // Whether it includes an ordinal number (01/02)
	SequenceNumber           string // Sequence number (e.g., "01")
	EnableDateInFilename     bool   // Does it contain a date
	DateFormat               string // Date format (default 2006-01-02)

	// File slicing configuration
	MaxSize    int  // Maximum size of individual file (MB)
	MaxBackups int  // Keep the number of old files
	MaxAge     int  // Number of days to retain
	Compress   bool // Whether to compress

	// stack configuration
	EnableCaller     bool
	EnableStacktrace bool
	StacktraceLevel  string // From which level to start recording the stack (default is error)
	StacktraceDepth  int    // Stack depth limit (0=unlimited, recommended 5-10)
}

// ManagerConfig global manager configuration (shared by all modules)
type ManagerConfig struct {
	BaseLogDir               string `mapstructure:"base_log_dir"` // Fix root directory (default logs/)
	Level                    string `mapstructure:"level"`
	AppName                  string `mapstructure:"app_name"`      // Application name (automatically injects all logs, including null values)
	Encoding                 string `mapstructure:"encoding"`
	ConsoleEncoding          string `mapstructure:"console_encoding"`
	EnableConsole            bool   `mapstructure:"enable_console"`
	EnableLevelInFilename    bool   `mapstructure:"enable_level_in_filename"`
	EnableSequenceInFilename bool   `mapstructure:"enable_sequence_in_filename"`
	EnableDateInFilename     bool   `mapstructure:"enable_date_in_filename"`
	DateFormat               string `mapstructure:"date_format"`
	MaxSize                  int    `mapstructure:"max_size"`
	MaxBackups               int    `mapstructure:"max_backups"`
	MaxAge                   int    `mapstructure:"max_age"`
	Compress                 bool   `mapstructure:"compress"`
	EnableCaller             bool   `mapstructure:"enable_caller"`
	EnableStacktrace         bool   `mapstructure:"enable_stacktrace"`
	StacktraceLevel          string `mapstructure:"stacktrace_level"`
	StacktraceDepth          int    `mapstructure:"stacktrace_depth"` // stack depth (0=unlimited)
	LoggerName               string `mapstructure:"logger_name"`
	ModuleNumber             int    `mapstructure:"module_number"`

	// Render style configuration (valid for console_pretty encoder only)
	// Optional values: single_line (default), key_value
	RenderStyle string `mapstructure:"render_style"`

	// Trace ID configuration
	EnableTraceID    bool   `mapstructure:"enable_trace_id"`     // Whether to enable automatic extraction of traceID
	TraceIDKey       string `mapstructure:"trace_id_key"`        // the key in context (default "trace_id")
	TraceIDFieldName string `mapstructure:"trace_id_field_name"` // Log field name (default "trace_id")
}

// Returns default manager configuration
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		BaseLogDir:               "logs",
		LoggerName:               "logger",
		Level:                    "info",
		Encoding:                 "json",
		EnableConsole:            true,
		EnableLevelInFilename:    true,
		EnableSequenceInFilename: false,
		EnableDateInFilename:     true,
		DateFormat:               "2006-01-02",
		MaxSize:                  100,
		MaxBackups:               3,
		MaxAge:                   28,
		Compress:                 true,
		EnableCaller:             true,
		EnableStacktrace:         true,
		StacktraceLevel:          "error", // Start logging stack from ERROR level
		StacktraceDepth:          5,       // By default, only record 5 stack levels to avoid overly long logs
		EnableTraceID:            true,
		TraceIDKey:               "trace_id",
		TraceIDFieldName:         "trace_id",
	}
}

// ApplyDefaults fills zero-valued fields with default values (in-place modification)
// For handling missing or zero-valued fields in configuration files
func (c *ManagerConfig) ApplyDefaults() {
	defaults := DefaultManagerConfig()

	// String type: an empty string is considered unconfigured
	if c.BaseLogDir == "" {
		c.BaseLogDir = defaults.BaseLogDir
	}
	if c.ModuleNumber == 0 {
		c.ModuleNumber = 50
	}
	if c.LoggerName == "" {
		c.LoggerName = defaults.LoggerName
	}
	if c.Level == "" {
		c.Level = defaults.Level
	}
	if c.Encoding == "" {
		c.Encoding = defaults.Encoding
	}
	if c.ConsoleEncoding == "" {
		c.ConsoleEncoding = defaults.ConsoleEncoding
	}
	if c.DateFormat == "" {
		c.DateFormat = defaults.DateFormat
	}
	if c.StacktraceLevel == "" {
		c.StacktraceLevel = defaults.StacktraceLevel
	}
	if c.TraceIDKey == "" {
		c.TraceIDKey = defaults.TraceIDKey
	}
	if c.TraceIDFieldName == "" {
		c.TraceIDFieldName = defaults.TraceIDFieldName
	}

	// Numeric type: 0 is considered unconfigured (note: MaxBackups=0 is a valid value but rarely used)
	if c.MaxSize == 0 {
		c.MaxSize = defaults.MaxSize
	}
	if c.MaxBackups == 0 {
		c.MaxBackups = defaults.MaxBackups
	}
	if c.MaxAge == 0 {
		c.MaxAge = defaults.MaxAge
	}

	// Boolean type: Unable to determine if configured, retain original value
	// If default values are needed, they should be explicitly set in the configuration file
}

// Validate configuration (implement config.Validator interface)
func (c *Config) Validate() error {
	// 1. Basic validation
	if c.logDir == "" {
		return fmt.Errorf("[Logger] [Logger] Log directory cannot be empty")
	}

	// 2. Enum validation
	validLevels := []string{"debug", "info", "warn", "error", "fatal"}
	if !contains(validLevels, c.Level) {
		return fmt.Errorf("[Logger] Log level must be: %v, current: %s", validLevels, c.Level)
	}

	validEncodings := []string{"json", "console", "console_pretty"}
	if !contains(validEncodings, c.Encoding) {
		return fmt.Errorf("[Logger] Encoding format must be: %v, current: %s", validEncodings, c.Encoding)
	}

	// 3. Range validation
	if c.MaxSize < 1 || c.MaxSize > 10000 {
		return fmt.Errorf("[Logger] File size must be between 1-10000MB, current: %d", c.MaxSize)
	}

	if c.MaxBackups < 0 || c.MaxBackups > 100 {
		return fmt.Errorf("[Logger] Number of backups must be between 0-100, current: %d", c.MaxBackups)
	}

	if c.MaxAge < 1 || c.MaxAge > 365 {
		return fmt.Errorf("[Logger] Days to retain must be between 1-365, current: %d", c.MaxAge)
	}

	// Business logic validation
	if c.EnableDateInFilename && c.DateFormat == "" {
		return fmt.Errorf("[Logger] [Logger] Date format must be specified when enabling date in filename")
	}

	return nil
}

// ParseLevel parse log level string
func ParseLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// Validate ManagerConfig configuration
func (c ManagerConfig) Validate() error {
	// Verify log level
	validLevels := []string{"debug", "info", "warn", "error", "fatal"}
	if !contains(validLevels, c.Level) {
		return fmt.Errorf("Invalid log level: %s (valid values: %v)", c.Level, validLevels)
	}

	// Validate encoding format
	validEncodings := []string{"json", "console", "console_pretty"}
	if !contains(validEncodings, c.Encoding) {
		return fmt.Errorf("Invalid log encoding: %s (valid values: %v)", c.Encoding, validEncodings)
	}

	// Verify file size
	if c.MaxSize < 1 || c.MaxSize > 10000 {
		return fmt.Errorf("MaxSize must be between 1-10000 MB, current: %d", c.MaxSize)
	}

	// Verify backup count
	if c.MaxBackups < 0 || c.MaxBackups > 1000 {
		return fmt.Errorf("MaxBackups must be between 0-1000, current: %d", c.MaxBackups)
	}

	// Validate reserved days
	if c.MaxAge < 0 || c.MaxAge > 3650 {
		return fmt.Errorf("MaxAge must be between 0-3650 days, current: %d", c.MaxAge)
	}

	// Verify stack trace level
	if !contains(validLevels, c.StacktraceLevel) {
		return fmt.Errorf("Invalid stack trace level: %s (valid values: %v)", c.StacktraceLevel, validLevels)
	}

	return nil
}

// contains Check if the string slice contains the specified string
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// Get module log directory (internal method)
// Return: logs/order/ or logs/auth/
func (c Config) getModuleLogDir() string {
	if c.moduleName == "" {
		return c.logDir
	}
	return filepath.Join(c.logDir, c.moduleName)
}

// getInfoFilePath Get the complete path of the Info log (internal method)
func (c Config) getInfoFilePath() string {
	return c.buildFilePath("info")
}

// Get error log full path (internal method)
func (c Config) getErrorFilePath() string {
	return c.buildFilePath("error")
}

// buildFilePath Build log file path (internal method)
// Support formats:
// - logs/order/order.log (module name only)
// - logs/order/order-info.log (module name + level)
// - logs/order/order-info-01.log (module name + level + sequence number)
// - logs/order/order-info-2024-12-19.log (module name + level + date)
// - logs/order/order-info-01-2024-12-19.log (full format)
func (c Config) buildFilePath(level string) string {
	parts := []string{c.moduleName}

	// Add level
	if c.EnableLevelInFilename {
		parts = append(parts, level)
	}

	// Add numbering
	if c.EnableSequenceInFilename && c.SequenceNumber != "" {
		parts = append(parts, c.SequenceNumber)
	}

	// Add date
	if c.EnableDateInFilename {
		date := time.Now().Format(c.DateFormat)
		parts = append(parts, date)
	}

	// Combine file names
	filename := strings.Join(parts, "-")

	// Return full path
	return filepath.Join(c.getModuleLogDir(), filename+".log")
}
