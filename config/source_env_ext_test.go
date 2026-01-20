package config

import (
	"os"
	"testing"
)

// TestEnvSource_AddBinding 测试添加 key 映射
func TestEnvSource_AddBinding(t *testing.T) {
	// 设置环境变量
	os.Setenv("MY_PREFIX_CUSTOM_KEY", "custom_value")
	defer os.Unsetenv("MY_PREFIX_CUSTOM_KEY")

	source := NewEnvSource("MY_PREFIX", 50)
	source.AddBinding("app.custom", "CUSTOM_KEY")

	data, err := source.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if data["app.custom"] != "custom_value" {
		t.Errorf("app.custom = %v, want custom_value", data["app.custom"])
	}
}

// TestEnvSource_AddBinding_WithPrefix 测试带前缀的 binding
func TestEnvSource_AddBinding_WithPrefix(t *testing.T) {
	// 设置环境变量
	os.Setenv("TEST_DB_HOST", "localhost")
	defer os.Unsetenv("TEST_DB_HOST")

	source := NewEnvSource("TEST", 50)
	source.AddBinding("database.host", "TEST_DB_HOST") // 已经有前缀

	data, err := source.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if data["database.host"] != "localhost" {
		t.Errorf("database.host = %v, want localhost", data["database.host"])
	}
}

// TestEnvSource_NoPrefix 测试无前缀
func TestEnvSource_NoPrefix(t *testing.T) {
	source := NewEnvSource("", 50)

	data, err := source.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// 无前缀时应该返回空 map
	if len(data) != 0 {
		t.Errorf("expected empty map for no prefix, got %d items", len(data))
	}
}

// TestEnvSource_EmptyBinding 测试空的 binding 值
func TestEnvSource_EmptyBinding(t *testing.T) {
	// 不设置环境变量
	source := NewEnvSource("EMPTY", 50)
	source.AddBinding("app.missing", "MISSING_KEY")

	data, err := source.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// 空值不应该被添加
	if _, ok := data["app.missing"]; ok {
		t.Error("empty env value should not be added")
	}
}

// TestEnvSource_MultipleBindings 测试多个 bindings
func TestEnvSource_MultipleBindings(t *testing.T) {
	os.Setenv("APP_DB_HOST", "db.example.com")
	os.Setenv("APP_DB_PORT", "5432")
	os.Setenv("APP_REDIS_HOST", "redis.example.com")
	defer func() {
		os.Unsetenv("APP_DB_HOST")
		os.Unsetenv("APP_DB_PORT")
		os.Unsetenv("APP_REDIS_HOST")
	}()

	source := NewEnvSource("APP", 50)
	source.AddBinding("db.host", "DB_HOST")
	source.AddBinding("db.port", "DB_PORT")
	source.AddBinding("redis.host", "REDIS_HOST")

	data, err := source.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if data["db.host"] != "db.example.com" {
		t.Errorf("db.host = %v, want db.example.com", data["db.host"])
	}
	if data["db.port"] != "5432" {
		t.Errorf("db.port = %v, want 5432", data["db.port"])
	}
	if data["redis.host"] != "redis.example.com" {
		t.Errorf("redis.host = %v, want redis.example.com", data["redis.host"])
	}
}
