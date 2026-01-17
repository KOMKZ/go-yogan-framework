package cache

import (
	"context"
	"testing"
)

// mockConfigLoader 模拟配置加载器
type mockConfigLoader struct {
	data map[string]any
}

func (m *mockConfigLoader) Get(key string) interface{} {
	return m.data[key]
}

func (m *mockConfigLoader) Unmarshal(key string, v interface{}) error {
	// 简单实现：返回空配置让组件使用默认值
	return nil
}

func (m *mockConfigLoader) GetString(key string) string {
	if v, ok := m.data[key].(string); ok {
		return v
	}
	return ""
}

func (m *mockConfigLoader) GetInt(key string) int {
	if v, ok := m.data[key].(int); ok {
		return v
	}
	return 0
}

func (m *mockConfigLoader) GetBool(key string) bool {
	if v, ok := m.data[key].(bool); ok {
		return v
	}
	return false
}

func (m *mockConfigLoader) IsSet(key string) bool {
	_, ok := m.data[key]
	return ok
}

func TestComponent_Name(t *testing.T) {
	c := NewComponent()
	if c.Name() != "cache" {
		t.Errorf("Name() = %v, want cache", c.Name())
	}
}

func TestComponent_DependsOn(t *testing.T) {
	c := NewComponent()
	deps := c.DependsOn()

	expected := []string{"config", "logger", "optional:redis", "optional:event"}
	if len(deps) != len(expected) {
		t.Errorf("DependsOn() len = %d, want %d", len(deps), len(expected))
	}

	for i, dep := range expected {
		if deps[i] != dep {
			t.Errorf("DependsOn()[%d] = %v, want %v", i, deps[i], dep)
		}
	}
}

func TestComponent_Init(t *testing.T) {
	c := NewComponent()
	ctx := context.Background()
	loader := &mockConfigLoader{data: make(map[string]any)}

	err := c.Init(ctx, loader)
	if err != nil {
		t.Errorf("Init() error = %v", err)
	}

	// 验证默认配置
	if c.config == nil {
		t.Error("config should not be nil after Init")
	}
	if c.config.Enabled {
		t.Error("cache should be disabled by default")
	}
}

func TestComponent_StartStop(t *testing.T) {
	c := NewComponent()
	ctx := context.Background()
	loader := &mockConfigLoader{data: make(map[string]any)}

	c.Init(ctx, loader)

	// Start (disabled)
	err := c.Start(ctx)
	if err != nil {
		t.Errorf("Start() error = %v", err)
	}

	// Stop
	err = c.Stop(ctx)
	if err != nil {
		t.Errorf("Stop() error = %v", err)
	}
}

func TestComponent_HealthCheck(t *testing.T) {
	c := NewComponent()
	ctx := context.Background()
	loader := &mockConfigLoader{data: make(map[string]any)}

	c.Init(ctx, loader)

	// Check health (disabled)
	err := c.Check(ctx)
	if err != nil {
		t.Errorf("Check() error = %v", err)
	}
}

func TestComponent_GetHealthChecker(t *testing.T) {
	c := NewComponent()
	checker := c.GetHealthChecker()
	if checker == nil {
		t.Error("GetHealthChecker() should not return nil")
	}
	if checker.Name() != "cache" {
		t.Errorf("GetHealthChecker().Name() = %v, want cache", checker.Name())
	}
}

func TestComponent_RegisterLoader(t *testing.T) {
	c := NewComponent()

	// 未初始化时调用不应 panic
	c.RegisterLoader("test", func(ctx context.Context, args ...any) (any, error) {
		return nil, nil
	})
}

func TestComponent_Call(t *testing.T) {
	c := NewComponent()
	ctx := context.Background()

	// 未初始化时调用应返回错误
	_, err := c.Call(ctx, "test")
	if err == nil {
		t.Error("Call() expected error when not initialized")
	}
}

func TestComponent_Invalidate(t *testing.T) {
	c := NewComponent()
	ctx := context.Background()

	// 未初始化时调用应返回错误
	err := c.Invalidate(ctx, "test")
	if err == nil {
		t.Error("Invalidate() expected error when not initialized")
	}
}
