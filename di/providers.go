package di

import (
	"github.com/KOMKZ/go-yogan-framework/auth"
	"github.com/KOMKZ/go-yogan-framework/cache"
	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/KOMKZ/go-yogan-framework/database"
	"github.com/KOMKZ/go-yogan-framework/event"
	"github.com/KOMKZ/go-yogan-framework/jwt"
	"github.com/KOMKZ/go-yogan-framework/redis"
	"github.com/KOMKZ/go-yogan-framework/registry"
	"github.com/samber/do/v2"
	"gorm.io/gorm"
)

// RegisterCoreComponents 注册内核组件到 do.Injector
// 从 Registry 获取已初始化的内核组件并注册到 do 容器
func RegisterCoreComponents(injector *do.RootScope, reg *registry.Registry) {
	// Database (gorm.DB) - 默认使用 master 连接
	if dbComp, ok := registry.GetTyped[*database.Component](reg, component.ComponentDatabase); ok {
		if mgr := dbComp.GetManager(); mgr != nil {
			if db := mgr.DB("master"); db != nil {
				do.ProvideValue(injector, db)
			}
		}
	}

	// Redis Client - 默认使用 main 实例
	if redisComp, ok := registry.GetTyped[*redis.Component](reg, component.ComponentRedis); ok {
		if mgr := redisComp.GetManager(); mgr != nil {
			if client := mgr.Client("main"); client != nil {
				do.ProvideValue(injector, client)
			}
		}
	}

	// JWT TokenManager
	if jwtComp, ok := registry.GetTyped[*jwt.Component](reg, component.ComponentJWT); ok {
		do.ProvideValue[jwt.TokenManager](injector, jwtComp.GetTokenManager())
		do.ProvideValue(injector, jwtComp.GetConfig())
	}

	// Auth Service
	if authComp, ok := registry.GetTyped[*auth.Component](reg, component.ComponentAuth); ok {
		do.ProvideValue(injector, authComp.GetAuthService())
	}

	// Event Dispatcher
	if eventComp, ok := registry.GetTyped[*event.Component](reg, component.ComponentEvent); ok {
		do.ProvideValue[event.Dispatcher](injector, eventComp.GetDispatcher())
	}

	// Cache Component
	if cacheComp, ok := registry.GetTyped[*cache.Component](reg, component.ComponentCache); ok {
		do.ProvideValue(injector, cacheComp)
	}
}

// ProvideDB 创建 *gorm.DB 的 Provider（从 Registry 获取，默认 master）
func ProvideDB(reg *registry.Registry) func(do.Injector) (*gorm.DB, error) {
	return func(i do.Injector) (*gorm.DB, error) {
		dbComp, ok := registry.GetTyped[*database.Component](reg, component.ComponentDatabase)
		if !ok {
			return nil, ErrComponentNotFound("database")
		}
		mgr := dbComp.GetManager()
		if mgr == nil {
			return nil, ErrComponentNotFound("database manager")
		}
		db := mgr.DB("master")
		if db == nil {
			return nil, ErrComponentNotFound("database connection: master")
		}
		return db, nil
	}
}

// ProvideJWTManager 创建 jwt.TokenManager 的 Provider
func ProvideJWTManager(reg *registry.Registry) func(do.Injector) (jwt.TokenManager, error) {
	return func(i do.Injector) (jwt.TokenManager, error) {
		jwtComp, ok := registry.GetTyped[*jwt.Component](reg, component.ComponentJWT)
		if !ok {
			return nil, ErrComponentNotFound("jwt")
		}
		return jwtComp.GetTokenManager(), nil
	}
}

// ProvideJWTConfig 创建 *jwt.Config 的 Provider
func ProvideJWTConfig(reg *registry.Registry) func(do.Injector) (*jwt.Config, error) {
	return func(i do.Injector) (*jwt.Config, error) {
		jwtComp, ok := registry.GetTyped[*jwt.Component](reg, component.ComponentJWT)
		if !ok {
			return nil, ErrComponentNotFound("jwt")
		}
		return jwtComp.GetConfig(), nil
	}
}

// ProvideEventDispatcher 创建 event.Dispatcher 的 Provider
func ProvideEventDispatcher(reg *registry.Registry) func(do.Injector) (event.Dispatcher, error) {
	return func(i do.Injector) (event.Dispatcher, error) {
		eventComp, ok := registry.GetTyped[*event.Component](reg, component.ComponentEvent)
		if !ok {
			return nil, ErrComponentNotFound("event")
		}
		return eventComp.GetDispatcher(), nil
	}
}

// ProvideCacheComponent 创建 *cache.Component 的 Provider
func ProvideCacheComponent(reg *registry.Registry) func(do.Injector) (*cache.Component, error) {
	return func(i do.Injector) (*cache.Component, error) {
		cacheComp, ok := registry.GetTyped[*cache.Component](reg, component.ComponentCache)
		if !ok {
			return nil, ErrComponentNotFound("cache")
		}
		return cacheComp, nil
	}
}

// ErrComponentNotFound 组件未找到错误
func ErrComponentNotFound(name string) error {
	return &ComponentNotFoundError{Name: name}
}

// ComponentNotFoundError 组件未找到错误类型
type ComponentNotFoundError struct {
	Name string
}

func (e *ComponentNotFoundError) Error() string {
	return "component not found: " + e.Name
}
