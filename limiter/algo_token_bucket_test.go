package limiter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenBucket_Allow(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewTokenBucketAlgorithm()
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:  string(AlgorithmTokenBucket),
		Rate:       10, // 10 tokens/second
		Capacity:   10, // 桶容量10
		InitTokens: 10, // 初始满桶
	}

	// 前10个请求应该通过
	for i := 0; i < 10; i++ {
		resp, err := algo.Allow(ctx, store, "test", 1, cfg)
		require.NoError(t, err)
		assert.True(t, resp.Allowed, "第%d个请求应该通过", i+1)
		assert.Equal(t, int64(10-i-1), resp.Remaining)
	}

	// 第11个请求应该被拒绝
	resp, err := algo.Allow(ctx, store, "test", 1, cfg)
	require.NoError(t, err)
	assert.False(t, resp.Allowed, "第11个请求应该被拒绝")
	assert.Greater(t, resp.RetryAfter, time.Duration(0))
}

func TestTokenBucket_AllowN(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewTokenBucketAlgorithm()
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:  string(AlgorithmTokenBucket),
		Rate:       10,
		Capacity:   20,
		InitTokens: 20,
	}

	// 一次获取5个令牌
	resp, err := algo.Allow(ctx, store, "test", 5, cfg)
	require.NoError(t, err)
	assert.True(t, resp.Allowed)
	assert.Equal(t, int64(15), resp.Remaining)

	// 再获取10个令牌
	resp, err = algo.Allow(ctx, store, "test", 10, cfg)
	require.NoError(t, err)
	assert.True(t, resp.Allowed)
	assert.Equal(t, int64(5), resp.Remaining)

	// 再获取10个令牌应该失败（只剩5个）
	resp, err = algo.Allow(ctx, store, "test", 10, cfg)
	require.NoError(t, err)
	assert.False(t, resp.Allowed)
}

func TestTokenBucket_Refill(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewTokenBucketAlgorithm()
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:  string(AlgorithmTokenBucket),
		Rate:       10, // 10 tokens/second
		Capacity:   10,
		InitTokens: 10,
	}

	// 消耗所有令牌
	for i := 0; i < 10; i++ {
		algo.Allow(ctx, store, "test", 1, cfg)
	}

	// 下一个请求应该被拒绝
	resp, err := algo.Allow(ctx, store, "test", 1, cfg)
	require.NoError(t, err)
	assert.False(t, resp.Allowed)

	// 等待1秒，应该补充10个令牌
	time.Sleep(1 * time.Second)

	// 现在应该可以再次请求
	resp, err = algo.Allow(ctx, store, "test", 1, cfg)
	require.NoError(t, err)
	assert.True(t, resp.Allowed)
}

func TestTokenBucket_Wait(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewTokenBucketAlgorithm()
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:  string(AlgorithmTokenBucket),
		Rate:       10, // 10 tokens/second
		Capacity:   5,
		InitTokens: 5,
	}

	// 消耗所有令牌
	for i := 0; i < 5; i++ {
		algo.Allow(ctx, store, "test", 1, cfg)
	}

	// Wait应该等待一段时间后成功
	start := time.Now()
	err := algo.Wait(ctx, store, "test", 1, cfg, 2*time.Second)
	elapsed := time.Since(start)

	require.NoError(t, err)
	// 应该等待至少100ms（生成1个令牌）
	assert.Greater(t, elapsed, 50*time.Millisecond)
}

func TestTokenBucket_WaitTimeout(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewTokenBucketAlgorithm()
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:  string(AlgorithmTokenBucket),
		Rate:       10, // 10 tokens/second
		Capacity:   10,
		InitTokens: 10,
	}

	// 先消耗所有令牌
	for i := 0; i < 10; i++ {
		algo.Allow(ctx, store, "test", 1, cfg)
	}

	// 修改配置为很慢的速率
	cfg.Rate = 1 // 1 token/second (很慢)

	// Wait应该超时（100ms无法生成足够令牌）
	err := algo.Wait(ctx, store, "test", 1, cfg, 100*time.Millisecond)
	assert.ErrorIs(t, err, ErrWaitTimeout)
}

