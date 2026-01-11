package limiter

import (
	"context"
	"fmt"

	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/KOMKZ/go-yogan-framework/logger"
	rediscomp "github.com/KOMKZ/go-yogan-framework/redis"
	"github.com/KOMKZ/go-yogan-framework/registry"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Component 限流组件
//
// 实现 component.Component 接口，提供限流管理能力
// 依赖：config, logger, redis
type Component struct {
	manager  *Manager
	config   Config
	registry *registry.Registry // 🎯 使用具体类型，支持泛型方法
}

// NewComponent 创建限流组件
func NewComponent() *Component {
	return &Component{}
}

// Name 组件名称
func (c *Component) Name() string {
	return component.ComponentLimiter
}

// DependsOn 限流组件依赖配置和日志组件
// 注意：redis 依赖是可选的，仅在使用 redis 存储时需要
// 用户需要根据配置决定是否注册 redis 组件
func (c *Component) DependsOn() []string {
	// 基础依赖
	return []string{component.ComponentConfig, component.ComponentLogger, component.ComponentRedis}
}

// Init 初始化限流管理器
//
// 🎯 从 ConfigLoader 读取配置
func (c *Component) Init(ctx context.Context, loader component.ConfigLoader) error {
	ctxLogger := logger.GetLogger("yogan")
	ctxLogger.DebugCtx(ctx, "🔧 限流组件开始初始化...")

	// 直接从 ConfigLoader 读取限流配置
	var cfg Config
	if err := loader.Unmarshal("limiter", &cfg); err != nil {
		ctxLogger.DebugCtx(ctx, "未配置限流器，跳过初始化")
		return nil
	}

	// 如果未启用，跳过初始化
	if !cfg.Enabled {
		ctxLogger.DebugCtx(ctx, "⏭️  限流器未启用")
		return nil
	}

	ctxLogger.DebugCtx(ctx, "✅ 读取配置成功",
		zap.Bool("enabled", cfg.Enabled),
		zap.String("store_type", cfg.StoreType))

	// 保存配置，延迟到 Start 创建 Manager
	c.config = cfg

	ctxLogger.DebugCtx(ctx, "✅ 限流器配置已加载",
		zap.String("store_type", cfg.StoreType),
		zap.Int("resources", len(cfg.Resources)))
	return nil
}

// Start 启动限流组件
// 在此创建 Manager，并从 Registry 获取 Redis 客户端（如果需要）
func (c *Component) Start(ctx context.Context) error {
	// 如果配置未加载或未启用，跳过
	if !c.config.Enabled {
		return nil
	}

	ctxLogger := logger.GetLogger("yogan")

	// 如果使用 redis 存储，从 Registry 获取 Redis 组件
	var redisClient *redis.Client
	if c.config.StoreType == string(StoreTypeRedis) {
		if c.registry == nil {
			return fmt.Errorf("Registry 未注入，无法获取 Redis 组件")
		}

		// 🎯 使用泛型函数，一步到位获取类型化组件
		redisComp, ok := registry.GetTyped[*rediscomp.Component](c.registry, component.ComponentRedis)
		if !ok {
			return fmt.Errorf("使用 redis 存储但未注册 redis 组件或类型错误，请先调用 app.Register(redis.NewComponent())")
		}

		// 直接使用，无需再次类型断言
		redisManager := redisComp.GetManager()
		if redisManager == nil {
			return fmt.Errorf("RedisManager 未初始化")
		}

		// 获取指定实例的客户端
		redisClient = redisManager.Client(c.config.Redis.Instance)
		if redisClient == nil {
			return fmt.Errorf("Redis 实例 '%s' 不存在，请在 redis.instances 中配置", c.config.Redis.Instance)
		}

		ctxLogger.DebugCtx(ctx, "✅ 从 Registry 获取 Redis 客户端成功（泛型方法）",
			zap.String("instance", c.config.Redis.Instance),
			zap.String("key_prefix", c.config.Redis.KeyPrefix))
	}

	// 创建限流管理器（provider 传 nil，仅自适应算法需要）
	manager, err := NewManagerWithLogger(c.config, ctxLogger, redisClient, nil)
	if err != nil {
		return fmt.Errorf("创建限流管理器失败: %w", err)
	}

	c.manager = manager
	ctxLogger.DebugCtx(ctx, "✅ 限流器启动成功",
		zap.String("store_type", c.config.StoreType))
	return nil
}

// SetRegistry 设置 Registry 引用（由 Registry.Register 自动调用）
func (c *Component) SetRegistry(r *registry.Registry) {
	c.registry = r
}

// Stop 停止限流组件（关闭资源）
func (c *Component) Stop(ctx context.Context) error {
	if c.manager != nil {
		if err := c.manager.Close(); err != nil {
			return fmt.Errorf("关闭限流器失败: %w", err)
		}
	}
	return nil
}

// GetManager 获取限流管理器
func (c *Component) GetManager() *Manager {
	return c.manager
}
