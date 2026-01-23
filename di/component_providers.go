package di

import (
	"context"

	"github.com/KOMKZ/go-yogan-framework/auth"
	"github.com/KOMKZ/go-yogan-framework/breaker"
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
// Basic components Provider (Config, Logger)
// These are the lowest level dependencies, upon which all other components rely.
// ============================================

// ConfigOptions configuration component options
type ConfigOptions struct {
	ConfigPath   string      // Configure directory path
	ConfigPrefix string      // Environment variable prefix
	AppType      string      // Application type: grpc, http, mixed
	Flags        interface{} // command line arguments
}

// ProvideConfigLoader creates a Provider for config.Loader
// This is the most basic component, with no dependencies
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

// ProvideLoggerManager creates a Provider for logger.Manager
// Dependencies: config.Loader (reads logger configuration from config)
func ProvideLoggerManager(i do.Injector) (*logger.Manager, error) {
	var loggerCfg logger.ManagerConfig

	// Try to load logger configuration from config
	loader, err := do.Invoke[*config.Loader](i)
	if err == nil && loader != nil {
		if v := loader.GetViper(); v != nil {
			_ = v.UnmarshalKey("logger", &loggerCfg)
		}
	}

	loggerCfg.ApplyDefaults()

	// Initialize the global Manager concurrently (compatible with old code)
	logger.InitManager(loggerCfg)

	return logger.NewManager(loggerCfg), nil
}

// CreateCtxLogger provides a factory for the named CtxZapLogger provider
// For the application layer to obtain a logger for a specific module
func ProvideCtxLogger(moduleName string) func(do.Injector) (*logger.CtxZapLogger, error) {
	return func(i do.Injector) (*logger.CtxZapLogger, error) {
		// Try to get from Manager first (recommended)
		mgr, err := do.Invoke[*logger.Manager](i)
		if err == nil && mgr != nil {
			if ctxLogger := mgr.GetLogger(moduleName); ctxLogger != nil {
				return ctxLogger, nil
			}
		}
		// fallback to global logger
		return logger.GetLogger(moduleName), nil
	}
}

// ============================================
// Database component provider
// Dependencies: Config, Logger
// ============================================

// ProvideDatabaseManager creates a Provider for database.Manager
// Dependencies: config.Loader (reads database configuration)
func ProvideDatabaseManager(i do.Injector) (*database.Manager, error) {
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		return nil, err
	}

	log, _ := do.Invoke[*logger.CtxZapLogger](i)
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	// Read database configuration
	var dbConfigs map[string]database.Config
	if err := loader.GetViper().UnmarshalKey("database.connections", &dbConfigs); err != nil {
		return nil, err
	}

	if len(dbConfigs) == 0 {
		return nil, nil // Database not configured
	}

	// Create GORM Logger factory
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
// Redis component Provider
// Dependencies: Config, Logger
// ============================================

// ProvideRedisManager creates a Provider for redis.Manager
// Dependency: config.Loader (reads Redis configuration)
func ProvideRedisManager(i do.Injector) (*redis.Manager, error) {
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		return nil, err
	}

	log, _ := do.Invoke[*logger.CtxZapLogger](i)
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	// Read Redis configuration
	var redisConfigs map[string]redis.Config
	if err := loader.GetViper().UnmarshalKey("redis.instances", &redisConfigs); err != nil {
		return nil, err
	}

	if len(redisConfigs) == 0 {
		return nil, nil // Redis not configured
	}

	return redis.NewManager(redisConfigs, log)
}

// ============================================
// JWT component provider
// Dependencies: Config, Logger, Redis(optional)
// ============================================

