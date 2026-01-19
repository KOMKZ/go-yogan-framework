package di

import (
	"github.com/KOMKZ/go-yogan-framework/cache"
	"github.com/KOMKZ/go-yogan-framework/config"
	"github.com/KOMKZ/go-yogan-framework/database"
	"github.com/KOMKZ/go-yogan-framework/event"
	"github.com/KOMKZ/go-yogan-framework/grpc"
	"github.com/KOMKZ/go-yogan-framework/health"
	"github.com/KOMKZ/go-yogan-framework/jwt"
	"github.com/KOMKZ/go-yogan-framework/kafka"
	"github.com/KOMKZ/go-yogan-framework/limiter"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/KOMKZ/go-yogan-framework/redis"
	"github.com/KOMKZ/go-yogan-framework/telemetry"
	goredis "github.com/redis/go-redis/v9"
	"github.com/samber/do/v2"
	gormlogger "gorm.io/gorm/logger"
)

// ============================================
// 基础组件 Provider（Config, Logger）
// 这些是最底层的依赖，其他组件都依赖它们
// ============================================

// ConfigOptions 配置组件选项
type ConfigOptions struct {
	ConfigPath   string      // 配置目录路径
	ConfigPrefix string      // 环境变量前缀
	AppType      string      // 应用类型：grpc, http, mixed
	Flags        interface{} // 命令行参数
}

// ProvideConfigLoader 创建 config.Loader 的 Provider
// 这是最基础的组件，无依赖
func ProvideConfigLoader(opts ConfigOptions) func(do.Injector) (*config.Loader, error) {
	return func(i do.Injector) (*config.Loader, error) {
		if opts.ConfigPath == "" {
			opts.ConfigPath = "../configs"
		}
		if opts.AppType == "" {
			opts.AppType = "grpc"
		}

		loader, err := config.NewLoaderBuilder().
			WithConfigPath(opts.ConfigPath).
			WithEnvPrefix(opts.ConfigPrefix).
			WithAppType(opts.AppType).
			WithFlags(opts.Flags).
			Build()
		if err != nil {
			return nil, err
		}
		return loader, nil
	}
}

// ProvideLoggerManager 创建 logger.Manager 的 Provider
// 依赖：config.Loader（从配置读取 logger 配置）
func ProvideLoggerManager(i do.Injector) (*logger.Manager, error) {
	var loggerCfg logger.ManagerConfig

	// 尝试从配置加载 logger 配置
	loader, err := do.Invoke[*config.Loader](i)
	if err == nil && loader != nil {
		if v := loader.GetViper(); v != nil {
			_ = v.UnmarshalKey("logger", &loggerCfg)
		}
	}

	loggerCfg.ApplyDefaults()

	// 同时初始化全局 Manager（兼容旧代码）
	logger.InitManager(loggerCfg)

	return logger.NewManager(loggerCfg), nil
}

// ProvideCtxLogger 创建命名 CtxZapLogger 的 Provider 工厂
// 用于应用层获取特定模块的 logger
func ProvideCtxLogger(moduleName string) func(do.Injector) (*logger.CtxZapLogger, error) {
	return func(i do.Injector) (*logger.CtxZapLogger, error) {
		// 先尝试从 Manager 获取（推荐）
		mgr, err := do.Invoke[*logger.Manager](i)
		if err == nil && mgr != nil {
			if ctxLogger := mgr.GetLogger(moduleName); ctxLogger != nil {
				return ctxLogger, nil
			}
		}
		// 回退到全局 logger
		return logger.GetLogger(moduleName), nil
	}
}

// ============================================
// Database 组件 Provider
// 依赖：Config, Logger
// ============================================

// ProvideDatabaseManager 创建 database.Manager 的 Provider
// 依赖：config.Loader（读取数据库配置）
func ProvideDatabaseManager(i do.Injector) (*database.Manager, error) {
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		return nil, err
	}

	log, _ := do.Invoke[*logger.CtxZapLogger](i)
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	// 读取数据库配置
	var dbConfigs map[string]database.Config
	if err := loader.GetViper().UnmarshalKey("database.connections", &dbConfigs); err != nil {
		return nil, err
	}

	if len(dbConfigs) == 0 {
		return nil, nil // 未配置数据库
	}

	// 创建 GORM Logger 工厂
	gormLoggerFactory := func(dbCfg database.Config) gormlogger.Interface {
		if dbCfg.EnableLog {
			loggerCfg := logger.DefaultGormLoggerConfig()
			loggerCfg.SlowThreshold = dbCfg.SlowThreshold
			loggerCfg.LogLevel = gormlogger.Info
			loggerCfg.EnableAudit = dbCfg.EnableAudit
			return logger.NewGormLogger(loggerCfg)
		}
		return gormlogger.Default.LogMode(gormlogger.Silent)
	}

	return database.NewManager(dbConfigs, gormLoggerFactory, log)
}

