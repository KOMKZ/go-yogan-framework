package limiter

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupMiniRedis creates a miniredis instance for testing
func setupMiniRedis(t *testing.T) (*miniredis.Miniredis, Store) {
	// Create mini redis server
	mr := miniredis.RunT(t)

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// Create Redis Store
	store := NewRedisStore(client, "limiter:")

	return mr, store
}

func TestRedisStore_SetGet(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Test Set and Get
	err := store.Set(ctx, "key1", "value1", 0)
	require.NoError(t, err)

	val, err := store.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, "value1", val)
}

func TestRedisStore_SetWithTTL(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Set key with TTL
	err := store.Set(ctx, "key_ttl", "value_ttl", 1*time.Second)
	require.NoError(t, err)

	// immediately get the existing one
	val, err := store.Get(ctx, "key_ttl")
	require.NoError(t, err)
	assert.Equal(t, "value_ttl", val)

	// fast forward time
	mr.FastForward(2 * time.Second)

	// Should not exist after expiration
	val, err = store.Get(ctx, "key_ttl")
	require.NoError(t, err)
	assert.Equal(t, "", val)
}

func TestRedisStore_GetNonExistent(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Get non-existent key
	val, err := store.Get(ctx, "non_existent")
	require.NoError(t, err)
	assert.Equal(t, "", val)
}

func TestRedisStore_Del(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Set key
	err := store.Set(ctx, "key_del", "value", 0)
	require.NoError(t, err)

	// Delete key
	err = store.Del(ctx, "key_del")
	require.NoError(t, err)

	// Verify deletion
	val, err := store.Get(ctx, "key_del")
	require.NoError(t, err)
	assert.Equal(t, "", val)
}

func TestRedisStore_DelMultiple(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Set multiple keys
	store.Set(ctx, "key1", "value1", 0)
	store.Set(ctx, "key2", "value2", 0)
	store.Set(ctx, "key3", "value3", 0)

	// Delete multiple keys
	err := store.Del(ctx, "key1", "key2", "key3")
	require.NoError(t, err)

	// Validation has been deleted
	val, _ := store.Get(ctx, "key1")
	assert.Equal(t, "", val)
	val, _ = store.Get(ctx, "key2")
	assert.Equal(t, "", val)
	val, _ = store.Get(ctx, "key3")
	assert.Equal(t, "", val)
}

func TestRedisStore_DelEmpty(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Deleting an empty list should not result in an error
	err := store.Del(ctx)
	require.NoError(t, err)
}

func TestRedisStore_Exists(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Check for non-existent keys
	exists, err := store.Exists(ctx, "key_exists")
	require.NoError(t, err)
	assert.False(t, exists)

	// Set key
	err = store.Set(ctx, "key_exists", "value", 0)
	require.NoError(t, err)

	// Check existing keys
	exists, err = store.Exists(ctx, "key_exists")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestRedisStore_GetSetInt64(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Set integer value
	err := store.SetInt64(ctx, "int_key", 123, 0)
	require.NoError(t, err)

	// Get integer value
	val, err := store.GetInt64(ctx, "int_key")
	require.NoError(t, err)
	assert.Equal(t, int64(123), val)
}

func TestRedisStore_GetInt64NonExistent(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Return 0 for non-existent integer values
	val, err := store.GetInt64(ctx, "non_existent_int")
	require.NoError(t, err)
	assert.Equal(t, int64(0), val)
}

func TestRedisStore_SetInt64WithTTL(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Set integer value with TTL
	err := store.SetInt64(ctx, "int_ttl", 456, 1*time.Second)
	require.NoError(t, err)

	// immediate acquisition
	val, err := store.GetInt64(ctx, "int_ttl")
	require.NoError(t, err)
	assert.Equal(t, int64(456), val)

	// fast forward time
	mr.FastForward(2 * time.Second)

	// After expiration, 0 should be returned
	val, err = store.GetInt64(ctx, "int_ttl")
	require.NoError(t, err)
	assert.Equal(t, int64(0), val)
}

func TestRedisStore_Incr(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// First increment (key does not exist)
	val, err := store.Incr(ctx, "counter")
	require.NoError(t, err)
	assert.Equal(t, int64(1), val)

	// second increment
	val, err = store.Incr(ctx, "counter")
	require.NoError(t, err)
	assert.Equal(t, int64(2), val)

	// Third increment
English: Third increment
English: Third increment
	val, err = store.Incr(ctx, "counter")
	require.NoError(t, err)
	assert.Equal(t, int64(3), val)
}

func TestRedisStore_IncrBy(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Initial increment
	val, err := store.IncrBy(ctx, "counter", 5)
	require.NoError(t, err)
	assert.Equal(t, int64(5), val)

	// increment again
	val, err = store.IncrBy(ctx, "counter", 3)
	require.NoError(t, err)
	assert.Equal(t, int64(8), val)

	// negative increment
	val, err = store.IncrBy(ctx, "counter", -2)
	require.NoError(t, err)
	assert.Equal(t, int64(6), val)
}

func TestRedisStore_Decr(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Initialize the initial values
	store.SetInt64(ctx, "counter_decr", 10, 0)

	// decrement
	val, err := store.Decr(ctx, "counter_decr")
	require.NoError(t, err)
	assert.Equal(t, int64(9), val)

	// decrement again
	val, err = store.Decr(ctx, "counter_decr")
	require.NoError(t, err)
	assert.Equal(t, int64(8), val)
}

func TestRedisStore_DecrBy(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Initialize values first
	err := store.SetInt64(ctx, "counter_decrby", 100, 0)
	require.NoError(t, err)

	// reduce quantity
	val, err := store.DecrBy(ctx, "counter_decrby", 30)
	require.NoError(t, err)
	assert.Equal(t, int64(70), val)

	// reduce again
	val, err = store.DecrBy(ctx, "counter_decrby", 20)
	require.NoError(t, err)
	assert.Equal(t, int64(50), val)
}

func TestRedisStore_TTL(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Non-existent key
	ttl, err := store.TTL(ctx, "no_ttl_key")
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), ttl)

	// Set key with TTL
	err = store.Set(ctx, "ttl_key", "value", 10*time.Second)
	require.NoError(t, err)

	// Get TTL
	ttl, err = store.TTL(ctx, "ttl_key")
	require.NoError(t, err)
	assert.True(t, ttl > 0 && ttl <= 10*time.Second)

	// Set key with no TTL
	err = store.Set(ctx, "no_expire_key", "value", 0)
	require.NoError(t, err)

	ttl, err = store.TTL(ctx, "no_expire_key")
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), ttl)
}

