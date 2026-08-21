// Package application provides a generic application startup framework
// BaseApplication is the core abstraction for all application types (HTTP/CLI/Cron)
package application

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/KOMKZ/go-yogan-framework/auth"
	"github.com/KOMKZ/go-yogan-framework/breaker"
	"github.com/KOMKZ/go-yogan-framework/config"
	"github.com/KOMKZ/go-yogan-framework/database"
	"github.com/KOMKZ/go-yogan-framework/di"
	"github.com/KOMKZ/go-yogan-framework/event"
	"github.com/KOMKZ/go-yogan-framework/jwt"
	"github.com/KOMKZ/go-yogan-framework/kafka"
	"github.com/KOMKZ/go-yogan-framework/limiter"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/KOMKZ/go-yogan-framework/redis"
	"github.com/KOMKZ/go-yogan-framework/telemetry"
	"github.com/samber/do/v2"
	"go.uber.org/zap"
)

// BaseApplication core framework (80% generic logic)
// Supports all application types such as HTTP/CLI/Cron etc.
// 🎯 Use sambert/do for comprehensive component lifecycle management, no longer use Registry
type BaseApplication struct {
	// ═══════════════════════════════════════════════════════════
	// DI container (the unique component management approach)
	// ═══════════════════════════════════════════════════════════
	injector *do.RootScope // samber/do injector

	// configuration management
	configPath   string
	configPrefix string
	appConfig    *AppConfig

	// Core component cache (fast access)
	logger       *logger.CtxZapLogger
	configLoader *config.Loader

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	state  AppState
	mu     sync.RWMutex

	// Idempotent shutdown (first caller performs cleanup, later calls return the same result)
	shutdownOnce sync.Once
	shutdownErr  error

	// Apply metadata
	version   string
	startTime time.Time // Start time initialization

	// callback function
	onSetup        func(*BaseApplication) error
	onReady        func(*BaseApplication) error
	onConfigReload func(*config.Loader)
	onShutdown     func(context.Context) error
}

// Application state
type AppState int

const (
	StateInit AppState = iota
	StateSetup
	StateRunning
	StateStopping
	StateStopped
)

// String status string representation
func (s AppState) String() string {
	switch s {
	case StateInit:
		return "Init"
	case StateSetup:
		return "Setup"
	case StateRunning:
		return "Running"
	case StateStopping:
		return "Stopping"
	case StateStopped:
		return "Stopped"
	default:
		return "Unknown"
	}
}

// Create a base application instance
// 🎯 Use sambert/do for managing all components, no longer use Registry
func NewBase(configPath, configPrefix, appType string, flags interface{}) *BaseApplication {
	startTime := time.Now() // Record the start time of initialization
	ctx, cancel := context.WithCancel(context.Background())
	injector := do.New()

	// 🎯 The runtime environment travels with the flags (per application) into
	// the config loader for env-file selection, never through global APP_ENV.
	var env string
	if flags != nil {
		if appFlags, ok := flags.(*AppFlags); ok {
			env = appFlags.Env
		}
	}

	// Register all core components Providers (centralized in di/core_registrar.go)
	di.RegisterCoreProviders(injector, di.ConfigOptions{
		ConfigPath:   configPath,
		ConfigPrefix: configPrefix,
		AppType:      appType,
		Flags:        flags,
		Env:          env,
	})

	// Immediatley obtain Config and Logger (basic dependencies)
	configLoader := do.MustInvoke[*config.Loader](injector)
	coreLogger := do.MustInvoke[*logger.CtxZapLogger](injector)

	// Load AppConfig
	var appCfg AppConfig
	if err := configLoader.Unmarshal(&appCfg); err != nil {
		panic(fmt.Sprintf("加载 AppConfig 失败: %v", err))
	}

	coreLogger.DebugCtx(ctx, "✅ English: ✓ Basic application initialization complete (pure DI mode)（English: ✓ Basic application initialization complete (pure DI mode) DI English: ✓ Basic application initialization complete (pure DI mode)）",
		zap.String("configPath", configPath),
		zap.String("appType", appType))

	return &BaseApplication{
		injector:     injector,
		configPath:   configPath,
		configPrefix: configPrefix,
		logger:       coreLogger,
		configLoader: configLoader,
		appConfig:    &appCfg,
		ctx:          ctx,
		cancel:       cancel,
		state:        StateInit,
		startTime:    startTime,
	}
}

// NewBaseWithDefaults creates a base application instance (using default configuration path)
// appName: application name (such as user-api), used to construct default configuration paths
// appType: application type (http/grpc/cli/cron)
// Default configuration path: ../configs/{appName}
// Environment prefix: derived from appName (e.g., "user-api" -> "USER_API")
func NewBaseWithDefaults(appName, appType string) *BaseApplication {
	defaultPath := "../configs/" + appName
	return NewBase(defaultPath, EnvPrefixFor(appName), appType, nil)
}

