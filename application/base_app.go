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

	"github.com/KOMKZ/go-yogan-framework/config"
	"github.com/KOMKZ/go-yogan-framework/di"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/samber/do/v2"
	"go.uber.org/zap"
)

// BaseApplication 应用核心框架（80% 通用逻辑）
// 支持 HTTP/CLI/Cron 等所有应用类型
// 🎯 全面使用 samber/do 管理组件生命周期，不再使用 Registry
type BaseApplication struct {
	// ═══════════════════════════════════════════════════════════
	// DI 容器（唯一的组件管理方式）
	// ═══════════════════════════════════════════════════════════
	injector *do.RootScope // samber/do 注入器

	// 配置管理
	configPath   string
	configPrefix string
	appConfig    *AppConfig

	// 核心组件缓存（快速访问）
	logger       *logger.CtxZapLogger
	configLoader *config.Loader

	// 生命周期
	ctx    context.Context
	cancel context.CancelFunc
	state  AppState
	mu     sync.RWMutex

	// 应用元信息
	version string

	// 回调函数
	onSetup        func(*BaseApplication) error
	onReady        func(*BaseApplication) error
	onConfigReload func(*config.Loader)
	onShutdown     func(context.Context) error
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

// NewBase 创建基础应用实例
// 🎯 全面使用 samber/do 管理所有组件，不再使用 Registry
func NewBase(configPath, configPrefix, appType string, flags interface{}) *BaseApplication {
	ctx, cancel := context.WithCancel(context.Background())
	injector := do.New()

	// 注册所有核心组件 Provider（集中管理于 di/core_registrar.go）
	di.RegisterCoreProviders(injector, di.ConfigOptions{
		ConfigPath:   configPath,
		ConfigPrefix: configPrefix,
		AppType:      appType,
		Flags:        flags,
	})

	// 立即获取 Config 和 Logger（基础依赖）
	configLoader := do.MustInvoke[*config.Loader](injector)
	coreLogger := do.MustInvoke[*logger.CtxZapLogger](injector)

	// 加载 AppConfig
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
// 🎯 组件生命周期：Provider 创建时完成 Init+Start，Shutdown 时调用 Stop
func (b *BaseApplication) Setup() error {
	b.setState(StateSetup)

	// 启动核心组件（集中管理于 di/lifecycle.go）
	if err := di.StartCoreComponents(b.ctx, b.injector, b.logger); err != nil {
		return fmt.Errorf("启动核心组件失败: %w", err)
	}

	// 触发 OnSetup 回调
	if b.onSetup != nil {
		if err := b.onSetup(b); err != nil {
			return fmt.Errorf("onSetup failed: %w", err)
		}
	}

	return nil
}

// Shutdown 优雅关闭（核心逻辑）
// 🎯 使用 samber/do 的 Shutdown 自动关闭所有实现 Shutdownable 的组件
func (b *BaseApplication) Shutdown(timeout time.Duration) error {
	b.setState(StateStopping)

	log := b.MustGetLogger()
	log.DebugCtx(b.ctx, "🔻 Starting graceful shutdown...")

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 1. 触发 OnShutdown 回调（业务层清理）
	if b.onShutdown != nil {
		if err := b.onShutdown(ctx); err != nil {
			log.ErrorCtx(ctx, "OnShutdown callback failed", zap.Error(err))
		}
	}

	// 2. 关闭 DI 容器（自动关闭所有实现 Shutdownable 的组件）
	if err := b.injector.Shutdown(); err != nil {
		log.ErrorCtx(ctx, "DI container shutdown failed", zap.Error(err))
	}

	log.DebugCtx(ctx, "✅ 所有组件已关闭")
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

// OnSetup 注册 Setup 阶段回调
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
