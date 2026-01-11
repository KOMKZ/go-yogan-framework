package limiter

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore Redis存储实现
type RedisStore struct {
	client    *redis.Client
	keyPrefix string // key前缀
}

// NewRedisStore 创建Redis存储
func NewRedisStore(client *redis.Client, keyPrefix string) *RedisStore {
	if keyPrefix == "" {
		keyPrefix = "limiter:"
	}
	return &RedisStore{
		client:    client,
		keyPrefix: keyPrefix,
	}
}

// buildKey 构建完整的key
func (s *RedisStore) buildKey(key string) string {
	return s.keyPrefix + key
}

// Get 获取值（字符串）
func (s *RedisStore) Get(ctx context.Context, key string) (string, error) {
	fullKey := s.buildKey(key)
	val, err := s.client.Get(ctx, fullKey).Result()
	if err == redis.Nil {
		return "", nil // key不存在返回空字符串
	}
	return val, err
}

// Set 设置值（字符串）
func (s *RedisStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	fullKey := s.buildKey(key)
	return s.client.Set(ctx, fullKey, value, ttl).Err()
}

// GetInt64 获取整数值
func (s *RedisStore) GetInt64(ctx context.Context, key string) (int64, error) {
	fullKey := s.buildKey(key)
	val, err := s.client.Get(ctx, fullKey).Result()
	if err == redis.Nil {
		return 0, nil // key不存在返回0
	}
	if err != nil {
		return 0, fmt.Errorf("redis get failed: %w", err)
	}

	count, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse count failed: %w", err)
	}
	return count, nil
}

// SetInt64 设置整数值
func (s *RedisStore) SetInt64(ctx context.Context, key string, value int64, ttl time.Duration) error {
	fullKey := s.buildKey(key)
	return s.client.Set(ctx, fullKey, value, ttl).Err()
}

// Decr 原子递减
func (s *RedisStore) Decr(ctx context.Context, key string) (int64, error) {
	fullKey := s.buildKey(key)
	return s.client.Decr(ctx, fullKey).Result()
}

// DecrBy 原子递减指定值
func (s *RedisStore) DecrBy(ctx context.Context, key string, delta int64) (int64, error) {
	fullKey := s.buildKey(key)
	return s.client.DecrBy(ctx, fullKey, delta).Result()
}

// Incr 原子递增
func (s *RedisStore) Incr(ctx context.Context, key string) (int64, error) {
	fullKey := s.buildKey(key)
	return s.client.Incr(ctx, fullKey).Result()
}

// IncrBy 原子递增指定值
func (s *RedisStore) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	fullKey := s.buildKey(key)
	return s.client.IncrBy(ctx, fullKey, value).Result()
}

// Expire 设置过期时间
func (s *RedisStore) Expire(ctx context.Context, key string, expiration time.Duration) error {
	fullKey := s.buildKey(key)
	return s.client.Expire(ctx, fullKey, expiration).Err()
}

// Del 删除key
func (s *RedisStore) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	fullKeys := make([]string, len(keys))
	for i, key := range keys {
		fullKeys[i] = s.buildKey(key)
	}
	return s.client.Del(ctx, fullKeys...).Err()
}

// Exists 检查key是否存在
func (s *RedisStore) Exists(ctx context.Context, key string) (bool, error) {
	fullKey := s.buildKey(key)
	count, err := s.client.Exists(ctx, fullKey).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// TTL 获取key的剩余过期时间
func (s *RedisStore) TTL(ctx context.Context, key string) (time.Duration, error) {
	fullKey := s.buildKey(key)
	return s.client.TTL(ctx, fullKey).Result()
}

// ZAdd 添加有序集合成员
func (s *RedisStore) ZAdd(ctx context.Context, key string, score float64, member string) error {
	fullKey := s.buildKey(key)
	return s.client.ZAdd(ctx, fullKey, redis.Z{
		Score:  score,
		Member: member,
	}).Err()
}

// ZRemRangeByScore 删除有序集合中指定分数范围的成员
func (s *RedisStore) ZRemRangeByScore(ctx context.Context, key string, min, max float64) error {
	fullKey := s.buildKey(key)
	minStr := fmt.Sprintf("%f", min)
	maxStr := fmt.Sprintf("%f", max)
	return s.client.ZRemRangeByScore(ctx, fullKey, minStr, maxStr).Err()
}

// ZCount 统计有序集合中指定分数范围的成员数量
func (s *RedisStore) ZCount(ctx context.Context, key string, min, max float64) (int64, error) {
	fullKey := s.buildKey(key)
	minStr := fmt.Sprintf("%f", min)
	maxStr := fmt.Sprintf("%f", max)
	return s.client.ZCount(ctx, fullKey, minStr, maxStr).Result()
}

// Eval 执行Lua脚本
func (s *RedisStore) Eval(ctx context.Context, script string, keys []string, args []interface{}) (interface{}, error) {
	fullKeys := make([]string, len(keys))
	for i, key := range keys {
		fullKeys[i] = s.buildKey(key)
	}
	return s.client.Eval(ctx, script, fullKeys, args).Result()
}

// Close 关闭连接（RedisStore不拥有client，所以不需要关闭）
func (s *RedisStore) Close() error {
	// Redis客户端由RedisManager管理，这里不关闭
	return nil
}