// WithVersion sets the application version number (chained call)
// The version number will be automatically printed when the application starts
func (b *BaseApplication) WithVersion(version string) *BaseApplication {
	b.version = version
	return b
}

// GetVersion get application version number
func (b *BaseApplication) GetVersion() string {
	return b.version
}

// GetStartDuration Obtain application startup duration
func (b *BaseApplication) GetStartDuration() time.Duration {
	return time.Since(b.startTime)
}

// GetStartupTimeMs Obtain application startup time in milliseconds
func (b *BaseApplication) GetStartupTimeMs() int64 {
	return time.Since(b.startTime).Milliseconds()
}

// Setup initialize application (core logic)
// 🎯 Component lifecycle: Provider completes Init+Start (lazy loading) internally, automatically stops on Shutdown
func (b *BaseApplication) Setup() error {
	b.setState(StateSetup)

	// 🎯 Register component Metrics to MetricsRegistry (all app types)
	b.registerComponentMetrics()

	// Trigger OnSetup callback
	if b.onSetup != nil {
		if err := b.onSetup(b); err != nil {
			return fmt.Errorf("onSetup failed: %w", err)
		}
	}

	return nil
}

// registerComponentMetrics registers all component metrics to the MetricsRegistry
// This is called during Setup for all application types (HTTP, gRPC, CLI, Cron)
func (b *BaseApplication) registerComponentMetrics() {
	// Get MetricsRegistry from DI (optional)
	registry, err := do.Invoke[*telemetry.MetricsRegistry](b.injector)
	if err != nil || registry == nil {
		return // Telemetry/Metrics not enabled
	}

	b.logger.DebugCtx(b.ctx, "telemetry is enabled.")

	// Get TelemetryManager for config
	telemetryMgr, _ := do.Invoke[*telemetry.Manager](b.injector)
	if telemetryMgr == nil || !telemetryMgr.IsEnabled() {
		return
	}

	metricsCfg := telemetryMgr.GetConfig().Metrics

	// Register Redis Metrics
	if metricsCfg.Redis.Enabled {
		if redisMgr, err := do.Invoke[*redis.Manager](b.injector); err == nil && redisMgr != nil {
			redisMetrics := redis.NewRedisMetrics(redis.RedisMetricsConfig{
				Enabled:         true,
				RecordHitMiss:   metricsCfg.Redis.RecordHitMiss,
				RecordPoolStats: metricsCfg.Redis.RecordPoolStats,
			})
			if err := registry.Register(redisMetrics); err == nil {
				// 注入 Hook 到 Redis Manager，实现自动指标记录
				redisMgr.SetMetrics(redisMetrics)
				b.logger.DebugCtx(b.ctx, "✅ Redis Metrics registered with Hook")
			}
		}
	}

	// Register JWT Metrics
	if metricsCfg.JWT.Enabled {
		if jwtMgr, err := do.Invoke[jwt.TokenManager](b.injector); err == nil && jwtMgr != nil {
			jwtMetrics := jwt.NewJWTMetrics(jwt.JWTMetricsConfig{Enabled: true})
			if err := registry.Register(jwtMetrics); err == nil {
				// 注入 Metrics 到 TokenManager，实现自动指标记录
				if jwt.SetTokenManagerMetrics(jwtMgr, jwtMetrics) {
					b.logger.DebugCtx(b.ctx, "✅ JWT Metrics registered with Hook")
				} else {
					b.logger.DebugCtx(b.ctx, "✅ JWT Metrics registered (no hook support)")
				}
			}
		}
	}

	// Register Auth Metrics
	if metricsCfg.Auth.Enabled {
		authMetrics := auth.NewAuthMetrics(auth.AuthMetricsConfig{Enabled: true})
		if err := registry.Register(authMetrics); err == nil {
			// 注入 Metrics 到 PasswordService，实现密码验证指标记录
			if pwdSvc, err := do.Invoke[*auth.PasswordService](b.injector); err == nil && pwdSvc != nil {
				pwdSvc.SetMetrics(authMetrics)
				b.logger.DebugCtx(b.ctx, "✅ Auth Metrics registered with PasswordService")
			} else {
				b.logger.DebugCtx(b.ctx, "✅ Auth Metrics registered (no PasswordService)")
			}
		}
	}

	// Register Event Metrics
	if metricsCfg.Event.Enabled {
		if eventDisp, err := do.Invoke[event.Dispatcher](b.injector); err == nil && eventDisp != nil {
			eventMetrics := event.NewEventMetrics(event.EventMetricsConfig{Enabled: true})
			if err := registry.Register(eventMetrics); err == nil {
				// 注入 Metrics 到 Dispatcher，实现事件指标记录
				if event.SetDispatcherMetrics(eventDisp, eventMetrics) {
					b.logger.DebugCtx(b.ctx, "✅ Event Metrics registered with Dispatcher")
				} else {
					b.logger.DebugCtx(b.ctx, "✅ Event Metrics registered (no hook support)")
				}
			}
		}
	}

	// Register Kafka Metrics
	if metricsCfg.Kafka.Enabled {
		if kafkaMgr, err := do.Invoke[*kafka.Manager](b.injector); err == nil && kafkaMgr != nil {
			kafkaMetrics := kafka.NewKafkaMetrics(kafka.KafkaMetricsConfig{
				Enabled:   true,
				RecordLag: metricsCfg.Kafka.RecordLag,
			})
			if err := registry.Register(kafkaMetrics); err == nil {
				// 注入 Metrics 到 Kafka Manager，实现消息指标记录
				kafkaMgr.SetMetrics(kafkaMetrics)
				b.logger.DebugCtx(b.ctx, "✅ Kafka Metrics registered with Manager")
			}
		}
	}

	// Register Database Metrics
	if metricsCfg.Database.Enabled {
		if dbMgr, err := do.Invoke[*database.Manager](b.injector); err == nil && dbMgr != nil {
			// 为每个数据库实例创建并注册 DBMetrics
			for _, dbName := range dbMgr.GetDBNames() {
				db := dbMgr.DB(dbName)
				if db == nil {
					continue
				}
				dbMetrics, err := database.NewDBMetrics(
					db,
					metricsCfg.Database.RecordSQLText,
					metricsCfg.Database.SlowQuerySeconds,
				)
				if err != nil {
					b.logger.WarnCtx(b.ctx, "⚠️ Failed to create DB Metrics",
						zap.String("db", dbName), zap.Error(err))
					continue
				}
				// 注册 GORM Plugin 到数据库实例
				if err := dbMgr.SetMetricsPlugin(dbName, dbMetrics); err != nil {
					b.logger.WarnCtx(b.ctx, "⚠️ Failed to set DB Metrics Plugin",
						zap.String("db", dbName), zap.Error(err))
					continue
				}
				b.logger.DebugCtx(b.ctx, "✅ Database Metrics registered",
					zap.String("db", dbName))
			}
		}
	}

	// Register Breaker Metrics
	if metricsCfg.Breaker.Enabled {
		if breakerMgr, err := do.Invoke[*breaker.Manager](b.injector); err == nil && breakerMgr != nil {
			breakerMetrics := breaker.NewOTelBreakerMetrics(breaker.BreakerMetricsConfig{
				Enabled:     true,
				RecordState: metricsCfg.Breaker.RecordState,
			})
			if err := registry.Register(breakerMetrics); err == nil {
				// 注入 Metrics 到 Breaker Manager，实现熔断指标记录
				breakerMgr.SetMetrics(breakerMetrics)
				b.logger.DebugCtx(b.ctx, "✅ Breaker Metrics registered with Manager")
			}
		}
	}

	// Register Limiter Metrics
	if metricsCfg.Limiter.Enabled {
		if limiterMgr, err := do.Invoke[*limiter.Manager](b.injector); err == nil && limiterMgr != nil {
			limiterMetrics := limiter.NewOTelMetrics(limiter.MetricsConfig{Enabled: true})
			if err := registry.Register(limiterMetrics); err == nil {
				// 注入 Metrics 到 Limiter Manager，实现限流指标记录
				limiterMgr.SetMetrics(limiterMetrics)
				b.logger.DebugCtx(b.ctx, "✅ Limiter Metrics registered with Manager")
			}
		}
	}
}

