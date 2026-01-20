// src/pkg/logger/config_test.go
package logger

import (
	"testing"
)

// TestManagerConfig_ApplyDefaults test default value population
func TestManagerConfig_ApplyDefaults(t *testing.T) {
	tests := []struct {
		name     string
		input    ManagerConfig
		expected ManagerConfig
	}{
		{
			name:  "空配置应填充所有默认值",
			input: ManagerConfig{},
			expected: ManagerConfig{
				BaseLogDir:               "logs",
				LoggerName:               "logger",
				Level:                    "info",
				Encoding:                 "json",
				ConsoleEncoding:          "",
				EnableConsole:            false, // Boolean type retains original value
				EnableLevelInFilename:    false,
				EnableSequenceInFilename: false,
				EnableDateInFilename:     false,
				DateFormat:               "2006-01-02",
				MaxSize:                  100,
				MaxBackups:               3,
				MaxAge:                   28,
				Compress:                 false,
				EnableCaller:             false,
				EnableStacktrace:         false,
				StacktraceLevel:          "error",
				EnableTraceID:            false,
				TraceIDKey:               "trace_id",
				TraceIDFieldName:         "trace_id",
			},
		},
		{
			name: "部分配置应保留用户值",
			input: ManagerConfig{
				Level:   "debug",
				MaxSize: 200,
			},
			expected: ManagerConfig{
				BaseLogDir:       "logs",      // Fill with default values
				LoggerName:       "logger",    // Fill with default values
				Level:            "debug",     // preserve user values
				Encoding:         "json",      // Fill with default values
				DateFormat:       "2006-01-02", // Set default values
				MaxSize:          200,         // preserve user values
				MaxBackups:       3,           // Set default values
				MaxAge:           28,          // Fill with default values
				StacktraceLevel:  "error",     // Fill with default values
				TraceIDKey:       "trace_id",  // Fill with default values
				TraceIDFieldName: "trace_id",  // Fill with default values
			},
		},
		{
			name: "完整配置不应被覆盖",
			input: ManagerConfig{
				BaseLogDir:      "custom/logs",
				LoggerName:      "custom",
				Level:           "warn",
				Encoding:        "console",
				ConsoleEncoding: "console_pretty",
				DateFormat:      "2006-01-02-15",
				MaxSize:         500,
				MaxBackups:      10,
				MaxAge:          90,
				StacktraceLevel: "fatal",
				TraceIDKey:      "request_id",
				TraceIDFieldName: "req_id",
			},
			expected: ManagerConfig{
				BaseLogDir:       "custom/logs",     // preserve user values
				LoggerName:       "custom",          // preserve user values
				Level:            "warn",            // preserve user values
				Encoding:         "console",         // preserve user values
				ConsoleEncoding:  "console_pretty",  // preserve user values
				DateFormat:       "2006-01-02-15",   // preserve user values
				MaxSize:          500,               // preserve user values
				MaxBackups:       10,                // preserve user values
				MaxAge:           90,                // preserve user values
				StacktraceLevel:  "fatal",           // preserve user values
				TraceIDKey:       "request_id",      // preserve user values
				TraceIDFieldName: "req_id",          // retain user values
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.input
			cfg.ApplyDefaults()

			// Validate string field
			if cfg.BaseLogDir != tt.expected.BaseLogDir {
				t.Errorf("BaseLogDir: got %s, want %s", cfg.BaseLogDir, tt.expected.BaseLogDir)
			}
			if cfg.LoggerName != tt.expected.LoggerName {
				t.Errorf("LoggerName: got %s, want %s", cfg.LoggerName, tt.expected.LoggerName)
			}
			if cfg.Level != tt.expected.Level {
				t.Errorf("Level: got %s, want %s", cfg.Level, tt.expected.Level)
			}
			if cfg.Encoding != tt.expected.Encoding {
				t.Errorf("Encoding: got %s, want %s", cfg.Encoding, tt.expected.Encoding)
			}
			if cfg.DateFormat != tt.expected.DateFormat {
				t.Errorf("DateFormat: got %s, want %s", cfg.DateFormat, tt.expected.DateFormat)
			}
			if cfg.StacktraceLevel != tt.expected.StacktraceLevel {
				t.Errorf("StacktraceLevel: got %s, want %s", cfg.StacktraceLevel, tt.expected.StacktraceLevel)
			}
			if cfg.TraceIDKey != tt.expected.TraceIDKey {
				t.Errorf("TraceIDKey: got %s, want %s", cfg.TraceIDKey, tt.expected.TraceIDKey)
			}
			if cfg.TraceIDFieldName != tt.expected.TraceIDFieldName {
				t.Errorf("TraceIDFieldName: got %s, want %s", cfg.TraceIDFieldName, tt.expected.TraceIDFieldName)
			}

			// Validate numeric fields
			if cfg.MaxSize != tt.expected.MaxSize {
				t.Errorf("MaxSize: got %d, want %d", cfg.MaxSize, tt.expected.MaxSize)
			}
			if cfg.MaxBackups != tt.expected.MaxBackups {
				t.Errorf("MaxBackups: got %d, want %d", cfg.MaxBackups, tt.expected.MaxBackups)
			}
			if cfg.MaxAge != tt.expected.MaxAge {
				t.Errorf("MaxAge: got %d, want %d", cfg.MaxAge, tt.expected.MaxAge)
			}

			// Validate boolean fields (retain original values, do not fill)
			if cfg.EnableConsole != tt.expected.EnableConsole {
				t.Errorf("EnableConsole: got %v, want %v", cfg.EnableConsole, tt.expected.EnableConsole)
			}
		})
	}
}

