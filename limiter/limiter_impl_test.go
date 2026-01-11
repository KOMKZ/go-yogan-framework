package limiter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_NewManager(t *testing.T) {
	cfg := Config{
		Enabled:   true,
		StoreType: "memory",
		Default:   DefaultResourceConfig(),
	}

	mgr, err := NewManager(cfg)
	require.NoError(t, err)
	defer mgr.Close()

	assert.True(t, mgr.IsEnabled())
}

func TestManager_Disabled(t *testing.T) {
	cfg := Config{
		Enabled: false,
	}

	mgr, err := NewManager(cfg)
	require.NoError(t, err)
	defer mgr.Close()

	assert.False(t, mgr.IsEnabled())

	// 未启用时应该直接允许
	ctx := context.Background()
	allowed, err := mgr.Allow(ctx, "test")
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestManager_Allow(t *testing.T) {
	cfg := Config{
		Enabled:   true,
		StoreType: "memory",
		Default: ResourceConfig{
			Algorithm:  "token_bucket",
			Rate:       10,
			Capacity:   10,
			InitTokens: 10,
		},
		Resources: map[string]ResourceConfig{
			"test": {
				Algorithm:  "token_bucket",
				Rate:       10,
				Capacity:   10,
				InitTokens: 10,
			},
		},
	}

	mgr, err := NewManager(cfg)
	require.NoError(t, err)
	defer mgr.Close()

	ctx := context.Background()

	// 前10个请求应该通过
	for i := 0; i < 10; i++ {
		allowed, err := mgr.Allow(ctx, "test")
		require.NoError(t, err)
		assert.True(t, allowed, "第%d个请求应该通过", i+1)
	}

	// 第11个请求应该被拒绝
	allowed, err := mgr.Allow(ctx, "test")
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestManager_AllowN(t *testing.T) {
	cfg := Config{
		Enabled:   true,
		StoreType: "memory",
		Default: ResourceConfig{
			Algorithm:  "token_bucket",
			Rate:       20,
			Capacity:   20,
			InitTokens: 20,
		},
		Resources: map[string]ResourceConfig{
			"test": {
				Algorithm:  "token_bucket",
				Rate:       20,
				Capacity:   20,
				InitTokens: 20,
			},
		},
	}

	mgr, err := NewManager(cfg)
	require.NoError(t, err)
	defer mgr.Close()

	ctx := context.Background()

	// 一次获取5个令牌
	allowed, err := mgr.AllowN(ctx, "test", 5)
	require.NoError(t, err)
	assert.True(t, allowed)

	// 再获取10个令牌
	allowed, err = mgr.AllowN(ctx, "test", 10)
	require.NoError(t, err)
	assert.True(t, allowed)

	// 再获取10个令牌应该失败（只剩5个）
	allowed, err = mgr.AllowN(ctx, "test", 10)
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestManager_Wait(t *testing.T) {
	cfg := Config{
		Enabled:   true,
		StoreType: "memory",
		Default: ResourceConfig{
			Algorithm:  "token_bucket",
			Rate:       10,
			Capacity:   5,
			InitTokens: 5,
			Timeout:    2 * time.Second,
		},
		Resources: map[string]ResourceConfig{
			"test": {
				Algorithm:  "token_bucket",
				Rate:       10,
				Capacity:   5,
				InitTokens: 5,
				Timeout:    2 * time.Second,
			},
		},
	}

	mgr, err := NewManager(cfg)
	require.NoError(t, err)
	defer mgr.Close()

	ctx := context.Background()

	// 消耗所有令牌
	for i := 0; i < 5; i++ {
		mgr.Allow(ctx, "test")
	}

	// Wait应该等待一段时间后成功
	start := time.Now()
	err = mgr.Wait(ctx, "test")
	elapsed := time.Since(start)

	require.NoError(t, err)
	// 应该等待至少50ms（生成1个令牌）
	assert.Greater(t, elapsed, 50*time.Millisecond)
}

func TestManager_GetMetrics(t *testing.T) {
	cfg := Config{
		Enabled:   true,
		StoreType: "memory",
		Default: ResourceConfig{
			Algorithm:  "token_bucket",
			Rate:       10,
			Capacity:   20,
			InitTokens: 20,
		},
		Resources: map[string]ResourceConfig{
			"test": {
				Algorithm:  "token_bucket",
				Rate:       10,
				Capacity:   20,
				InitTokens: 20,
			},
		},
	}

	mgr, err := NewManager(cfg)
	require.NoError(t, err)
	defer mgr.Close()

	ctx := context.Background()

	// 消耗一些令牌
	mgr.AllowN(ctx, "test", 5)

	// 获取指标
	metrics := mgr.GetMetrics("test")
	require.NotNil(t, metrics)
	assert.Equal(t, "test", metrics.Resource)
	assert.Equal(t, int64(1), metrics.TotalRequests)
	assert.Equal(t, int64(1), metrics.Allowed)
	assert.Equal(t, int64(0), metrics.Rejected)
}

func TestManager_Reset(t *testing.T) {
	cfg := Config{
		Enabled:   true,
		StoreType: "memory",
		Default: ResourceConfig{
			Algorithm:  "token_bucket",
			Rate:       10,
			Capacity:   10,
			InitTokens: 10,
		},
		Resources: map[string]ResourceConfig{
			"test": {
				Algorithm:  "token_bucket",
				Rate:       10,
				Capacity:   10,
				InitTokens: 10,
			},
		},
	}

	mgr, err := NewManager(cfg)
	require.NoError(t, err)
	defer mgr.Close()

	ctx := context.Background()

	// 消耗所有令牌
	for i := 0; i < 10; i++ {
		mgr.Allow(ctx, "test")
	}

	// 下一个请求应该被拒绝
	allowed, err := mgr.Allow(ctx, "test")
	require.NoError(t, err)
	assert.False(t, allowed)

	// 重置
	mgr.Reset("test")

	// 重置后应该恢复到满桶状态
	allowed, err = mgr.Allow(ctx, "test")
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestManager_MultipleResources(t *testing.T) {
	cfg := Config{
		Enabled:   true,
		StoreType: "memory",
		Default: ResourceConfig{
			Algorithm:  "token_bucket",
			Rate:       10,
			Capacity:   5,
			InitTokens: 5,
		},
		Resources: map[string]ResourceConfig{
			"resource1": {
				Algorithm:  "token_bucket",
				Rate:       10,
				Capacity:   5,
				InitTokens: 5,
			},
			"resource2": {
				Algorithm:  "token_bucket",
				Rate:       10,
				Capacity:   5,
				InitTokens: 5,
			},
		},
	}

	mgr, err := NewManager(cfg)
	require.NoError(t, err)
	defer mgr.Close()

	ctx := context.Background()

	// 资源1消耗所有令牌
	for i := 0; i < 5; i++ {
		mgr.Allow(ctx, "resource1")
	}

	// 资源1下一个请求应该被拒绝
	allowed, err := mgr.Allow(ctx, "resource1")
	require.NoError(t, err)
	assert.False(t, allowed)

	// 资源2应该独立，不受影响
	allowed, err = mgr.Allow(ctx, "resource2")
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestManager_ResourceConfig(t *testing.T) {
	cfg := Config{
		Enabled:   true,
		StoreType: "memory",
		Default: ResourceConfig{
			Algorithm:  "token_bucket",
			Rate:       100,
			Capacity:   200,
			InitTokens: 200,
		},
		Resources: map[string]ResourceConfig{
			"api1": {
				Algorithm:  "token_bucket",
				Rate:       50,
				Capacity:   100,
				InitTokens: 100,
			},
		},
	}

	err := cfg.Validate()
	require.NoError(t, err)

	mgr, err := NewManager(cfg)
	require.NoError(t, err)
	defer mgr.Close()

	ctx := context.Background()

	// api1使用特定配置（100个令牌）
	for i := 0; i < 100; i++ {
		allowed, err := mgr.Allow(ctx, "api1")
		require.NoError(t, err)
		assert.True(t, allowed, "第%d个请求应该通过", i+1)
	}

	// 第101个请求应该被拒绝
	allowed, err := mgr.Allow(ctx, "api1")
	require.NoError(t, err)
	assert.False(t, allowed)

	// api2使用默认配置（200个令牌）
	for i := 0; i < 200; i++ {
		allowed, err := mgr.Allow(ctx, "api2")
		require.NoError(t, err)
		assert.True(t, allowed, "第%d个请求应该通过", i+1)
	}
}

func TestManager_EventBus(t *testing.T) {
	cfg := Config{
		Enabled:        true,
		StoreType:      "memory",
		EventBusBuffer: 100,
		Default: ResourceConfig{
			Algorithm:  "token_bucket",
			Rate:       10,
			Capacity:   5,
			InitTokens: 5,
		},
		Resources: map[string]ResourceConfig{
			"test": {
				Algorithm:  "token_bucket",
				Rate:       10,
				Capacity:   5,
				InitTokens: 5,
			},
		},
	}

	mgr, err := NewManager(cfg)
	require.NoError(t, err)
	defer mgr.Close()

	// 订阅事件
	eventBus := mgr.GetEventBus()
	require.NotNil(t, eventBus)

	var allowedCount, rejectedCount int
	eventBus.Subscribe(EventListenerFunc(func(event Event) {
		switch event.Type() {
		case EventAllowed:
			allowedCount++
		case EventRejected:
			rejectedCount++
		}
	}))

	ctx := context.Background()

	// 触发一些事件
	for i := 0; i < 7; i++ {
		mgr.Allow(ctx, "test")
		time.Sleep(10 * time.Millisecond) // 给事件处理留时间
	}

	// 等待事件处理
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, 5, allowedCount)
	assert.Equal(t, 2, rejectedCount)
}

func TestManager_InvalidConfig(t *testing.T) {
	cfg := Config{
		Enabled:   true,
		StoreType: "invalid",
	}

	_, err := NewManager(cfg)
	assert.Error(t, err)
}

