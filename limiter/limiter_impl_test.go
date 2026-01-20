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

	// should allow directly when disabled
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

	// The first 10 requests should pass
	for i := 0; i < 10; i++ {
		allowed, err := mgr.Allow(ctx, "test")
		require.NoError(t, err)
		assert.True(t, allowed, "第%d个请求应该通过", i+1)
	}

	// The 11th request should be rejected
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

	// Get 5 tokens at once
	allowed, err := mgr.AllowN(ctx, "test", 5)
	require.NoError(t, err)
	assert.True(t, allowed)

	// Retrieve another 10 tokens
	allowed, err = mgr.AllowN(ctx, "test", 10)
	require.NoError(t, err)
	assert.True(t, allowed)

	// Getting another 10 tokens should fail (only 5 left)
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

	// Consume all tokens
	for i := 0; i < 5; i++ {
		mgr.Allow(ctx, "test")
	}

	// The wait should succeed after a period of time
	start := time.Now()
	err = mgr.Wait(ctx, "test")
	elapsed := time.Since(start)

	require.NoError(t, err)
	// Should wait at least 50ms (generate 1 token)
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

	// Consume some tokens
	mgr.AllowN(ctx, "test", 5)

	// Get metric
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

	// Consume all tokens
	for i := 0; i < 10; i++ {
		mgr.Allow(ctx, "test")
	}

	// The next request should be rejected
	allowed, err := mgr.Allow(ctx, "test")
	require.NoError(t, err)
	assert.False(t, allowed)

	// reset
	mgr.Reset("test")

	// Should revert to full bucket state after reset
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

	// Resource 1 consumes all tokens
	for i := 0; i < 5; i++ {
		mgr.Allow(ctx, "resource1")
	}

	// Resource 1 should reject the next request
	allowed, err := mgr.Allow(ctx, "resource1")
	require.NoError(t, err)
	assert.False(t, allowed)

	// Resource 2 should be independent and unaffected
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

	// api1 uses specific configuration (100 tokens)
	for i := 0; i < 100; i++ {
		allowed, err := mgr.Allow(ctx, "api1")
		require.NoError(t, err)
		assert.True(t, allowed, "第%d个请求应该通过", i+1)
	}

	// The 101st request should be rejected
	allowed, err := mgr.Allow(ctx, "api1")
	require.NoError(t, err)
	assert.False(t, allowed)

	// api2 uses default configuration (200 tokens)
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

	// Subscribe to event
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

	// Trigger some events
	for i := 0; i < 7; i++ {
		mgr.Allow(ctx, "test")
		time.Sleep(10 * time.Millisecond) // Allow time for event handling
	}

	// wait for event handling
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

