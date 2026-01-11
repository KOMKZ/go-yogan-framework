package limiter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlidingWindow_Allow(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewSlidingWindowAlgorithm()
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:  string(AlgorithmSlidingWindow),
		Limit:      10,
		WindowSize: 1 * time.Second,
		BucketSize: 100 * time.Millisecond,
	}

	// 前10个请求应该通过
	for i := 0; i < 10; i++ {
		resp, err := algo.Allow(ctx, store, "test_sw_allow", 1, cfg)
		require.NoError(t, err)
		if !assert.True(t, resp.Allowed, "第%d个请求应该通过", i+1) {
			t.Logf("第%d个请求: Allowed=%v, Remaining=%d, Limit=%d", i+1, resp.Allowed, resp.Remaining, resp.Limit)
		}
	}

	// 第11个请求应该被拒绝
	resp, err := algo.Allow(ctx, store, "test_sw_allow", 1, cfg)
	require.NoError(t, err)
	if !assert.False(t, resp.Allowed, "第11个请求应该被拒绝") {
		t.Logf("第11个请求: Allowed=%v, Remaining=%d, Limit=%d", resp.Allowed, resp.Remaining, resp.Limit)
		
		// 获取指标看看实际请求数
		metrics, _ := algo.GetMetrics(ctx, store, "test_sw_allow")
		t.Logf("Metrics: Current=%d", metrics.Current)
	}
}

func TestSlidingWindow_WindowExpire(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewSlidingWindowAlgorithm()
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:  string(AlgorithmSlidingWindow),
		Limit:      5,
		WindowSize: 500 * time.Millisecond,
	}

	// 消耗所有配额
	for i := 0; i < 5; i++ {
		resp, err := algo.Allow(ctx, store, "test_sw_expire", 1, cfg)
		require.NoError(t, err)
		require.True(t, resp.Allowed, "第%d个请求应该通过", i+1)
	}

	// 下一个请求应该被拒绝（窗口内已有5个请求）
	resp, err := algo.Allow(ctx, store, "test_sw_expire", 1, cfg)
	require.NoError(t, err)
	if !assert.False(t, resp.Allowed, "第6个请求应该被拒绝，remaining=%d", resp.Remaining) {
		t.Logf("调试信息: Allowed=%v, Remaining=%d, Limit=%d", resp.Allowed, resp.Remaining, resp.Limit)
	}

	// 等待窗口过期（多等一点时间确保所有请求都过期）
	time.Sleep(600 * time.Millisecond)

	// 窗口过期后，所有旧请求应该被清除，现在应该可以再次请求
	resp, err = algo.Allow(ctx, store, "test_sw_expire", 1, cfg)
	require.NoError(t, err)
	if !assert.True(t, resp.Allowed, "窗口过期后应该允许请求") {
		t.Logf("调试信息: Allowed=%v, Remaining=%d, Limit=%d", resp.Allowed, resp.Remaining, resp.Limit)
	}
}

func TestSlidingWindow_Reset(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewSlidingWindowAlgorithm()
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:  string(AlgorithmSlidingWindow),
		Limit:      5,
		WindowSize: 1 * time.Second,
	}

	// 消耗所有配额
	for i := 0; i < 5; i++ {
		algo.Allow(ctx, store, "test_sw_reset", 1, cfg)
	}

	// 重置
	err := algo.Reset(ctx, store, "test_sw_reset")
	require.NoError(t, err)

	// 重置后应该可以再次请求
	resp, err := algo.Allow(ctx, store, "test_sw_reset", 1, cfg)
	require.NoError(t, err)
	assert.True(t, resp.Allowed)
}

func TestSlidingWindow_GetMetrics(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewSlidingWindowAlgorithm()
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:  string(AlgorithmSlidingWindow),
		Limit:      10,
		WindowSize: 1 * time.Second,
	}

	// 发送一些请求
	for i := 0; i < 5; i++ {
		algo.Allow(ctx, store, "test_sw_metrics", 1, cfg)
	}

	// 获取指标
	metrics, err := algo.GetMetrics(ctx, store, "test_sw_metrics")
	require.NoError(t, err)
	assert.Equal(t, int64(5), metrics.Current)
}

func TestConcurrency_Allow(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewConcurrencyAlgorithm()
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:      string(AlgorithmConcurrency),
		MaxConcurrency: 5,
	}

	// 前5个请求应该通过
	for i := 0; i < 5; i++ {
		resp, err := algo.Allow(ctx, store, "test", 1, cfg)
		require.NoError(t, err)
		assert.True(t, resp.Allowed, "第%d个请求应该通过", i+1)
	}

	// 第6个请求应该被拒绝
	resp, err := algo.Allow(ctx, store, "test", 1, cfg)
	require.NoError(t, err)
	assert.False(t, resp.Allowed)
}

