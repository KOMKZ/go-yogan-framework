package limiter

import (
	"context"
	"testing"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/stretchr/testify/assert"
)

// TestDefaultConfig_NotConfigured tests the behavior when default is not configured
func TestDefaultConfig_NotConfigured(t *testing.T) {
	// Preparing configuration: No default configuration (or default configuration is invalid)
	cfg := Config{
		Enabled:   true,
		StoreType: "memory",
		Default:   ResourceConfig{}, // Empty default configuration (invalid)
		Resources: map[string]ResourceConfig{
			"configured-resource": {
				Algorithm:  "token_bucket",
				Rate:       1,
				Capacity:   1,
				InitTokens: 1,
			},
		},
	}

	testLogger := logger.GetLogger("test")
	mgr, err := NewManagerWithLogger(cfg, testLogger, nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, mgr)

	ctx := context.Background()

	// Test 1: Configured resources should be subject to rate limiting control
	allowed, err := mgr.Allow(ctx, "configured-resource")
	assert.NoError(t, err)
	assert.True(t, allowed, "第一次请求应该通过")

	allowed, err = mgr.Allow(ctx, "configured-resource")
	assert.NoError(t, err)
	assert.False(t, allowed, "第二次请求应该被限流")

	// Test 2: Unconfigured resources should be allowed through (because default is invalid)
	for i := 0; i < 20; i++ {
		allowed, err = mgr.Allow(ctx, "unknown-resource")
		assert.NoError(t, err)
		assert.True(t, allowed, "未配置资源应该一直放行（default无效）")
	}

	mgr.Close()
}

// TestDefaultConfig_Configured Tests the behavior when a valid default is configured
func TestDefaultConfig_Configured(t *testing.T) {
	// Prepare configuration: effective default configured
	cfg := Config{
		Enabled:   true,
		StoreType: "memory",
		Default: ResourceConfig{
			Algorithm:  "token_bucket",
			Rate:       1,  // Strict rate limiting: 1 token per second
			Capacity:   1,  // Bucket capacity 1
			InitTokens: 1,  // Initial 1 token
		},
		Resources: map[string]ResourceConfig{
			"configured-resource": {
				Algorithm:  "token_bucket",
				Rate:       10,
				Capacity:   10,
				InitTokens: 10,
			},
		},
	}

	testLogger := logger.GetLogger("test")
	mgr, err := NewManagerWithLogger(cfg, testLogger, nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, mgr)

	ctx := context.Background()

	// Test 1: Configured resources use their own settings
	for i := 0; i < 5; i++ {
		allowed, err := mgr.Allow(ctx, "configured-resource")
		assert.NoError(t, err)
		assert.True(t, allowed, "已配置资源前5次请求应该通过（rate=10）")
	}

	// Test 2: Unconfigured resources should use the default configuration for rate limiting
	allowed, err := mgr.Allow(ctx, "unknown-resource")
	assert.NoError(t, err)
	assert.True(t, allowed, "未配置资源第1次请求应该通过（使用default配置）")

	allowed, err = mgr.Allow(ctx, "unknown-resource")
	assert.NoError(t, err)
	assert.False(t, allowed, "未配置资源第2次请求应该被限流（default: rate=1）")

	// Test 3: Another unconfigured resource should also use the default configuration
	allowed, err = mgr.Allow(ctx, "another-unknown-resource")
	assert.NoError(t, err)
	assert.True(t, allowed, "另一个未配置资源第1次请求应该通过")

	allowed, err = mgr.Allow(ctx, "another-unknown-resource")
	assert.NoError(t, err)
	assert.False(t, allowed, "另一个未配置资源第2次请求应该被限流")

	mgr.Close()
}

// TestDefaultConfig_MixedScenario test mixed scenario
func TestDefaultConfig_MixedScenario(t *testing.T) {
	cfg := Config{
		Enabled:   true,
		StoreType: "memory",
		Default: ResourceConfig{
			Algorithm:  "token_bucket",
			Rate:       5,  // Default 5 tokens per second
			Capacity:   5,
			InitTokens: 5,
		},
		Resources: map[string]ResourceConfig{
			"high-qps-service": { // High QPS service
				Algorithm:  "token_bucket",
				Rate:       100,
				Capacity:   100,
				InitTokens: 100,
			},
			"low-qps-service": { // low QPS service
				Algorithm:  "token_bucket",
				Rate:       1,
				Capacity:   1,
				InitTokens: 1,
			},
		},
	}

	testLogger := logger.GetLogger("test")
	mgr, err := NewManagerWithLogger(cfg, testLogger, nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, mgr)

	ctx := context.Background()

	// Test 1: High QPS service uses its own configuration
	successCount := 0
	for i := 0; i < 50; i++ {
		allowed, err := mgr.Allow(ctx, "high-qps-service")
		assert.NoError(t, err)
		if allowed {
			successCount++
		}
	}
	assert.GreaterOrEqual(t, successCount, 40, "高QPS服务应该通过大部分请求")

	// Test 2: Low QPS service uses its own configuration
	allowed, err := mgr.Allow(ctx, "low-qps-service")
	assert.NoError(t, err)
	assert.True(t, allowed, "低QPS服务第1次应该通过")

	allowed, err = mgr.Allow(ctx, "low-qps-service")
	assert.NoError(t, err)
	assert.False(t, allowed, "低QPS服务第2次应该被限流")

	// Test 3: Unconfigured service uses default configuration
	successCount = 0
	for i := 0; i < 10; i++ {
		allowed, err = mgr.Allow(ctx, "unconfigured-service")
		assert.NoError(t, err)
		if allowed {
			successCount++
		}
	}
	assert.LessOrEqual(t, successCount, 5, "未配置服务应该受default限流（rate=5）")

	mgr.Close()
}

