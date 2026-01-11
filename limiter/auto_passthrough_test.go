package limiter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAutoPassthrough 测试未配置资源的自动放行功能
func TestAutoPassthrough(t *testing.T) {
	// 创建配置：只配置一个资源
	cfg := Config{
		Enabled:   true,
		StoreType: "memory",
		Default: ResourceConfig{
			Algorithm:  "token_bucket",
			Rate:       100,
			Capacity:   100,
			InitTokens: 100,
		},
		Resources: map[string]ResourceConfig{
			"configured-resource": {
				Algorithm:  "token_bucket",
				Rate:       1, // 每秒1个请求（很严格）
				Capacity:   1,
				InitTokens: 1,
			},
		},
	}

	// 创建 Manager
	manager, err := NewManager(cfg)
	assert.NoError(t, err)
	defer manager.Close()

	ctx := context.Background()

	// ===========================
	// 测试1：配置的资源应该受限流控制
	// ===========================
	allowed1, err := manager.Allow(ctx, "configured-resource")
	assert.NoError(t, err)
	assert.True(t, allowed1, "第一个请求应该被允许")

	// 第二个请求应该被拒绝（因为 rate=1, capacity=1, 没有足够的令牌）
	allowed2, err := manager.Allow(ctx, "configured-resource")
	assert.NoError(t, err)
	assert.False(t, allowed2, "第二个请求应该被拒绝")

	// ===========================
	// 测试2：未配置的资源应该自动放行
	// ===========================
	// 发送多个请求，都应该被允许
	for i := 0; i < 100; i++ {
		allowed, err := manager.Allow(ctx, "unconfigured-resource")
		assert.NoError(t, err)
		assert.True(t, allowed, "未配置的资源应该自动放行，第 %d 个请求", i+1)
	}

	// ===========================
	// 测试3：另一个未配置的资源也应该自动放行
	// ===========================
	for i := 0; i < 50; i++ {
		allowed, err := manager.Allow(ctx, "another-unconfigured")
		assert.NoError(t, err)
		assert.True(t, allowed, "另一个未配置的资源应该自动放行，第 %d 个请求", i+1)
	}

	t.Log("✅ 未配置资源自动放行测试通过")
}

// TestAutoPassthrough_Disabled 测试禁用状态下的自动放行
func TestAutoPassthrough_Disabled(t *testing.T) {
	// 创建配置：禁用限流器
	cfg := Config{
		Enabled:   false,
		StoreType: "memory",
		Default: ResourceConfig{
			Algorithm:  "token_bucket",
			Rate:       100,
			Capacity:   100,
			InitTokens: 100,
		},
		Resources: map[string]ResourceConfig{
			"configured-resource": {
				Algorithm:  "token_bucket",
				Rate:       1,
				Capacity:   1,
				InitTokens: 0, // 0个初始令牌
			},
		},
	}

	// 创建 Manager
	manager, err := NewManager(cfg)
	assert.NoError(t, err)
	defer manager.Close()

	ctx := context.Background()

	// 即使是配置的资源，也应该被放行（因为禁用了）
	for i := 0; i < 100; i++ {
		allowed, err := manager.Allow(ctx, "configured-resource")
		assert.NoError(t, err)
		assert.True(t, allowed, "禁用状态下所有请求都应该被允许，第 %d 个请求", i+1)
	}

	t.Log("✅ 禁用状态自动放行测试通过")
}

// TestAutoPassthrough_MixedResources 测试混合资源场景
func TestAutoPassthrough_MixedResources(t *testing.T) {
	// 创建配置：配置多个资源
	cfg := Config{
		Enabled:   true,
		StoreType: "memory",
		Default: ResourceConfig{
			Algorithm:  "token_bucket",
			Rate:       100,
			Capacity:   100,
			InitTokens: 100,
		},
		Resources: map[string]ResourceConfig{
			"api:users": {
				Algorithm:  "token_bucket",
				Rate:       10,
				Capacity:   10,
				InitTokens: 10,
			},
			"api:orders": {
				Algorithm:  "token_bucket",
				Rate:       5,
				Capacity:   5,
				InitTokens: 5,
			},
		},
	}

	// 创建 Manager
	manager, err := NewManager(cfg)
	assert.NoError(t, err)
	defer manager.Close()

	ctx := context.Background()

	// 测试配置的资源
	for i := 0; i < 10; i++ {
		allowed, err := manager.Allow(ctx, "api:users")
		assert.NoError(t, err)
		assert.True(t, allowed, "api:users 前10个请求应该被允许")
	}

	// 第11个请求应该被拒绝
	allowed, err := manager.Allow(ctx, "api:users")
	assert.NoError(t, err)
	assert.False(t, allowed, "api:users 第11个请求应该被拒绝")

	// 测试未配置的资源（应该无限制）
	for i := 0; i < 1000; i++ {
		allowed, err := manager.Allow(ctx, "api:products")
		assert.NoError(t, err)
		assert.True(t, allowed, "api:products 未配置，应该无限制")
	}

	t.Log("✅ 混合资源测试通过")
}

