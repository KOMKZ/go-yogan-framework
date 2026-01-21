package limiter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAutoPassthrough test automatic passthrough for unconfigured resources
func TestAutoPassthrough(t *testing.T) {
	// Create configuration: configure only one resource
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
				Rate:       1, // One request per second (very strict)
				Capacity:   1,
				InitTokens: 1,
			},
		},
	}

	// Create Manager
	manager, err := NewManager(cfg)
	assert.NoError(t, err)
	defer manager.Close()

	ctx := context.Background()

	// ===========================
	// Test 1: Configured resources should be subject to rate limiting control
	// ===========================
	allowed1, err := manager.Allow(ctx, "configured-resource")
	assert.NoError(t, err)
	assert.True(t, allowed1, "第一个请求应该被允许")

	// The second request should be rejected (because rate=1, capacity=1, not enough tokens)
	allowed2, err := manager.Allow(ctx, "configured-resource")
	assert.NoError(t, err)
	assert.False(t, allowed2, "第二个请求应该被拒绝")

	// ===========================
	// Test 2: Unconfigured resources should be automatically allowed through
	// ===========================
	// Multiple requests should all be allowed
	for i := 0; i < 100; i++ {
		allowed, err := manager.Allow(ctx, "unconfigured-resource")
		assert.NoError(t, err)
		assert.True(t, allowed, "未配置的资源应该自动放行，第 %d 个请求", i+1)
	}

	// ===========================
	// Test 3: Another unconfigured resource should also be automatically allowed
	// ===========================
	for i := 0; i < 50; i++ {
		allowed, err := manager.Allow(ctx, "another-unconfigured")
		assert.NoError(t, err)
		assert.True(t, allowed, "另一个未配置的资源应该自动放行，第 %d 个请求", i+1)
	}

	t.Log("✅ 未配置资源自动放行测试通过")
}

// TestAutoPassthrough_Disabled Test automatic passthrough in disabled state
func TestAutoPassthrough_Disabled(t *testing.T) {
	// Create configuration: disable rate limiter
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
				InitTokens: 0, // initial zero tokens
			},
		},
	}

	// Create Manager
	manager, err := NewManager(cfg)
	assert.NoError(t, err)
	defer manager.Close()

	ctx := context.Background()

	// Even configured resources should be allowed (because they are disabled)
	for i := 0; i < 100; i++ {
		allowed, err := manager.Allow(ctx, "configured-resource")
		assert.NoError(t, err)
		assert.True(t, allowed, "禁用状态下所有请求都应该被允许，第 %d 个请求", i+1)
	}

	t.Log("✅ 禁用状态自动放行测试通过")
}

// TestAutoPassthrough_MixedResources test mixed resource scenario
func TestAutoPassthrough_MixedResources(t *testing.T) {
	// Create configuration: Configure multiple resources
	cfg := Config{
		Enabled:   true,
		StoreType: "memory",
		// 不配置 Default，这样未配置的资源才会真正无限制
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

	// Create Manager
	manager, err := NewManager(cfg)
	assert.NoError(t, err)
	defer manager.Close()

	ctx := context.Background()

	// Test configured resources
	for i := 0; i < 10; i++ {
		allowed, err := manager.Allow(ctx, "api:users")
		assert.NoError(t, err)
		assert.True(t, allowed, "api:users 前10个请求应该被允许")
	}

	// The 11th request should be rejected
	allowed, err := manager.Allow(ctx, "api:users")
	assert.NoError(t, err)
	assert.False(t, allowed, "api:users 第11个请求应该被拒绝")

	// Test unconfigured resources (should be unrestricted)
	for i := 0; i < 1000; i++ {
		allowed, err := manager.Allow(ctx, "api:products")
		assert.NoError(t, err)
		assert.True(t, allowed, "api:products 未配置，应该无限制")
	}

	t.Log("✅ 混合资源测试通过")
}

