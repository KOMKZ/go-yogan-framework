package event

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockConfigLoader 模拟配置加载器
type mockConfigLoader struct {
	data      map[string]interface{}
	shouldErr bool
}

func (m *mockConfigLoader) Unmarshal(key string, v interface{}) error {
	if m.shouldErr {
		return assert.AnError
	}
	if cfg, ok := v.(*Config); ok {
		if eventCfg, exists := m.data[key]; exists {
			if ec, ok := eventCfg.(Config); ok {
				*cfg = ec
			}
		}
	}
	return nil
}

func (m *mockConfigLoader) Get(key string) interface{} {
	return m.data[key]
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
	_, exists := m.data[key]
	return exists
}

// ===== Component 测试 =====

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 100, cfg.PoolSize)
}

func TestNewComponent(t *testing.T) {
	c := NewComponent()
	assert.NotNil(t, c)
}

func TestComponent_Name(t *testing.T) {
	c := NewComponent()
	assert.Equal(t, "event", c.Name())
}

func TestComponent_DependsOn(t *testing.T) {
	c := NewComponent()
	deps := c.DependsOn()
	assert.Contains(t, deps, "config")
	assert.Contains(t, deps, "logger")
}

func TestComponent_Init(t *testing.T) {
	c := NewComponent()
	loader := &mockConfigLoader{
		data: map[string]interface{}{
			"event": Config{Enabled: true, PoolSize: 50},
		},
	}

	err := c.Init(context.Background(), loader)
	require.NoError(t, err)
	assert.NotNil(t, c.dispatcher)
	assert.Equal(t, 50, c.config.PoolSize)
}

func TestComponent_Init_DefaultConfig(t *testing.T) {
	c := NewComponent()
	loader := &mockConfigLoader{
		shouldErr: true,
	}

	err := c.Init(context.Background(), loader)
	require.NoError(t, err)
	assert.NotNil(t, c.dispatcher)
	assert.Equal(t, 100, c.config.PoolSize) // 默认值
}

func TestComponent_Init_Disabled(t *testing.T) {
	c := NewComponent()
	loader := &mockConfigLoader{
		data: map[string]interface{}{
			"event": Config{Enabled: false, PoolSize: 50},
		},
	}

	err := c.Init(context.Background(), loader)
	require.NoError(t, err)
	assert.Nil(t, c.dispatcher)
}

func TestComponent_Start(t *testing.T) {
	c := NewComponent()
	err := c.Start(context.Background())
	assert.NoError(t, err)
}

func TestComponent_Stop(t *testing.T) {
	c := NewComponent()
	loader := &mockConfigLoader{
		data: map[string]interface{}{
			"event": Config{Enabled: true, PoolSize: 50},
		},
	}
	_ = c.Init(context.Background(), loader)

	err := c.Stop(context.Background())
	assert.NoError(t, err)
}

func TestComponent_Stop_NilDispatcher(t *testing.T) {
	c := NewComponent()
	err := c.Stop(context.Background())
	assert.NoError(t, err)
}

func TestComponent_GetDispatcher(t *testing.T) {
	c := NewComponent()
	loader := &mockConfigLoader{
		data: map[string]interface{}{
			"event": Config{Enabled: true, PoolSize: 50},
		},
	}
	_ = c.Init(context.Background(), loader)

	d := c.GetDispatcher()
	assert.NotNil(t, d)
}

func TestComponent_IsEnabled(t *testing.T) {
	c := NewComponent()
	loader := &mockConfigLoader{
		data: map[string]interface{}{
			"event": Config{Enabled: true, PoolSize: 50},
		},
	}
	_ = c.Init(context.Background(), loader)

	assert.True(t, c.IsEnabled())
}

func TestComponent_IsEnabled_Disabled(t *testing.T) {
	c := NewComponent()
	c.config.Enabled = false
	assert.False(t, c.IsEnabled())
}

func TestComponent_IsEnabled_NilDispatcher(t *testing.T) {
	c := NewComponent()
	c.config.Enabled = true
	c.dispatcher = nil
	assert.False(t, c.IsEnabled())
}

