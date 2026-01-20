package cache

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore Redis cache storage
type RedisStore struct {
	name      string
	client    *redis.Client
	keyPrefix string
}

// Create new Redis storage
func NewRedisStore(name string, client *redis.Client, keyPrefix string) *RedisStore {
	return &RedisStore{
		name:      name,
		client:    client,
		keyPrefix: keyPrefix,
	}
}

// Returns the storage name
func (s *RedisStore) Name() string {
	return s.name
}

// buildKey Construct the complete key
func (s *RedisStore) buildKey(key string) string {
	if s.keyPrefix == "" {
		return key
	}
	return s.keyPrefix + key
}

// Get cache value
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

// Set cache value
func (s *RedisStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	fullKey := s.buildKey(key)
	err := s.client.Set(ctx, fullKey, value, ttl).Err()
	if err != nil {
		return ErrStoreSet.Wrap(err)
	}
	return nil
}

// Delete cache
func (s *RedisStore) Delete(ctx context.Context, key string) error {
	fullKey := s.buildKey(key)
	err := s.client.Del(ctx, fullKey).Err()
	if err != nil {
		return ErrStoreDelete.Wrap(err)
	}
	return nil
}

// DeleteByPrefix delete by prefix
func (s *RedisStore) DeleteByPrefix(ctx context.Context, prefix string) error {
	fullPrefix := s.buildKey(prefix)
	
	// Use SCAN to avoid blocking
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

// Exists check if Key exists
func (s *RedisStore) Exists(ctx context.Context, key string) bool {
	fullKey := s.buildKey(key)
	n, err := s.client.Exists(ctx, fullKey).Result()
	if err != nil {
		return false
	}
	return n > 0
}

// Close storage
func (s *RedisStore) Close() error {
	// The Redis client is managed externally, so we do not close it here.
	return nil
}

// DeleteByPattern delete by pattern (supports wildcards)
func (s *RedisStore) DeleteByPattern(ctx context.Context, pattern string) error {
	// If pattern does not contain wildcards, delete directly
	if !strings.Contains(pattern, "*") {
		return s.Delete(ctx, pattern)
	}
	return s.DeleteByPrefix(ctx, strings.TrimSuffix(pattern, "*"))
}