func TestTokenBucket_GetMetrics(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewTokenBucketAlgorithm()
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:  string(AlgorithmTokenBucket),
		Rate:       10,
		Capacity:   20,
		InitTokens: 20,
	}

	// 消耗一些令牌
	algo.Allow(ctx, store, "test", 5, cfg)

	// 获取指标
	metrics, err := algo.GetMetrics(ctx, store, "test")
	require.NoError(t, err)
	assert.Equal(t, int64(15), metrics.Remaining)
}

func TestTokenBucket_Reset(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewTokenBucketAlgorithm()
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:  string(AlgorithmTokenBucket),
		Rate:       10,
		Capacity:   10,
		InitTokens: 10,
	}

	// 消耗所有令牌
	for i := 0; i < 10; i++ {
		algo.Allow(ctx, store, "test", 1, cfg)
	}

	// 下一个请求应该被拒绝
	resp, err := algo.Allow(ctx, store, "test", 1, cfg)
	require.NoError(t, err)
	assert.False(t, resp.Allowed)

	// 重置
	err = algo.Reset(ctx, store, "test")
	require.NoError(t, err)

	// 重置后应该恢复到满桶状态
	resp, err = algo.Allow(ctx, store, "test", 1, cfg)
	require.NoError(t, err)
	assert.True(t, resp.Allowed)
	assert.Equal(t, int64(9), resp.Remaining)
}

func TestTokenBucket_MultipleResources(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewTokenBucketAlgorithm()
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:  string(AlgorithmTokenBucket),
		Rate:       10,
		Capacity:   5,
		InitTokens: 5,
	}

	// 资源1消耗所有令牌
	for i := 0; i < 5; i++ {
		algo.Allow(ctx, store, "resource1", 1, cfg)
	}

	// 资源1下一个请求应该被拒绝
	resp, err := algo.Allow(ctx, store, "resource1", 1, cfg)
	require.NoError(t, err)
	assert.False(t, resp.Allowed)

	// 资源2应该独立，不受影响
	resp, err = algo.Allow(ctx, store, "resource2", 1, cfg)
	require.NoError(t, err)
	assert.True(t, resp.Allowed)
}

func TestTokenBucket_BurstTraffic(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewTokenBucketAlgorithm()
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:  string(AlgorithmTokenBucket),
		Rate:       100, // 100 QPS
		Capacity:   200, // 允许200突发
		InitTokens: 200,
	}

	// 突发200个请求应该全部通过
	for i := 0; i < 200; i++ {
		resp, err := algo.Allow(ctx, store, "test", 1, cfg)
		require.NoError(t, err)
		assert.True(t, resp.Allowed, "突发请求%d应该通过", i+1)
	}

	// 第201个请求应该被拒绝
	resp, err := algo.Allow(ctx, store, "test", 1, cfg)
	require.NoError(t, err)
	assert.False(t, resp.Allowed)
}

func TestTokenBucket_PartialRefill(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewTokenBucketAlgorithm()
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:  string(AlgorithmTokenBucket),
		Rate:       10, // 10 tokens/second
		Capacity:   10,
		InitTokens: 10,
	}

	// 先消耗所有令牌
	for i := 0; i < 10; i++ {
		algo.Allow(ctx, store, "test", 1, cfg)
	}

	// 第11个请求应该被拒绝（桶空）
	resp, err := algo.Allow(ctx, store, "test", 1, cfg)
	require.NoError(t, err)
	assert.False(t, resp.Allowed)

	// 等待500ms（应该生成约5个令牌）
	time.Sleep(550 * time.Millisecond) // 稍微多等一点以确保生成5个令牌

	// 现在应该可以通过5个请求
	for i := 0; i < 5; i++ {
		resp, err := algo.Allow(ctx, store, "test", 1, cfg)
		require.NoError(t, err)
		assert.True(t, resp.Allowed, "第%d个请求应该通过", i+1)
	}

	// 第6个请求应该被拒绝
	resp, err = algo.Allow(ctx, store, "test", 1, cfg)
	require.NoError(t, err)
	assert.False(t, resp.Allowed)
}
