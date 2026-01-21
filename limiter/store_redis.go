package limiter

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore Redis storage implementation
type RedisStore struct {
	client    *redis.Client
	keyPrefix string // key prefix
}

// NewRedisStore creates Redis storage
func NewRedisStore(client *redis.Client, keyPrefix string) *RedisStore {
	if keyPrefix == "" {
		keyPrefix = "limiter:"
	}
	return &RedisStore{
		client:    client,
		keyPrefix: keyPrefix,
	}
}

// buildKey Construct the complete key
func (s *RedisStore) buildKey(key string) string {
	return s.keyPrefix + key
}

// Get Retrieve value (string)
func (s *RedisStore) Get(ctx context.Context, key string) (string, error) {
	fullKey := s.buildKey(key)
	val, err := s.client.Get(ctx, fullKey).Result()
	if err == redis.Nil {
		return "", nil // return an empty string if the key does not exist
	}
	return val, err
}

// Set the value (string)
func (s *RedisStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	fullKey := s.buildKey(key)
	return s.client.Set(ctx, fullKey, value, ttl).Err()
}

// GetInt64 get integer value
func (s *RedisStore) GetInt64(ctx context.Context, key string) (int64, error) {
	fullKey := s.buildKey(key)
	val, err := s.client.Get(ctx, fullKey).Result()
	if err == redis.Nil {
		return 0, nil // return 0 if key does not exist
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

// SetInt64 set integer value
func (s *RedisStore) SetInt64(ctx context.Context, key string, value int64, ttl time.Duration) error {
	fullKey := s.buildKey(key)
	return s.client.Set(ctx, fullKey, value, ttl).Err()
}

// Decrement atomic decrement
func (s *RedisStore) Decr(ctx context.Context, key string) (int64, error) {
	fullKey := s.buildKey(key)
	return s.client.Decr(ctx, fullKey).Result()
}

// Atomically decrement by the specified value
func (s *RedisStore) DecrBy(ctx context.Context, key string, delta int64) (int64, error) {
	fullKey := s.buildKey(key)
	return s.client.DecrBy(ctx, fullKey, delta).Result()
}

// Increment atomic increment
func (s *RedisStore) Incr(ctx context.Context, key string) (int64, error) {
	fullKey := s.buildKey(key)
	return s.client.Incr(ctx, fullKey).Result()
}

// IncrBy atomic increment by specified value
func (s *RedisStore) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	fullKey := s.buildKey(key)
	return s.client.IncrBy(ctx, fullKey, value).Result()
}

// Set expiration time
func (s *RedisStore) Expire(ctx context.Context, key string, expiration time.Duration) error {
	fullKey := s.buildKey(key)
	return s.client.Expire(ctx, fullKey, expiration).Err()
}

// Delete key deletion
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

// Exists check if key exists
func (s *RedisStore) Exists(ctx context.Context, key string) (bool, error) {
	fullKey := s.buildKey(key)
	count, err := s.client.Exists(ctx, fullKey).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Get the remaining time-to-live for the key
// Returns 0 if key doesn't exist or has no TTL
func (s *RedisStore) TTL(ctx context.Context, key string) (time.Duration, error) {
	fullKey := s.buildKey(key)
	ttl, err := s.client.TTL(ctx, fullKey).Result()
	if err != nil {
		return 0, err
	}
	// Redis returns -2 if key doesn't exist, -1 if no TTL
	if ttl < 0 {
		return 0, nil
	}
	return ttl, nil
}

// Add ordered set members
func (s *RedisStore) ZAdd(ctx context.Context, key string, score float64, member string) error {
	fullKey := s.buildKey(key)
	return s.client.ZAdd(ctx, fullKey, redis.Z{
		Score:  score,
		Member: member,
	}).Err()
}

// Remove members from a sorted set that are within a specified score range
func (s *RedisStore) ZRemRangeByScore(ctx context.Context, key string, min, max float64) error {
	fullKey := s.buildKey(key)
	minStr := fmt.Sprintf("%f", min)
	maxStr := fmt.Sprintf("%f", max)
	return s.client.ZRemRangeByScore(ctx, fullKey, minStr, maxStr).Err()
}

// ZCount statistics the number of members in a sorted set within a specified score range
func (s *RedisStore) ZCount(ctx context.Context, key string, min, max float64) (int64, error) {
	fullKey := s.buildKey(key)
	minStr := fmt.Sprintf("%f", min)
	maxStr := fmt.Sprintf("%f", max)
	return s.client.ZCount(ctx, fullKey, minStr, maxStr).Result()
}

// Evaluate execution of Lua script
func (s *RedisStore) Eval(ctx context.Context, script string, keys []string, args []interface{}) (interface{}, error) {
	fullKeys := make([]string, len(keys))
	for i, key := range keys {
		fullKeys[i] = s.buildKey(key)
	}
	return s.client.Eval(ctx, script, fullKeys, args).Result()
}

// Close Close the connection (RedisStore does not own the client, so there is no need to close)
func (s *RedisStore) Close() error {
	// The Redis client is managed by RedisManager, so it is not closed here
	return nil
}