// Provide JWT Config
func ProvideJWTConfig(i do.Injector) (*jwt.Config, error) {
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		return nil, err
	}

	var cfg jwt.Config
	if err := loader.GetViper().UnmarshalKey("jwt", &cfg); err != nil {
		return nil, nil // JWT not configured
	}
	cfg.ApplyDefaults()

	if !cfg.Enabled {
		return nil, nil // JWT is not enabled
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Create an independent Provider for jwt.TokenManager
// Dependencies: config.Loader, redis.Manager (optional)
// Note: Distinguish from ProvideJWTManager in providers.go, which retrieves from the Registry
func ProvideJWTTokenManagerIndependent(i do.Injector) (jwt.TokenManager, error) {
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		return nil, err
	}

	// Read JWT configuration
	var cfg jwt.Config
	if err := loader.GetViper().UnmarshalKey("jwt", &cfg); err != nil {
		return nil, nil // JWT not configured
	}
	cfg.ApplyDefaults()

	if !cfg.Enabled {
		return nil, nil // JWT is not enabled
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	log, _ := do.Invoke[*logger.CtxZapLogger](i)
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	// Create TokenStore
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
// Event Component Provider
// Dependencies: Config, Logger
// ============================================

// Create an independent Provider for event.Dispatcher
// Note: Distinct from ProvideEventDispatcher in providers.go, which retrieves from the Registry
func ProvideEventDispatcherIndependent(i do.Injector) (event.Dispatcher, error) {
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		return nil, err
	}

	// Read Event configuration
	var cfg event.Config
	if err := loader.GetViper().UnmarshalKey("event", &cfg); err != nil {
		cfg = event.DefaultConfig()
	}

	if !cfg.Enabled {
		return nil, nil // Event not enabled
	}

	log, _ := do.Invoke[*logger.CtxZapLogger](i)
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	// Create Dispatcher (using Option pattern)
	return event.NewDispatcher(
		event.WithPoolSize(cfg.PoolSize),
		event.WithSetAllSync(cfg.SetAllSync),
	), nil
}

// ============================================
// Kafka Component Provider
// Dependencies: Config, Logger
// ============================================

// ProvideKafkaManager creates an independent Provider for kafka.Manager
func ProvideKafkaManager(i do.Injector) (*kafka.Manager, error) {
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		return nil, err
	}

	log, _ := do.Invoke[*logger.CtxZapLogger](i)
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	// Read Kafka configuration
	var cfg kafka.Config
	if err := loader.GetViper().UnmarshalKey("kafka", &cfg); err != nil {
		return nil, nil // Kafka not configured
	}

	if len(cfg.Brokers) == 0 {
		return nil, nil // Kafka brokers not configured
	}

	return kafka.NewManager(cfg, log)
}

// ============================================
// Telemetry Component Provider
// Dependencies: Config, Logger
// ============================================

// CreateTelemetryManager creates an independent Provider for telemetry.Manager
func ProvideTelemetryManager(i do.Injector) (*telemetry.Manager, error) {
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		return nil, err
	}

	// Read Telemetry configuration
	var cfg telemetry.Config
	if loader.IsSet("telemetry") {
		if err := loader.GetViper().UnmarshalKey("telemetry", &cfg); err != nil {
			cfg = telemetry.DefaultConfig()
		}
	} else {
		cfg = telemetry.DefaultConfig()
	}

	if !cfg.Enabled {
		return nil, nil // Telemetry is not enabled
	}

	log, _ := do.Invoke[*logger.CtxZapLogger](i)
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	// Create and start Manager
	mgr := telemetry.NewManager(cfg, log)
	if err := mgr.Start(context.Background()); err != nil {
		return nil, err
	}
	return mgr, nil
}

// ProvideMetricsRegistry creates an independent Provider for telemetry.MetricsRegistry
// Dependencies: TelemetryManager
func ProvideMetricsRegistry(i do.Injector) (*telemetry.MetricsRegistry, error) {
	// Get TelemetryManager (optional)
	mgr, err := do.Invoke[*telemetry.Manager](i)
	if err != nil || mgr == nil {
		// Telemetry not enabled, return nil registry
		return nil, nil
	}

	// Get MetricsManager from TelemetryManager
	metricsMgr := mgr.GetMetricsManager()
	if metricsMgr == nil || !metricsMgr.IsEnabled() {
		return nil, nil
	}

	log, _ := do.Invoke[*logger.CtxZapLogger](i)
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	// Create MetricsRegistry with global MeterProvider
	registry := telemetry.NewMetricsRegistry(nil,
		telemetry.WithNamespace(mgr.GetConfig().Metrics.Namespace),
		telemetry.WithLogger(log),
	)

	return registry, nil
}

// ============================================
// Health component provider
// Dependencies: Config, Logger
// ============================================

// ProvideHealthAggregator creates an independent Provider for health.Aggregator
func ProvideHealthAggregator(i do.Injector) (*health.Aggregator, error) {
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		return nil, err
	}

	// Read Health configuration
	cfg := health.DefaultConfig()
	if loader.IsSet("health") {
		_ = loader.GetViper().UnmarshalKey("health", &cfg)
	}

	if !cfg.Enabled {
		return nil, nil // Health not enabled
	}

	return health.NewAggregator(cfg.Timeout), nil
}

// ============================================
// Cache Component Provider
// Dependencies: Config, Logger, Redis (optional), Event (optional)
// ============================================

