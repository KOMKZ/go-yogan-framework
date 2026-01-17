package cache

import (
	"context"

	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/KOMKZ/go-yogan-framework/event"
	"github.com/KOMKZ/go-yogan-framework/logger"
	frameworkRedis "github.com/KOMKZ/go-yogan-framework/redis"
	"github.com/KOMKZ/go-yogan-framework/registry"
	"go.uber.org/zap"
)

// ComponentName 组件名称
const ComponentName = "cache"

// Component 缓存组件
type Component struct {
	config       *Config
	orchestrator *DefaultOrchestrator
	log          *logger.CtxZapLogger
	registry     *registry.Registry // 🎯 使用具体类型，支持泛型方法

	// 外部依赖（启动时从 Registry 获取）
	redisManager *frameworkRedis.Manager
	dispatcher   event.Dispatcher
}

// NewComponent 创建缓存组件
func NewComponent() *Component {
	return &Component{}
}

// Name 返回组件名称
func (c *Component) Name() string {
	return ComponentName
}

// DependsOn 依赖的组件
func (c *Component) DependsOn() []string {
	return []string{
		"config",         // 强制依赖配置
		"logger",         // 强制依赖日志
		"optional:redis", // 可选依赖 Redis
		"optional:event", // 可选依赖事件
	}
}

// Init 初始化组件
func (c *Component) Init(ctx context.Context, loader component.ConfigLoader) error {
	// 加载配置
	var cfg Config
	if err := loader.Unmarshal("cache", &cfg); err != nil {
		// 配置不存在时使用默认配置
		cfg = Config{
			Enabled:      false,
			DefaultTTL:   300, // 5 minutes
			DefaultStore: "memory",
		}
	}

	if err := cfg.Validate(); err != nil {
		return ErrConfigInvalid.Wrap(err)
	}

	c.config = &cfg
	c.log = logger.GetLogger("yogan")

	return nil
}

// Start 启动组件
func (c *Component) Start(ctx context.Context) error {
	if !c.config.Enabled {
		c.log.Info("cache component disabled")
		return nil
	}

	// 从 Registry 获取可选依赖
	if c.registry != nil {
		// 获取 Redis 组件
		if redisComp, ok := registry.GetTyped[*frameworkRedis.Component](c.registry, component.ComponentRedis); ok {
			c.redisManager = redisComp.GetManager()
			if c.redisManager != nil {
				c.log.Debug("cache component: redis manager obtained from registry")
			}
		}

		// 获取事件组件
		if eventComp, ok := registry.GetTyped[*event.Component](c.registry, component.ComponentEvent); ok {
			if eventComp.IsEnabled() {
				c.dispatcher = eventComp.GetDispatcher()
				c.log.Debug("cache component: event dispatcher obtained from registry")
			}
		}
	}

	// 创建编排中心
	c.orchestrator = NewOrchestrator(c.config, c.dispatcher, c.log)

	// 初始化存储后端
	if err := c.initStores(ctx); err != nil {
		return err
	}

	c.log.Info("cache component started")
	return nil
}

// Stop 停止组件
func (c *Component) Stop(ctx context.Context) error {
	if c.orchestrator != nil {
		c.orchestrator.Close()
	}
	c.log.Info("cache component stopped")
	return nil
}

// initStores 初始化存储后端
func (c *Component) initStores(ctx context.Context) error {
	for name, storeCfg := range c.config.Stores {
		store, err := c.createStore(name, storeCfg)
		if err != nil {
			c.log.Warn("failed to create store, skipping",
				zap.String("name", name),
				zap.Error(err),
			)
			continue
		}
		c.orchestrator.RegisterStore(name, store)
	}

	// 确保有默认存储
	if _, err := c.orchestrator.GetStore(c.config.DefaultStore); err != nil {
		// 创建默认内存存储
		memStore := NewMemoryStore("memory", 10000)
		c.orchestrator.RegisterStore("memory", memStore)
		c.log.Info("created default memory store")
	}

	return nil
}

