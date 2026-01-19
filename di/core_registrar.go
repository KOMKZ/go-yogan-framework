// Package di 提供依赖注入相关功能
package di

import (
	"github.com/samber/do/v2"
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
