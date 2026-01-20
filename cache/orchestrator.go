package cache

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/KOMKZ/go-yogan-framework/event"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

// CacheInvalidator cache invalidation interface
// After an event implements this interface, the cache component can automatically extract parameters to precisely invalidate caches
type CacheInvalidator interface {
	// Returns the parameters list used for constructing cache keys
	// For example: ArticleDeletedEvent returns []any{articleID}
	CacheArgs() []any
}

// DefaultOrchestrator default cache orchestration center implementation
type DefaultOrchestrator struct {
	config     *Config
	stores     map[string]Store
	loaders    map[string]LoaderFunc
	cacheables map[string]*CacheableConfig
	serializer Serializer
	dispatcher event.Dispatcher
	logger     *logger.CtxZapLogger
	sf         singleflight.Group
	mu         sync.RWMutex

	// Statistical analysis
	hits        int64
	misses      int64
	invalidates int64
	errors      int64
}

// NewOrchestrator creates the orchestrator center
func NewOrchestrator(cfg *Config, dispatcher event.Dispatcher, log *logger.CtxZapLogger) *DefaultOrchestrator {
	cfg.ApplyDefaults()

	o := &DefaultOrchestrator{
		config:     cfg,
		stores:     make(map[string]Store),
		loaders:    make(map[string]LoaderFunc),
		cacheables: make(map[string]*CacheableConfig),
		serializer: NewJSONSerializer(),
		dispatcher: dispatcher,
		logger:     log,
	}

	// Load cache item configuration
	for i := range cfg.Cacheables {
		c := &cfg.Cacheables[i]
		o.cacheables[c.Name] = c
	}

	// subscription expiration event
	if dispatcher != nil {
		o.subscribeInvalidationEvents()
	}

	return o
}

// RegisterLoader registers data loader
func (o *DefaultOrchestrator) RegisterLoader(name string, loader LoaderFunc) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.loaders[name] = loader
	if o.logger != nil {
		o.logger.Debug("cache loader registered", zap.String("name", name))
	}
}

// RegisterStore register storage backend
func (o *DefaultOrchestrator) RegisterStore(name string, store Store) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.stores[name] = store
	if o.logger != nil {
		o.logger.Debug("cache store registered", zap.String("name", name))
	}
}

// GetStore Retrieve storage backend
func (o *DefaultOrchestrator) GetStore(name string) (Store, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	store, ok := o.stores[name]
	if !ok {
		return nil, ErrStoreNotFound.WithMsgf("存储后端未找到: %s", name)
	}
	return store, nil
}

// Call execute cache call
func (o *DefaultOrchestrator) Call(ctx context.Context, name string, args ...any) (any, error) {
	o.mu.RLock()
	config, ok := o.cacheables[name]
	loader, hasLoader := o.loaders[name]
	o.mu.RUnlock()

	if !ok {
		return nil, ErrCacheableNotFound.WithMsgf("缓存项未配置: %s", name)
	}

	if !hasLoader {
		return nil, ErrLoaderNotFound.WithMsgf("加载器未注册: %s", name)
	}

	if !config.Enabled || !o.config.Enabled {
		// Cache disabled, call loader directly
		return loader(ctx, args...)
	}

	// Retrieve storage backend
	store, err := o.getStoreForCacheable(config)
	if err != nil {
		// Degraded to direct call when storage is unavailable
		atomic.AddInt64(&o.errors, 1)
		if o.logger != nil {
			o.logger.Warn("cache store unavailable, fallback to loader",
				zap.String("name", name),
				zap.Error(err),
			)
		}
		return loader(ctx, args...)
	}

	// Build Key
	key := o.buildKey(config.KeyPattern, args...)

	// 1. Try to get from cache
	data, err := store.Get(ctx, key)
	if err == nil {
		// cache hit
		var result any
		if err := o.serializer.Deserialize(data, &result); err == nil {
			atomic.AddInt64(&o.hits, 1)
			if o.logger != nil {
				o.logger.Debug("cache hit", zap.String("name", name), zap.String("key", key))
			}
			return result, nil
		}
		// Deserialization failed, treat as miss
		atomic.AddInt64(&o.errors, 1)
	}

	// 2. Cache miss, use singleflight to prevent penetration hits
	atomic.AddInt64(&o.misses, 1)
	if o.logger != nil {
		o.logger.Debug("cache miss", zap.String("name", name), zap.String("key", key))
	}

	result, err, _ := o.sf.Do(key, func() (any, error) {
		// Double-check: Recheck cache
		if data, err := store.Get(ctx, key); err == nil {
			var result any
			if err := o.serializer.Deserialize(data, &result); err == nil {
				return result, nil
			}
		}

		// call loader
		result, err := loader(ctx, args...)
		if err != nil {
			return nil, err
		}

		// Write to cache
		ttl := config.TTL
		if ttl <= 0 {
			ttl = o.config.DefaultTTL
		}
		data, serErr := o.serializer.Serialize(result)
		if serErr != nil {
			atomic.AddInt64(&o.errors, 1)
			if o.logger != nil {
				o.logger.Warn("cache serialize failed", zap.String("name", name), zap.Error(serErr))
			}
		} else {
			if setErr := store.Set(ctx, key, data, ttl); setErr != nil {
				atomic.AddInt64(&o.errors, 1)
				if o.logger != nil {
					o.logger.Warn("cache set failed", zap.String("name", name), zap.Error(setErr))
				}
			}
		}

		return result, nil
	})

	return result, err
}

