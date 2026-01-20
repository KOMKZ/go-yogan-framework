package limiter

import (
	"context"
	"time"
)

// Store interface (Strategy Pattern)
type Store interface {
	// Get retrieve value
	Get(ctx context.Context, key string) (string, error)

	// Set value (with expiration time)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error

	// GetInt64 get integer value
	GetInt64(ctx context.Context, key string) (int64, error)

	// SetInt64 set integer value
	SetInt64(ctx context.Context, key string, value int64, ttl time.Duration) error

	// Atomic increment
	Incr(ctx context.Context, key string) (int64, error)

	// IncrBy atomic increment by specified value
	IncrBy(ctx context.Context, key string, delta int64) (int64, error)

	// Atomic decrement
	Decr(ctx context.Context, key string) (int64, error)

	// Atomically decrement by the specified value
	DecrBy(ctx context.Context, key string, delta int64) (int64, error)

	// Set expiration time
	Expire(ctx context.Context, key string, ttl time.Duration) error

	// Get remaining TTL (Time To Live) duration
	TTL(ctx context.Context, key string) (time.Duration, error)

	// Delete delete key
	Del(ctx context.Context, keys ...string) error

	// Exists Check if key exists
	Exists(ctx context.Context, key string) (bool, error)

	// Add to sorted set
	ZAdd(ctx context.Context, key string, score float64, member string) error

	// Remove by score range
	ZRemRangeByScore(ctx context.Context, key string, min, max float64) error

	// ZCount statistics the number of elements within a score range
	ZCount(ctx context.Context, key string, min, max float64) (int64, error)

	// Eval executes Lua scripts (specific to Redis, can return unsupported errors for in-memory storage)
	Eval(ctx context.Context, script string, keys []string, args []interface{}) (interface{}, error)

	// Close connection
	Close() error
}

// StoreType storage type
type StoreType string

const (
	// StoreTypeMemory Memory Storage
	StoreTypeMemory StoreType = "memory"

	// StoreTypeRedis Redis storage
	StoreTypeRedis StoreType = "redis"
)

