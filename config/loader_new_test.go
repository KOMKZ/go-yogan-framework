package config

import (
	"testing"
)

// TestLoader_Basic test basic loading
func TestLoader_Basic(t *testing.T) {
	loader := NewLoader()

	// Add file data source
	loader.AddSource(NewFileSource("testdata/config.yaml", 10))

	// Load
	if err := loader.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Validate configuration values
	if loader.GetString("app.name") != "test-app" {
		t.Errorf("app.name = %s, want test-app", loader.GetString("app.name"))
	}

	if loader.GetInt("grpc.server.port") != 9002 {
		t.Errorf("grpc.server.port = %d, want 9002", loader.GetInt("grpc.server.port"))
	}
}

// TestLoader_MultipleFiles test multiple file loading (priority)
func TestLoader_MultipleFiles(t *testing.T) {
	loader := NewLoader()

	// Add basic configuration (low priority)
	loader.AddSource(NewFileSource("testdata/config.yaml", 10))

	// Add environment configuration (high priority)
	loader.AddSource(NewFileSource("testdata/dev.yaml", 20))

	// load
	if err := loader.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// dev.yaml should override config.yaml
	if loader.GetInt("grpc.server.port") != 9999 {
		t.Errorf("grpc.server.port = %d, want 9999 (from dev.yaml)", loader.GetInt("grpc.server.port"))
	}

	if loader.GetInt("api_server.port") != 8888 {
		t.Errorf("api_server.port = %d, want 8888 (from dev.yaml)", loader.GetInt("api_server.port"))
	}

	// Other configurations in config.yaml should be retained
	if loader.GetString("app.name") != "test-app" {
		t.Errorf("app.name = %s, want test-app", loader.GetString("app.name"))
	}
}

// TestLoader_WithFlags test command line arguments override
func TestLoader_WithFlags(t *testing.T) {
	type TestFlags struct {
		Port    int
		Address string
	}

	loader := NewLoader()

	// Add file configuration (low priority)
	loader.AddSource(NewFileSource("testdata/config.yaml", 10))

	// Add command line arguments (highest priority)
	loader.AddSource(NewFlagSource(&TestFlags{
		Port:    7777,
		Address: "10.0.0.1",
	}, "grpc", 100))

	// load
	if err := loader.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Command line arguments should override file configuration
	if loader.GetInt("grpc.server.port") != 7777 {
		t.Errorf("grpc.server.port = %d, want 7777 (from flags)", loader.GetInt("grpc.server.port"))
	}

	if loader.GetString("grpc.server.address") != "10.0.0.1" {
		t.Errorf("grpc.server.address = %s, want 10.0.0.1 (from flags)", loader.GetString("grpc.server.address"))
	}

	// Other configurations in the file should be retained
	if loader.GetString("app.name") != "test-app" {
		t.Errorf("app.name = %s, want test-app", loader.GetString("app.name"))
	}
}

// TestLoader_AllSources test all data sources (full priority)
func TestLoader_AllSources(t *testing.T) {
	type TestFlags struct {
		Port int
	}

	loader := NewLoader()

	// 1. Basic configuration file (priority 10)
	loader.AddSource(NewFileSource("testdata/config.yaml", 10))

	// Environment configuration file (priority 20)
	loader.AddSource(NewFileSource("testdata/dev.yaml", 20))

	// 3. Environment variables (priority 50) - temporarily skipped to avoid polluting the test environment

	// 4. Command-line arguments (priority 100)
	loader.AddSource(NewFlagSource(&TestFlags{
		Port: 6666,
	}, "grpc", 100))

	// load
	if err := loader.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Finally, command-line arguments should be used.
	if loader.GetInt("grpc.server.port") != 6666 {
		t.Errorf("grpc.server.port = %d, want 6666 (from flags, highest priority)", loader.GetInt("grpc.server.port"))
	}
}

// TestLoader_Unmarshal deserialize test
func TestLoader_Unmarshal(t *testing.T) {
	type AppConfig struct {
		App struct {
			Name    string `mapstructure:"name"`
			Version string `mapstructure:"version"`
		} `mapstructure:"app"`
		Grpc struct {
			Server struct {
				Port    int    `mapstructure:"port"`
				Address string `mapstructure:"address"`
			} `mapstructure:"server"`
		} `mapstructure:"grpc"`
	}

	loader := NewLoader()
	loader.AddSource(NewFileSource("testdata/config.yaml", 10))

	if err := loader.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	var cfg AppConfig
	if err := loader.Unmarshal(&cfg); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	// Validate deserialization results
	if cfg.App.Name != "test-app" {
		t.Errorf("cfg.App.Name = %s, want test-app", cfg.App.Name)
	}

	if cfg.Grpc.Server.Port != 9002 {
		t.Errorf("cfg.Grpc.Server.Port = %d, want 9002", cfg.Grpc.Server.Port)
	}
}

