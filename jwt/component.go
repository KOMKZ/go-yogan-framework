package jwt

import (
	"context"
	"fmt"

	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/KOMKZ/go-yogan-framework/redis"
)

// Component JWT 组件
type Component struct {
	config         *Config
	logger         *logger.CtxZapLogger
	tokenStore     TokenStore
	tokenManager   TokenManager
	redisComponent *redis.Component // Redis 组件依赖（blacklist.storage=redis 时需外部注入）
}

// NewComponent 创建 JWT 组件
func NewComponent() *Component {
	return &Component{}
}

// Name 组件名称
func (c *Component) Name() string {
	return component.ComponentJWT
}

// DependsOn 依赖的组件
func (c *Component) DependsOn() []string {
	return []string{
		component.ComponentConfig,
		component.ComponentLogger,
		component.ComponentRedis, // 可选依赖（blacklist.storage=redis 时需要）
	}
}

// Init 初始化组件
func (c *Component) Init(ctx context.Context, loader component.ConfigLoader) error {
	// 加载配置
	c.config = &Config{}
	if err := loader.Unmarshal("jwt", c.config); err != nil {
		// 配置不存在，使用默认配置
		c.config.Enabled = false
		c.config.ApplyDefaults()
	} else {
		c.config.ApplyDefaults()
	}

	if !c.config.Enabled {
		return nil
	}

	// 验证配置
	if err := c.config.Validate(); err != nil {
		return fmt.Errorf("jwt config validation failed: %w", err)
	}

	// 获取 Logger
	c.logger = logger.GetLogger("yogan")

	c.logger.InfoCtx(context.Background(), "jwt component initialized")

	return nil
}

// Start 启动组件（符合 component.Component 接口）
func (c *Component) Start(ctx context.Context) error {
	if !c.config.Enabled {
		return nil
	}

	// 创建 TokenStore（使用已注入的 registry）
	if err := c.createTokenStore(); err != nil {
		return fmt.Errorf("create token store failed: %w", err)
	}

	// 创建 TokenManager
	tokenManager, err := NewTokenManager(c.config, c.tokenStore, c.logger)
	if err != nil {
		return fmt.Errorf("create token manager failed: %w", err)
	}
	c.tokenManager = tokenManager

	c.logger.InfoCtx(ctx, "jwt component started")

	return nil
}

// Stop 停止组件
func (c *Component) Stop(ctx context.Context) error {
	if !c.config.Enabled {
		return nil
	}

	// 关闭 TokenStore
	if c.tokenStore != nil {
		if err := c.tokenStore.Close(); err != nil {
			c.logger.ErrorCtx(ctx, "failed to close token store")
		}
	}

	c.logger.InfoCtx(ctx, "jwt component stopped")

	return nil
}

// IsRequired 是否必需组件
func (c *Component) IsRequired() bool {
	return false // JWT 是可选组件
}

// GetTokenManager 获取 TokenManager
func (c *Component) GetTokenManager() TokenManager {
	return c.tokenManager
}

// SetRedisComponent 注入 Redis Component
// 当 blacklist.storage=redis 时必须调用此方法注入 Redis 组件
func (c *Component) SetRedisComponent(redisComp *redis.Component) {
	c.redisComponent = redisComp
}

// GetConfig 获取配置
func (c *Component) GetConfig() *Config {
	return c.config
}

// createTokenStore 创建 TokenStore
func (c *Component) createTokenStore() error {
	if !c.config.Blacklist.Enabled {
		// 不启用黑名单，使用空实现
		c.tokenStore = NewMemoryTokenStore(0, c.logger)
		return nil
	}

	switch c.config.Blacklist.Storage {
	case "redis":
		return c.createRedisTokenStore()
	case "memory":
		return c.createMemoryTokenStore()
	default:
		return fmt.Errorf("unsupported blacklist storage: %s", c.config.Blacklist.Storage)
	}
}

// createRedisTokenStore 创建 Redis TokenStore
func (c *Component) createRedisTokenStore() error {
	if c.redisComponent == nil {
		return fmt.Errorf("redis component not set, call SetRedisComponent first")
	}

	// 获取 Redis Client
	redisManager := c.redisComponent.GetManager()
	client := redisManager.Client("main")

	// 创建 RedisTokenStore
	c.tokenStore = NewRedisTokenStore(client, c.config.Blacklist.RedisKeyPrefix, c.logger)

	c.logger.InfoCtx(context.Background(), "redis token store created")

	return nil
}

// createMemoryTokenStore 创建 Memory TokenStore
func (c *Component) createMemoryTokenStore() error {
	c.tokenStore = NewMemoryTokenStore(c.config.Blacklist.CleanupInterval, c.logger)

	c.logger.InfoCtx(context.Background(), "memory token store created")

	return nil
}
