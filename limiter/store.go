package limiter

import (
	"context"
	"time"
)

// Store 存储接口（策略模式）
type Store interface {
	// Get 获取值
	Get(ctx context.Context, key string) (string, error)

	// Set 设置值（带过期时间）
	Set(ctx context.Context, key string, value string, ttl time.Duration) error

	// GetInt64 获取整数值
	GetInt64(ctx context.Context, key string) (int64, error)

	// SetInt64 设置整数值
	SetInt64(ctx context.Context, key string, value int64, ttl time.Duration) error

	// Incr 原子递增
	Incr(ctx context.Context, key string) (int64, error)

	// IncrBy 原子递增指定值
	IncrBy(ctx context.Context, key string, delta int64) (int64, error)

	// Decr 原子递减
	Decr(ctx context.Context, key string) (int64, error)

	// DecrBy 原子递减指定值
	DecrBy(ctx context.Context, key string, delta int64) (int64, error)

	// Expire 设置过期时间
	Expire(ctx context.Context, key string, ttl time.Duration) error

	// TTL 获取剩余过期时间
	TTL(ctx context.Context, key string) (time.Duration, error)

	// Del 删除键
	Del(ctx context.Context, keys ...string) error

	// Exists 检查键是否存在
	Exists(ctx context.Context, key string) (bool, error)

	// ZAdd 添加到有序集合
	ZAdd(ctx context.Context, key string, score float64, member string) error

	// ZRemRangeByScore 按分数范围删除
	ZRemRangeByScore(ctx context.Context, key string, min, max float64) error

	// ZCount 统计分数范围内的元素数量
	ZCount(ctx context.Context, key string, min, max float64) (int64, error)

	// Eval 执行Lua脚本（Redis专用，内存存储可返回不支持错误）
	Eval(ctx context.Context, script string, keys []string, args []interface{}) (interface{}, error)

	// Close 关闭连接
	Close() error
}

// StoreType 存储类型
type StoreType string

const (
	// StoreTypeMemory 内存存储
	StoreTypeMemory StoreType = "memory"

	// StoreTypeRedis Redis存储
	StoreTypeRedis StoreType = "redis"
)

