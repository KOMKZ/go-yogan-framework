// Package cache 提供缓存编排层实现
// 支持多存储后端、事件驱动失效、集中配置管理
package cache

import (
	"context"
	"time"
)

// Store 缓存存储接口
// 所有存储后端必须实现此接口
type Store interface {
	// Name 返回存储后端名称
	Name() string

	// Get 获取缓存值
	// 返回 ErrCacheMiss 表示未命中
	Get(ctx context.Context, key string) ([]byte, error)

	// Set 设置缓存值
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Delete 删除缓存
	Delete(ctx context.Context, key string) error

	// DeleteByPrefix 按前缀删除
	DeleteByPrefix(ctx context.Context, prefix string) error

	// Exists 检查 Key 是否存在
	Exists(ctx context.Context, key string) bool

	// Close 关闭存储连接
	Close() error
}

// Serializer 序列化接口
type Serializer interface {
	// Serialize 序列化对象为字节数组
	Serialize(v any) ([]byte, error)

	// Deserialize 反序列化字节数组为对象
	Deserialize(data []byte, v any) error

	// Name 返回序列化器名称
	Name() string
}

// LoaderFunc 数据加载函数
// 当缓存未命中时调用此函数获取数据
type LoaderFunc func(ctx context.Context, args ...any) (any, error)

// KeyBuilderFunc Key 生成函数
type KeyBuilderFunc func(args ...any) string

// Orchestrator 缓存编排中心接口
type Orchestrator interface {
	// RegisterLoader 注册数据加载器
	RegisterLoader(name string, loader LoaderFunc)

	// Call 执行缓存调用
	// 自动处理缓存读取、未命中加载、写入缓存
	Call(ctx context.Context, name string, args ...any) (any, error)

	// Invalidate 手动失效指定缓存
	Invalidate(ctx context.Context, name string, args ...any) error

	// InvalidateByPattern 按模式失效
	InvalidateByPattern(ctx context.Context, name string, pattern string) error

	// GetStore 获取存储后端
	GetStore(name string) (Store, error)

	// RegisterStore 注册存储后端
	RegisterStore(name string, store Store)

	// Stats 获取缓存统计信息
	Stats() *CacheStats
}

// CacheStats 缓存统计信息
type CacheStats struct {
	Hits        int64            `json:"hits"`
	Misses      int64            `json:"misses"`
	Invalidates int64            `json:"invalidates"`
	Errors      int64            `json:"errors"`
	ByName      map[string]int64 `json:"by_name"`
}