// TestNewManager_ApplyDefaults test NewManager auto-fill default values
func TestNewManager_ApplyDefaults(t *testing.T) {
	// 1. Empty configuration should be filled with default values
	m1 := NewManager(ManagerConfig{})
	if m1.baseConfig.Level != "info" {
		t.Errorf("Empty configuration should be filled with default Level=info, actual: %s Level=info，Empty configuration should be filled with default Level=info, actual: %s: %s", m1.baseConfig.Level)
	}
	if m1.baseConfig.MaxSize != 100 {
		t.Errorf("English: Empty configuration should be filled with default MaxSize=100, actual: %d MaxSize=100，English: Empty configuration should be filled with default MaxSize=100, actual: %d: %d", m1.baseConfig.MaxSize)
	}

	// 2. Some configurations should retain user values
	m2 := NewManager(ManagerConfig{
		Level:   "debug",
		MaxSize: 200,
	})
	if m2.baseConfig.Level != "debug" {
		t.Errorf("User configuration should be retained Level=debug, actual: %s Level=debug，User configuration should be retained Level=debug, actual: %s: %s", m2.baseConfig.Level)
	}
	if m2.baseConfig.MaxSize != 200 {
		t.Errorf("English: User configuration should retain MaxSize=200, actual: %d MaxSize=200，English: User configuration should retain MaxSize=200, actual: %d: %d", m2.baseConfig.MaxSize)
	}
	if m2.baseConfig.MaxBackups != 3 {
		t.Errorf("The unconfigured field should be filled with default MaxBackups=3, actual: %d MaxBackups=3，The unconfigured field should be filled with default MaxBackups=3, actual: %d: %d", m2.baseConfig.MaxBackups)
	}

	// 3. Verify that the loggers map has been initialized
	if m1.loggers == nil {
		t.Error("NewManager English: NewManager should initialize the loggers map loggers map")
	}
}

// TestManagerConfig_Validate test configuration validation
func TestManagerConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		cfg       ManagerConfig
		wantError bool
	}{
		{
			name:      "默认配置应通过验证",
			cfg:       DefaultManagerConfig(),
			wantError: false,
		},
		{
			name: "无效日志级别应报错",
			cfg: ManagerConfig{
				Level:           "invalid",
				Encoding:        "json",
				MaxSize:         100,
				MaxAge:          28,
				StacktraceLevel: "error",
			},
			wantError: true,
		},
		{
			name: "无效编码格式应报错",
			cfg: ManagerConfig{
				Level:           "info",
				Encoding:        "xml",
				MaxSize:         100,
				MaxAge:          28,
				StacktraceLevel: "error",
			},
			wantError: true,
		},
		{
			name: "MaxSize超出范围应报错",
			cfg: ManagerConfig{
				Level:           "info",
				Encoding:        "json",
				MaxSize:         20000,
				MaxAge:          28,
				StacktraceLevel: "error",
			},
			wantError: true,
		},
		{
			name: "MaxAge超出范围应报错",
			cfg: ManagerConfig{
				Level:           "info",
				Encoding:        "json",
				MaxSize:         100,
				MaxAge:          5000,
				StacktraceLevel: "error",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

