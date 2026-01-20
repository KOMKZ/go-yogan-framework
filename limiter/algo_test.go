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

	// The first 10 requests should pass
	for i := 0; i < 10; i++ {
		resp, err := algo.Allow(ctx, store, "test_sw_allow", 1, cfg)
		require.NoError(t, err)
		if !assert.True(t, resp.Allowed, "第%d个请求应该通过", i+1) {
			t.Logf("第%d个请求: Allowed=%v, Remaining=%d, Limit=%d", i+1, resp.Allowed, resp.Remaining, resp.Limit)
		}
	}

	// The 11th request should be rejected
	resp, err := algo.Allow(ctx, store, "test_sw_allow", 1, cfg)
	require.NoError(t, err)
	if !assert.False(t, resp.Allowed, "第11个请求应该被拒绝") {
		t.Logf("第11个请求: Allowed=%v, Remaining=%d, Limit=%d", resp.Allowed, resp.Remaining, resp.Limit)
		
		// Get metrics to check actual request count
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

	// Consume all quotas
	for i := 0; i < 5; i++ {
		resp, err := algo.Allow(ctx, store, "test_sw_expire", 1, cfg)
		require.NoError(t, err)
		require.True(t, resp.Allowed, "第%d个请求应该通过", i+1)
	}

	// The next request should be rejected (there are already 5 requests in the window)
	resp, err := algo.Allow(ctx, store, "test_sw_expire", 1, cfg)
	require.NoError(t, err)
	if !assert.False(t, resp.Allowed, "第6个请求应该被拒绝，remaining=%d", resp.Remaining) {
		t.Logf("调试信息: Allowed=%v, Remaining=%d, Limit=%d", resp.Allowed, resp.Remaining, resp.Limit)
	}

	// wait for the window to expire (wait a bit longer to ensure that all requests have expired)
	time.Sleep(600 * time.Millisecond)

	// After the window expires, all old requests should be cleared, and new requests can now be made.
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

	// Consume all quotas
	for i := 0; i < 5; i++ {
		algo.Allow(ctx, store, "test_sw_reset", 1, cfg)
	}

	// reset
	err := algo.Reset(ctx, store, "test_sw_reset")
	require.NoError(t, err)

	// Should be able to make another request after reset
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

	// Send some requests
	for i := 0; i < 5; i++ {
		algo.Allow(ctx, store, "test_sw_metrics", 1, cfg)
	}

	// Get metric
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

	// The first 5 requests should pass
	for i := 0; i < 5; i++ {
		resp, err := algo.Allow(ctx, store, "test", 1, cfg)
		require.NoError(t, err)
		assert.True(t, resp.Allowed, "第%d个请求应该通过", i+1)
	}

	// The sixth request should be rejected
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

	// Get 5 concurrent connections
	for i := 0; i < 5; i++ {
		algo.Allow(ctx, store, "test", 1, cfg)
	}

	// Release 2
	err := algo.Release(ctx, store, "test", 2)
	require.NoError(t, err)

	// Now it should be able to get 2 more
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

	// Consume all concurrency
	for i := 0; i < 3; i++ {
		algo.Allow(ctx, store, "test", 1, cfg)
	}

	// reset
	err := algo.Reset(ctx, store, "test")
	require.NoError(t, err)

	// Should be able to request again after reset
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

	// Get some concurrency
	for i := 0; i < 3; i++ {
		algo.Allow(ctx, store, "test", 1, cfg)
	}

	// Get metric
	metrics, err := algo.GetMetrics(ctx, store, "test")
	require.NoError(t, err)
	assert.Equal(t, int64(3), metrics.Current)
}

func TestAdaptive_WithoutProvider(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	// Use the maximum rate limit value when there is no provider
	algo := NewAdaptiveAlgorithm(nil)
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:      string(AlgorithmAdaptive),
		MinLimit:       100,
		MaxLimit:       1000,
		AdjustInterval: 100 * time.Millisecond,
	}

	// Should use MaxLimit(1000)
	resp, err := algo.Allow(ctx, store, "test", 1, cfg)
	require.NoError(t, err)
	assert.True(t, resp.Allowed)
}

// Mock Adaptive Data Provider
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

	// The first call should use the intermediate value
	resp, err := algo.Allow(ctx, store, "test", 1, cfg)
	require.NoError(t, err)
	assert.True(t, resp.Allowed)

	// wait for adjustment interval
	time.Sleep(150 * time.Millisecond)

	// CPU load is low, the throttling value should be increased
	provider.cpu = 0.5 // CPU usage is 50%, below the target of 70%

	resp, err = algo.Allow(ctx, store, "test", 1, cfg)
	require.NoError(t, err)
	assert.True(t, resp.Allowed)
}

func TestAdaptive_HighLoad(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	provider := &MockAdaptiveProvider{
		cpu: 0.9, // High CPU usage
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

	// Trigger adjustment
	algo.Allow(ctx, store, "test", 1, cfg)

	// wait for adjustment interval
	time.Sleep(100 * time.Millisecond)

	// High load should decrease rate limiting values
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

