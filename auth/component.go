package auth

import (
	"context"
	"fmt"

	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/KOMKZ/go-yogan-framework/redis"
	"github.com/KOMKZ/go-yogan-framework/registry"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Component 认证组件
type Component struct {
	config          *Config
	logger          *logger.CtxZapLogger
	authService     *AuthService // 认证服务
	passwordService *PasswordService
	attemptStore    LoginAttemptStore
	providers       map[string]AuthProvider // 认证提供者映射
	redisComponent  *redis.Component        // Redis 组件依赖（可选）
	registry        *registry.Registry      // 注册中心
}

// NewComponent 创建认证组件
func NewComponent() *Component {
	return &Component{
		providers: make(map[string]AuthProvider),
	}
}

// Name 组件名称
func (c *Component) Name() string {
	return component.ComponentAuth
}

// DependsOn 依赖的组件
func (c *Component) DependsOn() []string {
	return []string{
		component.ComponentConfig,
		component.ComponentLogger,
		"optional:" + component.ComponentRedis, // 可选依赖（login_attempt.storage=redis 时需要）
	}
}

// Init 初始化组件
func (c *Component) Init(ctx context.Context, loader component.ConfigLoader) error {
	// 加载配置
	c.config = &Config{}
	if err := loader.Unmarshal("auth", c.config); err != nil {
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
		return fmt.Errorf("auth config validation failed: %w", err)
	}

	// 获取 Logger
	c.logger = logger.GetLogger("yogan")

	c.logger.InfoCtx(ctx, "auth component initialized")

	return nil
}

// Start 启动组件
func (c *Component) Start(ctx context.Context) error {
	if !c.config.Enabled {
		return nil
	}

	// 1. 创建密码服务
	if c.config.Password.Enabled {
		c.passwordService = NewPasswordService(
			c.config.Password.Policy,
			c.config.Password.BcryptCost,
		)
		c.logger.InfoCtx(ctx, "password service created")
	}

	// 2. 创建登录尝试存储
	if c.config.LoginAttempt.Enabled {
		var redisClient *goredis.Client
		if c.config.LoginAttempt.Storage == "redis" {
			// 从 Redis 组件获取客户端
			if err := c.getRedisClient(); err != nil {
				return fmt.Errorf("get redis client failed: %w", err)
			}
			redisComponent := c.redisComponent
			redisClient = redisComponent.GetManager().Client("main")
		}

		store, err := createLoginAttemptStore(c.config.LoginAttempt, redisClient, c.logger)
		if err != nil {
			return fmt.Errorf("create login attempt store failed: %w", err)
		}
		c.attemptStore = store
		c.logger.InfoCtx(ctx, "login attempt store created")
	}

	// 3. 创建 AuthService
	c.authService = NewAuthService(c.logger)
	c.logger.InfoCtx(ctx, "auth service created")

	// 4. 注册认证提供者
	if err := c.registerProviders(); err != nil {
		return fmt.Errorf("register providers failed: %w", err)
	}

	c.logger.InfoCtx(ctx, "auth component started")

	return nil
}

// Stop 停止组件
func (c *Component) Stop(ctx context.Context) error {
	if !c.config.Enabled {
		return nil
	}

	// 关闭登录尝试存储
	if c.attemptStore != nil {
		if err := c.attemptStore.Close(); err != nil {
			c.logger.ErrorCtx(ctx, "failed to close login attempt store")
		}
	}

	c.logger.InfoCtx(ctx, "auth component stopped")

	return nil
}

// IsRequired 是否必需组件
func (c *Component) IsRequired() bool {
	return false // Auth 是可选组件
}

// SetRegistry 设置注册中心（由框架自动调用）
func (c *Component) SetRegistry(r *registry.Registry) {
	c.registry = r
}

// SetRedisComponent 注入 Redis Component（用于测试或手动注入）
func (c *Component) SetRedisComponent(redisComp *redis.Component) {
	c.redisComponent = redisComp
}

// GetAuthService 获取认证服务
func (c *Component) GetAuthService() *AuthService {
	return c.authService
}

// GetPasswordService 获取密码服务
func (c *Component) GetPasswordService() *PasswordService {
	return c.passwordService
}

// GetAttemptStore 获取登录尝试存储
func (c *Component) GetAttemptStore() LoginAttemptStore {
	return c.attemptStore
}

// GetProvider 获取认证提供者
func (c *Component) GetProvider(name string) (AuthProvider, bool) {
	provider, ok := c.providers[name]
	return provider, ok
}

// RegisterProvider 注册自定义认证提供者（业务层调用）
func (c *Component) RegisterProvider(provider AuthProvider) {
	c.providers[provider.Name()] = provider
	c.logger.InfoCtx(context.Background(), "custom provider registered",
		zap.String("provider", provider.Name()))
}

// GetConfig 获取配置
func (c *Component) GetConfig() *Config {
	return c.config
}

// getRedisClient 从 Redis 组件获取客户端
func (c *Component) getRedisClient() error {
	// 如果没有手动注入，从 Registry 获取
	if c.redisComponent == nil {
		if c.registry == nil {
			return fmt.Errorf("registry not set")
		}

		redisComp, ok := registry.GetTyped[*redis.Component](c.registry, component.ComponentRedis)
		if !ok {
			return fmt.Errorf("redis component not found or type mismatch")
		}
		c.redisComponent = redisComp
	}

	return nil
}

// registerProviders 注册内置认证提供者
func (c *Component) registerProviders() error {
	// 注册密码认证提供者
	if c.config.IsProviderEnabled("password") && c.passwordService != nil {
		// 注意：PasswordAuthProvider 需要 UserRepository，由业务层注入
		// 这里只预留接口，实际创建由业务层调用 RegisterProvider
		c.logger.InfoCtx(context.Background(), "password auth provider enabled (needs UserRepository)")
	}

	// 未来扩展：OAuth2、API Key 等

	return nil
}
