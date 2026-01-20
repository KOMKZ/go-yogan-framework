package cache

import (
	"fmt"
	"time"
)

// Configuration for cache component
type Config struct {
	// Enabled whether to enable caching
	Enabled bool `mapstructure:"enabled"`

	// DefaultTTL default expiration time
	DefaultTTL time.Duration `mapstructure:"default_ttl"`

	// DefaultStore default storage backend
	DefaultStore string `mapstructure:"default_store"`

	// Stores backend configuration
	Stores map[string]StoreConfig `mapstructure:"stores"`

	// Cacheable item configuration
	Cacheables []CacheableConfig `mapstructure:"cacheables"`

	// InvalidationRules invalidation rules
	InvalidationRules []InvalidationRule `mapstructure:"invalidation_rules"`
}

// StoreConfig stores backend configuration
type StoreConfig struct {
	// Type storage: redis, memory, chain
	Type string `mapstructure:"type"`

	// Redis related configuration
	Instance  string `mapstructure:"instance"`   // Redis instance name
	KeyPrefix string `mapstructure:"key_prefix"` // Key prefix

	// Memory related configurations
	MaxSize   int    `mapstructure:"max_size"`  // Maximum item count
	MaxMemory string `mapstructure:"max_memory"` // Maximum memory
	Eviction  string `mapstructure:"eviction"`   // Eviction strategy: lru, lfu

	// Chain related configurations
	Layers []string `mapstructure:"layers"` // cache layer list
}

// CacheableConfig cache item configuration
type CacheableConfig struct {
	// Name Cache item name (unique identifier)
	Name string `mapstructure:"name"`

	// KeyPattern key pattern, supports placeholders {0}, {1}, {hash}
	KeyPattern string `mapstructure:"key_pattern"`

	// TTL expiration time
	TTL time.Duration `mapstructure:"ttl"`

	// Store backend name
	Store string `mapstructure:"store"`

	// Whether to overlay local cache
	LocalCache bool `mapstructure:"local_cache"`

	// List of events for failed dependencies
	DependsOn []string `mapstructure:"depends_on"`

	// Enabled whether to enable
	Enabled bool `mapstructure:"enabled"`
}

// InvalidationRule invalidation rule
type InvalidationRule struct {
	// Event name for a failed trigger event
	Event string `mapstructure:"event"`

	// Invalidate list of cache items to be invalidated
	Invalidate []string `mapstructure:"invalidate"`

	// Pattern fails according to mode, e.g., "user:*"
	// Note: Use only when bulk invalidation is needed; precise invalidation is recommended using the CacheInvalidator interface
	Pattern string `mapstructure:"pattern"`
}

// Validate configuration
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil // Do not validate when disabled
	}

	if c.DefaultTTL <= 0 {
		c.DefaultTTL = 5 * time.Minute
	}

	if c.DefaultStore == "" {
		c.DefaultStore = "memory"
	}

	// Validate storage backend configuration
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

	// Validate cache item configuration
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

// ApplyDefaults Apply default values
func (c *Config) ApplyDefaults() {
	if c.DefaultTTL <= 0 {
		c.DefaultTTL = 5 * time.Minute
	}
	if c.DefaultStore == "" {
		c.DefaultStore = "memory"
	}

	// Apply default values to each cache item
	for i := range c.Cacheables {
		if c.Cacheables[i].TTL <= 0 {
			c.Cacheables[i].TTL = c.DefaultTTL
		}
		if c.Cacheables[i].Store == "" {
			c.Cacheables[i].Store = c.DefaultStore
		}
		// Default enabled
		if !c.Cacheables[i].Enabled {
			c.Cacheables[i].Enabled = true
		}
	}
}
