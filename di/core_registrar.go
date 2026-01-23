// Package di provides dependency injection related functionality
package di

import (
	"github.com/KOMKZ/go-yogan-framework/database"
	"github.com/KOMKZ/go-yogan-framework/redis"
	"github.com/KOMKZ/go-yogan-framework/swagger"
	goredis "github.com/redis/go-redis/v9"
	"github.com/samber/do/v2"
	"gorm.io/gorm"
)

// RegisterCoreProviders registers all core component providers to the injector
// Register by dependency level, lazy loading mode
func RegisterCoreProviders(injector *do.RootScope, opts ConfigOptions) {
	// ═══════════════════════════════════════════════════════════
	// Layer 0: Config (no dependencies)
	// ═══════════════════════════════════════════════════════════
	do.Provide(injector, ProvideConfigLoader(opts))

	// ═══════════════════════════════════════════════════════════
	// Layer 1: Logger (depends on Config)
	// ═══════════════════════════════════════════════════════════
	do.Provide(injector, ProvideLoggerManager)
	do.Provide(injector, ProvideCtxLogger("yogan"))

	// ═══════════════════════════════════════════════════════════
	// Layer 2: Infrastructure components (lazy loading)
	// ═══════════════════════════════════════════════════════════
	do.Provide(injector, ProvideDatabaseManager)
	do.Provide(injector, ProvideRedisManager)
	do.Provide(injector, ProvideKafkaManager)

	// Convenient access: *gorm.DB (from Manager get master)
	do.Provide(injector, ProvideDefaultDB)
	// Convenient access: redis.UniversalClient (obtained from Manager as main)
	do.Provide(injector, ProvideDefaultRedisClient)

	// ═══════════════════════════════════════════════════════════
	// Layer 3: Business Support Components (Lazy Loading)
	// ═══════════════════════════════════════════════════════════
	do.Provide(injector, ProvideJWTConfig)
	do.Provide(injector, ProvideJWTTokenManagerIndependent)
	do.Provide(injector, ProvideEventDispatcherIndependent)
	do.Provide(injector, ProvideCacheOrchestrator)
	do.Provide(injector, ProvideLimiterManager)
	do.Provide(injector, ProvideBreakerManager)
	do.Provide(injector, ProvideHealthAggregator)
	do.Provide(injector, ProvideTelemetryManager)
	do.Provide(injector, ProvideMetricsRegistry)
	do.Provide(injector, ProvidePasswordService)

	// ═══════════════════════════════════════════════════════════
	// Layer 4: Documentation and auxiliary components (lazy loading)
	// ═══════════════════════════════════════════════════════════
	do.Provide(injector, swagger.ProvideManager)
}

// ProvideDefaultDB provides default database connection (master)
func ProvideDefaultDB(i do.Injector) (*gorm.DB, error) {
	mgr, err := do.Invoke[*database.Manager](i)
	if err != nil || mgr == nil {
		return nil, err
	}
	return mgr.DB("master"), nil
}

// ProvideDefaultRedisClient Provides the default Redis client (main)
func ProvideDefaultRedisClient(i do.Injector) (goredis.UniversalClient, error) {
	mgr, err := do.Invoke[*redis.Manager](i)
	if err != nil || mgr == nil {
		return nil, err
	}
	return mgr.Client("main"), nil
}
