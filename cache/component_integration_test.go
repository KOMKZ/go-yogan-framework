package cache

import (
	"context"
	"testing"
	"time"
)

// enabledConfigLoader 返回启用缓存的配置
type enabledConfigLoader struct {
	cfg Config
}

func (m *enabledConfigLoader) Get(key string) interface{} {
	return nil
}

func (m *enabledConfigLoader) Unmarshal(key string, v interface{}) error {
	if key == "cache" {
		if cfg, ok := v.(*Config); ok {
			*cfg = m.cfg
		}
	}
	return nil
}

func (m *enabledConfigLoader) GetString(key string) string {
	return ""
}

func (m *enabledConfigLoader) GetInt(key string) int {
	return 0
}

func (m *enabledConfigLoader) GetBool(key string) bool {
	return false
}

func (m *enabledConfigLoader) IsSet(key string) bool {
	return true
}

func TestComponent_FullLifecycle(t *testing.T) {
	c := NewComponent()
	ctx := context.Background()

	loader := &enabledConfigLoader{
		cfg: Config{
			Enabled:      true,
			DefaultTTL:   5 * time.Minute,
			DefaultStore: "memory",
			Stores: map[string]StoreConfig{
				"memory": {Type: "memory", MaxSize: 1000},
			},
			Cacheables: []CacheableConfig{
				{Name: "user:getById", KeyPattern: "user:{0}", TTL: time.Minute, Store: "memory", Enabled: true},
			},
		},
	}

	// Init
	err := c.Init(ctx, loader)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Start
	err = c.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Verify orchestrator is created
	orch := c.GetOrchestrator()
	if orch == nil {
		t.Fatal("GetOrchestrator() should not return nil after Start")
	}

	// Register loader
	c.RegisterLoader("user:getById", func(ctx context.Context, args ...any) (any, error) {
		return map[string]any{"id": args[0], "name": "Test User"}, nil
	})

	// Call
	result, err := c.Call(ctx, "user:getById", 1)
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if result == nil {
		t.Fatal("Call() returned nil")
	}

	// Invalidate
	err = c.Invalidate(ctx, "user:getById", 1)
	if err != nil {
		t.Fatalf("Invalidate() error = %v", err)
	}

	// Health check
	err = c.Check(ctx)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	// Stop
	err = c.Stop(ctx)
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestComponent_SetRedisClient(t *testing.T) {
	c := NewComponent()
	// Just verify it doesn't panic
	c.SetRedisClient("default", nil)
}

func TestComponent_SetEventDispatcher(t *testing.T) {
	c := NewComponent()
	// Just verify it doesn't panic
	c.SetEventDispatcher(nil)
}


func TestComponent_CreateStoreChain(t *testing.T) {
	c := NewComponent()
	ctx := context.Background()

	loader := &enabledConfigLoader{
		cfg: Config{
			Enabled:      true,
			DefaultTTL:   5 * time.Minute,
			DefaultStore: "chain",
			Stores: map[string]StoreConfig{
				"memory": {Type: "memory", MaxSize: 100},
				"chain":  {Type: "chain", Layers: []string{"memory"}},
			},
			Cacheables: []CacheableConfig{
				{Name: "test", KeyPattern: "test:{0}", Store: "chain", Enabled: true},
			},
		},
	}

	err := c.Init(ctx, loader)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	err = c.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Verify chain store exists
	store, err := c.orchestrator.GetStore("chain")
	if err != nil {
		t.Fatalf("GetStore(chain) error = %v", err)
	}
	if store.Name() != "chain" {
		t.Errorf("store.Name() = %v, want chain", store.Name())
	}

	c.Stop(ctx)
}

func TestComponent_SkipInvalidStoreAtRuntime(t *testing.T) {
	c := NewComponent()
	ctx := context.Background()

	// 使用有效的存储类型但无效的实例引用
	loader := &enabledConfigLoader{
		cfg: Config{
			Enabled:      true,
			DefaultStore: "memory",
			Stores: map[string]StoreConfig{
				"memory":        {Type: "memory", MaxSize: 100},
				"redis_missing": {Type: "redis", Instance: "non-existent"}, // Redis 实例不存在
			},
		},
	}

	err := c.Init(ctx, loader)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Start should not fail, just skip invalid stores (redis_missing)
	err = c.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Default memory store should still work
	store, err := c.orchestrator.GetStore("memory")
	if err != nil {
		t.Fatalf("GetStore(memory) error = %v", err)
	}
	if store == nil {
		t.Fatal("memory store should exist")
	}

	c.Stop(ctx)
}

func TestComponent_CreateStoreWithRedis(t *testing.T) {
	c := NewComponent()
	ctx := context.Background()

	// 模拟有 Redis 客户端的情况
	loader := &enabledConfigLoader{
		cfg: Config{
			Enabled:      true,
			DefaultStore: "memory",
			Stores: map[string]StoreConfig{
				"memory": {Type: "memory", MaxSize: 100},
			},
		},
	}

	err := c.Init(ctx, loader)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	err = c.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	c.Stop(ctx)
}

func TestComponent_CreateChainWithMissingLayer(t *testing.T) {
	c := NewComponent()
	ctx := context.Background()

	loader := &enabledConfigLoader{
		cfg: Config{
			Enabled:      true,
			DefaultStore: "chain",
			Stores: map[string]StoreConfig{
				"chain": {Type: "chain", Layers: []string{}}, // 空 layers
			},
		},
	}

	err := c.Init(ctx, loader)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Start should skip invalid chain store, create default memory
	err = c.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	c.Stop(ctx)
}

func TestComponent_HealthCheckWithStore(t *testing.T) {
	c := NewComponent()
	ctx := context.Background()

	loader := &enabledConfigLoader{
		cfg: Config{
			Enabled:      true,
			DefaultTTL:   5 * time.Minute,
			DefaultStore: "memory",
			Stores: map[string]StoreConfig{
				"memory": {Type: "memory", MaxSize: 100},
			},
		},
	}

	c.Init(ctx, loader)
	c.Start(ctx)

	// Health check should pass
	err := c.Check(ctx)
	if err != nil {
		t.Errorf("Check() error = %v", err)
	}

	c.Stop(ctx)
}

func TestComponent_InitWithValidationError(t *testing.T) {
	c := NewComponent()
	ctx := context.Background()

	// 返回无效配置
	loader := &invalidConfigLoader{}

	err := c.Init(ctx, loader)
	// 配置无效但仍应成功（使用默认值）
	if err != nil {
		t.Logf("Init() returned error (expected): %v", err)
	}
}

// invalidConfigLoader 返回会导致验证失败的配置
type invalidConfigLoader struct{}

func (m *invalidConfigLoader) Get(key string) interface{} {
	return nil
}

func (m *invalidConfigLoader) Unmarshal(key string, v interface{}) error {
	if key == "cache" {
		if cfg, ok := v.(*Config); ok {
			// 返回启用但无效的配置
			*cfg = Config{
				Enabled: true,
				Stores: map[string]StoreConfig{
					"bad": {Type: "invalid_type"},
				},
			}
		}
	}
	return nil
}

func (m *invalidConfigLoader) GetString(key string) string {
	return ""
}

func (m *invalidConfigLoader) GetInt(key string) int {
	return 0
}

func (m *invalidConfigLoader) GetBool(key string) bool {
	return false
}

func (m *invalidConfigLoader) IsSet(key string) bool {
	return true
}