// ============================================
// Redis 组件 Provider
// 依赖：Config, Logger
// ============================================

// ProvideRedisManager 创建 redis.Manager 的 Provider
// 依赖：config.Loader（读取 Redis 配置）
func ProvideRedisManager(i do.Injector) (*redis.Manager, error) {
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		return nil, err
	}

	log, _ := do.Invoke[*logger.CtxZapLogger](i)
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	// 读取 Redis 配置
	var redisConfigs map[string]redis.Config
	if err := loader.GetViper().UnmarshalKey("redis.instances", &redisConfigs); err != nil {
		return nil, err
	}

	if len(redisConfigs) == 0 {
		return nil, nil // 未配置 Redis
	}

	return redis.NewManager(redisConfigs, log.GetZapLogger())
}

// ============================================
// JWT 组件 Provider
// 依赖：Config, Logger, Redis(可选)
// ============================================

// ProvideJWTConfig 提供 JWT 配置
func ProvideJWTConfig(i do.Injector) (*jwt.Config, error) {
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		return nil, err
	}

	var cfg jwt.Config
	if err := loader.GetViper().UnmarshalKey("jwt", &cfg); err != nil {
		return nil, nil // JWT 未配置
	}
	cfg.ApplyDefaults()

	if !cfg.Enabled {
		return nil, nil // JWT 未启用
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// ProvideJWTTokenManagerIndependent 创建 jwt.TokenManager 的独立 Provider
// 依赖：config.Loader, redis.Manager(可选)
// 注意：与 providers.go 中的 ProvideJWTManager 区分，后者从 Registry 获取
func ProvideJWTTokenManagerIndependent(i do.Injector) (jwt.TokenManager, error) {
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		return nil, err
	}

	// 读取 JWT 配置
	var cfg jwt.Config
	if err := loader.GetViper().UnmarshalKey("jwt", &cfg); err != nil {
		return nil, nil // JWT 未配置
	}
	cfg.ApplyDefaults()

	if !cfg.Enabled {
		return nil, nil // JWT 未启用
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	log, _ := do.Invoke[*logger.CtxZapLogger](i)
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	// 创建 TokenStore
	var tokenStore jwt.TokenStore
	if cfg.Blacklist.Enabled && cfg.Blacklist.Storage == "redis" {
		redisMgr, _ := do.Invoke[*redis.Manager](i)
		if redisMgr != nil {
			client := redisMgr.Client("main")
			if client != nil {
				tokenStore = jwt.NewRedisTokenStore(client, cfg.Blacklist.RedisKeyPrefix, log)
			}
		}
	}
	if tokenStore == nil && cfg.Blacklist.Enabled {
		tokenStore = jwt.NewMemoryTokenStore(cfg.Blacklist.CleanupInterval, log)
	}

	return jwt.NewTokenManager(&cfg, tokenStore, log)
}

// ============================================
// Event 组件 Provider
// 依赖：Config, Logger
// ============================================

// ProvideEventDispatcherIndependent 创建 event.Dispatcher 的独立 Provider
// 注意：与 providers.go 中的 ProvideEventDispatcher 区分，后者从 Registry 获取
func ProvideEventDispatcherIndependent(i do.Injector) (event.Dispatcher, error) {
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		return nil, err
	}

	// 读取 Event 配置
	var cfg event.Config
	if err := loader.GetViper().UnmarshalKey("event", &cfg); err != nil {
		cfg = event.DefaultConfig()
	}

	if !cfg.Enabled {
		return nil, nil // Event 未启用
	}

	log, _ := do.Invoke[*logger.CtxZapLogger](i)
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	// 创建 Dispatcher（使用 Option 模式）
	return event.NewDispatcher(
		event.WithPoolSize(cfg.PoolSize),
	), nil
}

// ============================================
// Kafka 组件 Provider
// 依赖：Config, Logger
// ============================================

// ProvideKafkaManager 创建 kafka.Manager 的独立 Provider
func ProvideKafkaManager(i do.Injector) (*kafka.Manager, error) {
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		return nil, err
	}

	log, _ := do.Invoke[*logger.CtxZapLogger](i)
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	// 读取 Kafka 配置
	var cfg kafka.Config
	if err := loader.GetViper().UnmarshalKey("kafka", &cfg); err != nil {
		return nil, nil // Kafka 未配置
	}

	if len(cfg.Brokers) == 0 {
		return nil, nil // Kafka brokers 未配置
	}

	return kafka.NewManager(cfg, log.GetZapLogger())
}

// ============================================
// Telemetry 组件 Provider
// 依赖：Config, Logger
// ============================================

