// Package application 提供通用的应用启动框架
// BaseApplication 是所有应用类型的核心抽象（HTTP/CLI/Cron）
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
	"github.com/KOMKZ/go-yogan-framework/cache"
	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/KOMKZ/go-yogan-framework/config"
	"github.com/KOMKZ/go-yogan-framework/database"
	"github.com/KOMKZ/go-yogan-framework/event"
	"github.com/KOMKZ/go-yogan-framework/health"
	"github.com/KOMKZ/go-yogan-framework/jwt"
	"github.com/KOMKZ/go-yogan-framework/kafka"
	"github.com/KOMKZ/go-yogan-framework/limiter"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/KOMKZ/go-yogan-framework/redis"
	"github.com/KOMKZ/go-yogan-framework/registry"
	"github.com/samber/do/v2"
	"go.uber.org/zap"
)

// BaseApplication 应用核心框架（80% 通用逻辑）
// 支持 HTTP/CLI/Cron 等所有应用类型
type BaseApplication struct {
	// ═══════════════════════════════════════════════════════════
	// 组件注册中心（统一管理所有组件）
	// ═══════════════════════════════════════════════════════════
	registry *registry.Registry // 🎯 使用具体类型，支持泛型方法
	injector *do.RootScope      // 🎯 samber/do 注入器（新）

	// 配置管理（仅用于初始化时）
	configPath   string
	configPrefix string
	appConfig    *AppConfig // 缓存加载的配置，避免重复反序列化

	// 核心组件缓存（避免重复从 Registry 获取）
	logger       *logger.CtxZapLogger
	configLoader *config.Loader

	// ═══════════════════════════════════════════════════════════
	// 依赖容器（业务扩展）
	// ═══════════════════════════════════════════════════════════
	// 业务应用可以注册额外的依赖（如 Redis、MQ 等）
	dependencies map[string]interface{}
	depsMu       sync.RWMutex

	// 生命周期
	ctx    context.Context
	cancel context.CancelFunc
	state  AppState
	mu     sync.RWMutex

	// 应用元信息
	version string // 应用版本号

	// 回调函数（应用自定义逻辑）
	onAfterInit    func(*BaseApplication) error // 组件初始化后、启动前回调（用于注入依赖）
	onSetup        func(*BaseApplication) error // Setup 阶段回调（组件启动后）
	onReady        func(*BaseApplication) error // 启动完成回调
	onConfigReload func(*config.Loader)         // 配置更新回调
	onShutdown     func(context.Context) error  // 关闭前回调
}

// AppState 应用状态
type AppState int

const (
	StateInit AppState = iota
	StateSetup
	StateRunning
	StateStopping
	StateStopped
)

// String 状态字符串表示
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

