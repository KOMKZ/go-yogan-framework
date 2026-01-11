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

// setupMiniRedis 创建一个 miniredis 实例用于测试
func setupMiniRedis(t *testing.T) (*miniredis.Miniredis, Store) {
	// 创建 miniredis 服务器
	mr := miniredis.RunT(t)

	// 创建 Redis 客户端
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// 创建 Redis Store
	store := NewRedisStore(client, "limiter:")

	return mr, store
}

func TestRedisStore_SetGet(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// 测试 Set 和 Get
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

	// 设置带 TTL 的键
	err := store.Set(ctx, "key_ttl", "value_ttl", 1*time.Second)
	require.NoError(t, err)

	// 立即获取应该存在
	val, err := store.Get(ctx, "key_ttl")
	require.NoError(t, err)
	assert.Equal(t, "value_ttl", val)

	// 快进时间
	mr.FastForward(2 * time.Second)

	// 过期后应该不存在
	val, err = store.Get(ctx, "key_ttl")
	require.NoError(t, err)
	assert.Equal(t, "", val)
}

func TestRedisStore_GetNonExistent(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// 获取不存在的键
	val, err := store.Get(ctx, "non_existent")
	require.NoError(t, err)
	assert.Equal(t, "", val)
}

func TestRedisStore_Del(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// 设置键
	err := store.Set(ctx, "key_del", "value", 0)
	require.NoError(t, err)

	// 删除键
	err = store.Del(ctx, "key_del")
	require.NoError(t, err)

	// 验证已删除
	val, err := store.Get(ctx, "key_del")
	require.NoError(t, err)
	assert.Equal(t, "", val)
}

func TestRedisStore_DelMultiple(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// 设置多个键
	store.Set(ctx, "key1", "value1", 0)
	store.Set(ctx, "key2", "value2", 0)
	store.Set(ctx, "key3", "value3", 0)

	// 删除多个键
	err := store.Del(ctx, "key1", "key2", "key3")
	require.NoError(t, err)

	// 验证都已删除
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

	// 删除空列表应该不报错
	err := store.Del(ctx)
	require.NoError(t, err)
}

func TestRedisStore_Exists(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// 检查不存在的键
	exists, err := store.Exists(ctx, "key_exists")
	require.NoError(t, err)
	assert.False(t, exists)

	// 设置键
	err = store.Set(ctx, "key_exists", "value", 0)
	require.NoError(t, err)

	// 检查存在的键
	exists, err = store.Exists(ctx, "key_exists")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestRedisStore_GetSetInt64(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// 设置整数值
	err := store.SetInt64(ctx, "int_key", 123, 0)
	require.NoError(t, err)

	// 获取整数值
	val, err := store.GetInt64(ctx, "int_key")
	require.NoError(t, err)
	assert.Equal(t, int64(123), val)
}

func TestRedisStore_GetInt64NonExistent(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// 获取不存在的整数值应该返回 0
	val, err := store.GetInt64(ctx, "non_existent_int")
	require.NoError(t, err)
	assert.Equal(t, int64(0), val)
}

func TestRedisStore_SetInt64WithTTL(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// 设置带 TTL 的整数值
	err := store.SetInt64(ctx, "int_ttl", 456, 1*time.Second)
	require.NoError(t, err)

	// 立即获取
	val, err := store.GetInt64(ctx, "int_ttl")
	require.NoError(t, err)
	assert.Equal(t, int64(456), val)

	// 快进时间
	mr.FastForward(2 * time.Second)

	// 过期后应该返回 0
	val, err = store.GetInt64(ctx, "int_ttl")
	require.NoError(t, err)
	assert.Equal(t, int64(0), val)
}

func TestRedisStore_Incr(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// 第一次递增（key 不存在）
	val, err := store.Incr(ctx, "counter")
	require.NoError(t, err)
	assert.Equal(t, int64(1), val)

	// 第二次递增
	val, err = store.Incr(ctx, "counter")
	require.NoError(t, err)
	assert.Equal(t, int64(2), val)

	// 第三次递增
	val, err = store.Incr(ctx, "counter")
	require.NoError(t, err)
	assert.Equal(t, int64(3), val)
}

func TestRedisStore_IncrBy(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// 初始增量
	val, err := store.IncrBy(ctx, "counter", 5)
	require.NoError(t, err)
	assert.Equal(t, int64(5), val)

	// 再次增量
	val, err = store.IncrBy(ctx, "counter", 3)
	require.NoError(t, err)
	assert.Equal(t, int64(8), val)

	// 负数增量
	val, err = store.IncrBy(ctx, "counter", -2)
	require.NoError(t, err)
	assert.Equal(t, int64(6), val)
}

func TestRedisStore_Decr(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// 先设置初始值
	store.SetInt64(ctx, "counter_decr", 10, 0)

	// 递减
	val, err := store.Decr(ctx, "counter_decr")
	require.NoError(t, err)
	assert.Equal(t, int64(9), val)

	// 再次递减
	val, err = store.Decr(ctx, "counter_decr")
	require.NoError(t, err)
	assert.Equal(t, int64(8), val)
}

func TestRedisStore_DecrBy(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// 先设置初始值
	err := store.SetInt64(ctx, "counter_decrby", 100, 0)
	require.NoError(t, err)

	// 减量
	val, err := store.DecrBy(ctx, "counter_decrby", 30)
	require.NoError(t, err)
	assert.Equal(t, int64(70), val)

	// 再次减量
	val, err = store.DecrBy(ctx, "counter_decrby", 20)
	require.NoError(t, err)
	assert.Equal(t, int64(50), val)
}

func TestRedisStore_TTL(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// 不存在的键
	ttl, err := store.TTL(ctx, "no_ttl_key")
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), ttl)

	// 设置带 TTL 的键
	err = store.Set(ctx, "ttl_key", "value", 10*time.Second)
	require.NoError(t, err)

	// 获取 TTL
	ttl, err = store.TTL(ctx, "ttl_key")
	require.NoError(t, err)
	assert.True(t, ttl > 0 && ttl <= 10*time.Second)

	// 设置无 TTL 的键
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

	// 设置键（无过期时间）
	err := store.Set(ctx, "expire_key", "value", 0)
	require.NoError(t, err)

	// 设置过期时间
	err = store.Expire(ctx, "expire_key", 5*time.Second)
	require.NoError(t, err)

	// 验证 TTL
	ttl, err := store.TTL(ctx, "expire_key")
	require.NoError(t, err)
	assert.True(t, ttl > 0 && ttl <= 5*time.Second)
}

