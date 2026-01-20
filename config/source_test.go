package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFileSource test file data source
func TestFileSource(t *testing.T) {
	testdataDir := "testdata"

	tests := []struct {
		name        string
		filePath    string
		priority    int
		expectError bool
		expectKeys  []string
	}{
		{
			name:        "加载基础配置文件",
			filePath:    filepath.Join(testdataDir, "config.yaml"),
			priority:    10,
			expectError: false,
			expectKeys:  []string{"app.name", "grpc.server.port", "api_server.port"},
		},
		{
			name:        "加载环境配置文件",
			filePath:    filepath.Join(testdataDir, "dev.yaml"),
			priority:    20,
			expectError: false,
			expectKeys:  []string{"grpc.server.port", "api_server.port"},
		},
		{
			name:        "文件不存在（非错误）",
			filePath:    filepath.Join(testdataDir, "notexist.yaml"),
			priority:    10,
			expectError: false,
			expectKeys:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := NewFileSource(tt.filePath, tt.priority)

			// Validate basic properties
			if source.Name() != "file:"+tt.filePath {
				t.Errorf("Name() = %s, want %s", source.Name(), "file:"+tt.filePath)
			}

			if source.Priority() != tt.priority {
				t.Errorf("Priority() = %d, want %d", source.Priority(), tt.priority)
			}

			// Load configuration
			data, err := source.Load()
			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Validate that the key exists
			for _, key := range tt.expectKeys {
				if _, ok := data[key]; !ok {
					t.Errorf("expected key %s not found in data", key)
				}
			}
		})
	}
}

// TestFileSource_Values test configuration values
func TestFileSource_Values(t *testing.T) {
	source := NewFileSource("testdata/config.yaml", 10)
	data, err := source.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	tests := []struct {
		key      string
		expected interface{}
	}{
		{"app.name", "test-app"},
		{"app.version", "1.0.0"},
		{"grpc.server.port", 9002},
		{"grpc.server.address", "0.0.0.0"},
		{"api_server.port", 8080},
		{"api_server.host", "127.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			value, ok := data[tt.key]
			if !ok {
				t.Errorf("key %s not found", tt.key)
				return
			}

			// Type conversion and comparison
			switch expected := tt.expected.(type) {
			case string:
				if value != expected {
					t.Errorf("value = %v, want %v", value, expected)
				}
			case int:
				// Viper may return int or int64
				switch v := value.(type) {
				case int:
					if v != expected {
						t.Errorf("value = %v, want %v", v, expected)
					}
				case int64:
					if int(v) != expected {
						t.Errorf("value = %v, want %v", v, expected)
					}
				default:
					t.Errorf("unexpected type %T for key %s", value, tt.key)
				}
			}
		})
	}
}

// TestEnvSource test environment variable data source
func TestEnvSource(t *testing.T) {
	// Set test environment variables
	os.Setenv("TEST_GRPC_SERVER_PORT", "9999")
	os.Setenv("TEST_GRPC_SERVER_ADDRESS", "192.168.1.1")
	os.Setenv("TEST_API_SERVER_PORT", "8888")
	defer func() {
		os.Unsetenv("TEST_GRPC_SERVER_PORT")
		os.Unsetenv("TEST_GRPC_SERVER_ADDRESS")
		os.Unsetenv("TEST_API_SERVER_PORT")
	}()

	source := NewEnvSource("TEST", 50)

	if source.Name() != "env:TEST" {
		t.Errorf("Name() = %s, want env:TEST", source.Name())
	}

	if source.Priority() != 50 {
		t.Errorf("Priority() = %d, want 50", source.Priority())
	}

	data, err := source.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	tests := []struct {
		key      string
		expected string
	}{
		{"grpc.server.port", "9999"},
		{"grpc.server.address", "192.168.1.1"},
		{"api.server.port", "8888"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			value, ok := data[tt.key]
			if !ok {
				t.Errorf("key %s not found", tt.key)
				return
			}

			if value != tt.expected {
				t.Errorf("value = %v, want %v", value, tt.expected)
			}
		})
	}
}

// TestFlagSource test command line parameter data source
func TestFlagSource(t *testing.T) {
	// Define the test flags structure
	type TestFlags struct {
		Port    int    `config:"grpc.server.port,api_server.port"`
		Address string `config:"grpc.server.address"`
	}

	tests := []struct {
		name        string
		flags       interface{}
		appType     string
		expectKeys  map[string]interface{}
	}{
		{
			name: "使用 config tag 映射",
			flags: &TestFlags{
				Port:    7777,
				Address: "10.0.0.1",
			},
			appType: "grpc",
			expectKeys: map[string]interface{}{
				"grpc.server.port":    7777,
				"api_server.port":     7777,
				"grpc.server.address": "10.0.0.1",
			},
		},
		{
			name: "零值不覆盖",
			flags: &TestFlags{
				Port:    0,
				Address: "",
			},
			appType:    "grpc",
			expectKeys: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := NewFlagSource(tt.flags, tt.appType, 100)

			if source.Name() != "flags" {
				t.Errorf("Name() = %s, want flags", source.Name())
			}

			if source.Priority() != 100 {
				t.Errorf("Priority() = %d, want 100", source.Priority())
			}

			data, err := source.Load()
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}

			// Verify the expected key
			for key, expected := range tt.expectKeys {
				value, ok := data[key]
				if !ok {
					t.Errorf("key %s not found", key)
					continue
				}

				if value != expected {
					t.Errorf("key %s: value = %v, want %v", key, value, expected)
				}
			}

			// Verify there are no extra keys
			if len(data) != len(tt.expectKeys) {
				t.Errorf("data length = %d, want %d, data=%v", len(data), len(tt.expectKeys), data)
			}
		})
	}
}

// TestFlagSource_DefaultMapping test default mapping rules
func TestFlagSource_DefaultMapping(t *testing.T) {
	// Use a structure similar to application.AppFlags
	type AppFlags struct {
		Port    int
		Address string
	}

	tests := []struct {
		name       string
		flags      *AppFlags
		appType    string
		expectKeys map[string]interface{}
	}{
		{
			name: "gRPC 应用默认映射",
			flags: &AppFlags{
				Port:    9003,
				Address: "192.168.1.100",
			},
			appType: "grpc",
			expectKeys: map[string]interface{}{
				"grpc.server.port":    9003,
				"grpc.server.address": "192.168.1.100",
			},
		},
		{
			name: "HTTP 应用默认映射",
			flags: &AppFlags{
				Port:    8081,
				Address: "0.0.0.0",
			},
			appType: "http",
			expectKeys: map[string]interface{}{
				"api_server.port": 8081,
				"api_server.host": "0.0.0.0",
			},
		},
		{
			name: "混合应用默认映射",
			flags: &AppFlags{
				Port:    9000,
				Address: "127.0.0.1",
			},
			appType: "mixed",
			expectKeys: map[string]interface{}{
				"grpc.server.port":    9000,
				"grpc.server.address": "127.0.0.1",
				"api_server.port":     9000,
				"api_server.host":     "127.0.0.1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := NewFlagSource(tt.flags, tt.appType, 100)
			data, err := source.Load()
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}

			for key, expected := range tt.expectKeys {
				value, ok := data[key]
				if !ok {
					t.Errorf("key %s not found", key)
					continue
				}

				if value != expected {
					t.Errorf("key %s: value = %v, want %v", key, value, expected)
				}
			}
		})
	}
}

