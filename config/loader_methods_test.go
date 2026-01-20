package config

import (
	"testing"
)

// TestLoader_Get 测试获取配置值
func TestLoader_Get(t *testing.T) {
	loader := NewLoader()
	loader.AddSource(NewFileSource("testdata/config.yaml", 10))

	if err := loader.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// 测试 Get 方法
	value := loader.Get("app.name")
	if value != "test-app" {
		t.Errorf("Get(app.name) = %v, want test-app", value)
	}

	// 测试 Get 不存在的 key
	nilValue := loader.Get("not.exist.key")
	if nilValue != nil {
		t.Errorf("Get(not.exist.key) = %v, want nil", nilValue)
	}
}

// TestLoader_GetBool 测试获取布尔配置
func TestLoader_GetBool(t *testing.T) {
	loader := NewLoader()
	loader.AddSource(NewFileSource("testdata/config.yaml", 10))

	if err := loader.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// 设置一个布尔值用于测试
	loader.v.Set("app.debug", true)

	value := loader.GetBool("app.debug")
	if !value {
		t.Errorf("GetBool(app.debug) = %v, want true", value)
	}

	// 测试默认值（不存在时返回 false）
	defaultValue := loader.GetBool("not.exist.key")
	if defaultValue {
		t.Errorf("GetBool(not.exist.key) = %v, want false", defaultValue)
	}
}

// TestLoader_IsSet 测试检查配置项是否存在
func TestLoader_IsSet(t *testing.T) {
	loader := NewLoader()
	loader.AddSource(NewFileSource("testdata/config.yaml", 10))

	if err := loader.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// 测试存在的 key
	if !loader.IsSet("app.name") {
		t.Error("IsSet(app.name) = false, want true")
	}

	// 测试不存在的 key
	if loader.IsSet("not.exist.key") {
		t.Error("IsSet(not.exist.key) = true, want false")
	}
}

// TestLoader_AllSettings 测试获取所有配置
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

	// 验证配置存在
	if _, ok := settings["app"]; !ok {
		t.Error("AllSettings() missing 'app' key")
	}
}

// TestLoader_GetViper 测试获取 Viper 实例
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

	// 通过 Viper 访问配置
	if v.GetString("app.name") != "test-app" {
		t.Errorf("GetViper().GetString(app.name) = %s, want test-app", v.GetString("app.name"))
	}
}

// TestLoader_SetNestedValue_OverwriteNonMap 测试覆盖非 map 值
func TestLoader_SetNestedValue_OverwriteNonMap(t *testing.T) {
	loader := NewLoader()

	// 创建一个初始值是非 map 的情况
	m := map[string]interface{}{
		"app": "not-a-map", // 这是一个字符串，不是 map
	}

	// 尝试设置嵌套值，这应该覆盖字符串为 map
	loader.setNestedValue(m, "app.name", "test")

	// 验证 app 变成了 map
	if app, ok := m["app"].(map[string]interface{}); ok {
		if app["name"] != "test" {
			t.Errorf("app.name = %v, want test", app["name"])
		}
	} else {
		t.Errorf("app should be a map, got %T", m["app"])
	}
}

// TestLoader_SetNestedValue_EmptyKey 测试空 key
func TestLoader_SetNestedValue_EmptyKey(t *testing.T) {
	loader := NewLoader()
	m := make(map[string]interface{})

	// 空 key 应该直接返回
	loader.setNestedValue(m, "", "test")

	if len(m) != 0 {
		t.Errorf("map should be empty for empty key, got %v", m)
	}
}