// Shut down gracefully (core logic)
// 🎯 Using sambert/do's Shutdown to automatically shut down all components implementing Shutdownable
// Idempotent: only the first call performs the actual shutdown; later calls return the same result.
// This allows manual Shutdown() and the blocking Run() path to race safely.
func (b *BaseApplication) Shutdown(timeout time.Duration) error {
	b.shutdownOnce.Do(func() {
		b.shutdownErr = b.doShutdown(timeout)
	})
	return b.shutdownErr
}

// doShutdown executes the actual shutdown logic (guarded by shutdownOnce)
func (b *BaseApplication) doShutdown(timeout time.Duration) error {
	b.setState(StateStopping)

	log := b.MustGetLogger()
	log.DebugCtx(b.ctx, "🔻 Starting graceful shutdown...")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Trigger OnShutdown callback (business layer cleanup)
	if b.onShutdown != nil {
		if err := b.onShutdown(ctx); err != nil {
			log.ErrorCtx(ctx, "OnShutdown callback failed", zap.Error(err))
		}
	}

	// 2. Close the DI container (automatically shut down all components that implement Shutdownable)
	if err := b.injector.Shutdown(); err != nil && err.Error() != "" {
		log.ErrorCtx(ctx, "DI container shutdown failed", zap.Error(err))
	}

	log.DebugCtx(ctx, "✅ All components have been shut down")
	b.setState(StateStopped)
	return nil
}