// createStore 创建存储后端
func (c *Component) createStore(name string, cfg StoreConfig) (Store, error) {
	switch cfg.Type {
	case "memory":
		maxSize := cfg.MaxSize
		if maxSize <= 0 {
			maxSize = 10000
		}
		return NewMemoryStore(name, maxSize), nil

	case "redis":
		if c.redisManager == nil {
			return nil, ErrStoreNotFound.WithMsg("Redis Manager 未初始化")
		}
		client := c.redisManager.Client(cfg.Instance)
		if client == nil {
			return nil, ErrStoreNotFound.WithMsgf("Redis 实例未找到: %s", cfg.Instance)
		}
		return NewRedisStore(name, client, cfg.KeyPrefix), nil

	case "chain":
		var stores []Store
		for _, layerName := range cfg.Layers {
			// 先检查已创建的存储
			if store, err := c.orchestrator.GetStore(layerName); err == nil {
				stores = append(stores, store)
				continue
			}
			// 尝试从配置创建
			if layerCfg, ok := c.config.Stores[layerName]; ok {
				store, err := c.createStore(layerName, layerCfg)
				if err != nil {
					return nil, err
				}
				c.orchestrator.RegisterStore(layerName, store)
				stores = append(stores, store)
			}
		}
		if len(stores) == 0 {
			return nil, ErrConfigInvalid.WithMsgf("链式存储无有效层: %s", name)
		}
		return NewChainStore(name, stores...), nil

	default:
		return nil, ErrConfigInvalid.WithMsgf("未知的存储类型: %s", cfg.Type)
	}
}

// SetRegistry 设置注册中心（由框架调用）
func (c *Component) SetRegistry(r *registry.Registry) {
	c.registry = r
}

// SetRedisManager 设置 Redis 管理器（可选，用于测试或手动注入）
func (c *Component) SetRedisManager(manager *frameworkRedis.Manager) {
	c.redisManager = manager
}

// SetEventDispatcher 设置事件分发器（可选，用于测试或手动注入）
func (c *Component) SetEventDispatcher(dispatcher event.Dispatcher) {
	c.dispatcher = dispatcher
}

// GetOrchestrator 获取编排中心
func (c *Component) GetOrchestrator() Orchestrator {
	return c.orchestrator
}

// RegisterLoader 注册数据加载器（快捷方法）
func (c *Component) RegisterLoader(name string, loader LoaderFunc) {
	if c.orchestrator != nil {
		c.orchestrator.RegisterLoader(name, loader)
	}
}

// Call 执行缓存调用（快捷方法）
func (c *Component) Call(ctx context.Context, name string, args ...any) (any, error) {
	if c.orchestrator == nil {
		return nil, ErrConfigInvalid.WithMsg("缓存组件未初始化")
	}
	return c.orchestrator.Call(ctx, name, args...)
}

// Invalidate 失效缓存（快捷方法）
func (c *Component) Invalidate(ctx context.Context, name string, args ...any) error {
	if c.orchestrator == nil {
		return ErrConfigInvalid.WithMsg("缓存组件未初始化")
	}
	return c.orchestrator.Invalidate(ctx, name, args...)
}

// GetHealthChecker 获取健康检查器
func (c *Component) GetHealthChecker() component.HealthChecker {
	return c
}

// Check 健康检查
func (c *Component) Check(ctx context.Context) error {
	if c.orchestrator == nil {
		return nil // 未启用时视为健康
	}

	// 检查默认存储是否可用
	store, err := c.orchestrator.GetStore(c.config.DefaultStore)
	if err != nil {
		return err
	}

	// 简单 ping 测试
	testKey := "__health_check__"
	if err := store.Set(ctx, testKey, []byte("ok"), 1); err != nil {
		return err
	}
	store.Delete(ctx, testKey)

	return nil
}
