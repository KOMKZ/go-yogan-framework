package cache

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore Redis 缓存存储
type RedisStore struct {
	name      string
	client    *redis.Client
	keyPrefix string
}

// NewRedisStore 创建 Redis 存储
func NewRedisStore(name string, client *redis.Client, keyPrefix string) *RedisStore {
	return &RedisStore{
		name:      name,
		client:    client,
		keyPrefix: keyPrefix,
	}
}

// Name 返回存储名称
func (s *RedisStore) Name() string {
	return s.name
}

// buildKey 构建完整的 Key
func (s *RedisStore) buildKey(key string) string {
	if s.keyPrefix == "" {
		return key
	}
	return s.keyPrefix + key
}

// Get 获取缓存值
func (s *RedisStore) Get(ctx context.Context, key string) ([]byte, error) {
	fullKey := s.buildKey(key)
	result, err := s.client.Get(ctx, fullKey).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrCacheMiss
		}
		return nil, ErrStoreGet.Wrap(err)
	}
	return result, nil
}

// Set 设置缓存值
func (s *RedisStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	fullKey := s.buildKey(key)
	err := s.client.Set(ctx, fullKey, value, ttl).Err()
	if err != nil {
		return ErrStoreSet.Wrap(err)
	}
	return nil
}

// Delete 删除缓存
func (s *RedisStore) Delete(ctx context.Context, key string) error {
	fullKey := s.buildKey(key)
	err := s.client.Del(ctx, fullKey).Err()
	if err != nil {
		return ErrStoreDelete.Wrap(err)
	}
	return nil
}

// DeleteByPrefix 按前缀删除
func (s *RedisStore) DeleteByPrefix(ctx context.Context, prefix string) error {
	fullPrefix := s.buildKey(prefix)
	
	// 使用 SCAN 避免阻塞
	var cursor uint64
	var keys []string
	
	for {
		var err error
		var batch []string
		batch, cursor, err = s.client.Scan(ctx, cursor, fullPrefix+"*", 100).Result()
		if err != nil {
			return ErrStoreDelete.Wrap(err)
		}
		keys = append(keys, batch...)
		if cursor == 0 {
			break
		}
	}
	
	if len(keys) > 0 {
		if err := s.client.Del(ctx, keys...).Err(); err != nil {
			return ErrStoreDelete.Wrap(err)
		}
	}
	
	return nil
}

// Exists 检查 Key 是否存在
func (s *RedisStore) Exists(ctx context.Context, key string) bool {
	fullKey := s.buildKey(key)
	n, err := s.client.Exists(ctx, fullKey).Result()
	if err != nil {
		return false
	}
	return n > 0
}

// Close 关闭存储
func (s *RedisStore) Close() error {
	// Redis client 由外部管理，这里不关闭
	return nil
}

// DeleteByPattern 按模式删除（支持通配符）
func (s *RedisStore) DeleteByPattern(ctx context.Context, pattern string) error {
	// 如果 pattern 不包含通配符，直接删除
	if !strings.Contains(pattern, "*") {
		return s.Delete(ctx, pattern)
	}
	return s.DeleteByPrefix(ctx, strings.TrimSuffix(pattern, "*"))
}
