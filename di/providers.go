// Package di provides dependency injection utilities based on samber/do.
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
)

// RegisterCoreComponents 注册内核组件到 do.Injector
// 从 Registry 获取已初始化的内核组件并注册到 do 容器
//
// Deprecated: 此函数是过渡方案，未来将移除 Registry 依赖
// 新应用应直接使用 component_providers.go 中的独立 Provider
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
