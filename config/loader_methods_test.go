package config

import (
	"testing"
)

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

// TestLoader_GetBool test for retrieving boolean configuration
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

// TestLoader_IsSet test checks if configuration item exists
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

// TestLoader_AllSettings test to get all settings
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

	// Validate configuration exists
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

	// Access configuration through Viper
	if v.GetString("app.name") != "test-app" {
		t.Errorf("GetViper().GetString(app.name) = %s, want test-app", v.GetString("app.name"))
	}
}

// TestLoader_SetNestedValue_OverrideNonMap tests overriding non-map values
func TestLoader_SetNestedValue_OverwriteNonMap(t *testing.T) {
	loader := NewLoader()

	// Create an initial state where it is not a map
	m := map[string]interface{}{
		"app": "not-a-map", // This is a string, not a map
	}

	// Try to set nested value, this should override string as map
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

// TestLoader_SetNestedValue_EmptyKey test empty key
func TestLoader_SetNestedValue_EmptyKey(t *testing.T) {
	loader := NewLoader()
	m := make(map[string]interface{})

	// empty key should be returned directly
	loader.setNestedValue(m, "", "test")

	if len(m) != 0 {
		t.Errorf("map should be empty for empty key, got %v", m)
	}
}
