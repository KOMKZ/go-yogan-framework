// Package cache provides a caching orchestration layer implementation
// Supports multi-storage backend, event-driven failure, centralized configuration management
package cache

import (
	"context"
	"time"
)

// Store cache storage interface
// All storage backends must implement this interface
type Store interface {
	// Name Returns the storage backend name
	Name() string

	// Get cache value
	// Return ErrCacheMiss indicates a cache miss
	Get(ctx context.Context, key string) ([]byte, error)

	// Set cache value
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Delete cache
	Delete(ctx context.Context, key string) error

	// DeleteByPrefix delete by prefix
	DeleteByPrefix(ctx context.Context, prefix string) error

	// Exists check if Key exists
	Exists(ctx context.Context, key string) bool

	// Close storage connection
	Close() error
}

// Serializer serialization interface
type Serializer interface {
	// Serialize object to byte array
	Serialize(v any) ([]byte, error)

	// Deserialize byte array to object
	Deserialize(data []byte, v any) error

	// Return serializer name
	Name() string
}

// Loader function for data loading
// Call this function to retrieve data when cache miss occurs
type LoaderFunc func(ctx context.Context, args ...any) (any, error)

// KeyBuilderFunc Key generation function
type KeyBuilderFunc func(args ...any) string

// Orchestrator caching orchestration center interface
type Orchestrator interface {
	// RegisterLoader register data loader
	RegisterLoader(name string, loader LoaderFunc)

	// Call execute cache call
	// Automatically handle cache reads, cache misses loading, and cache writes
	Call(ctx context.Context, name string, args ...any) (any, error)

	// Invalidate specified cache manually
	Invalidate(ctx context.Context, name string, args ...any) error

	// InvalidateByPattern invalidate by pattern
	InvalidateByPattern(ctx context.Context, name string, pattern string) error

	// GetStore Retrieve storage backend
	GetStore(name string) (Store, error)

	// RegisterStore register storage backend
	RegisterStore(name string, store Store)

	// Get cache statistics
	Stats() *CacheStats
}

// CacheStats cache statistics
type CacheStats struct {
	Hits        int64            `json:"hits"`
	Misses      int64            `json:"misses"`
	Invalidates int64            `json:"invalidates"`
	Errors      int64            `json:"errors"`
	ByName      map[string]int64 `json:"by_name"`
}