// Invalidate specified cache manually
func (o *DefaultOrchestrator) Invalidate(ctx context.Context, name string, args ...any) error {
	config, ok := o.cacheables[name]
	if !ok {
		return ErrCacheableNotFound.WithMsgf("缓存项未配置: %s", name)
	}

	store, err := o.getStoreForCacheable(config)
	if err != nil {
		return err
	}

	key := o.buildKey(config.KeyPattern, args...)
	if err := store.Delete(ctx, key); err != nil {
		return err
	}

	atomic.AddInt64(&o.invalidates, 1)
	if o.logger != nil {
		o.logger.Info("cache invalidated", zap.String("name", name), zap.String("key", key))
	}
	return nil
}

// InvalidateByPattern invalidate by pattern
func (o *DefaultOrchestrator) InvalidateByPattern(ctx context.Context, name string, pattern string) error {
	config, ok := o.cacheables[name]
	if !ok {
		return ErrCacheableNotFound.WithMsgf("缓存项未配置: %s", name)
	}

	store, err := o.getStoreForCacheable(config)
	if err != nil {
		return err
	}

	if err := store.DeleteByPrefix(ctx, pattern); err != nil {
		return err
	}

	atomic.AddInt64(&o.invalidates, 1)
	if o.logger != nil {
		o.logger.Info("cache invalidated by pattern", zap.String("name", name), zap.String("pattern", pattern))
	}
	return nil
}

// Get cache statistics
func (o *DefaultOrchestrator) Stats() *CacheStats {
	return &CacheStats{
		Hits:        atomic.LoadInt64(&o.hits),
		Misses:      atomic.LoadInt64(&o.misses),
		Invalidates: atomic.LoadInt64(&o.invalidates),
		Errors:      atomic.LoadInt64(&o.errors),
		ByName:      make(map[string]int64),
	}
}

// getStoreForCacheable Get the storage backend for a cache item
func (o *DefaultOrchestrator) getStoreForCacheable(config *CacheableConfig) (Store, error) {
	storeName := config.Store
	if storeName == "" {
		storeName = o.config.DefaultStore
	}
	return o.GetStore(storeName)
}

// buildKey to construct cache key
func (o *DefaultOrchestrator) buildKey(pattern string, args ...any) string {
	result := pattern
	for i, arg := range args {
		placeholder := fmt.Sprintf("{%d}", i)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", arg))
	}
	// Handle {hash} placeholder
	if strings.Contains(result, "{hash}") {
		hash := hashArgs(args...)
		result = strings.ReplaceAll(result, "{hash}", hash)
	}
	return result
}

// hashArgs calculates parameter hash
func hashArgs(args ...any) string {
	// Simple implementation: concatenate parameter string
	var sb strings.Builder
	for _, arg := range args {
		sb.WriteString(fmt.Sprintf("%v", arg))
	}
	// Return simple hash (should actually use MD5/SHA1)
	s := sb.String()
	if len(s) > 32 {
		return s[:32]
	}
	return s
}

// subscribeInvalidationEvents
func (o *DefaultOrchestrator) subscribeInvalidationEvents() {
	for _, rule := range o.config.InvalidationRules {
		rule := rule // capture
		o.dispatcher.Subscribe(rule.Event, o.createInvalidationHandler(rule))
		if o.logger != nil {
			o.logger.Debug("subscribed invalidation event", zap.String("event", rule.Event))
		}
	}
}

// createInvalidationHandler Create invalidation event handler
func (o *DefaultOrchestrator) createInvalidationHandler(rule InvalidationRule) event.Listener {
	return event.ListenerFunc(func(ctx context.Context, e event.Event) error {
		for _, cacheableName := range rule.Invalidate {
			if rule.Pattern != "" {
				// Fail according to pattern (wildcard)
				if err := o.InvalidateByPattern(ctx, cacheableName, rule.Pattern); err != nil {
					if o.logger != nil {
						o.logger.Warn("cache invalidate by pattern failed",
							zap.String("cacheable", cacheableName),
							zap.String("pattern", rule.Pattern),
							zap.Error(err),
						)
					}
				}
			} else if inv, ok := e.(CacheInvalidator); ok {
				// The event implements the CacheInvalidator interface for precise invalidation
				args := inv.CacheArgs()
				if err := o.Invalidate(ctx, cacheableName, args...); err != nil {
					if o.logger != nil {
						o.logger.Warn("cache invalidate failed",
							zap.String("cacheable", cacheableName),
							zap.Any("args", args),
							zap.Error(err),
						)
					}
				}
			} else {
				// Event interface not implemented, log warning
				if o.logger != nil {
					o.logger.Warn("event does not implement CacheInvalidator, cannot extract cache args",
						zap.String("event", e.Name()),
						zap.String("cacheable", cacheableName),
					)
				}
			}
		}
		return nil
	})
}

// SetSerializer set serializer
func (o *DefaultOrchestrator) SetSerializer(s Serializer) {
	o.serializer = s
}

// Close Orchestrator Center
func (o *DefaultOrchestrator) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	for _, store := range o.stores {
		store.Close()
	}
	return nil
}
