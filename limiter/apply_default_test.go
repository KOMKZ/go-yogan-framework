package limiter

import (
	"context"
	"testing"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/stretchr/testify/assert"
)

// TestDefaultConfig_NotConfigured 测试未配置 default 时的行为
func TestDefaultConfig_NotConfigured(t *testing.T) {
	// 准备配置：没有配置 default（或 default 无效）
	cfg := Config{
		Enabled:   true,
		StoreType: "memory",
		Default:   ResourceConfig{}, // 空 default 配置（无效）
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

	// 测试1：已配置的资源应该受限流控制
	allowed, err := mgr.Allow(ctx, "configured-resource")
	assert.NoError(t, err)
	assert.True(t, allowed, "第一次请求应该通过")

	allowed, err = mgr.Allow(ctx, "configured-resource")
	assert.NoError(t, err)
	assert.False(t, allowed, "第二次请求应该被限流")

	// 测试2：未配置的资源应该直接放行（因为 default 无效）
	for i := 0; i < 20; i++ {
		allowed, err = mgr.Allow(ctx, "unknown-resource")
		assert.NoError(t, err)
		assert.True(t, allowed, "未配置资源应该一直放行（default无效）")
	}

	mgr.Close()
}

// TestDefaultConfig_Configured 测试配置了有效 default 时的行为
func TestDefaultConfig_Configured(t *testing.T) {
	// 准备配置：配置了有效的 default
	cfg := Config{
		Enabled:   true,
		StoreType: "memory",
		Default: ResourceConfig{
			Algorithm:  "token_bucket",
			Rate:       1,  // 严格限流：每秒1个令牌
			Capacity:   1,  // 桶容量1
			InitTokens: 1,  // 初始1个令牌
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

	// 测试1：已配置的资源使用自己的配置
	for i := 0; i < 5; i++ {
		allowed, err := mgr.Allow(ctx, "configured-resource")
		assert.NoError(t, err)
		assert.True(t, allowed, "已配置资源前5次请求应该通过（rate=10）")
	}

	// 测试2：未配置的资源应该使用 default 配置限流
	allowed, err := mgr.Allow(ctx, "unknown-resource")
	assert.NoError(t, err)
	assert.True(t, allowed, "未配置资源第1次请求应该通过（使用default配置）")

	allowed, err = mgr.Allow(ctx, "unknown-resource")
	assert.NoError(t, err)
	assert.False(t, allowed, "未配置资源第2次请求应该被限流（default: rate=1）")

	// 测试3：另一个未配置资源也应该使用 default 配置
	allowed, err = mgr.Allow(ctx, "another-unknown-resource")
	assert.NoError(t, err)
	assert.True(t, allowed, "另一个未配置资源第1次请求应该通过")

	allowed, err = mgr.Allow(ctx, "another-unknown-resource")
	assert.NoError(t, err)
	assert.False(t, allowed, "另一个未配置资源第2次请求应该被限流")

	mgr.Close()
}

// TestDefaultConfig_MixedScenario 测试混合场景
func TestDefaultConfig_MixedScenario(t *testing.T) {
	cfg := Config{
		Enabled:   true,
		StoreType: "memory",
		Default: ResourceConfig{
			Algorithm:  "token_bucket",
			Rate:       5,  // 默认每秒5个令牌
			Capacity:   5,
			InitTokens: 5,
		},
		Resources: map[string]ResourceConfig{
			"high-qps-service": { // 高 QPS 服务
				Algorithm:  "token_bucket",
				Rate:       100,
				Capacity:   100,
				InitTokens: 100,
			},
			"low-qps-service": { // 低 QPS 服务
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

	// 测试1：高 QPS 服务使用自己的配置
	successCount := 0
	for i := 0; i < 50; i++ {
		allowed, err := mgr.Allow(ctx, "high-qps-service")
		assert.NoError(t, err)
		if allowed {
			successCount++
		}
	}
	assert.GreaterOrEqual(t, successCount, 40, "高QPS服务应该通过大部分请求")

	// 测试2：低 QPS 服务使用自己的配置
	allowed, err := mgr.Allow(ctx, "low-qps-service")
	assert.NoError(t, err)
	assert.True(t, allowed, "低QPS服务第1次应该通过")

	allowed, err = mgr.Allow(ctx, "low-qps-service")
	assert.NoError(t, err)
	assert.False(t, allowed, "低QPS服务第2次应该被限流")

	// 测试3：未配置服务使用 default 配置
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

