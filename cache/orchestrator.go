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

// DefaultOrchestrator 默认缓存编排中心实现
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

	// 统计
	hits        int64
	misses      int64
	invalidates int64
	errors      int64
}

// NewOrchestrator 创建编排中心
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

	// 加载缓存项配置
	for i := range cfg.Cacheables {
		c := &cfg.Cacheables[i]
		o.cacheables[c.Name] = c
	}

	// 订阅失效事件
	if dispatcher != nil {
		o.subscribeInvalidationEvents()
	}

	return o
}

// RegisterLoader 注册数据加载器
func (o *DefaultOrchestrator) RegisterLoader(name string, loader LoaderFunc) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.loaders[name] = loader
	if o.logger != nil {
		o.logger.Debug("cache loader registered", zap.String("name", name))
	}
}

// RegisterStore 注册存储后端
func (o *DefaultOrchestrator) RegisterStore(name string, store Store) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.stores[name] = store
	if o.logger != nil {
		o.logger.Debug("cache store registered", zap.String("name", name))
	}
}

// GetStore 获取存储后端
func (o *DefaultOrchestrator) GetStore(name string) (Store, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	store, ok := o.stores[name]
	if !ok {
		return nil, ErrStoreNotFound.WithMsgf("存储后端未找到: %s", name)
	}
	return store, nil
}

// Call 执行缓存调用
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
		// 缓存禁用，直接调用 loader
		return loader(ctx, args...)
	}

	// 获取存储后端
	store, err := o.getStoreForCacheable(config)
	if err != nil {
		// 存储不可用时降级到直接调用
		atomic.AddInt64(&o.errors, 1)
		if o.logger != nil {
			o.logger.Warn("cache store unavailable, fallback to loader",
				zap.String("name", name),
				zap.Error(err),
			)
		}
		return loader(ctx, args...)
	}

	// 构建 Key
	key := o.buildKey(config.KeyPattern, args...)

	// 1. 尝试从缓存获取
	data, err := store.Get(ctx, key)
	if err == nil {
		// 缓存命中
		var result any
		if err := o.serializer.Deserialize(data, &result); err == nil {
			atomic.AddInt64(&o.hits, 1)
			if o.logger != nil {
				o.logger.Debug("cache hit", zap.String("name", name), zap.String("key", key))
			}
			return result, nil
		}
		// 反序列化失败，按 miss 处理
		atomic.AddInt64(&o.errors, 1)
	}

	// 2. 缓存未命中，使用 singleflight 防止击穿
	atomic.AddInt64(&o.misses, 1)
	if o.logger != nil {
		o.logger.Debug("cache miss", zap.String("name", name), zap.String("key", key))
	}

	result, err, _ := o.sf.Do(key, func() (any, error) {
		// Double-check：再次检查缓存
		if data, err := store.Get(ctx, key); err == nil {
			var result any
			if err := o.serializer.Deserialize(data, &result); err == nil {
				return result, nil
			}
		}

		// 调用 loader
		result, err := loader(ctx, args...)
		if err != nil {
			return nil, err
		}

		// 写入缓存
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

// Invalidate 手动失效指定缓存
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

// InvalidateByPattern 按模式失效
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

// Stats 获取缓存统计信息
func (o *DefaultOrchestrator) Stats() *CacheStats {
	return &CacheStats{
		Hits:        atomic.LoadInt64(&o.hits),
		Misses:      atomic.LoadInt64(&o.misses),
		Invalidates: atomic.LoadInt64(&o.invalidates),
		Errors:      atomic.LoadInt64(&o.errors),
		ByName:      make(map[string]int64),
	}
}

// getStoreForCacheable 获取缓存项对应的存储后端
func (o *DefaultOrchestrator) getStoreForCacheable(config *CacheableConfig) (Store, error) {
	storeName := config.Store
	if storeName == "" {
		storeName = o.config.DefaultStore
	}
	return o.GetStore(storeName)
}

// buildKey 构建缓存 Key
func (o *DefaultOrchestrator) buildKey(pattern string, args ...any) string {
	result := pattern
	for i, arg := range args {
		placeholder := fmt.Sprintf("{%d}", i)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", arg))
	}
	// 处理 {hash} 占位符
	if strings.Contains(result, "{hash}") {
		hash := hashArgs(args...)
		result = strings.ReplaceAll(result, "{hash}", hash)
	}
	return result
}

// hashArgs 计算参数哈希
func hashArgs(args ...any) string {
	// 简单实现：拼接参数字符串
	var sb strings.Builder
	for _, arg := range args {
		sb.WriteString(fmt.Sprintf("%v", arg))
	}
	// 返回简单哈希（实际应使用 MD5/SHA1）
	s := sb.String()
	if len(s) > 32 {
		return s[:32]
	}
	return s
}

// subscribeInvalidationEvents 订阅失效事件
func (o *DefaultOrchestrator) subscribeInvalidationEvents() {
	for _, rule := range o.config.InvalidationRules {
		rule := rule // capture
		o.dispatcher.Subscribe(rule.Event, o.createInvalidationHandler(rule))
		if o.logger != nil {
			o.logger.Debug("subscribed invalidation event", zap.String("event", rule.Event))
		}
	}
}

// createInvalidationHandler 创建失效事件处理器
func (o *DefaultOrchestrator) createInvalidationHandler(rule InvalidationRule) event.Listener {
	return event.ListenerFunc(func(ctx context.Context, e event.Event) error {
		for _, cacheableName := range rule.Invalidate {
			if rule.Pattern != "" {
				// 按模式失效
				o.InvalidateByPattern(ctx, cacheableName, rule.Pattern)
			} else {
				// 尝试从事件中提取参数
				// 这里简化处理，实际需要反射提取字段
				if rule.KeyExtract != "" {
					o.InvalidateByPattern(ctx, cacheableName, rule.KeyExtract)
				}
			}
		}
		return nil
	})
}

// SetSerializer 设置序列化器
func (o *DefaultOrchestrator) SetSerializer(s Serializer) {
	o.serializer = s
}

// Close 关闭编排中心
func (o *DefaultOrchestrator) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	for _, store := range o.stores {
		store.Close()
	}
	return nil
}
