package jwt

import (
	"context"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockConfigLoader struct {
	config map[string]interface{}
}

func (m *mockConfigLoader) Get(key string) interface{} {
	return m.config[key]
}

func (m *mockConfigLoader) GetString(key string) string {
	if v, ok := m.config[key].(string); ok {
		return v
	}
	return ""
}

func (m *mockConfigLoader) GetInt(key string) int {
	if v, ok := m.config[key].(int); ok {
		return v
	}
	return 0
}

func (m *mockConfigLoader) GetBool(key string) bool {
	if v, ok := m.config[key].(bool); ok {
		return v
	}
	return false
}

func (m *mockConfigLoader) IsSet(key string) bool {
	_, ok := m.config[key]
	return ok
}

func (m *mockConfigLoader) Unmarshal(key string, target interface{}) error {
	if cfg, ok := m.config[key]; ok {
		// 使用 mapstructure 进行通用解析
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			Result:  target,
			TagName: "mapstructure",
		})
		if err != nil {
			return err
		}
		return decoder.Decode(cfg)
	}
	return nil
}

func TestComponent_Name(t *testing.T) {
	comp := NewComponent()
	assert.Equal(t, component.ComponentJWT, comp.Name())
}

func TestComponent_DependsOn(t *testing.T) {
	comp := NewComponent()
	deps := comp.DependsOn()

	assert.Contains(t, deps, component.ComponentConfig)
	assert.Contains(t, deps, component.ComponentLogger)
}

func TestComponent_IsRequired(t *testing.T) {
	comp := NewComponent()
	assert.False(t, comp.IsRequired())
}

func TestComponent_Init_Disabled(t *testing.T) {
	comp := NewComponent()
	loader := &mockConfigLoader{
		config: map[string]interface{}{
			"jwt": &Config{
				Enabled: false,
			},
		},
	}

	err := comp.Init(loader)
	assert.NoError(t, err)
	assert.False(t, comp.config.Enabled)
}

func TestComponent_Init_Success(t *testing.T) {
	comp := NewComponent()
	loader := &mockConfigLoader{
		config: map[string]interface{}{
			"jwt": &Config{
				Enabled:   true,
				Algorithm: "HS256",
				Secret:    "test-secret",
				AccessToken: AccessTokenConfig{
					TTL:    2 * time.Hour,
					Issuer: "test-issuer",
				},
				RefreshToken: RefreshTokenConfig{
					Enabled: true,
					TTL:     168 * time.Hour,
				},
				Blacklist: BlacklistConfig{
					Enabled: true,
					Storage: "memory",
				},
			},
		},
	}

	err := comp.Init(loader)
	assert.NoError(t, err)
	assert.True(t, comp.config.Enabled)
	assert.Equal(t, "HS256", comp.config.Algorithm)
	assert.NotNil(t, comp.logger)
}

func TestComponent_Init_InvalidConfig(t *testing.T) {
	comp := NewComponent()
	loader := &mockConfigLoader{
		config: map[string]interface{}{
			"jwt": &Config{
				Enabled:   true,
				Algorithm: "HS256",
				Secret:    "", // 空密钥
				AccessToken: AccessTokenConfig{
					TTL: 2 * time.Hour,
				},
			},
		},
	}

	err := comp.Init(loader)
	assert.Error(t, err)
}

func TestComponent_Start_Disabled(t *testing.T) {
	comp := NewComponent()
	comp.config = &Config{Enabled: false}

	err := comp.Start(context.Background())
	assert.NoError(t, err)
}

func TestComponent_Stop_Disabled(t *testing.T) {
	comp := NewComponent()
	comp.config = &Config{Enabled: false}

	err := comp.Stop(context.Background())
	assert.NoError(t, err)
}

func TestComponent_GetTokenManager(t *testing.T) {
	comp := NewComponent()
	loader := &mockConfigLoader{
		config: map[string]interface{}{
			"jwt": &Config{
				Enabled:   true,
				Algorithm: "HS256",
				Secret:    "test-secret",
				AccessToken: AccessTokenConfig{
					TTL: 2 * time.Hour,
				},
				Blacklist: BlacklistConfig{
					Enabled: true,
					Storage: "memory",
				},
			},
		},
	}

	err := comp.Init(loader)
	require.NoError(t, err)

	// 模拟 Start（创建 TokenManager）
	err = comp.createMemoryTokenStore()
	require.NoError(t, err)

	tokenManager, err := NewTokenManager(comp.config, comp.tokenStore, comp.logger)
	require.NoError(t, err)
	comp.tokenManager = tokenManager

	// 获取 TokenManager
	manager := comp.GetTokenManager()
	assert.NotNil(t, manager)
}

func TestComponent_GetConfig(t *testing.T) {
	comp := NewComponent()
	comp.config = &Config{
		Enabled:   true,
		Algorithm: "HS256",
	}

	config := comp.GetConfig()
	assert.NotNil(t, config)
	assert.True(t, config.Enabled)
	assert.Equal(t, "HS256", config.Algorithm)
}

func TestComponent_createMemoryTokenStore(t *testing.T) {
	comp := NewComponent()
	loader := &mockConfigLoader{
		config: map[string]interface{}{
			"jwt": &Config{
				Enabled:   true,
				Algorithm: "HS256",
				Secret:    "test-secret",
				AccessToken: AccessTokenConfig{
					TTL: 2 * time.Hour,
				},
				Blacklist: BlacklistConfig{
					Enabled:         true,
					Storage:         "memory",
					CleanupInterval: 1 * time.Hour,
				},
			},
		},
	}

	err := comp.Init(loader)
	require.NoError(t, err)

	err = comp.createMemoryTokenStore()
	assert.NoError(t, err)
	assert.NotNil(t, comp.tokenStore)

	// 清理
	comp.tokenStore.Close()
}

func TestComponent_createTokenStore_BlacklistDisabled(t *testing.T) {
	comp := NewComponent()
	loader := &mockConfigLoader{
		config: map[string]interface{}{
			"jwt": &Config{
				Enabled:   true,
				Algorithm: "HS256",
				Secret:    "test-secret",
				AccessToken: AccessTokenConfig{
					TTL: 2 * time.Hour,
				},
				Blacklist: BlacklistConfig{
					Enabled: false,
				},
			},
		},
	}

	err := comp.Init(loader)
	require.NoError(t, err)

	err = comp.createTokenStore()
	assert.NoError(t, err)
	assert.NotNil(t, comp.tokenStore)

	// 清理
	comp.tokenStore.Close()
}

func TestComponent_ApplyDefaults(t *testing.T) {
	comp := NewComponent()
	loader := &mockConfigLoader{
		config: map[string]interface{}{
			"jwt": &Config{
				Enabled:   true,
				Algorithm: "", // 空算法，应使用默认值
				Secret:    "test-secret",
			},
		},
	}

	err := comp.Init(loader)
	require.NoError(t, err)

	// 验证默认值
	assert.Equal(t, "HS256", comp.config.Algorithm)
	assert.Equal(t, 2*time.Hour, comp.config.AccessToken.TTL)
	assert.Equal(t, "yogan-api", comp.config.AccessToken.Issuer)
}

