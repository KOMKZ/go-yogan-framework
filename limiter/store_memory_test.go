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

	// 测试设置和获取
	err := store.Set(ctx, "key1", "value1", 0)
	require.NoError(t, err)

	val, err := store.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, "value1", val)

	// 测试键不存在
	_, err = store.Get(ctx, "nonexistent")
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

func TestMemoryStore_SetWithTTL(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()

	// 设置带TTL的键
	err := store.Set(ctx, "key1", "value1", 100*time.Millisecond)
	require.NoError(t, err)

	// 立即读取应该成功
	val, err := store.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, "value1", val)

	// 等待过期
	time.Sleep(150 * time.Millisecond)

	// 读取应该失败
	_, err = store.Get(ctx, "key1")
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

func TestMemoryStore_GetSetInt64(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()

	// 设置整数值
	err := store.SetInt64(ctx, "counter", 42, 0)
	require.NoError(t, err)

	// 获取整数值
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

	// 设置键
	err := store.Set(ctx, "key1", "value1", 0)
	require.NoError(t, err)

	// 设置过期时间
	err = store.Expire(ctx, "key1", 100*time.Millisecond)
	require.NoError(t, err)

	// 立即读取应该成功
	val, err := store.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, "value1", val)

	// 等待过期
	time.Sleep(150 * time.Millisecond)

	// 读取应该失败
	_, err = store.Get(ctx, "key1")
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

func TestMemoryStore_TTL(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()

	// 设置带TTL的键
	err := store.Set(ctx, "key1", "value1", 1*time.Second)
	require.NoError(t, err)

	// 获取TTL
	ttl, err := store.TTL(ctx, "key1")
	require.NoError(t, err)
	assert.True(t, ttl > 900*time.Millisecond && ttl <= 1*time.Second)

	// 设置永不过期的键
	err = store.Set(ctx, "key2", "value2", 0)
	require.NoError(t, err)

	ttl, err = store.TTL(ctx, "key2")
	require.NoError(t, err)
	assert.Equal(t, time.Duration(-1), ttl)

	// 不存在的键
	_, err = store.TTL(ctx, "nonexistent")
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

func TestMemoryStore_Del(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()

	// 设置多个键
	store.Set(ctx, "key1", "value1", 0)
	store.Set(ctx, "key2", "value2", 0)
	store.Set(ctx, "key3", "value3", 0)

	// 删除多个键
	err := store.Del(ctx, "key1", "key2")
	require.NoError(t, err)

	// key1和key2应该不存在
	_, err = store.Get(ctx, "key1")
	assert.ErrorIs(t, err, ErrKeyNotFound)

	_, err = store.Get(ctx, "key2")
	assert.ErrorIs(t, err, ErrKeyNotFound)

	// key3应该存在
	val, err := store.Get(ctx, "key3")
	require.NoError(t, err)
	assert.Equal(t, "value3", val)
}

func TestMemoryStore_Exists(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()

	// 键不存在
	exists, err := store.Exists(ctx, "key1")
	require.NoError(t, err)
	assert.False(t, exists)

	// 设置键
	store.Set(ctx, "key1", "value1", 0)

	// 键存在
	exists, err = store.Exists(ctx, "key1")
	require.NoError(t, err)
	assert.True(t, exists)

	// 过期的键不存在
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

	// 添加元素
	err := store.ZAdd(ctx, "zset1", 1.0, "member1")
	require.NoError(t, err)

	err = store.ZAdd(ctx, "zset1", 2.0, "member2")
	require.NoError(t, err)

	err = store.ZAdd(ctx, "zset1", 3.0, "member3")
	require.NoError(t, err)

	// 统计元素
	count, err := store.ZCount(ctx, "zset1", 1.0, 2.0)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	count, err = store.ZCount(ctx, "zset1", 0.0, 10.0)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	// 删除元素
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

	// 并发递增
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

	// 等待所有goroutine完成
	for i := 0; i < goroutines; i++ {
		<-done
	}

	// 验证结果
	val, err := store.GetInt64(ctx, "counter")
	require.NoError(t, err)
	assert.Equal(t, int64(goroutines*increments), val)
}

func TestMemoryStore_Close(t *testing.T) {
	store := NewMemoryStore()

	ctx := context.Background()

	// 设置一些数据
	store.Set(ctx, "key1", "value1", 0)

	// 关闭
	err := store.Close()
	require.NoError(t, err)

	// 关闭后操作应该失败
	err = store.Set(ctx, "key2", "value2", 0)
	assert.Error(t, err)

	_, err = store.Get(ctx, "key1")
	assert.Error(t, err)
}

func TestMemoryStore_Cleanup(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()

	// 设置多个带过期时间的键
	for i := 0; i < 10; i++ {
		key := "key" + string(rune(i))
		store.Set(ctx, key, "value", 50*time.Millisecond)
	}

	// 等待过期和清理
	time.Sleep(200 * time.Millisecond)

	// 所有键应该已被清理
	for i := 0; i < 10; i++ {
		key := "key" + string(rune(i))
		_, err := store.Get(ctx, key)
		assert.ErrorIs(t, err, ErrKeyNotFound)
	}
}

