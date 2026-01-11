package jwt

import (
	"context"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/application"
	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/KOMKZ/go-yogan-framework/redis"
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestComponent_createRedisTokenStore_WithInjection 测试通过注入的方式创建 Redis TokenStore
func TestComponent_createRedisTokenStore_WithInjection(t *testing.T) {
	// 创建 miniredis 实例
	mr := miniredis.RunT(t)
	defer mr.Close()

	// 创建并初始化 Redis Component
	redisComp := redis.NewComponent()

	// 创建 mock ConfigLoader 用于初始化 Redis Component
	redisConfigs := map[string]redis.Config{
		"main": {
			Mode:  "standalone",
			Addrs: []string{mr.Addr()},
		},
	}

	redisConfigLoader := &mockConfigLoader{
		config: map[string]interface{}{
			"redis.instances": redisConfigs,
		},
	}

	// 初始化 Redis Component
	err := redisComp.Init(context.Background(), redisConfigLoader)
	require.NoError(t, err)

	// 验证 Manager 是否创建成功
	manager := redisComp.GetManager()
	t.Logf("Redis Manager: %+v", manager)
	if manager == nil {
		t.Fatal("Redis Manager is nil after Init")
	}

	// 创建 JWT Component
	jwtComp := NewComponent()
	jwtComp.logger = logger.GetLogger("yogan")
	jwtComp.config = &Config{
		Enabled:   true,
		Algorithm: "HS256",
		Secret:    "test-secret",
		AccessToken: AccessTokenConfig{
			TTL: 2 * time.Hour,
		},
		Blacklist: BlacklistConfig{
			Enabled:        true,
			Storage:        "redis",
			RedisKeyPrefix: "jwt:test:",
		},
	}

	// 手动注入 Redis Component
	jwtComp.SetRedisComponent(redisComp)

	// 创建 Registry 并设置（模拟框架行为）
	registry := application.NewRegistry()
	jwtComp.SetRegistry(registry)

	// 测试 createRedisTokenStore
	err = jwtComp.createRedisTokenStore()
	assert.NoError(t, err)
	assert.NotNil(t, jwtComp.tokenStore)

	// 验证 TokenStore 可用
	ctx := context.Background()
	err = jwtComp.tokenStore.AddToBlacklist(ctx, "test-token", 1*time.Hour)
	assert.NoError(t, err)

	blacklisted, err := jwtComp.tokenStore.IsBlacklisted(ctx, "test-token")
	assert.NoError(t, err)
	assert.True(t, blacklisted)
}

// TestComponent_createRedisTokenStore_RedisNotFound 测试 Redis Component 不存在
func TestComponent_createRedisTokenStore_RedisNotFound(t *testing.T) {
	jwtComp := NewComponent()

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
					Storage: "redis",
				},
			},
		},
	}

	err := jwtComp.Init(loader)
	require.NoError(t, err)

	// 创建空的 Registry（没有 Redis Component）
	registry := application.NewRegistry()
	jwtComp.SetRegistry(registry)

	// 测试 createRedisTokenStore（应该失败）
	err = jwtComp.createRedisTokenStore()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis component not found")
}

// TestComponent_createRedisTokenStore_InvalidRedisComponent 测试无效的 Redis Component 类型
func TestComponent_createRedisTokenStore_InvalidRedisComponent(t *testing.T) {
	jwtComp := NewComponent()

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
					Storage: "redis",
				},
			},
		},
	}

	err := jwtComp.Init(loader)
	require.NoError(t, err)

	// 创建 Registry 并注册一个错误类型的组件
	registry := application.NewRegistry()
	// 注册一个不是 *redis.Component 的组件
	registry.Register(&mockInvalidComponent{})

	// 设置 Registry
	jwtComp.SetRegistry(registry)

	// 测试 createRedisTokenStore（应该失败）
	err = jwtComp.createRedisTokenStore()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid redis component type")
}

// mockInvalidComponent 模拟一个无效的组件
type mockInvalidComponent struct{}

func (m *mockInvalidComponent) Name() string {
	return component.ComponentRedis
}

func (m *mockInvalidComponent) DependsOn() []string {
	return nil
}

func (m *mockInvalidComponent) Init(ctx context.Context, loader component.ConfigLoader) error {
	return nil
}

func (m *mockInvalidComponent) Start(ctx context.Context) error {
	return nil
}

func (m *mockInvalidComponent) Stop(ctx context.Context) error {
	return nil
}

func (m *mockInvalidComponent) IsRequired() bool {
	return false
}