// Wait for shutdown signal (core logic)
// Supports SIGINT (Ctrl+C) and SIGTERM (kill) signals
// 🎯 Dual signal mechanism: The first signal triggers graceful shutdown, the second signal forces immediate exit
func (b *BaseApplication) WaitShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	logger := b.MustGetLogger()

	select {
	case sig := <-quit:
		logger.DebugCtx(b.ctx, "Shutdown signal received (graceful shutdown)", zap.String("signal", sig.String()))
		logger.DebugCtx(b.ctx, "💡 Tip: Press Ctrl+C again to force exit immediately")

		// 🎯 Cancel root context, notify all components dependent on this context
		b.cancel()

		// 🎯 Start background goroutine to listen for second signal
		go func() {
			sig := <-quit
			logger.WarnCtx(context.Background(), "⚠️  Second signal received, forcing exit!", zap.String("signal", sig.String()))
			os.Exit(1) // Force exit
		}()

	case <-b.ctx.Done():
		logger.DebugCtx(context.Background(), "Context cancelled, starting graceful shutdown")
	}
}

// Cancel manually triggered shutdown (for testing or program control)
func (b *BaseApplication) Cancel() {
	b.cancel()
}

// OnSetup registers the callback for the Setup phase
func (b *BaseApplication) OnSetup(fn func(*BaseApplication) error) *BaseApplication {
	b.onSetup = fn
	return b
}

// OnReady register startup completion callback (type-specific initialization)
func (b *BaseApplication) OnReady(fn func(*BaseApplication) error) *BaseApplication {
	b.onReady = fn
	return b
}

// Register configuration update callback
func (b *BaseApplication) OnConfigReload(fn func(*config.Loader)) *BaseApplication {
	b.onConfigReload = fn
	return b
}

// OnShutdown register shutdown callback (clean up resources)
func (b *BaseApplication) OnShutdown(fn func(context.Context) error) *BaseApplication {
	b.onShutdown = fn
	return b
}

// MustGetLogger Get logger instance (directly return cached field, initialized in Setup phase)
func (b *BaseApplication) MustGetLogger() *logger.CtxZapLogger {
	if b.logger == nil {
		panic("logger not initialized, please call Setup() first")
	}
	return b.logger
}

// GetConfigLoader Retrieve configuration loader (directly return cached field, initialized during Setup phase)
func (b *BaseApplication) GetConfigLoader() *config.Loader {
	if b.configLoader == nil {
		panic("config loader not initialized, please call Setup() first")
	}
	return b.configLoader
}

// GetInjector obtains the sanber/do injector
func (b *BaseApplication) GetInjector() *do.RootScope {
	return b.injector
}

// LoadAppConfig retrieves common configurations (already loaded and cached in NewBase)
func (b *BaseApplication) LoadAppConfig() (*AppConfig, error) {
	if b.appConfig == nil {
		return nil, fmt.Errorf("AppConfig AppConfig not initialized")
	}
	return b.appConfig, nil
}

// GetState Get current state (thread-safe)
func (b *BaseApplication) GetState() AppState {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

// Retrieve application context
func (b *BaseApplication) Context() context.Context {
	return b.ctx
}

// ═══════════════════════════════════════════════════════════
// Depends on container method (BaseApplication as IoC container)
// ═══════════════════════════════════════════════════════════

// ServiceApp unified entry interface for long-running service applications
// (HTTP/Cron/gRPC). CLI applications are synchronous one-shot executables and
// keep their own Execute() signature.
type ServiceApp interface {
	Run() error
}

// Compile-time assertions: all service-type applications share one entry shape.
var (
	_ ServiceApp = (*Application)(nil)
	_ ServiceApp = (*CronApplication)(nil)
	_ ServiceApp = (*GRPCApplication)(nil)
)

// set state (thread-safe)
func (b *BaseApplication) setState(state AppState) {
	b.mu.Lock()
	defer b.mu.Unlock()

	oldState := b.state
	b.state = state

	// Using cached logger (initialized after Setup)
	if b.logger != nil {
		b.logger.DebugCtx(b.ctx, "State changed",
			zap.String("from", oldState.String()),
			zap.String("to", state.String()))
	}
}