func TestRedisStore_Expire(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Set key (no expiration)
	err := store.Set(ctx, "expire_key", "value", 0)
	require.NoError(t, err)

	// Set expiration time
	err = store.Expire(ctx, "expire_key", 5*time.Second)
	require.NoError(t, err)

	// Validate TTL
	ttl, err := store.TTL(ctx, "expire_key")
	require.NoError(t, err)
	assert.True(t, ttl > 0 && ttl <= 5*time.Second)
}

func TestRedisStore_ZAdd(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Add member
	err := store.ZAdd(ctx, "zset", 1.0, "member1")
	require.NoError(t, err)

	err = store.ZAdd(ctx, "zset", 2.0, "member2")
	require.NoError(t, err)

	err = store.ZAdd(ctx, "zset", 3.0, "member3")
	require.NoError(t, err)

	// Verify member count (using ZCount to tally all)
	count, err := store.ZCount(ctx, "zset", 0, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestRedisStore_ZRemRangeByScore(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Add member
	store.ZAdd(ctx, "zset_rem", 1.0, "m1")
	store.ZAdd(ctx, "zset_rem", 2.0, "m2")
	store.ZAdd(ctx, "zset_rem", 3.0, "m3")
	store.ZAdd(ctx, "zset_rem", 4.0, "m4")
	store.ZAdd(ctx, "zset_rem", 5.0, "m5")

	// Delete members with scores 2.0 to 4.0
	err := store.ZRemRangeByScore(ctx, "zset_rem", 2.0, 4.0)
	require.NoError(t, err)

	// Verify remaining member count (should be m1, m5)
	count, err := store.ZCount(ctx, "zset_rem", 0, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	// Verify that the remainder consists of m1 and m5
	count1, _ := store.ZCount(ctx, "zset_rem", 1.0, 1.0)
	assert.Equal(t, int64(1), count1) // m1 exists

	count5, _ := store.ZCount(ctx, "zset_rem", 5.0, 5.0)
	assert.Equal(t, int64(1), count5) // M5 exists

	count2, _ := store.ZCount(ctx, "zset_rem", 2.0, 2.0)
	assert.Equal(t, int64(0), count2) // m2 does not exist
}

func TestRedisStore_ZCount(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Add member
	store.ZAdd(ctx, "zset_count", 1.0, "m1")
	store.ZAdd(ctx, "zset_count", 2.5, "m2")
	store.ZAdd(ctx, "zset_count", 3.5, "m3")
	store.ZAdd(ctx, "zset_count", 5.0, "m4")

	// Count members with scores between 2.0 and 4.0
	count, err := store.ZCount(ctx, "zset_count", 2.0, 4.0)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count) // m2(2.5) and m3(3.5)

	// Count all members
	count, err = store.ZCount(ctx, "zset_count", 0, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(4), count)

	// Count empty ranges
	count, err = store.ZCount(ctx, "zset_count", 10, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestRedisStore_ZCountEmptySet(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// empty set
	count, err := store.ZCount(ctx, "empty_zset", 0, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestRedisStore_Eval(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// A simple Lua script: set key and return value
	script := `
		redis.call('SET', KEYS[1], ARGV[1])
		return redis.call('GET', KEYS[1])
	`

	result, err := store.Eval(ctx, script, []string{"test_key"}, []interface{}{"test_value"})
	require.NoError(t, err)
	assert.Equal(t, "test_value", result)

	// Validate key is set
	val, err := store.Get(ctx, "test_key")
	require.NoError(t, err)
	assert.Equal(t, "test_value", val)
}

func TestRedisStore_EvalIncrement(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Lua script: atomic increment and return
	script := `
		local current = redis.call('GET', KEYS[1])
		if not current then
			current = 0
		end
		local new = tonumber(current) + tonumber(ARGV[1])
		redis.call('SET', KEYS[1], new)
		return new
	`

	// First incremental update
	result, err := store.Eval(ctx, script, []string{"lua_counter"}, []interface{}{10})
	require.NoError(t, err)
	assert.Equal(t, int64(10), result)

	// Second incremental update
	result, err = store.Eval(ctx, script, []string{"lua_counter"}, []interface{}{5})
	require.NoError(t, err)
	assert.Equal(t, int64(15), result)
}

func TestRedisStore_Close(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	// Closing should not result in an error (as it is managed externally)
	err := store.Close()
	assert.NoError(t, err)
}

func TestRedisStore_ZSet_Integration(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Simulate the usage scenario of the sliding window
	now := time.Now()

	// Add multiple requests (using timestamp as score)
	for i := 0; i < 10; i++ {
		timestamp := now.Add(time.Duration(i*100) * time.Millisecond)
		score := float64(timestamp.UnixNano())
		member := strconv.FormatInt(timestamp.UnixNano(), 10)
		err := store.ZAdd(ctx, "requests", score, member)
		require.NoError(t, err)
	}

	// Validate total count
	total, err := store.ZCount(ctx, "requests", 0, float64(now.Add(2*time.Second).UnixNano()))
	require.NoError(t, err)
	assert.Equal(t, int64(10), total)

	// Calculate the number of requests within the window (latest 600ms)
	windowStart := now.Add(400 * time.Millisecond)
	minScore := float64(windowStart.UnixNano())
	maxScore := float64(now.Add(1 * time.Second).UnixNano())
	count, err := store.ZCount(ctx, "requests", minScore, maxScore)
	require.NoError(t, err)
	assert.True(t, count >= 5 && count <= 7) // There should be 5-7 requests in the window

	// Delete expired requests (earlier than windowStart)
	err = store.ZRemRangeByScore(ctx, "requests", 0, minScore-1)
	require.NoError(t, err)

	// Verify remaining quantity
	remaining, err := store.ZCount(ctx, "requests", 0, float64(now.Add(2*time.Second).UnixNano()))
	require.NoError(t, err)
	assert.True(t, remaining >= 5 && remaining <= 7)
}

func TestRedisStore_ConcurrentOperations(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// concurrent incremental operation
	concurrency := 10
	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				store.IncrBy(ctx, "concurrent_counter", 1)
			}
			done <- true
		}()
	}

	// wait for all goroutines to finish
	for i := 0; i < concurrency; i++ {
		<-done
	}

	// Verify final value
	val, err := store.GetInt64(ctx, "concurrent_counter")
	require.NoError(t, err)
	assert.Equal(t, int64(concurrency*10), val)
}

func TestRedisStore_PipelineScenario(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Test complex scenarios: incremental + set expiration
	val, err := store.IncrBy(ctx, "pipeline_test", 10)
	require.NoError(t, err)
	assert.Equal(t, int64(10), val)

	// Set expiration time
	err = store.Expire(ctx, "pipeline_test", 5*time.Second)
	require.NoError(t, err)

	// Validate value and TTL
	storedVal, err := store.GetInt64(ctx, "pipeline_test")
	require.NoError(t, err)
	assert.Equal(t, int64(10), storedVal)

	ttl, err := store.TTL(ctx, "pipeline_test")
	require.NoError(t, err)
	assert.True(t, ttl > 0 && ttl <= 5*time.Second)
}

func TestRedisStore_MultipleOperations(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Combination test: string, integer, ZSet

	// String operations
	store.Set(ctx, "str1", "value1", 0)
	store.Set(ctx, "str2", "value2", 0)

	val1, _ := store.Get(ctx, "str1")
	assert.Equal(t, "value1", val1)

	// 2. Integer operations
	store.SetInt64(ctx, "int1", 100, 0)
	store.IncrBy(ctx, "int1", 50)

	intVal, _ := store.GetInt64(ctx, "int1")
	assert.Equal(t, int64(150), intVal)

	// 3. ZSet operations
	store.ZAdd(ctx, "zset1", 1.0, "a")
	store.ZAdd(ctx, "zset1", 2.0, "b")
	store.ZAdd(ctx, "zset1", 3.0, "c")

	count, _ := store.ZCount(ctx, "zset1", 0, 10)
	assert.Equal(t, int64(3), count)

	// 4. Delete operation
	store.Del(ctx, "str1", "int1")

	exists1, _ := store.Exists(ctx, "str1")
	assert.False(t, exists1)

	exists2, _ := store.Exists(ctx, "int1")
	assert.False(t, exists2)

	// str2 and zset1 should still exist
	exists3, _ := store.Exists(ctx, "str2")
	assert.True(t, exists3)
}
