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
		Capacity:   10, // Bucket capacity 10
		InitTokens: 10, // Initial full bucket
	}

	// The first 10 requests should pass
	for i := 0; i < 10; i++ {
		resp, err := algo.Allow(ctx, store, "test", 1, cfg)
		require.NoError(t, err)
		assert.True(t, resp.Allowed, "第%d个请求应该通过", i+1)
		assert.Equal(t, int64(10-i-1), resp.Remaining)
	}

	// The 11th request should be rejected
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

	// Get 5 tokens at a time
	resp, err := algo.Allow(ctx, store, "test", 5, cfg)
	require.NoError(t, err)
	assert.True(t, resp.Allowed)
	assert.Equal(t, int64(15), resp.Remaining)

	// Retrieve 10 more tokens
	resp, err = algo.Allow(ctx, store, "test", 10, cfg)
	require.NoError(t, err)
	assert.True(t, resp.Allowed)
	assert.Equal(t, int64(5), resp.Remaining)

	// Getting another 10 tokens should fail (only 5 left)
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

	// Consume all tokens
	for i := 0; i < 10; i++ {
		algo.Allow(ctx, store, "test", 1, cfg)
	}

	// The next request should be rejected
	resp, err := algo.Allow(ctx, store, "test", 1, cfg)
	require.NoError(t, err)
	assert.False(t, resp.Allowed)

	// Wait for 1 second, should replenish 10 tokens
	time.Sleep(1 * time.Second)

	// Now should be able to make the request again
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

	// Consume all tokens
	for i := 0; i < 5; i++ {
		algo.Allow(ctx, store, "test", 1, cfg)
	}

	// The wait should succeed after a period of time
	start := time.Now()
	err := algo.Wait(ctx, store, "test", 1, cfg, 2*time.Second)
	elapsed := time.Since(start)

	require.NoError(t, err)
	// Should wait at least 100ms (generate one token)
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

	// Consume all tokens first
	for i := 0; i < 10; i++ {
		algo.Allow(ctx, store, "test", 1, cfg)
	}

	// Modify configuration to very slow rate
	cfg.Rate = 1 // 1 token/second (very slow)

	// Wait should timeout if insufficient tokens are generated (within 100ms)
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

	// Consume some tokens
	algo.Allow(ctx, store, "test", 5, cfg)

	// Get metric
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

	// Consume all tokens
	for i := 0; i < 10; i++ {
		algo.Allow(ctx, store, "test", 1, cfg)
	}

	// The next request should be rejected
	resp, err := algo.Allow(ctx, store, "test", 1, cfg)
	require.NoError(t, err)
	assert.False(t, resp.Allowed)

	// reset
	err = algo.Reset(ctx, store, "test")
	require.NoError(t, err)

	// Should be reset to full bucket state after reset
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

	// Resource 1 consumes all tokens
	for i := 0; i < 5; i++ {
		algo.Allow(ctx, store, "resource1", 1, cfg)
	}

	// Resource 1 should reject the next request
	resp, err := algo.Allow(ctx, store, "resource1", 1, cfg)
	require.NoError(t, err)
	assert.False(t, resp.Allowed)

	// Resource 2 should be independent and unaffected
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
		Capacity:   200, // Allow 200 burst requests
		InitTokens: 200,
	}

	// A sudden 200 requests should all pass_through
	for i := 0; i < 200; i++ {
		resp, err := algo.Allow(ctx, store, "test", 1, cfg)
		require.NoError(t, err)
		assert.True(t, resp.Allowed, "突发请求%d应该通过", i+1)
	}

	// The 201st request should be rejected
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

	// Consume all tokens first
	for i := 0; i < 10; i++ {
		algo.Allow(ctx, store, "test", 1, cfg)
	}

	// The 11th request should be rejected (bucket empty)
	resp, err := algo.Allow(ctx, store, "test", 1, cfg)
	require.NoError(t, err)
	assert.False(t, resp.Allowed)

	// wait 500ms (approximately 5 tokens should be generated)
	time.Sleep(550 * time.Millisecond) // Wait a little longer to ensure five tokens are generated

	// Now it should be possible with 5 requests
	for i := 0; i < 5; i++ {
		resp, err := algo.Allow(ctx, store, "test", 1, cfg)
		require.NoError(t, err)
		assert.True(t, resp.Allowed, "第%d个请求应该通过", i+1)
	}

	// The sixth request should be rejected
	resp, err = algo.Allow(ctx, store, "test", 1, cfg)
	require.NoError(t, err)
	assert.False(t, resp.Allowed)
}
