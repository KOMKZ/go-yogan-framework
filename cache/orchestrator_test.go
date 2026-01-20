package cache

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
)

func TestOrchestrator_RegisterLoader(t *testing.T) {
	cfg := &Config{Enabled: true}
	o := NewOrchestrator(cfg, nil, nil)

	loader := func(ctx context.Context, args ...any) (any, error) {
		return "loaded", nil
	}

	o.RegisterLoader("test:get", loader)

	// Verify loader is registered
	o.mu.RLock()
	_, ok := o.loaders["test:get"]
	o.mu.RUnlock()

	if !ok {
		t.Error("Loader not registered")
	}
}

func TestOrchestrator_RegisterStore(t *testing.T) {
	cfg := &Config{Enabled: true}
	o := NewOrchestrator(cfg, nil, nil)

	store := NewMemoryStore("test", 100)
	o.RegisterStore("memory", store)

	s, err := o.GetStore("memory")
	if err != nil {
		t.Errorf("GetStore() error = %v", err)
	}
	if s.Name() != "test" {
		t.Errorf("GetStore().Name() = %v, want test", s.Name())
	}
}

func TestOrchestrator_GetStoreNotFound(t *testing.T) {
	cfg := &Config{Enabled: true}
	o := NewOrchestrator(cfg, nil, nil)

	_, err := o.GetStore("non-existent")
	if err == nil {
		t.Error("GetStore() expected error for non-existent store")
	}
}

func TestOrchestrator_Call(t *testing.T) {
	cfg := &Config{
		Enabled:      true,
		DefaultTTL:   5 * time.Minute,
		DefaultStore: "memory",
		Cacheables: []CacheableConfig{
			{Name: "user:getById", KeyPattern: "user:{0}", TTL: time.Minute, Store: "memory", Enabled: true},
		},
	}

	o := NewOrchestrator(cfg, nil, nil)
	o.RegisterStore("memory", NewMemoryStore("memory", 100))

	var loadCount int32
	o.RegisterLoader("user:getById", func(ctx context.Context, args ...any) (any, error) {
		atomic.AddInt32(&loadCount, 1)
		id := args[0].(int)
		return map[string]any{"id": id, "name": "User"}, nil
	})

	ctx := context.Background()

	t.Run("first call loads from loader", func(t *testing.T) {
		result, err := o.Call(ctx, "user:getById", 1)
		if err != nil {
			t.Errorf("Call() error = %v", err)
		}
		if result == nil {
			t.Error("Call() returned nil")
		}
		if atomic.LoadInt32(&loadCount) != 1 {
			t.Errorf("loadCount = %d, want 1", loadCount)
		}
	})

	t.Run("second call hits cache", func(t *testing.T) {
		_, err := o.Call(ctx, "user:getById", 1)
		if err != nil {
			t.Errorf("Call() error = %v", err)
		}
		if atomic.LoadInt32(&loadCount) != 1 {
			t.Errorf("loadCount = %d, want 1 (should hit cache)", loadCount)
		}
	})

	t.Run("different args loads again", func(t *testing.T) {
		_, err := o.Call(ctx, "user:getById", 2)
		if err != nil {
			t.Errorf("Call() error = %v", err)
		}
		if atomic.LoadInt32(&loadCount) != 2 {
			t.Errorf("loadCount = %d, want 2", loadCount)
		}
	})
}

func TestOrchestrator_CallCacheableNotFound(t *testing.T) {
	cfg := &Config{Enabled: true}
	o := NewOrchestrator(cfg, nil, nil)
	o.RegisterStore("memory", NewMemoryStore("memory", 100))

	_, err := o.Call(context.Background(), "non-existent", 1)
	if err == nil {
		t.Error("Call() expected error for non-existent cacheable")
	}
}

func TestOrchestrator_CallLoaderNotFound(t *testing.T) {
	cfg := &Config{
		Enabled: true,
		Cacheables: []CacheableConfig{
			{Name: "test", KeyPattern: "test:{0}", Enabled: true},
		},
	}
	o := NewOrchestrator(cfg, nil, nil)

	_, err := o.Call(context.Background(), "test", 1)
	if err == nil {
		t.Error("Call() expected error for unregistered loader")
	}
}

