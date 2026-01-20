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

	"github.com/KOMKZ/go-yogan-framework/config"
	"github.com/KOMKZ/go-yogan-framework/di"
	"github.com/KOMKZ/go-yogan-framework/logger"
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

	// Register all core components Providers (centralized in di/core_registrar.go)
	di.RegisterCoreProviders(injector, di.ConfigOptions{
		ConfigPath:   configPath,
		ConfigPrefix: configPrefix,
		AppType:      appType,
		Flags:        flags,
	})

	// Immediatley obtain Config and Logger (basic dependencies)
	configLoader := do.MustInvoke[*config.Loader](injector)
	coreLogger := do.MustInvoke[*logger.CtxZapLogger](injector)

	// Load AppConfig
	var appCfg AppConfig
	if err := configLoader.Unmarshal(&appCfg); err != nil {
		panic(fmt.Sprintf("加载 AppConfig 失败: %v", err))
	}

	coreLogger.DebugCtx(ctx, "✅ 基础应用初始化完成（纯 DI 模式）",
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
// Default environment prefix: APP
func NewBaseWithDefaults(appName, appType string) *BaseApplication {
	defaultPath := "../configs/" + appName
	return NewBase(defaultPath, "APP", appType, nil)
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

// Setup initialize application (core logic)
// 🎯 Component lifecycle: Provider completes Init+Start (lazy loading) internally, automatically stops on Shutdown
func (b *BaseApplication) Setup() error {
	b.setState(StateSetup)

	// Trigger OnSetup callback
	if b.onSetup != nil {
		if err := b.onSetup(b); err != nil {
			return fmt.Errorf("onSetup failed: %w", err)
		}
	}

	return nil
}

// Shut down gracefully (core logic)
// 🎯 Using sambert/do's Shutdown to automatically shut down all components implementing Shutdownable
func (b *BaseApplication) Shutdown(timeout time.Duration) error {
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
	if err := b.injector.Shutdown(); err != nil {
		log.ErrorCtx(ctx, "DI container shutdown failed", zap.Error(err))
	}

	log.DebugCtx(ctx, "✅ 所有组件已关闭")
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

// GetInjector获取samber/do注入器
func (b *BaseApplication) GetInjector() *do.RootScope {
	return b.injector
}

// LoadAppConfig retrieves common configurations (already loaded and cached in NewBase)
func (b *BaseApplication) LoadAppConfig() (*AppConfig, error) {
	if b.appConfig == nil {
		return nil, fmt.Errorf("AppConfig 未初始化")
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
