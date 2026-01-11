package jwt

import (
	"context"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/application"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestComponent_FullLifecycle 测试完整的组件生命周期
func TestComponent_FullLifecycle(t *testing.T) {
	comp := NewComponent()

	// 配置
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
				Blacklist: BlacklistConfig{
					Enabled: true,
					Storage: "memory",
				},
			},
		},
	}

	// Init
	err := comp.Init(loader)
	require.NoError(t, err)

	// Start
	registry := application.NewRegistry()
	comp.SetRegistry(registry)
	err = comp.Start(context.Background())
	require.NoError(t, err)

	// 验证 TokenManager 已创建
	manager := comp.GetTokenManager()
	assert.NotNil(t, manager)

	// 测试生成 Token
	ctx := context.Background()
	token, err := manager.GenerateAccessToken(ctx, "user123", nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// Stop
	err = comp.Stop(context.Background())
	assert.NoError(t, err)
}

// TestComponent_Init_UnsupportedStorage 测试不支持的存储类型
func TestComponent_Init_UnsupportedStorage(t *testing.T) {
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
					Storage: "mysql", // 不支持的存储类型
				},
			},
		},
	}

	err := comp.Init(loader)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "blacklist storage must be redis or memory")
}

