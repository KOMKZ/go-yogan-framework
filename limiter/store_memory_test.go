package limiter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryStore_SetGet(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()

	// Test set and get operations
	err := store.Set(ctx, "key1", "value1", 0)
	require.NoError(t, err)

	val, err := store.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, "value1", val)

	// Test key does not exist
	_, err = store.Get(ctx, "nonexistent")
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

func TestMemoryStore_SetWithTTL(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()

	// Set key with TTL
	err := store.Set(ctx, "key1", "value1", 100*time.Millisecond)
	require.NoError(t, err)

	// Immediate read should succeed
	val, err := store.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, "value1", val)

	// wait for expiration
	time.Sleep(150 * time.Millisecond)

	// reading should fail
	_, err = store.Get(ctx, "key1")
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

func TestMemoryStore_GetSetInt64(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()

	// Set integer value
	err := store.SetInt64(ctx, "counter", 42, 0)
	require.NoError(t, err)

	// Get integer value
	val, err := store.GetInt64(ctx, "counter")
	require.NoError(t, err)
	assert.Equal(t, int64(42), val)
}

func TestMemoryStore_IncrDecr(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()

	// Incr from 0
	val, err := store.Incr(ctx, "counter")
	require.NoError(t, err)
	assert.Equal(t, int64(1), val)

	// IncrBy
	val, err = store.IncrBy(ctx, "counter", 5)
	require.NoError(t, err)
	assert.Equal(t, int64(6), val)

	// Decr
	val, err = store.Decr(ctx, "counter")
	require.NoError(t, err)
	assert.Equal(t, int64(5), val)

	// DecrBy
	val, err = store.DecrBy(ctx, "counter", 3)
	require.NoError(t, err)
	assert.Equal(t, int64(2), val)
}

func TestMemoryStore_Expire(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()

	// Set key
	err := store.Set(ctx, "key1", "value1", 0)
	require.NoError(t, err)

	// Set expiration time
	err = store.Expire(ctx, "key1", 100*time.Millisecond)
	require.NoError(t, err)

	// Immediate read should succeed
	val, err := store.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, "value1", val)

	// wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Read should fail
	_, err = store.Get(ctx, "key1")
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

func TestMemoryStore_TTL(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()

	// Set key with TTL
	err := store.Set(ctx, "key1", "value1", 1*time.Second)
	require.NoError(t, err)

	// Get TTL
	ttl, err := store.TTL(ctx, "key1")
	require.NoError(t, err)
	assert.True(t, ttl > 900*time.Millisecond && ttl <= 1*time.Second)

	// Set an everlasting key
	err = store.Set(ctx, "key2", "value2", 0)
	require.NoError(t, err)

	ttl, err = store.TTL(ctx, "key2")
	require.NoError(t, err)
	assert.Equal(t, time.Duration(-1), ttl)

	// Nonexistent key
	_, err = store.TTL(ctx, "nonexistent")
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

func TestMemoryStore_Del(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()

	// Set multiple keys
	store.Set(ctx, "key1", "value1", 0)
	store.Set(ctx, "key2", "value2", 0)
	store.Set(ctx, "key3", "value3", 0)

	// Delete multiple keys
	err := store.Del(ctx, "key1", "key2")
	require.NoError(t, err)

	// key1 and key2 should not exist
	_, err = store.Get(ctx, "key1")
	assert.ErrorIs(t, err, ErrKeyNotFound)

	_, err = store.Get(ctx, "key2")
	assert.ErrorIs(t, err, ErrKeyNotFound)

	// key3 should exist
	val, err := store.Get(ctx, "key3")
	require.NoError(t, err)
	assert.Equal(t, "value3", val)
}

func TestMemoryStore_Exists(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()

	// Key does not exist
	exists, err := store.Exists(ctx, "key1")
	require.NoError(t, err)
	assert.False(t, exists)

	// Set key
	store.Set(ctx, "key1", "value1", 0)

	// Key exists
	exists, err = store.Exists(ctx, "key1")
	require.NoError(t, err)
	assert.True(t, exists)

	// Expired keys do not exist
	store.Set(ctx, "key2", "value2", 10*time.Millisecond)
	time.Sleep(20 * time.Millisecond)

	exists, err = store.Exists(ctx, "key2")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestMemoryStore_ZSet(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()

	// Add element
	err := store.ZAdd(ctx, "zset1", 1.0, "member1")
	require.NoError(t, err)

	err = store.ZAdd(ctx, "zset1", 2.0, "member2")
	require.NoError(t, err)

	err = store.ZAdd(ctx, "zset1", 3.0, "member3")
	require.NoError(t, err)

	// Count elements
	count, err := store.ZCount(ctx, "zset1", 1.0, 2.0)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	count, err = store.ZCount(ctx, "zset1", 0.0, 10.0)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	// Remove element
	err = store.ZRemRangeByScore(ctx, "zset1", 1.0, 2.0)
	require.NoError(t, err)

	count, err = store.ZCount(ctx, "zset1", 0.0, 10.0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestMemoryStore_Concurrent(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()

	// Concurrent increment
	const goroutines = 100
	const increments = 100

	done := make(chan bool)

	for i := 0; i < goroutines; i++ {
		go func() {
			for j := 0; j < increments; j++ {
				store.Incr(ctx, "counter")
			}
			done <- true
		}()
	}

	// wait for all goroutines to finish
	for i := 0; i < goroutines; i++ {
		<-done
	}

	// Verify the result
	val, err := store.GetInt64(ctx, "counter")
	require.NoError(t, err)
	assert.Equal(t, int64(goroutines*increments), val)
}

func TestMemoryStore_Close(t *testing.T) {
	store := NewMemoryStore()

	ctx := context.Background()

	// Set some data
	store.Set(ctx, "key1", "value1", 0)

	// Close
	err := store.Close()
	require.NoError(t, err)

	// The operation should fail after closure
	err = store.Set(ctx, "key2", "value2", 0)
	assert.Error(t, err)

	_, err = store.Get(ctx, "key1")
	assert.Error(t, err)
}

func TestMemoryStore_Cleanup(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()

	// Set multiple keys with expiration times
	for i := 0; i < 10; i++ {
		key := "key" + string(rune(i))
		store.Set(ctx, key, "value", 50*time.Millisecond)
	}

	// wait for expiration and cleanup
	time.Sleep(200 * time.Millisecond)

	// All keys should have been cleaned
	for i := 0; i < 10; i++ {
		key := "key" + string(rune(i))
		_, err := store.Get(ctx, key)
		assert.ErrorIs(t, err, ErrKeyNotFound)
	}
}