// TestLoader_GetLoadedFiles test get loaded files list
func TestLoader_GetLoadedFiles(t *testing.T) {
	loader := NewLoader()

	loader.AddSource(NewFileSource("testdata/config.yaml", 10))
	loader.AddSource(NewFileSource("testdata/dev.yaml", 20))

	if err := loader.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	files := loader.GetLoadedFiles()
	if len(files) != 2 {
		t.Errorf("GetLoadedFiles() = %d files, want 2", len(files))
	}

	// Verify file path
	expectedFiles := []string{
		"testdata/config.yaml",
		"testdata/dev.yaml",
	}

	for i, expected := range expectedFiles {
		if files[i] != expected {
			t.Errorf("files[%d] = %s, want %s", i, files[i], expected)
		}
	}
}

// TestLoader_Reload test reload
func TestLoader_Reload(t *testing.T) {
	loader := NewLoader()
	loader.AddSource(NewFileSource("testdata/config.yaml", 10))

	// First load
	if err := loader.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	port1 := loader.GetInt("grpc.server.port")

	// reload
	if err := loader.Reload(); err != nil {
		t.Fatalf("Reload() error: %v", err)
	}

	port2 := loader.GetInt("grpc.server.port")

	// Should remain consistent
	if port1 != port2 {
		t.Errorf("port changed after reload: %d -> %d", port1, port2)
	}
}

// TestSplitKey test key splitting
func TestSplitKey(t *testing.T) {
	tests := []struct {
		key      string
		expected []string
	}{
		{"grpc.server.port", []string{"grpc", "server", "port"}},
		{"app.name", []string{"app", "name"}},
		{"simple", []string{"simple"}},
		{"", []string{}},
		{"a.b.c.d.e", []string{"a", "b", "c", "d", "e"}},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := splitKey(tt.key)

			if len(result) != len(tt.expected) {
				t.Errorf("splitKey(%s) = %v, want %v", tt.key, result, tt.expected)
				return
			}

			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("splitKey(%s)[%d] = %s, want %s", tt.key, i, v, tt.expected[i])
				}
			}
		})
	}
}

// TestLoader_Get test to retrieve configuration values
func TestLoader_Get(t *testing.T) {
	loader := NewLoader()
	loader.AddSource(NewFileSource("testdata/config.yaml", 10))

	if err := loader.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Test Get method
	value := loader.Get("app.name")
	if value != "test-app" {
		t.Errorf("Get(app.name) = %v, want test-app", value)
	}

	// Test getting a non-existent key
	nilValue := loader.Get("not.exist.key")
	if nilValue != nil {
		t.Errorf("Get(not.exist.key) = %v, want nil", nilValue)
	}
}

// TestLoader_GetBool test to retrieve boolean configuration
func TestLoader_GetBool(t *testing.T) {
	loader := NewLoader()
	loader.AddSource(NewFileSource("testdata/config.yaml", 10))

	if err := loader.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Set a boolean value for testing
	loader.v.Set("app.debug", true)

	value := loader.GetBool("app.debug")
	if !value {
		t.Errorf("GetBool(app.debug) = %v, want true", value)
	}

	// Test default values (return false when not existing)
	defaultValue := loader.GetBool("not.exist.key")
	if defaultValue {
		t.Errorf("GetBool(not.exist.key) = %v, want false", defaultValue)
	}
}

// TestLoader_IsSet test to check if configuration item exists
func TestLoader_IsSet(t *testing.T) {
	loader := NewLoader()
	loader.AddSource(NewFileSource("testdata/config.yaml", 10))

	if err := loader.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Test existing keys
	if !loader.IsSet("app.name") {
		t.Error("IsSet(app.name) = false, want true")
	}

	// Test non-existent key
	if loader.IsSet("not.exist.key") {
		t.Error("IsSet(not.exist.key) = true, want false")
	}
}

// TestLoader_AllSettings test to retrieve all settings
func TestLoader_AllSettings(t *testing.T) {
	loader := NewLoader()
	loader.AddSource(NewFileSource("testdata/config.yaml", 10))

	if err := loader.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	settings := loader.AllSettings()
	if settings == nil {
		t.Error("AllSettings() = nil, want map")
	}

	// Verify configuration exists
	if _, ok := settings["app"]; !ok {
		t.Error("AllSettings() missing 'app' key")
	}
}

// TestLoader_GetViper test to obtain Viper instance
func TestLoader_GetViper(t *testing.T) {
	loader := NewLoader()
	loader.AddSource(NewFileSource("testdata/config.yaml", 10))

	if err := loader.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	v := loader.GetViper()
	if v == nil {
		t.Error("GetViper() = nil, want *viper.Viper")
	}

	// Access configuration via Viper
	if v.GetString("app.name") != "test-app" {
		t.Errorf("GetViper().GetString(app.name) = %s, want test-app", v.GetString("app.name"))
	}
}

// TestLoader_SetNestedValue_OverwriteNonMap test overwriting non-map values
func TestLoader_SetNestedValue_OverwriteNonMap(t *testing.T) {
	loader := NewLoader()

	// Create a case where the initial value is not a map
	m := map[string]interface{}{
		"app": "not-a-map", // This is a string, not a map
	}

	// Try to set nested values, this should override the string as a map
	loader.setNestedValue(m, "app.name", "test")

	// Verify that the app has changed to map mode
	if app, ok := m["app"].(map[string]interface{}); ok {
		if app["name"] != "test" {
			t.Errorf("app.name = %v, want test", app["name"])
		}
	} else {
		t.Errorf("app should be a map, got %T", m["app"])
	}
}
