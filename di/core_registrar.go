// Package di 提供依赖注入相关功能
package di

import (
	"github.com/KOMKZ/go-yogan-framework/database"
	"github.com/KOMKZ/go-yogan-framework/redis"
	goredis "github.com/redis/go-redis/v9"
	"github.com/samber/do/v2"
	"gorm.io/gorm"
)

// RegisterCoreProviders 注册所有核心组件 Provider 到 injector
// 按依赖层级注册，懒加载模式
func RegisterCoreProviders(injector *do.RootScope, opts ConfigOptions) {
	// ═══════════════════════════════════════════════════════════
	// Layer 0: Config（无依赖）
	// ═══════════════════════════════════════════════════════════
	do.Provide(injector, ProvideConfigLoader(opts))

	// ═══════════════════════════════════════════════════════════
	// Layer 1: Logger（依赖 Config）
	// ═══════════════════════════════════════════════════════════
	do.Provide(injector, ProvideLoggerManager)
	do.Provide(injector, ProvideCtxLogger("yogan"))

	// ═══════════════════════════════════════════════════════════
	// Layer 2: 基础设施组件（懒加载）
	// ═══════════════════════════════════════════════════════════
	do.Provide(injector, ProvideDatabaseManager)
	do.Provide(injector, ProvideRedisManager)
	do.Provide(injector, ProvideKafkaManager)

	// 便捷访问：*gorm.DB（从 Manager 获取 master）
	do.Provide(injector, ProvideDefaultDB)
	// 便捷访问：redis.UniversalClient（从 Manager 获取 main）
	do.Provide(injector, ProvideDefaultRedisClient)

	// ═══════════════════════════════════════════════════════════
	// Layer 3: 业务支撑组件（懒加载）
	// ═══════════════════════════════════════════════════════════
	do.Provide(injector, ProvideJWTConfig)
	do.Provide(injector, ProvideJWTTokenManagerIndependent)
	do.Provide(injector, ProvideEventDispatcherIndependent)
	do.Provide(injector, ProvideCacheOrchestrator)
	do.Provide(injector, ProvideLimiterManager)
	do.Provide(injector, ProvideHealthAggregator)
}

// ProvideDefaultDB 提供默认数据库连接（master）
func ProvideDefaultDB(i do.Injector) (*gorm.DB, error) {
	mgr, err := do.Invoke[*database.Manager](i)
	if err != nil || mgr == nil {
		return nil, err
	}
	return mgr.DB("master"), nil
}

// ProvideDefaultRedisClient 提供默认 Redis 客户端（main）
func ProvideDefaultRedisClient(i do.Injector) (goredis.UniversalClient, error) {
	mgr, err := do.Invoke[*redis.Manager](i)
	if err != nil || mgr == nil {
		return nil, err
	}
	return mgr.Client("main"), nil
}