// ProvideCacheOrchestrator creates an independent Provider for cache.Orchestrator
func ProvideCacheOrchestrator(i do.Injector) (*cache.DefaultOrchestrator, error) {
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		return nil, err
	}

	// Read cache configuration
	var cfg cache.Config
	if err := loader.GetViper().UnmarshalKey("cache", &cfg); err != nil {
		return nil, nil // Cache not configured
	}

	if !cfg.Enabled {
		return nil, nil // Cache not enabled
	}

	log, _ := do.Invoke[*logger.CtxZapLogger](i)
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	// Try to get the Event Dispatcher
	dispatcher, _ := do.Invoke[event.Dispatcher](i)

	return cache.NewOrchestrator(&cfg, dispatcher, log), nil
}

// ============================================
// Limiter Component Provider
// Dependencies: Config, Logger, Redis
// ============================================

// ProvideLimiterManager creates an independent Provider for limiter.Manager
func ProvideLimiterManager(i do.Injector) (*limiter.Manager, error) {
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		return nil, err
	}

	// Read Limiter configuration
	var cfg limiter.Config
	if err := loader.GetViper().UnmarshalKey("limiter", &cfg); err != nil {
		return nil, nil // Limiter not configured
	}

	if !cfg.Enabled {
		return nil, nil // Limiter not enabled
	}

	log, _ := do.Invoke[*logger.CtxZapLogger](i)
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	// Try to get Redis client
	redisMgr, _ := do.Invoke[*redis.Manager](i)
	var redisClient *goredis.Client
	if redisMgr != nil && cfg.Redis.Instance != "" {
		redisClient = redisMgr.Client(cfg.Redis.Instance)
	}

	return limiter.NewManagerWithLogger(cfg, log, redisClient, nil)
}

// ============================================
// gRPC Component Provider
// Dependencies: Config, Logger, Limiter (optional), Telemetry (optional)
// ============================================

// ProvideGRPCServer creates an independent Provider for grpc.Server
func ProvideGRPCServer(i do.Injector) (*grpc.Server, error) {
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		return nil, err
	}

	// Read gRPC configuration
	var cfg grpc.Config
	if err := loader.GetViper().UnmarshalKey("grpc", &cfg); err != nil {
		return nil, nil // gRPC not configured
	}

	if !cfg.Server.Enabled {
		return nil, nil // gRPC Server is not enabled
	}

	log, _ := do.Invoke[*logger.CtxZapLogger](i)
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	return grpc.NewServer(cfg.Server, log), nil
}

// ProvideGRPCClientManager creates an independent Provider for grpc.ClientManager
func ProvideGRPCClientManager(i do.Injector) (*grpc.ClientManager, error) {
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		return nil, err
	}

	// Read gRPC configuration
	var cfg grpc.Config
	if err := loader.GetViper().UnmarshalKey("grpc", &cfg); err != nil {
		return nil, nil // gRPC not configured
	}

	if len(cfg.Clients) == 0 {
		return nil, nil // gRPC client not configured
	}

	log, _ := do.Invoke[*logger.CtxZapLogger](i)
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	return grpc.NewClientManager(cfg.Clients, log), nil
}

// ============================================
// Auth Component Provider
// Dependencies: Config
// ============================================

// ProvidePasswordService creates an independent Provider for auth.PasswordService
func ProvidePasswordService(i do.Injector) (*auth.PasswordService, error) {
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		return nil, err
	}

	// Read auth configuration
	if !loader.IsSet("auth") {
		return nil, nil // auth not configured
	}

	// Default password policy
	policy := auth.PasswordPolicy{
		MinLength:          6,
		MaxLength:          128,
		RequireUppercase:   false,
		RequireLowercase:   false,
		RequireDigit:       false,
		RequireSpecialChar: false,
	}

	// Override from config if exists
	if loader.IsSet("auth.password.policy") {
		if err := loader.GetViper().UnmarshalKey("auth.password.policy", &policy); err != nil {
			// Use defaults on error
		}
	}

	// Bcrypt cost (default 10)
	bcryptCost := 10
	if loader.IsSet("auth.password.bcrypt_cost") {
		bcryptCost = loader.GetViper().GetInt("auth.password.bcrypt_cost")
	}

	return auth.NewPasswordService(policy, bcryptCost), nil
}

// ============================================
// Breaker Component Provider
// Dependencies: Config, Logger
// ============================================

// ProvideBreakerManager creates an independent Provider for breaker.Manager
func ProvideBreakerManager(i do.Injector) (*breaker.Manager, error) {
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		return nil, err
	}

	// Read breaker configuration
	if !loader.IsSet("breaker") {
		return nil, nil // breaker not configured
	}

	var cfg breaker.Config
	if err := loader.GetViper().UnmarshalKey("breaker", &cfg); err != nil {
		return nil, nil
	}

	if !cfg.Enabled {
		return nil, nil
	}

	log, _ := do.Invoke[*logger.CtxZapLogger](i)
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	return breaker.NewManagerWithLogger(cfg, log)
}