func TestRedisStore_ZAdd(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// 添加成员
	err := store.ZAdd(ctx, "zset", 1.0, "member1")
	require.NoError(t, err)

	err = store.ZAdd(ctx, "zset", 2.0, "member2")
	require.NoError(t, err)

	err = store.ZAdd(ctx, "zset", 3.0, "member3")
	require.NoError(t, err)

	// 验证成员数量（使用 ZCount 统计所有）
	count, err := store.ZCount(ctx, "zset", 0, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestRedisStore_ZRemRangeByScore(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// 添加成员
	store.ZAdd(ctx, "zset_rem", 1.0, "m1")
	store.ZAdd(ctx, "zset_rem", 2.0, "m2")
	store.ZAdd(ctx, "zset_rem", 3.0, "m3")
	store.ZAdd(ctx, "zset_rem", 4.0, "m4")
	store.ZAdd(ctx, "zset_rem", 5.0, "m5")

	// 删除分数 2.0 到 4.0 的成员
	err := store.ZRemRangeByScore(ctx, "zset_rem", 2.0, 4.0)
	require.NoError(t, err)

	// 验证剩余成员数量（应该剩余 m1, m5）
	count, err := store.ZCount(ctx, "zset_rem", 0, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	// 验证剩余的是 m1 和 m5
	count1, _ := store.ZCount(ctx, "zset_rem", 1.0, 1.0)
	assert.Equal(t, int64(1), count1) // m1 存在

	count5, _ := store.ZCount(ctx, "zset_rem", 5.0, 5.0)
	assert.Equal(t, int64(1), count5) // m5 存在

	count2, _ := store.ZCount(ctx, "zset_rem", 2.0, 2.0)
	assert.Equal(t, int64(0), count2) // m2 不存在
}

func TestRedisStore_ZCount(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// 添加成员
	store.ZAdd(ctx, "zset_count", 1.0, "m1")
	store.ZAdd(ctx, "zset_count", 2.5, "m2")
	store.ZAdd(ctx, "zset_count", 3.5, "m3")
	store.ZAdd(ctx, "zset_count", 5.0, "m4")

	// 统计分数在 2.0 到 4.0 之间的成员
	count, err := store.ZCount(ctx, "zset_count", 2.0, 4.0)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count) // m2(2.5) 和 m3(3.5)

	// 统计所有成员
	count, err = store.ZCount(ctx, "zset_count", 0, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(4), count)

	// 统计空范围
	count, err = store.ZCount(ctx, "zset_count", 10, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestRedisStore_ZCountEmptySet(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// 空集合
	count, err := store.ZCount(ctx, "empty_zset", 0, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestRedisStore_Eval(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// 简单的 Lua 脚本：设置键并返回值
	script := `
		redis.call('SET', KEYS[1], ARGV[1])
		return redis.call('GET', KEYS[1])
	`

	result, err := store.Eval(ctx, script, []string{"test_key"}, []interface{}{"test_value"})
	require.NoError(t, err)
	assert.Equal(t, "test_value", result)

	// 验证键已设置
	val, err := store.Get(ctx, "test_key")
	require.NoError(t, err)
	assert.Equal(t, "test_value", val)
}

func TestRedisStore_EvalIncrement(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Lua 脚本：原子增量并返回
	script := `
		local current = redis.call('GET', KEYS[1])
		if not current then
			current = 0
		end
		local new = tonumber(current) + tonumber(ARGV[1])
		redis.call('SET', KEYS[1], new)
		return new
	`

	// 第一次增量
	result, err := store.Eval(ctx, script, []string{"lua_counter"}, []interface{}{10})
	require.NoError(t, err)
	assert.Equal(t, int64(10), result)

	// 第二次增量
	result, err = store.Eval(ctx, script, []string{"lua_counter"}, []interface{}{5})
	require.NoError(t, err)
	assert.Equal(t, int64(15), result)
}

func TestRedisStore_Close(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	// Close 应该不会报错（因为由外部管理）
	err := store.Close()
	assert.NoError(t, err)
}

func TestRedisStore_ZSet_Integration(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// 模拟滑动窗口的使用场景
	now := time.Now()

	// 添加多个请求（使用时间戳作为分数）
	for i := 0; i < 10; i++ {
		timestamp := now.Add(time.Duration(i*100) * time.Millisecond)
		score := float64(timestamp.UnixNano())
		member := strconv.FormatInt(timestamp.UnixNano(), 10)
		err := store.ZAdd(ctx, "requests", score, member)
		require.NoError(t, err)
	}

	// 验证总数
	total, err := store.ZCount(ctx, "requests", 0, float64(now.Add(2*time.Second).UnixNano()))
	require.NoError(t, err)
	assert.Equal(t, int64(10), total)

	// 计算窗口内的请求数（最近 600ms）
	windowStart := now.Add(400 * time.Millisecond)
	minScore := float64(windowStart.UnixNano())
	maxScore := float64(now.Add(1 * time.Second).UnixNano())
	count, err := store.ZCount(ctx, "requests", minScore, maxScore)
	require.NoError(t, err)
	assert.True(t, count >= 5 && count <= 7) // 应该有 5-7 个请求在窗口内

	// 删除过期的请求（早于 windowStart 的）
	err = store.ZRemRangeByScore(ctx, "requests", 0, minScore-1)
	require.NoError(t, err)

	// 验证剩余数量
	remaining, err := store.ZCount(ctx, "requests", 0, float64(now.Add(2*time.Second).UnixNano()))
	require.NoError(t, err)
	assert.True(t, remaining >= 5 && remaining <= 7)
}

func TestRedisStore_ConcurrentOperations(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// 并发增量操作
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

	// 等待所有 goroutine 完成
	for i := 0; i < concurrency; i++ {
		<-done
	}

	// 验证最终值
	val, err := store.GetInt64(ctx, "concurrent_counter")
	require.NoError(t, err)
	assert.Equal(t, int64(concurrency*10), val)
}

func TestRedisStore_PipelineScenario(t *testing.T) {
	mr, store := setupMiniRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// 测试复杂场景：增量 + 设置过期
	val, err := store.IncrBy(ctx, "pipeline_test", 10)
	require.NoError(t, err)
	assert.Equal(t, int64(10), val)

	// 设置过期时间
	err = store.Expire(ctx, "pipeline_test", 5*time.Second)
	require.NoError(t, err)

	// 验证值和 TTL
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

	// 组合测试：字符串、整数、ZSet

	// 1. 字符串操作
	store.Set(ctx, "str1", "value1", 0)
	store.Set(ctx, "str2", "value2", 0)

	val1, _ := store.Get(ctx, "str1")
	assert.Equal(t, "value1", val1)

	// 2. 整数操作
	store.SetInt64(ctx, "int1", 100, 0)
	store.IncrBy(ctx, "int1", 50)

	intVal, _ := store.GetInt64(ctx, "int1")
	assert.Equal(t, int64(150), intVal)

	// 3. ZSet 操作
	store.ZAdd(ctx, "zset1", 1.0, "a")
	store.ZAdd(ctx, "zset1", 2.0, "b")
	store.ZAdd(ctx, "zset1", 3.0, "c")

	count, _ := store.ZCount(ctx, "zset1", 0, 10)
	assert.Equal(t, int64(3), count)

	// 4. 删除操作
	store.Del(ctx, "str1", "int1")

	exists1, _ := store.Exists(ctx, "str1")
	assert.False(t, exists1)

	exists2, _ := store.Exists(ctx, "int1")
	assert.False(t, exists2)

	// str2 和 zset1 应该还存在
	exists3, _ := store.Exists(ctx, "str2")
	assert.True(t, exists3)
}