func TestOrchestrator_CallLoaderError(t *testing.T) {
	cfg := &Config{
		Enabled:      true,
		DefaultStore: "memory",
		Cacheables: []CacheableConfig{
			{Name: "test", KeyPattern: "test:{0}", Store: "memory", Enabled: true},
		},
	}
	o := NewOrchestrator(cfg, nil, nil)
	o.RegisterStore("memory", NewMemoryStore("memory", 100))
	o.RegisterLoader("test", func(ctx context.Context, args ...any) (any, error) {
		return nil, errors.New("loader error")
	})

	_, err := o.Call(context.Background(), "test", 1)
	if err == nil {
		t.Error("Call() expected error from loader")
	}
}

func TestOrchestrator_CallDisabled(t *testing.T) {
	cfg := &Config{
		Enabled: false,
		Cacheables: []CacheableConfig{
			{Name: "test", KeyPattern: "test:{0}", Enabled: true},
		},
	}
	o := NewOrchestrator(cfg, nil, nil)

	var loadCount int32
	o.RegisterLoader("test", func(ctx context.Context, args ...any) (any, error) {
		atomic.AddInt32(&loadCount, 1)
		return "result", nil
	})

	// Call twice
	o.Call(context.Background(), "test", 1)
	o.Call(context.Background(), "test", 1)

	// Should call loader both times (no caching)
	if atomic.LoadInt32(&loadCount) != 2 {
		t.Errorf("loadCount = %d, want 2 (cache disabled)", loadCount)
	}
}

func TestOrchestrator_Invalidate(t *testing.T) {
	cfg := &Config{
		Enabled:      true,
		DefaultStore: "memory",
		Cacheables: []CacheableConfig{
			{Name: "user:getById", KeyPattern: "user:{0}", Store: "memory", Enabled: true},
		},
	}
	o := NewOrchestrator(cfg, nil, nil)
	o.RegisterStore("memory", NewMemoryStore("memory", 100))

	var loadCount int32
	o.RegisterLoader("user:getById", func(ctx context.Context, args ...any) (any, error) {
		atomic.AddInt32(&loadCount, 1)
		return "user", nil
	})

	ctx := context.Background()

	// First call
	o.Call(ctx, "user:getById", 1)
	if atomic.LoadInt32(&loadCount) != 1 {
		t.Errorf("loadCount = %d, want 1", loadCount)
	}

	// Second call (cached)
	o.Call(ctx, "user:getById", 1)
	if atomic.LoadInt32(&loadCount) != 1 {
		t.Errorf("loadCount = %d, want 1 (cached)", loadCount)
	}

	// Invalidate
	err := o.Invalidate(ctx, "user:getById", 1)
	if err != nil {
		t.Errorf("Invalidate() error = %v", err)
	}

	// Third call (should reload)
	o.Call(ctx, "user:getById", 1)
	if atomic.LoadInt32(&loadCount) != 2 {
		t.Errorf("loadCount = %d, want 2 (after invalidate)", loadCount)
	}
}

func TestOrchestrator_InvalidateByPattern(t *testing.T) {
	cfg := &Config{
		Enabled:      true,
		DefaultStore: "memory",
		Cacheables: []CacheableConfig{
			{Name: "user:getById", KeyPattern: "user:{0}", Store: "memory", Enabled: true},
		},
	}
	o := NewOrchestrator(cfg, nil, nil)
	o.RegisterStore("memory", NewMemoryStore("memory", 100))

	var loadCount int32
	o.RegisterLoader("user:getById", func(ctx context.Context, args ...any) (any, error) {
		atomic.AddInt32(&loadCount, 1)
		return "user", nil
	})

	ctx := context.Background()

	// Cache multiple users
	o.Call(ctx, "user:getById", 1)
	o.Call(ctx, "user:getById", 2)
	o.Call(ctx, "user:getById", 3)
	if atomic.LoadInt32(&loadCount) != 3 {
		t.Errorf("loadCount = %d, want 3", loadCount)
	}

	// Invalidate by pattern
	err := o.InvalidateByPattern(ctx, "user:getById", "user:")
	if err != nil {
		t.Errorf("InvalidateByPattern() error = %v", err)
	}

	// All should reload
	o.Call(ctx, "user:getById", 1)
	o.Call(ctx, "user:getById", 2)
	if atomic.LoadInt32(&loadCount) != 5 {
		t.Errorf("loadCount = %d, want 5 (after pattern invalidate)", loadCount)
	}
}

