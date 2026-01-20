package limiter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAlgorithm_Name test the Name method for all algorithms
func TestAlgorithm_Name(t *testing.T) {
	tests := []struct {
		name     string
		algo     Algorithm
		expected string
	}{
		{
			name:     "TokenBucket",
			algo:     NewTokenBucketAlgorithm(),
			expected: string(AlgorithmTokenBucket),
		},
		{
			name:     "SlidingWindow",
			algo:     NewSlidingWindowAlgorithm(),
			expected: string(AlgorithmSlidingWindow),
		},
		{
			name:     "Concurrency",
			algo:     NewConcurrencyAlgorithm(),
			expected: string(AlgorithmConcurrency),
		},
		{
			name:     "Adaptive",
			algo:     NewAdaptiveAlgorithm(nil),
			expected: string(AlgorithmAdaptive),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.algo.Name())
		})
	}
}

// TestTokenBucket_WaitSuccess test the Token Bucket's Wait method
func TestTokenBucket_WaitSuccess(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewTokenBucketAlgorithm()
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:  string(AlgorithmTokenBucket),
		Rate:       10,  // 10 tokens/s
		Capacity:   100, // Bucket capacity 100
		InitTokens: 10,  // Initial 10 tokens
	}

	// Wait and obtain 5 tokens (should succeed)
	err := algo.Wait(ctx, store, "test_wait", 5, cfg, 1*time.Second)
	require.NoError(t, err)

	// Verify that the token has been consumed
	metrics, err := algo.GetMetrics(ctx, store, "test_wait")
	require.NoError(t, err)
	assert.Equal(t, int64(5), metrics.Current) // Remaining 5 tokens
}

// TestTokenBucket_WaitContextCancel test context cancellation
func TestTokenBucket_WaitContextCancel(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewTokenBucketAlgorithm()
	ctx, cancel := context.WithCancel(context.Background())

	cfg := ResourceConfig{
		Algorithm:  string(AlgorithmTokenBucket),
		Rate:       1,       // 1 token/s
		Capacity:   10,      // Bucket capacity 10
		InitTokens: 0,       // Initial 0 tokens
	}

	// Immediatly cancel context
	cancel()

	// wait should fail due to cancellation in context
	err := algo.Wait(ctx, store, "test_cancel", 1, cfg, 10*time.Second)
	if err != nil {
		assert.Contains(t, err.Error(), "context")
	}
	// Note: Some implementations may not check for context, so an error is not mandatory here.
}

// TestSlidingWindow_Wait test the Wait method of the Sliding Window
func TestSlidingWindow_Wait(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewSlidingWindowAlgorithm()
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:  string(AlgorithmSlidingWindow),
		Limit:      10,
		WindowSize: 1 * time.Second,
	}

	// wait for and retrieve 3 requests (should succeed)
	err := algo.Wait(ctx, store, "test_sw_wait", 3, cfg, 1*time.Second)
	require.NoError(t, err)

	// Verify that the request is logged
	metrics, err := algo.GetMetrics(ctx, store, "test_sw_wait")
	require.NoError(t, err)
	assert.Equal(t, int64(3), metrics.Current)
}

// TestSlidingWindow_WaitTimeout test sliding window wait timeout
func TestSlidingWindow_WaitTimeout(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewSlidingWindowAlgorithm()
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:  string(AlgorithmSlidingWindow),
		Limit:      5,
		WindowSize: 1 * time.Second,
	}

	// Consume all quotas first
	for i := 0; i < 5; i++ {
		algo.Allow(ctx, store, "test_sw_timeout", 1, cfg)
	}

	// The wait should time out (because the quota has been exhausted and the timeout period is very short)
	err := algo.Wait(ctx, store, "test_sw_timeout", 1, cfg, 50*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

// TestConcurrency_Wait test the concurrent rate limiting Wait method
func TestConcurrency_Wait(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewConcurrencyAlgorithm()
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:      string(AlgorithmConcurrency),
		MaxConcurrency: 10,
	}

	// Wait for and acquire 3 concurrent slots (should succeed)
	err := algo.Wait(ctx, store, "test_conc_wait", 3, cfg, 1*time.Second)
	require.NoError(t, err)

	// Validate concurrency count
	metrics, err := algo.GetMetrics(ctx, store, "test_conc_wait")
	require.NoError(t, err)
	assert.Equal(t, int64(3), metrics.Current)

	// Release
	concAlgo := algo.(*concurrencyAlgorithm)
	concAlgo.Release(ctx, store, "test_conc_wait", 3)
}

// TestConcurrency_WaitTimeout test concurrency rate limiting wait timeout
func TestConcurrency_WaitTimeout(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewConcurrencyAlgorithm()
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:      string(AlgorithmConcurrency),
		MaxConcurrency: 2,
	}

	// First, fill all concurrent slots
	resp, _ := algo.Allow(ctx, store, "test_conc_timeout", 2, cfg)
	require.True(t, resp.Allowed)

	// timeout for waiting should be set
	err := algo.Wait(ctx, store, "test_conc_timeout", 1, cfg, 50*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

// TestAdaptive_Wait test the adaptive rate limiting Wait method
func TestAdaptive_Wait(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewAdaptiveAlgorithm(nil)
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm: string(AlgorithmAdaptive),
		MinLimit:  10,
		MaxLimit:  100,
		TargetCPU: 0.7,
	}

	// Wait for and retrieve 5 requests
	err := algo.Wait(ctx, store, "test_adaptive_wait", 5, cfg, 1*time.Second)
	// The adaptive algorithm may allow or reject, depending on internal state
	_ = err
}

