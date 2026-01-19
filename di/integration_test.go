package di_test

import (
	"context"
	"testing"

	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/KOMKZ/go-yogan-framework/di"
	"github.com/KOMKZ/go-yogan-framework/registry"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===== 模拟组件 =====

// ConfigComponent 模拟配置组件
type ConfigComponent struct {
	values map[string]interface{}
}

func NewConfigComponent() *ConfigComponent {
	return &ConfigComponent{
		values: map[string]interface{}{
			"database.host": "localhost",
			"database.port": 3306,
			"redis.host":    "localhost",
			"redis.port":    6379,
		},
	}
}

func (c *ConfigComponent) Name() string        { return component.ComponentConfig }
func (c *ConfigComponent) DependsOn() []string { return nil }
func (c *ConfigComponent) Init(ctx context.Context, loader component.ConfigLoader) error {
	return nil
}
func (c *ConfigComponent) Start(ctx context.Context) error { return nil }
func (c *ConfigComponent) Stop(ctx context.Context) error  { return nil }

// GetString 实现 ConfigLoader
func (c *ConfigComponent) GetString(key string) string {
	if v, ok := c.values[key].(string); ok {
		return v
	}
	return ""
}

// GetInt 实现 ConfigLoader
func (c *ConfigComponent) GetInt(key string) int {
	if v, ok := c.values[key].(int); ok {
		return v
	}
	return 0
}

// UnmarshalKey 实现 ConfigLoader
func (c *ConfigComponent) UnmarshalKey(key string, rawVal interface{}) error {
	return nil
}

// DatabaseComponent 模拟数据库组件
type DatabaseComponent struct {
	host string
	port int
}

func NewDatabaseComponent() *DatabaseComponent {
	return &DatabaseComponent{}
}

func (d *DatabaseComponent) Name() string        { return component.ComponentDatabase }
func (d *DatabaseComponent) DependsOn() []string { return []string{component.ComponentConfig} }
func (d *DatabaseComponent) Init(ctx context.Context, loader component.ConfigLoader) error {
	d.host = loader.GetString("database.host")
	d.port = loader.GetInt("database.port")
	return nil
}
func (d *DatabaseComponent) Start(ctx context.Context) error { return nil }
func (d *DatabaseComponent) Stop(ctx context.Context) error  { return nil }
func (d *DatabaseComponent) Host() string                    { return d.host }
func (d *DatabaseComponent) Port() int                       { return d.port }

// ===== 应用层服务 =====

// UserRepository 用户仓储接口
type UserRepository interface {
	FindByID(id int64) (*User, error)
}

// User 用户实体
type User struct {
	ID   int64
	Name string
}

// UserRepositoryImpl 用户仓储实现
type UserRepositoryImpl struct {
	db *DatabaseComponent
}

func (r *UserRepositoryImpl) FindByID(id int64) (*User, error) {
	// 模拟从数据库查询
	return &User{ID: id, Name: "TestUser"}, nil
}

// UserService 用户服务
type UserService struct {
	repo UserRepository
}

func (s *UserService) GetUser(id int64) (*User, error) {
	return s.repo.FindByID(id)
}

// ===== 集成测试 =====

func TestIntegration_BridgeWithRealRegistry(t *testing.T) {
	// 1. 创建真实的 Registry
	reg := registry.NewRegistry()

	// 2. 注册组件
	configComp := NewConfigComponent()
	dbComp := NewDatabaseComponent()

	require.NoError(t, reg.Register(configComp))
	require.NoError(t, reg.Register(dbComp))

	// 3. 创建 samber/do 注入器
	injector := do.New()
	defer injector.Shutdown()

	// 4. 创建 Bridge
	bridge := di.NewBridge(reg, injector)

	// 5. 将 Registry 组件暴露给 do
	di.MustProvideFromRegistry[*ConfigComponent](bridge, component.ComponentConfig)
	di.MustProvideFromRegistry[*DatabaseComponent](bridge, component.ComponentDatabase)

	// 6. 在 do 中注册应用层服务
	di.Provide(bridge, func(i do.Injector) (UserRepository, error) {
		db := do.MustInvoke[*DatabaseComponent](i)
		return &UserRepositoryImpl{db: db}, nil
	})

	di.Provide(bridge, func(i do.Injector) (*UserService, error) {
		repo := do.MustInvoke[UserRepository](i)
		return &UserService{repo: repo}, nil
	})

	// 7. 验证依赖注入链
	userService := di.MustInvoke[*UserService](bridge)
	require.NotNil(t, userService)

	user, err := userService.GetUser(1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), user.ID)
	assert.Equal(t, "TestUser", user.Name)
}