func TestOrchestrator_Stats(t *testing.T) {
	cfg := &Config{
		Enabled:      true,
		DefaultStore: "memory",
		Cacheables: []CacheableConfig{
			{Name: "test", KeyPattern: "test:{0}", Store: "memory", Enabled: true},
		},
	}
	o := NewOrchestrator(cfg, nil, nil)
	o.RegisterStore("memory", NewMemoryStore("memory", 100))
	o.RegisterLoader("test", func(ctx context.Context, args ...any) (any, error) {
		return "result", nil
	})

	ctx := context.Background()

	// First call (miss)
	o.Call(ctx, "test", 1)
	// Second call (hit)
	o.Call(ctx, "test", 1)
	// Invalidate
	o.Invalidate(ctx, "test", 1)

	stats := o.Stats()
	if stats.Hits != 1 {
		t.Errorf("Stats.Hits = %d, want 1", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Stats.Misses = %d, want 1", stats.Misses)
	}
	if stats.Invalidates != 1 {
		t.Errorf("Stats.Invalidates = %d, want 1", stats.Invalidates)
	}
}

func TestOrchestrator_BuildKey(t *testing.T) {
	cfg := &Config{Enabled: true}
	o := NewOrchestrator(cfg, nil, nil)

	tests := []struct {
		pattern string
		args    []any
		want    string
	}{
		{"user:{0}", []any{123}, "user:123"},
		{"order:{0}:{1}", []any{1, 2}, "order:1:2"},
		{"simple", nil, "simple"},
		{"complex:{0}:detail:{1}", []any{"abc", 456}, "complex:abc:detail:456"},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			result := o.buildKey(tt.pattern, tt.args...)
			if result != tt.want {
				t.Errorf("buildKey(%s, %v) = %s, want %s", tt.pattern, tt.args, result, tt.want)
			}
		})
	}
}