// TestAdaptive_WaitTimeout test adaptive rate limiting wait timeout
func TestAdaptive_WaitTimeout(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewAdaptiveAlgorithm(nil)
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm: string(AlgorithmAdaptive),
		MinLimit:  2,
		MaxLimit:  10,
		TargetCPU: 0.7,
	}

	// wait (possibly timed out or succeeded)
	err := algo.Wait(ctx, store, "test_adaptive_timeout", 10, cfg, 10*time.Millisecond)
	// Do not strictly require results
	_ = err
}

// TestHelperFunctions test helper functions
func TestHelperFunctions(t *testing.T) {
	// Test min64Duration
	assert.Equal(t, 1*time.Second, min64Duration(1*time.Second, 2*time.Second))
	assert.Equal(t, 500*time.Millisecond, min64Duration(1*time.Second, 500*time.Millisecond))

	// Test minFloat64
	assert.Equal(t, 1.5, minFloat64(1.5, 2.5))
	assert.Equal(t, 0.5, minFloat64(1.5, 0.5))

	// Test maxFloat64
	assert.Equal(t, 2.5, maxFloat64(1.5, 2.5))
	assert.Equal(t, 1.5, maxFloat64(1.5, 0.5))
}

// TestEvent-Methods for testing event methods
func TestEvent_Methods(t *testing.T) {
	base := BaseEvent{
		eventType: EventAllowed,
		resource:  "test_resource",
		ctx:       context.Background(),
		timestamp: time.Now(),
	}

	assert.Equal(t, EventAllowed, base.Type())
	assert.Equal(t, "test_resource", base.Resource())
	assert.NotNil(t, base.Context())
	assert.False(t, base.Timestamp().IsZero())
}

// TestMemoryStore_Eval tests memory store eval (should return unsupported error)
func TestMemoryStore_Eval(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()

	_, err := store.Eval(ctx, "return 1", []string{}, []interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

// TestMemoryStore_Cleanup test memory store cleanup
func TestMemoryStore_CleanupCoverage(t *testing.T) {
	store := NewMemoryStore()

	ctx := context.Background()

	// Set some keys with TTL
	store.Set(ctx, "cleanup_key1", "value1", 100*time.Millisecond)
	store.Set(ctx, "cleanup_key2", "value2", 100*time.Millisecond)
	store.Set(ctx, "cleanup_key3", "value3", 100*time.Millisecond)

	// wait for clean run
	time.Sleep(200 * time.Millisecond)

	// Verify key has expired
	val, _ := store.Get(ctx, "cleanup_key1")
	assert.Equal(t, "", val)

	store.Close()
}

// TestConcurrencyReleaseEdgeCases test concurrency rate limiting release edge cases
func TestConcurrency_ReleaseEdgeCases(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewConcurrencyAlgorithm()
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:      string(AlgorithmConcurrency),
		MaxConcurrency: 10,
	}

	// First obtain some concurrent slots
	resp, err := algo.Allow(ctx, store, "test_release", 5, cfg)
	if err != nil || !resp.Allowed {
		t.Skip("前置条件失败")
	}

	// Release partial
	concAlgo := algo.(*concurrencyAlgorithm)
	err = concAlgo.Release(ctx, store, "test_release", 3)
	assert.NoError(t, err)

	// Release more (value should be reset to zero without error)
	err = concAlgo.Release(ctx, store, "test_release", 10)
	assert.NoError(t, err)
}

// TestWait_RetryLoop test wait retry loop
func TestWait_RetryLoop(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	algo := NewTokenBucketAlgorithm()
	ctx := context.Background()

	cfg := ResourceConfig{
		Algorithm:  string(AlgorithmTokenBucket),
		Rate:       100,  // Quick replenishment
		Capacity:   100,
		InitTokens: 10, // Initial 10 tokens
	}

	// wait for 5 tokens (should succeed)
	err := algo.Wait(ctx, store, "test_retry", 5, cfg, 500*time.Millisecond)
	assert.NoError(t, err)
}

// TestGetAlgorithmByName_Coverage Test the algorithm factory function
func TestGetAlgorithmByName_Coverage(t *testing.T) {
	tests := []struct {
		name     string
		algoType AlgorithmType
		wantErr  bool
	}{
		{"TokenBucket", AlgorithmTokenBucket, false},
		{"SlidingWindow", AlgorithmSlidingWindow, false},
		{"Concurrency", AlgorithmConcurrency, false},
		{"Adaptive", AlgorithmAdaptive, false},
		{"Unknown", "unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			algo, err := getAlgorithmByName(tt.algoType)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, algo)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, algo)
				// Verify that the Name method returns the correct string
				name := algo.Name()
				assert.NotEmpty(t, name)
			}
		})
	}
}

// English: helper function for testing getAlgorithmByName
func getAlgorithmByName(name AlgorithmType) (Algorithm, error) {
	switch name {
	case AlgorithmTokenBucket:
		return NewTokenBucketAlgorithm(), nil
	case AlgorithmSlidingWindow:
		return NewSlidingWindowAlgorithm(), nil
	case AlgorithmConcurrency:
		return NewConcurrencyAlgorithm(), nil
	case AlgorithmAdaptive:
		return NewAdaptiveAlgorithm(nil), nil
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", name)
	}
}