func TestConcurrency_Release(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewConcurrencyAlgorithm().(*concurrencyAlgorithm)
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:      string(AlgorithmConcurrency),
		MaxConcurrency: 5,
	}

	// 获取5个并发
	for i := 0; i < 5; i++ {
		algo.Allow(ctx, store, "test", 1, cfg)
	}

	// 释放2个
	err := algo.Release(ctx, store, "test", 2)
	require.NoError(t, err)

	// 现在应该可以再获取2个
	resp, err := algo.Allow(ctx, store, "test", 2, cfg)
	require.NoError(t, err)
	assert.True(t, resp.Allowed)
}

func TestConcurrency_Reset(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewConcurrencyAlgorithm()
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:      string(AlgorithmConcurrency),
		MaxConcurrency: 3,
	}

	// 消耗所有并发
	for i := 0; i < 3; i++ {
		algo.Allow(ctx, store, "test", 1, cfg)
	}

	// 重置
	err := algo.Reset(ctx, store, "test")
	require.NoError(t, err)

	// 重置后应该可以再次请求
	resp, err := algo.Allow(ctx, store, "test", 1, cfg)
	require.NoError(t, err)
	assert.True(t, resp.Allowed)
}

func TestConcurrency_GetMetrics(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewConcurrencyAlgorithm()
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:      string(AlgorithmConcurrency),
		MaxConcurrency: 10,
	}

	// 获取一些并发
	for i := 0; i < 3; i++ {
		algo.Allow(ctx, store, "test", 1, cfg)
	}

	// 获取指标
	metrics, err := algo.GetMetrics(ctx, store, "test")
	require.NoError(t, err)
	assert.Equal(t, int64(3), metrics.Current)
}

func TestAdaptive_WithoutProvider(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	// 没有provider时应该使用最大限流值
	algo := NewAdaptiveAlgorithm(nil)
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:      string(AlgorithmAdaptive),
		MinLimit:       100,
		MaxLimit:       1000,
		AdjustInterval: 100 * time.Millisecond,
	}

	// 应该使用MaxLimit（1000）
	resp, err := algo.Allow(ctx, store, "test", 1, cfg)
	require.NoError(t, err)
	assert.True(t, resp.Allowed)
}

// MockAdaptiveProvider 模拟的自适应数据提供者
type MockAdaptiveProvider struct {
	cpu    float64
	memory float64
	load   float64
}

func (p *MockAdaptiveProvider) GetCPUUsage() float64 {
	return p.cpu
}

func (p *MockAdaptiveProvider) GetMemoryUsage() float64 {
	return p.memory
}

func (p *MockAdaptiveProvider) GetSystemLoad() float64 {
	return p.load
}

func TestAdaptive_WithProvider(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	provider := &MockAdaptiveProvider{
		cpu:    0.5,
		memory: 0.6,
		load:   0.55,
	}

	algo := NewAdaptiveAlgorithm(provider)
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:      string(AlgorithmAdaptive),
		MinLimit:       100,
		MaxLimit:       1000,
		TargetCPU:      0.7,
		AdjustInterval: 100 * time.Millisecond,
	}

	// 第一次调用应该使用中间值
	resp, err := algo.Allow(ctx, store, "test", 1, cfg)
	require.NoError(t, err)
	assert.True(t, resp.Allowed)

	// 等待调整间隔
	time.Sleep(150 * time.Millisecond)

	// CPU负载较低，应该提高限流值
	provider.cpu = 0.5 // CPU使用率50%，低于目标70%

	resp, err = algo.Allow(ctx, store, "test", 1, cfg)
	require.NoError(t, err)
	assert.True(t, resp.Allowed)
}

func TestAdaptive_HighLoad(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	provider := &MockAdaptiveProvider{
		cpu: 0.9, // 高CPU使用率
	}

	algo := NewAdaptiveAlgorithm(provider)
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:      string(AlgorithmAdaptive),
		MinLimit:       100,
		MaxLimit:       1000,
		TargetCPU:      0.7,
		AdjustInterval: 50 * time.Millisecond,
	}

	// 触发调整
	algo.Allow(ctx, store, "test", 1, cfg)

	// 等待调整间隔
	time.Sleep(100 * time.Millisecond)

	// 高负载应该降低限流值
	algo.Allow(ctx, store, "test", 1, cfg)

	metrics, err := algo.GetMetrics(ctx, store, "test")
	require.NoError(t, err)
	assert.NotNil(t, metrics)
}

func TestAdaptive_Reset(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewAdaptiveAlgorithm(nil)
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm: string(AlgorithmAdaptive),
		MinLimit:  100,
		MaxLimit:  1000,
	}

	algo.Allow(ctx, store, "test", 1, cfg)

	err := algo.Reset(ctx, store, "test")
	require.NoError(t, err)
}