func TestOrchestrator_Close(t *testing.T) {
	cfg := &Config{Enabled: true}
	o := NewOrchestrator(cfg, nil, nil)
	o.RegisterStore("memory", NewMemoryStore("memory", 100))

	err := o.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestOrchestrator_SetSerializer(t *testing.T) {
	cfg := &Config{Enabled: true}
	o := NewOrchestrator(cfg, nil, nil)

	newSerializer := NewJSONSerializer()
	o.SetSerializer(newSerializer)

	if o.serializer != newSerializer {
		t.Error("SetSerializer() did not update serializer")
	}
}

func TestOrchestrator_BuildKeyWithHash(t *testing.T) {
	cfg := &Config{Enabled: true}
	o := NewOrchestrator(cfg, nil, nil)

	result := o.buildKey("key:{hash}", "arg1", "arg2")
	if result == "key:{hash}" {
		t.Error("buildKey should replace {hash} placeholder")
	}
	if result == "" {
		t.Error("buildKey should not return empty string")
	}
}

func TestOrchestrator_InvalidateCacheableNotFound(t *testing.T) {
	cfg := &Config{Enabled: true}
	o := NewOrchestrator(cfg, nil, nil)

	err := o.Invalidate(context.Background(), "non-existent")
	if err == nil {
		t.Error("Invalidate() expected error for non-existent cacheable")
	}
}

func TestOrchestrator_InvalidateByPatternCacheableNotFound(t *testing.T) {
	cfg := &Config{Enabled: true}
	o := NewOrchestrator(cfg, nil, nil)

	err := o.InvalidateByPattern(context.Background(), "non-existent", "pattern")
	if err == nil {
		t.Error("InvalidateByPattern() expected error for non-existent cacheable")
	}
}

func TestOrchestrator_CallStoreNotFound(t *testing.T) {
	cfg := &Config{
		Enabled: true,
		Cacheables: []CacheableConfig{
			{Name: "test", KeyPattern: "test:{0}", Store: "non-existent", Enabled: true},
		},
	}
	o := NewOrchestrator(cfg, nil, nil)

	var loadCount int32
	o.RegisterLoader("test", func(ctx context.Context, args ...any) (any, error) {
		loadCount++
		return "result", nil
	})

	// Should fallback to loader when store not found
	result, err := o.Call(context.Background(), "test", 1)
	if err != nil {
		t.Errorf("Call() error = %v", err)
	}
	if result != "result" {
		t.Errorf("Call() = %v, want result", result)
	}
	if loadCount != 1 {
		t.Errorf("loadCount = %d, want 1", loadCount)
	}
}

func TestOrchestrator_CallCacheableItemDisabled(t *testing.T) {
	// NewOrchestrator internally calls ApplyDefaults, which changes Enabled: false to true
	// So the internal cacheables of the orchestrator need to be modified directly after creation here.
	cfg := &Config{
		Enabled:      true,
		DefaultStore: "memory",
		Cacheables: []CacheableConfig{
			{Name: "test", KeyPattern: "test:{0}", Store: "memory", Enabled: true},
		},
	}

	o := NewOrchestrator(cfg, nil, nil)
	o.RegisterStore("memory", NewMemoryStore("memory", 100))

	// Disable cache item after creation (since it is a pointer reference, direct modification is possible)
	o.cacheables["test"].Enabled = false

	var loadCount int32
	o.RegisterLoader("test", func(ctx context.Context, args ...any) (any, error) {
		loadCount++
		return "result", nil
	})

	// Call twice
	o.Call(context.Background(), "test", 1)
	o.Call(context.Background(), "test", 1)

	// Should call loader both times (cacheable disabled)
	if loadCount != 2 {
		t.Errorf("loadCount = %d, want 2 (cacheable disabled)", loadCount)
	}
}

func TestOrchestrator_CallWithDefaultStore(t *testing.T) {
	cfg := &Config{
		Enabled:      true,
		DefaultTTL:   time.Minute,
		DefaultStore: "memory",
		Cacheables: []CacheableConfig{
			{Name: "test", KeyPattern: "test:{0}", Enabled: true}, // Do not specify Store, use default
		},
	}

	o := NewOrchestrator(cfg, nil, nil)
	o.RegisterStore("memory", NewMemoryStore("memory", 100))
	o.RegisterLoader("test", func(ctx context.Context, args ...any) (any, error) {
		return "result", nil
	})

	result, err := o.Call(context.Background(), "test", 1)
	if err != nil {
		t.Errorf("Call() error = %v", err)
	}
	if result != "result" {
		t.Errorf("Call() = %v, want result", result)
	}
}

func TestOrchestrator_InvalidateWithDefaultStore(t *testing.T) {
	cfg := &Config{
		Enabled:      true,
		DefaultStore: "memory",
		Cacheables: []CacheableConfig{
			{Name: "test", KeyPattern: "test:{0}", Enabled: true},
		},
	}

	o := NewOrchestrator(cfg, nil, nil)
	o.RegisterStore("memory", NewMemoryStore("memory", 100))
	o.RegisterLoader("test", func(ctx context.Context, args ...any) (any, error) {
		return "result", nil
	})

	ctx := context.Background()
	o.Call(ctx, "test", 1)

	err := o.Invalidate(ctx, "test", 1)
	if err != nil {
		t.Errorf("Invalidate() error = %v", err)
	}
}

func TestOrchestrator_InvalidateByPatternWithDefaultStore(t *testing.T) {
	cfg := &Config{
		Enabled:      true,
		DefaultStore: "memory",
		Cacheables: []CacheableConfig{
			{Name: "test", KeyPattern: "test:{0}", Enabled: true},
		},
	}

	o := NewOrchestrator(cfg, nil, nil)
	o.RegisterStore("memory", NewMemoryStore("memory", 100))

	ctx := context.Background()
	err := o.InvalidateByPattern(ctx, "test", "test:")
	if err != nil {
		t.Errorf("InvalidateByPattern() error = %v", err)
	}
}

func TestOrchestrator_CallWithDefaultTTL(t *testing.T) {
	cfg := &Config{
		Enabled:      true,
		DefaultTTL:   5 * time.Minute,
		DefaultStore: "memory",
		Cacheables: []CacheableConfig{
			{Name: "test", KeyPattern: "test:{0}", TTL: 0, Enabled: true}, // TTL is 0, use default
		},
	}

	o := NewOrchestrator(cfg, nil, nil)
	o.RegisterStore("memory", NewMemoryStore("memory", 100))
	o.RegisterLoader("test", func(ctx context.Context, args ...any) (any, error) {
		return "result", nil
	})

	result, err := o.Call(context.Background(), "test", 1)
	if err != nil {
		t.Errorf("Call() error = %v", err)
	}
	if result != "result" {
		t.Errorf("Call() = %v, want result", result)
	}
}

func TestOrchestrator_CallWithLogger(t *testing.T) {
	cfg := &Config{
		Enabled:      true,
		DefaultStore: "memory",
		Cacheables: []CacheableConfig{
			{Name: "test", KeyPattern: "test:{0}", TTL: time.Minute, Enabled: true},
		},
	}

	log := logger.GetLogger("test")
	o := NewOrchestrator(cfg, nil, log)
	o.RegisterStore("memory", NewMemoryStore("memory", 100))

	var loadCount int32
	o.RegisterLoader("test", func(ctx context.Context, args ...any) (any, error) {
		atomic.AddInt32(&loadCount, 1)
		return "result", nil
	})

	ctx := context.Background()

	// First call - miss, should log
	o.Call(ctx, "test", 1)
	// Second call - hit, should log
	o.Call(ctx, "test", 1)

	if atomic.LoadInt32(&loadCount) != 1 {
		t.Errorf("loadCount = %d, want 1", loadCount)
	}
}

func TestOrchestrator_InvalidateStoreError(t *testing.T) {
	cfg := &Config{
		Enabled:      true,
		DefaultStore: "non-existent",
		Cacheables: []CacheableConfig{
			{Name: "test", KeyPattern: "test:{0}", Enabled: true},
		},
	}

	o := NewOrchestrator(cfg, nil, nil)
	// Do not register storage

	err := o.Invalidate(context.Background(), "test", 1)
	if err == nil {
		t.Error("Invalidate() expected error when store not found")
	}
}

func TestOrchestrator_InvalidateByPatternStoreError(t *testing.T) {
	cfg := &Config{
		Enabled:      true,
		DefaultStore: "non-existent",
		Cacheables: []CacheableConfig{
			{Name: "test", KeyPattern: "test:{0}", Enabled: true},
		},
	}

	o := NewOrchestrator(cfg, nil, nil)
	// Do not register storage

	err := o.InvalidateByPattern(context.Background(), "test", "test:")
	if err == nil {
		t.Error("InvalidateByPattern() expected error when store not found")
	}
}

func TestOrchestrator_InvalidateSuccess(t *testing.T) {
	cfg := &Config{
		Enabled:      true,
		DefaultStore: "memory",
		Cacheables: []CacheableConfig{
			{Name: "test", KeyPattern: "test:{0}", Store: "memory", Enabled: true},
		},
	}

	log := logger.GetLogger("test")
	o := NewOrchestrator(cfg, nil, log)
	o.RegisterStore("memory", NewMemoryStore("memory", 100))
	o.RegisterLoader("test", func(ctx context.Context, args ...any) (any, error) {
		return "result", nil
	})

	ctx := context.Background()

	// Cache a value
	o.Call(ctx, "test", 1)

	// Invalidate it
	err := o.Invalidate(ctx, "test", 1)
	if err != nil {
		t.Errorf("Invalidate() error = %v", err)
	}

	// Check stats
	stats := o.Stats()
	if stats.Invalidates != 1 {
		t.Errorf("Stats.Invalidates = %d, want 1", stats.Invalidates)
	}
}

func TestOrchestrator_InvalidateByPatternSuccess(t *testing.T) {
	cfg := &Config{
		Enabled:      true,
		DefaultStore: "memory",
		Cacheables: []CacheableConfig{
			{Name: "test", KeyPattern: "test:{0}", Store: "memory", Enabled: true},
		},
	}

	log := logger.GetLogger("test")
	o := NewOrchestrator(cfg, nil, log)
	o.RegisterStore("memory", NewMemoryStore("memory", 100))

	ctx := context.Background()

	err := o.InvalidateByPattern(ctx, "test", "test:")
	if err != nil {
		t.Errorf("InvalidateByPattern() error = %v", err)
	}

	stats := o.Stats()
	if stats.Invalidates != 1 {
		t.Errorf("Stats.Invalidates = %d, want 1", stats.Invalidates)
	}
}

func TestOrchestrator_GetStoreForCacheableWithEmptyStore(t *testing.T) {
	cfg := &Config{
		Enabled:      true,
		DefaultStore: "memory",
		Cacheables: []CacheableConfig{
			{Name: "test", KeyPattern: "test:{0}", Store: "", Enabled: true}, // empty store, use default
		},
	}

	o := NewOrchestrator(cfg, nil, nil)
	o.RegisterStore("memory", NewMemoryStore("memory", 100))
	o.RegisterLoader("test", func(ctx context.Context, args ...any) (any, error) {
		return "result", nil
	})

	result, err := o.Call(context.Background(), "test", 1)
	if err != nil {
		t.Errorf("Call() error = %v", err)
	}
	if result != "result" {
		t.Errorf("Call() = %v, want result", result)
	}
}