// NewBase 创建基础应用实例（内部使用）
// configPath: 配置目录路径（如 ../configs/user-api）
// configPrefix: 环境变量前缀（如 "APP"）
// appType: 应用类型（http/grpc/cli/cron）
// flags: 命令行参数（可选，nil 表示不使用）
func NewBase(configPath, configPrefix, appType string, flags interface{}) *BaseApplication {
	ctx, cancel := context.WithCancel(context.Background())

	// ═══════════════════════════════════════════════════════════
	// 1. 手动初始化 Config 和 Logger 组件（优先级最高）
	// ═══════════════════════════════════════════════════════════
	// Config 组件初始化
	configComp := NewConfigComponent(configPath, configPrefix, appType, flags)
	if err := configComp.Init(ctx, nil); err != nil {
		panic(fmt.Sprintf("配置组件初始化失败: %v", err))
	}

	// Logger 组件初始化（复用组件自己的 Init 逻辑）
	loggerComp := NewLoggerComponent()
	if err := loggerComp.Init(ctx, configComp); err != nil {
		panic(fmt.Sprintf("日志组件初始化失败: %v", err))
	}
	coreLogger := loggerComp.GetLogger()

	// ═══════════════════════════════════════════════════════════
	// 2. 创建 Registry 和 Injector
	// ═══════════════════════════════════════════════════════════
	reg := NewRegistry()
	reg.SetLogger(coreLogger) // ← 注入 Logger，Registry 从此有日志能力
	injector := do.New()      // 🎯 创建 samber/do 注入器

	// ═══════════════════════════════════════════════════════════
	// 3. 注册 Config 和 Logger 组件到 Registry（已初始化）
	// ═══════════════════════════════════════════════════════════
	reg.MustRegister(configComp)
	reg.MustRegister(loggerComp)

	// ═══════════════════════════════════════════════════════════
	// 4. 注册 Config 和 Logger 到 samber/do（统一依赖注入）
	// ═══════════════════════════════════════════════════════════
	do.ProvideValue(injector, configComp.GetLoader()) // *config.Loader
	do.ProvideValue(injector, coreLogger)             // *logger.CtxZapLogger

	// ═══════════════════════════════════════════════════════════
	// 5. 加载通用 AppConfig（configLoader 已可用）
	// ═══════════════════════════════════════════════════════════
	var appCfg AppConfig
	if err := configComp.GetLoader().Unmarshal(&appCfg); err != nil {
		panic(fmt.Sprintf("加载 AppConfig 失败: %v", err))
	}

	coreLogger.DebugCtx(ctx, "✅ 基础应用初始化完成",
		zap.String("configPath", configPath),
		zap.String("prefix", configPrefix),
		zap.String("appType", appType))

	return &BaseApplication{
		registry:     reg,
		injector:     injector,               // 🎯 samber/do 注入器
		configPath:   configPath,
		configPrefix: configPrefix,
		logger:       coreLogger,             // ← 直接缓存
		configLoader: configComp.GetLoader(), // ← 直接缓存
		appConfig:    &appCfg,                // ← 直接缓存
		ctx:          ctx,
		cancel:       cancel,
		state:        StateInit,
		dependencies: make(map[string]interface{}),
	}
}

// NewBaseWithDefaults 创建基础应用实例（使用默认配置路径）
// appName: 应用名称（如 user-api），用于构建默认配置路径
// appType: 应用类型（http/grpc/cli/cron）
// 默认配置路径：../configs/{appName}
// 默认环境前缀：APP
func NewBaseWithDefaults(appName, appType string) *BaseApplication {
	defaultPath := "../configs/" + appName
	return NewBase(defaultPath, "APP", appType, nil)
}

// Register 注册组件（链式调用）
// 业务应用可以注册额外的组件（Database、Redis、自定义组件等）
// 注册失败会 panic（Fail Fast 策略）
func (b *BaseApplication) Register(components ...component.Component) *BaseApplication {
	for _, comp := range components {
		if err := b.registry.Register(comp); err != nil {
			panic(fmt.Sprintf("注册组件 '%s' 失败: %v", comp.Name(), err))
		}
	}
	return b
}

// WithVersion 设置应用版本号（链式调用）
// 版本号将在应用启动时自动打印
func (b *BaseApplication) WithVersion(version string) *BaseApplication {
	b.version = version
	return b
}

// GetVersion 获取应用版本号
func (b *BaseApplication) GetVersion() string {
	return b.version
}

// Setup 初始化所有组件（核心逻辑）
func (b *BaseApplication) Setup() error {
	b.setState(StateSetup)

	// 1. 初始化所有组件（按依赖顺序）- Registry 已有 Logger，从一开始就有日志
	if err := b.registry.Init(b.ctx); err != nil {
		return fmt.Errorf("组件初始化失败: %w", err)
	}

	// 2. 自动注入核心组件间的依赖（内核职责，应用层无需关心）
	b.injectCoreComponentDependencies()

	// 3. 触发 OnAfterInit 回调（用于应用层特定的依赖注入）
	if b.onAfterInit != nil {
		if err := b.onAfterInit(b); err != nil {
			return fmt.Errorf("onAfterInit failed: %w", err)
		}
	}

	// 4. 启动所有组件 - Registry 会输出启动日志
	if err := b.registry.Start(b.ctx); err != nil {
		return fmt.Errorf("组件启动失败: %w", err)
	}

	// 5. 自动注册核心组件到 samber/do（组件启动后才能获取 Manager 等）
	b.registerCoreComponentsToDo()

	// 6. 触发 OnSetup 回调（应用自定义准备）
	if b.onSetup != nil {
		if err := b.onSetup(b); err != nil {
			return fmt.Errorf("onSetup failed: %w", err)
		}
	}

	return nil
}

