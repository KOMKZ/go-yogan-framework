package cache

import (
	"fmt"
	"time"
)

// Config 缓存组件配置
type Config struct {
	// Enabled 是否启用缓存
	Enabled bool `mapstructure:"enabled"`

	// DefaultTTL 默认过期时间
	DefaultTTL time.Duration `mapstructure:"default_ttl"`

	// DefaultStore 默认存储后端
	DefaultStore string `mapstructure:"default_store"`

	// Stores 存储后端配置
	Stores map[string]StoreConfig `mapstructure:"stores"`

	// Cacheables 缓存项配置
	Cacheables []CacheableConfig `mapstructure:"cacheables"`

	// InvalidationRules 失效规则
	InvalidationRules []InvalidationRule `mapstructure:"invalidation_rules"`
}

// StoreConfig 存储后端配置
type StoreConfig struct {
	// Type 存储类型：redis, memory, chain
	Type string `mapstructure:"type"`

	// Redis 相关配置
	Instance  string `mapstructure:"instance"`   // Redis 实例名
	KeyPrefix string `mapstructure:"key_prefix"` // Key 前缀

	// Memory 相关配置
	MaxSize   int    `mapstructure:"max_size"`  // 最大条目数
	MaxMemory string `mapstructure:"max_memory"` // 最大内存
	Eviction  string `mapstructure:"eviction"`   // 淘汰策略：lru, lfu

	// Chain 相关配置
	Layers []string `mapstructure:"layers"` // 缓存层列表
}

// CacheableConfig 缓存项配置
type CacheableConfig struct {
	// Name 缓存项名称（唯一标识）
	Name string `mapstructure:"name"`

	// KeyPattern Key 模式，支持占位符 {0}, {1}, {hash}
	KeyPattern string `mapstructure:"key_pattern"`

	// TTL 过期时间
	TTL time.Duration `mapstructure:"ttl"`

	// Store 存储后端名称
	Store string `mapstructure:"store"`

	// LocalCache 是否叠加本地缓存
	LocalCache bool `mapstructure:"local_cache"`

	// DependsOn 失效依赖的事件列表
	DependsOn []string `mapstructure:"depends_on"`

	// Enabled 是否启用
	Enabled bool `mapstructure:"enabled"`
}

// InvalidationRule 失效规则
type InvalidationRule struct {
	// Event 触发失效的事件名
	Event string `mapstructure:"event"`

	// Invalidate 要失效的缓存项名称列表
	Invalidate []string `mapstructure:"invalidate"`

	// Pattern 按模式失效，如 "user:*"
	// 注意：仅在需要批量失效时使用，推荐使用 CacheInvalidator 接口精确失效
	Pattern string `mapstructure:"pattern"`
}

// Validate 验证配置
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil // 未启用时不验证
	}

	if c.DefaultTTL <= 0 {
		c.DefaultTTL = 5 * time.Minute
	}

	if c.DefaultStore == "" {
		c.DefaultStore = "memory"
	}

	// 验证存储后端配置
	for name, store := range c.Stores {
		if store.Type == "" {
			return fmt.Errorf("store %s: type is required", name)
		}
		switch store.Type {
		case "redis", "memory", "chain":
			// valid
		default:
			return fmt.Errorf("store %s: unknown type %s", name, store.Type)
		}
	}

	// 验证缓存项配置
	for _, cacheable := range c.Cacheables {
		if cacheable.Name == "" {
			return fmt.Errorf("cacheable: name is required")
		}
		if cacheable.KeyPattern == "" {
			return fmt.Errorf("cacheable %s: key_pattern is required", cacheable.Name)
		}
	}

	return nil
}

// ApplyDefaults 应用默认值
func (c *Config) ApplyDefaults() {
	if c.DefaultTTL <= 0 {
		c.DefaultTTL = 5 * time.Minute
	}
	if c.DefaultStore == "" {
		c.DefaultStore = "memory"
	}

	// 为每个缓存项应用默认值
	for i := range c.Cacheables {
		if c.Cacheables[i].TTL <= 0 {
			c.Cacheables[i].TTL = c.DefaultTTL
		}
		if c.Cacheables[i].Store == "" {
			c.Cacheables[i].Store = c.DefaultStore
		}
		// 默认启用
		if !c.Cacheables[i].Enabled {
			c.Cacheables[i].Enabled = true
		}
	}
}