// ProvideTelemetryComponent 创建 telemetry.Component 的独立 Provider
func ProvideTelemetryComponent(i do.Injector) (*telemetry.Component, error) {
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		return nil, err
	}

	// 读取 Telemetry 配置
	var cfg telemetry.Config
	if loader.IsSet("telemetry") {
		if err := loader.GetViper().UnmarshalKey("telemetry", &cfg); err != nil {
			cfg = telemetry.DefaultConfig()
		}
	} else {
		cfg = telemetry.DefaultConfig()
	}

	if !cfg.Enabled {
		return nil, nil // Telemetry 未启用
	}

	// 创建组件（需要手动初始化）
	comp := telemetry.NewComponent()
	// 注意：完整初始化需要调用 Init 和 Start，这里只返回组件实例
	return comp, nil
}

// ============================================
// Health 组件 Provider
// 依赖：Config, Logger
// ============================================

// ProvideHealthAggregator 创建 health.Aggregator 的独立 Provider
func ProvideHealthAggregator(i do.Injector) (*health.Aggregator, error) {
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		return nil, err
	}

	// 读取 Health 配置
	cfg := health.DefaultConfig()
	if loader.IsSet("health") {
		_ = loader.GetViper().UnmarshalKey("health", &cfg)
	}

	if !cfg.Enabled {
		return nil, nil // Health 未启用
	}

	return health.NewAggregator(cfg.Timeout), nil
}

// ============================================
// Cache 组件 Provider
// 依赖：Config, Logger, Redis(可选), Event(可选)
// ============================================

// ProvideCacheOrchestrator 创建 cache.Orchestrator 的独立 Provider
func ProvideCacheOrchestrator(i do.Injector) (*cache.DefaultOrchestrator, error) {
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		return nil, err
	}

	// 读取 Cache 配置
	var cfg cache.Config
	if err := loader.GetViper().UnmarshalKey("cache", &cfg); err != nil {
		return nil, nil // Cache 未配置
	}

	if !cfg.Enabled {
		return nil, nil // Cache 未启用
	}

	log, _ := do.Invoke[*logger.CtxZapLogger](i)
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	// 尝试获取 Event Dispatcher
	dispatcher, _ := do.Invoke[event.Dispatcher](i)

	return cache.NewOrchestrator(&cfg, dispatcher, log), nil
}

// ============================================
// Limiter 组件 Provider
// 依赖：Config, Logger, Redis
// ============================================

// ProvideLimiterManager 创建 limiter.Manager 的独立 Provider
func ProvideLimiterManager(i do.Injector) (*limiter.Manager, error) {
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		return nil, err
	}

	// 读取 Limiter 配置
	var cfg limiter.Config
	if err := loader.GetViper().UnmarshalKey("limiter", &cfg); err != nil {
		return nil, nil // Limiter 未配置
	}

	if !cfg.Enabled {
		return nil, nil // Limiter 未启用
	}

	log, _ := do.Invoke[*logger.CtxZapLogger](i)
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	// 尝试获取 Redis Client
	redisMgr, _ := do.Invoke[*redis.Manager](i)
	var redisClient *goredis.Client
	if redisMgr != nil && cfg.Redis.Instance != "" {
		redisClient = redisMgr.Client(cfg.Redis.Instance)
	}

	return limiter.NewManagerWithLogger(cfg, log, redisClient, nil)
}

// ============================================
// gRPC 组件 Provider
// 依赖：Config, Logger, Limiter(可选), Telemetry(可选)
// ============================================

// ProvideGRPCServer 创建 grpc.Server 的独立 Provider
func ProvideGRPCServer(i do.Injector) (*grpc.Server, error) {
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		return nil, err
	}

	// 读取 gRPC 配置
	var cfg grpc.Config
	if err := loader.GetViper().UnmarshalKey("grpc", &cfg); err != nil {
		return nil, nil // gRPC 未配置
	}

	if !cfg.Server.Enabled {
		return nil, nil // gRPC Server 未启用
	}

	log, _ := do.Invoke[*logger.CtxZapLogger](i)
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	return grpc.NewServer(cfg.Server, log), nil
}

// ProvideGRPCClientManager 创建 grpc.ClientManager 的独立 Provider
func ProvideGRPCClientManager(i do.Injector) (*grpc.ClientManager, error) {
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		return nil, err
	}

	// 读取 gRPC 配置
	var cfg grpc.Config
	if err := loader.GetViper().UnmarshalKey("grpc", &cfg); err != nil {
		return nil, nil // gRPC 未配置
	}

	if len(cfg.Clients) == 0 {
		return nil, nil // gRPC Client 未配置
	}

	log, _ := do.Invoke[*logger.CtxZapLogger](i)
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	return grpc.NewClientManager(cfg.Clients, log), nil
}
