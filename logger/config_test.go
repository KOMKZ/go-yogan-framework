// src/pkg/logger/config_test.go
package logger

import (
	"testing"
)

// TestManagerConfig_ApplyDefaults 测试默认值填充
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
				EnableConsole:            false, // 布尔类型保持原值
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
				BaseLogDir:       "logs",      // 填充默认值
				LoggerName:       "logger",    // 填充默认值
				Level:            "debug",     // 保留用户值
				Encoding:         "json",      // 填充默认值
				DateFormat:       "2006-01-02", // 填充默认值
				MaxSize:          200,         // 保留用户值
				MaxBackups:       3,           // 填充默认值
				MaxAge:           28,          // 填充默认值
				StacktraceLevel:  "error",     // 填充默认值
				TraceIDKey:       "trace_id",  // 填充默认值
				TraceIDFieldName: "trace_id",  // 填充默认值
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
				BaseLogDir:       "custom/logs",     // 保留用户值
				LoggerName:       "custom",          // 保留用户值
				Level:            "warn",            // 保留用户值
				Encoding:         "console",         // 保留用户值
				ConsoleEncoding:  "console_pretty",  // 保留用户值
				DateFormat:       "2006-01-02-15",   // 保留用户值
				MaxSize:          500,               // 保留用户值
				MaxBackups:       10,                // 保留用户值
				MaxAge:           90,                // 保留用户值
				StacktraceLevel:  "fatal",           // 保留用户值
				TraceIDKey:       "request_id",      // 保留用户值
				TraceIDFieldName: "req_id",          // 保留用户值
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.input
			cfg.ApplyDefaults()

			// 验证字符串字段
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

			// 验证数值字段
			if cfg.MaxSize != tt.expected.MaxSize {
				t.Errorf("MaxSize: got %d, want %d", cfg.MaxSize, tt.expected.MaxSize)
			}
			if cfg.MaxBackups != tt.expected.MaxBackups {
				t.Errorf("MaxBackups: got %d, want %d", cfg.MaxBackups, tt.expected.MaxBackups)
			}
			if cfg.MaxAge != tt.expected.MaxAge {
				t.Errorf("MaxAge: got %d, want %d", cfg.MaxAge, tt.expected.MaxAge)
			}

			// 验证布尔字段（保持原值，不填充）
			if cfg.EnableConsole != tt.expected.EnableConsole {
				t.Errorf("EnableConsole: got %v, want %v", cfg.EnableConsole, tt.expected.EnableConsole)
			}
		})
	}
}

// TestNewManager_ApplyDefaults 测试 NewManager 自动填充默认值
func TestNewManager_ApplyDefaults(t *testing.T) {
	// 1. 空配置应填充默认值
	m1 := NewManager(ManagerConfig{})
	if m1.baseConfig.Level != "info" {
		t.Errorf("空配置应填充默认 Level=info，实际: %s", m1.baseConfig.Level)
	}
	if m1.baseConfig.MaxSize != 100 {
		t.Errorf("空配置应填充默认 MaxSize=100，实际: %d", m1.baseConfig.MaxSize)
	}

	// 2. 部分配置应保留用户值
	m2 := NewManager(ManagerConfig{
		Level:   "debug",
		MaxSize: 200,
	})
	if m2.baseConfig.Level != "debug" {
		t.Errorf("应保留用户配置 Level=debug，实际: %s", m2.baseConfig.Level)
	}
	if m2.baseConfig.MaxSize != 200 {
		t.Errorf("应保留用户配置 MaxSize=200，实际: %d", m2.baseConfig.MaxSize)
	}
	if m2.baseConfig.MaxBackups != 3 {
		t.Errorf("未配置字段应填充默认 MaxBackups=3，实际: %d", m2.baseConfig.MaxBackups)
	}

	// 3. 验证 loggers map 已初始化
	if m1.loggers == nil {
		t.Error("NewManager 应初始化 loggers map")
	}
}

// TestManagerConfig_Validate 测试配置验证
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