// Shutdown 优雅关闭（核心逻辑）
func (b *BaseApplication) Shutdown(timeout time.Duration) error {
	b.setState(StateStopping)

	logger := b.MustGetLogger()
	logger.DebugCtx(b.ctx, "Starting graceful shutdown...")

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 1. 触发 OnShutdown 回调（业务层清理）
	if b.onShutdown != nil {
		if err := b.onShutdown(ctx); err != nil {
			logger.ErrorCtx(ctx, "OnShutdown callback failed", zap.Error(err))
			// 继续执行清理流程
		}
	}

	// 2. 停止所有组件（反向顺序）
	if err := b.registry.Stop(ctx); err != nil {
		logger.ErrorCtx(ctx, "Component stop failed", zap.Error(err))
	}

	b.setState(StateStopped)
	return nil
}

// WaitShutdown 等待关闭信号（核心逻辑）
// 支持 SIGINT (Ctrl+C) 和 SIGTERM (kill) 信号
// 🎯 双信号机制：第一次信号触发优雅关停，第二次信号立即强制退出
func (b *BaseApplication) WaitShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	logger := b.MustGetLogger()

	select {
	case sig := <-quit:
		logger.DebugCtx(b.ctx, "Shutdown signal received (graceful shutdown)", zap.String("signal", sig.String()))
		logger.DebugCtx(b.ctx, "💡 Tip: Press Ctrl+C again to force exit immediately")

		// 🎯 取消 root context，通知所有依赖此 context 的组件
		b.cancel()

		// 🎯 启动后台 goroutine 监听第二次信号
		go func() {
			sig := <-quit
			logger.WarnCtx(context.Background(), "⚠️  Second signal received, forcing exit!", zap.String("signal", sig.String()))
			os.Exit(1) // 强制退出
		}()

	case <-b.ctx.Done():
		logger.DebugCtx(context.Background(), "Context cancelled, starting graceful shutdown")
	}
}

// Cancel 手动触发关闭（用于测试或程序控制）
func (b *BaseApplication) Cancel() {
	b.cancel()
}

// OnSetup 注册 Setup 阶段回调（在配置加载后触发）
// OnAfterInit 注册组件初始化后回调
// 在所有组件 Init 完成后、Start 之前触发
// 用于在组件启动前注入依赖（如 SetRedisComponent）
func (b *BaseApplication) OnAfterInit(fn func(*BaseApplication) error) *BaseApplication {
	b.onAfterInit = fn
	return b
}

func (b *BaseApplication) OnSetup(fn func(*BaseApplication) error) *BaseApplication {
	b.onSetup = fn
	return b
}

// OnReady 注册启动完成回调（应用类型特定的初始化）
func (b *BaseApplication) OnReady(fn func(*BaseApplication) error) *BaseApplication {
	b.onReady = fn
	return b
}

// OnConfigReload 注册配置更新回调
func (b *BaseApplication) OnConfigReload(fn func(*config.Loader)) *BaseApplication {
	b.onConfigReload = fn
	return b
}

// OnShutdown 注册关闭前回调（清理资源）
func (b *BaseApplication) OnShutdown(fn func(context.Context) error) *BaseApplication {
	b.onShutdown = fn
	return b
}

// MustGetLogger 获取日志实例（直接返回缓存字段，Setup 阶段已初始化）
func (b *BaseApplication) MustGetLogger() *logger.CtxZapLogger {
	if b.logger == nil {
		panic("logger not initialized, please call Setup() first")
	}
	return b.logger
}

// GetConfigLoader 获取配置加载器（直接返回缓存字段，Setup 阶段已初始化）
func (b *BaseApplication) GetConfigLoader() *config.Loader {
	if b.configLoader == nil {
		panic("config loader not initialized, please call Setup() first")
	}
	return b.configLoader
}