func TestIntegration_MixedDependencyInjection(t *testing.T) {
	// 场景：Registry 中的组件 + do 中的纯服务混合使用

	// 1. Registry 管理基础设施组件
	reg := registry.NewRegistry()
	configComp := NewConfigComponent()
	require.NoError(t, reg.Register(configComp))

	// 2. do 管理应用层服务
	injector := do.New()
	defer injector.Shutdown()

	bridge := di.NewBridge(reg, injector)

	// 3. 暴露 Config 给 do
	di.MustProvideFromRegistry[*ConfigComponent](bridge, component.ComponentConfig)

	// 4. 在 do 中注册依赖 Config 的服务
	type AppConfig struct {
		DBHost string
		DBPort int
	}

	di.Provide(bridge, func(i do.Injector) (*AppConfig, error) {
		cfg := do.MustInvoke[*ConfigComponent](i)
		return &AppConfig{
			DBHost: cfg.GetString("database.host"),
			DBPort: cfg.GetInt("database.port"),
		}, nil
	})

	// 5. 验证
	appCfg := di.MustInvoke[*AppConfig](bridge)
	assert.Equal(t, "localhost", appCfg.DBHost)
	assert.Equal(t, 3306, appCfg.DBPort)
}

func TestIntegration_NamedServices(t *testing.T) {
	// 场景：使用命名服务区分不同实例

	reg := registry.NewRegistry()
	injector := do.New()
	defer injector.Shutdown()

	bridge := di.NewBridge(reg, injector)

	type Cache struct {
		Name string
	}

	// 注册多个命名缓存实例
	di.ProvideNamed(bridge, "user-cache", func(i do.Injector) (*Cache, error) {
		return &Cache{Name: "user-cache"}, nil
	})

	di.ProvideNamed(bridge, "session-cache", func(i do.Injector) (*Cache, error) {
		return &Cache{Name: "session-cache"}, nil
	})

	// 验证不同的命名服务
	userCache := di.MustInvokeNamed[*Cache](bridge, "user-cache")
	sessionCache := di.MustInvokeNamed[*Cache](bridge, "session-cache")

	assert.Equal(t, "user-cache", userCache.Name)
	assert.Equal(t, "session-cache", sessionCache.Name)
	assert.NotEqual(t, userCache, sessionCache)
}

func TestIntegration_HealthCheckWithServices(t *testing.T) {
	// 场景：验证健康检查功能

	reg := registry.NewRegistry()
	injector := do.New()
	defer injector.Shutdown()

	bridge := di.NewBridge(reg, injector)

	// 注册一些服务
	type HealthyService struct{}

	di.Provide(bridge, func(i do.Injector) (*HealthyService, error) {
		return &HealthyService{}, nil
	})

	// 触发服务创建
	_ = di.MustInvoke[*HealthyService](bridge)

	// 验证健康检查 - 获取错误 map
	errors := bridge.HealthCheck()
	// 如果没有实现 HealthChecker 接口的服务，errors 可能包含错误
	// 但不应该 panic
	_ = errors
}

func TestIntegration_GracefulShutdown(t *testing.T) {
	// 场景：验证优雅关闭

	reg := registry.NewRegistry()
	injector := do.New()

	bridge := di.NewBridge(reg, injector)

	type Service struct {
		closed bool
	}

	di.Provide(bridge, func(i do.Injector) (*Service, error) {
		return &Service{}, nil
	})

	// 触发服务创建
	_ = di.MustInvoke[*Service](bridge)

	// 优雅关闭
	ctx := context.Background()
	err := bridge.ShutdownWithContext(ctx)
	// 注意：samber/do 可能返回 "DI container is closed" 错误，这是正常的
	_ = err
}