// GetInjector 获取 samber/do 注入器
func (b *BaseApplication) GetInjector() *do.RootScope {
	return b.injector
}

// LoadAppConfig 获取通用配置（已在 NewBase 中加载并缓存）
func (b *BaseApplication) LoadAppConfig() (*AppConfig, error) {
	if b.appConfig == nil {
		return nil, fmt.Errorf("AppConfig 未初始化")
	}
	return b.appConfig, nil
}

// GetState 获取当前状态（线程安全）
func (b *BaseApplication) GetState() AppState {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

// Context 获取应用上下文
func (b *BaseApplication) Context() context.Context {
	return b.ctx
}

// ═══════════════════════════════════════════════════════════
// 依赖容器方法（BaseApplication 作为 IoC 容器）
// ═══════════════════════════════════════════════════════════

// Set 注册依赖到容器（线程安全）
func (b *BaseApplication) Set(key string, value interface{}) {
	b.depsMu.Lock()
	defer b.depsMu.Unlock()
	b.dependencies[key] = value
}

// Get 从容器获取依赖（线程安全）
func (b *BaseApplication) Get(key string) interface{} {
	b.depsMu.RLock()
	defer b.depsMu.RUnlock()
	return b.dependencies[key]
}

// MustGet 从容器获取依赖（不存在则 panic）
func (b *BaseApplication) MustGet(key string) interface{} {
	val := b.Get(key)
	if val == nil {
		panic(fmt.Sprintf("dependency '%s' not found", key))
	}
	return val
}

// Has 检查依赖是否存在
func (b *BaseApplication) Has(key string) bool {
	b.depsMu.RLock()
	defer b.depsMu.RUnlock()
	_, exists := b.dependencies[key]
	return exists
}

// ═══════════════════════════════════════════════════════════
// 组件访问方法（推荐使用 Registry 直接获取）
// ═══════════════════════════════════════════════════════════

// GetRegistry 获取组件注册中心
func (b *BaseApplication) GetRegistry() *registry.Registry {
	return b.registry
}

// setState 设置状态（线程安全）
func (b *BaseApplication) setState(state AppState) {
	b.mu.Lock()
	defer b.mu.Unlock()

	oldState := b.state
	b.state = state

	// 使用缓存的 logger（Setup 后已初始化）
	if b.logger != nil {
		b.logger.DebugCtx(b.ctx, "State changed",
			zap.String("from", oldState.String()),
			zap.String("to", state.String()))
	}
}

// injectCoreComponentDependencies 自动注入核心组件间的依赖
// 在组件 Init 后、Start 前调用
// 内核职责：JWT/Auth/Limiter/Cache 需要 Redis，由内核自动处理
func (b *BaseApplication) injectCoreComponentDependencies() {
	// 获取 Redis 组件（已初始化）
	redisComp, ok := registry.GetTyped[*redis.Component](b.registry, component.ComponentRedis)
	if !ok {
		// Redis 未注册，跳过依赖注入
		return
	}

	// 注入 Redis 到 JWT 组件
	if jwtComp, ok := registry.GetTyped[*jwt.Component](b.registry, component.ComponentJWT); ok {
		jwtComp.SetRedisComponent(redisComp)
		b.logger.DebugCtx(b.ctx, "✅ Redis 注入到 JWT 组件")
	}

	// 注入 Redis 到 Auth 组件
	if authComp, ok := registry.GetTyped[*auth.Component](b.registry, component.ComponentAuth); ok {
		authComp.SetRedisComponent(redisComp)
		b.logger.DebugCtx(b.ctx, "✅ Redis 注入到 Auth 组件")
	}

	// 注入 Redis 到 Limiter 组件
	if limiterComp, ok := registry.GetTyped[*limiter.Component](b.registry, component.ComponentLimiter); ok {
		limiterComp.SetRedisComponent(redisComp)
		b.logger.DebugCtx(b.ctx, "✅ Redis 注入到 Limiter 组件")
	}

	// 注入 Redis Manager 到 Cache 组件
	if cacheComp, ok := registry.GetTyped[*cache.Component](b.registry, component.ComponentCache); ok {
		if redisComp.GetManager() != nil {
			cacheComp.SetRedisManager(redisComp.GetManager())
			b.logger.DebugCtx(b.ctx, "✅ Redis Manager 注入到 Cache 组件")
		}
	}
}

// registerCoreComponentsToDo 自动注册核心组件到 samber/do
// 在组件启动后调用，确保 Manager 等已初始化
// 注册策略：同时注册 Manager（多实例访问）和默认实例（便捷访问）
func (b *BaseApplication) registerCoreComponentsToDo() {
	// Database - 注册 Manager 和默认 DB
	if dbComp, ok := registry.GetTyped[*database.Component](b.registry, component.ComponentDatabase); ok {
		if mgr := dbComp.GetManager(); mgr != nil {
			do.ProvideValue(b.injector, mgr) // *database.Manager（多连接访问）
			if db := mgr.DB("master"); db != nil {
				do.ProvideValue(b.injector, db) // *gorm.DB（默认 master）
			}
		}
	}

	// Redis - 注册 Manager 和默认 Client
	if redisComp, ok := registry.GetTyped[*redis.Component](b.registry, component.ComponentRedis); ok {
		if mgr := redisComp.GetManager(); mgr != nil {
			do.ProvideValue(b.injector, mgr) // *redis.Manager（多实例访问）
			if client := mgr.Client("main"); client != nil {
				do.ProvideValue(b.injector, client) // *goredis.Client（默认 main）
			}
		}
	}

	// JWT - 注册 TokenManager 和 Config
	if jwtComp, ok := registry.GetTyped[*jwt.Component](b.registry, component.ComponentJWT); ok {
		do.ProvideValue[jwt.TokenManager](b.injector, jwtComp.GetTokenManager())
		do.ProvideValue(b.injector, jwtComp.GetConfig())
	}

	// Auth - 注册 AuthService
	if authComp, ok := registry.GetTyped[*auth.Component](b.registry, component.ComponentAuth); ok {
		do.ProvideValue(b.injector, authComp.GetAuthService())
	}

	// Event - 注册 Component 和 Dispatcher
	if eventComp, ok := registry.GetTyped[*event.Component](b.registry, component.ComponentEvent); ok {
		do.ProvideValue(b.injector, eventComp)                               // *event.Component
		do.ProvideValue[event.Dispatcher](b.injector, eventComp.GetDispatcher()) // event.Dispatcher
	}

	// Cache - 注册 Component
	if cacheComp, ok := registry.GetTyped[*cache.Component](b.registry, component.ComponentCache); ok {
		do.ProvideValue(b.injector, cacheComp)
	}

	// Health - 注册 Component
	if healthComp, ok := registry.GetTyped[*health.Component](b.registry, component.ComponentHealth); ok {
		do.ProvideValue(b.injector, healthComp)
	}

	// Kafka - 注册 Manager
	if kafkaComp, ok := registry.GetTyped[*kafka.Component](b.registry, component.ComponentKafka); ok {
		if mgr := kafkaComp.GetManager(); mgr != nil {
			do.ProvideValue(b.injector, mgr) // *kafka.Manager
		}
	}

	// Limiter - 注册 Manager
	if limiterComp, ok := registry.GetTyped[*limiter.Component](b.registry, component.ComponentLimiter); ok {
		if mgr := limiterComp.GetManager(); mgr != nil {
			do.ProvideValue(b.injector, mgr) // *limiter.Manager
		}
	}

	// Event ← Kafka：自动配置 Kafka 发布者
	if eventComp, ok := registry.GetTyped[*event.Component](b.registry, component.ComponentEvent); ok {
		if kafkaComp, ok := registry.GetTyped[*kafka.Component](b.registry, component.ComponentKafka); ok {
			if mgr := kafkaComp.GetManager(); mgr != nil {
				eventComp.SetKafkaPublisher(mgr)
				b.logger.DebugCtx(b.ctx, "✅ Kafka 注入到 Event 组件")
			}
		}
	}

	b.logger.DebugCtx(b.ctx, "✅ 核心组件已注册到 samber/do")
}
