// Package di 提供基于 samber/do 的依赖注入支持
package di

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/KOMKZ/go-yogan-framework/config"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/samber/do/v2"
	"go.uber.org/zap"
)

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

// DoApplication 基于 samber/do 的应用框架
// 替代原有 BaseApplication，使用 samber/do 管理组件生命周期
type DoApplication struct {
	// ═══════════════════════════════════════════════════════════
	// 核心：samber/do 注入器
	// ═══════════════════════════════════════════════════════════
	injector *do.RootScope

	// 配置管理
	configPath   string
	configPrefix string
	configLoader *config.Loader

	// 日志
	logger *logger.CtxZapLogger

	// 生命周期
	ctx    context.Context
	cancel context.CancelFunc
	state  AppState
	mu     sync.RWMutex

	// 应用元信息
	name    string
	version string

	// 回调函数
	onSetup        func(*DoApplication) error
	onReady        func(*DoApplication) error
	onConfigReload func(*config.Loader)
	onShutdown     func(context.Context) error
}

// DoAppOption 应用选项函数
type DoAppOption func(*DoApplication)

// WithConfigPath 设置配置路径
func WithConfigPath(path string) DoAppOption {
	return func(app *DoApplication) {
		app.configPath = path
	}
}

// WithConfigPrefix 设置配置前缀
func WithConfigPrefix(prefix string) DoAppOption {
	return func(app *DoApplication) {
		app.configPrefix = prefix
	}
}

// WithName 设置应用名称
func WithName(name string) DoAppOption {
	return func(app *DoApplication) {
		app.name = name
	}
}

// WithVersion 设置应用版本
func WithVersion(version string) DoAppOption {
	return func(app *DoApplication) {
		app.version = version
	}
}

// WithOnSetup 设置 Setup 回调
func WithOnSetup(fn func(*DoApplication) error) DoAppOption {
	return func(app *DoApplication) {
		app.onSetup = fn
	}
}

// WithOnReady 设置 Ready 回调
func WithOnReady(fn func(*DoApplication) error) DoAppOption {
	return func(app *DoApplication) {
		app.onReady = fn
	}
}

// WithOnShutdown 设置 Shutdown 回调
func WithOnShutdown(fn func(context.Context) error) DoAppOption {
	return func(app *DoApplication) {
		app.onShutdown = fn
	}
}

// NewDoApplication 创建基于 samber/do 的应用实例
func NewDoApplication(opts ...DoAppOption) *DoApplication {
	ctx, cancel := context.WithCancel(context.Background())

	app := &DoApplication{
		injector:   do.New(),
		configPath: "./configs",
		ctx:        ctx,
		cancel:     cancel,
		state:      StateInit,
		name:       "yogan-app",
		version:    "0.0.1",
	}

	// 应用选项
	for _, opt := range opts {
		opt(app)
	}

	return app
}

// Injector 获取 do.Injector
func (app *DoApplication) Injector() *do.RootScope {
	return app.injector
}

// Logger 获取日志实例
func (app *DoApplication) Logger() *logger.CtxZapLogger {
	return app.logger
}

// ConfigLoader 获取配置加载器
func (app *DoApplication) ConfigLoader() *config.Loader {
	return app.configLoader
}

// State 获取当前状态
func (app *DoApplication) State() AppState {
	app.mu.RLock()
	defer app.mu.RUnlock()
	return app.state
}

// setState 设置状态
func (app *DoApplication) setState(state AppState) {
	app.mu.Lock()
	defer app.mu.Unlock()
	app.state = state
}

// Setup 初始化阶段
// 1. 加载配置
// 2. 初始化日志
// 3. 注册核心 Provider
func (app *DoApplication) Setup() error {
	app.setState(StateSetup)

	// 1. 初始化配置
	opts := ConfigOptions{
		ConfigPath:   app.configPath,
		ConfigPrefix: app.configPrefix,
		AppType:      "http",
	}
	do.Provide(app.injector, ProvideConfigLoader(opts))

	loader, err := do.Invoke[*config.Loader](app.injector)
	if err != nil {
		return fmt.Errorf("初始化配置失败: %w", err)
	}
	app.configLoader = loader

	// 2. 初始化日志
	do.Provide(app.injector, ProvideLoggerManager)
	do.Provide(app.injector, ProvideCtxLogger(app.name))

	appLogger, err := do.Invoke[*logger.CtxZapLogger](app.injector)
	if err != nil {
		return fmt.Errorf("初始化日志失败: %w", err)
	}
	app.logger = appLogger

	app.logger.Info("🔧 应用初始化中...",
		zap.String("name", app.name),
		zap.String("version", app.version),
		zap.String("config_path", app.configPath),
	)

	// 3. 调用 Setup 回调
	if app.onSetup != nil {
		if err := app.onSetup(app); err != nil {
			return fmt.Errorf("setup 回调失败: %w", err)
		}
	}

	return nil
}

// Start 启动应用
func (app *DoApplication) Start() error {
	app.setState(StateRunning)

	app.logger.Info("✅ 应用启动完成",
		zap.String("name", app.name),
		zap.String("version", app.version),
		zap.String("state", app.State().String()),
	)

	// 调用 Ready 回调
	if app.onReady != nil {
		if err := app.onReady(app); err != nil {
			return fmt.Errorf("ready 回调失败: %w", err)
		}
	}

	return nil
}

// Run 运行应用（阻塞等待信号）
func (app *DoApplication) Run() error {
	// Setup
	if err := app.Setup(); err != nil {
		return err
	}

	// Start
	if err := app.Start(); err != nil {
		return err
	}

	// 等待退出信号
	app.waitForSignal()

	return nil
}

// waitForSignal 等待退出信号
func (app *DoApplication) waitForSignal() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	sig := <-quit
	app.logger.Info("📥 收到退出信号", zap.String("signal", sig.String()))

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := app.Shutdown(ctx); err != nil {
		app.logger.Error("关闭失败", zap.Error(err))
	}
}

// Shutdown 优雅关闭
// samber/do 会自动按依赖顺序反向关闭
func (app *DoApplication) Shutdown(ctx context.Context) error {
	app.setState(StateStopping)
	app.logger.Info("🔄 开始优雅关闭...")

	// 1. 调用用户自定义关闭回调
	if app.onShutdown != nil {
		if err := app.onShutdown(ctx); err != nil {
			app.logger.Warn("shutdown 回调失败", zap.Error(err))
		}
	}

	// 2. 取消上下文
	app.cancel()

	// 3. 关闭 samber/do 容器（自动按依赖顺序关闭）
	if err := app.injector.Shutdown(); err != nil {
		app.logger.Warn("injector shutdown 失败", zap.Error(err))
	}

	app.setState(StateStopped)
	app.logger.Info("✅ 应用已关闭")

	return nil
}

// HealthCheck 健康检查
func (app *DoApplication) HealthCheck() map[string]error {
	return app.injector.HealthCheck()
}

// IsHealthy 是否健康
func (app *DoApplication) IsHealthy() bool {
	checks := app.HealthCheck()
	for _, err := range checks {
		if err != nil {
			return false
		}
	}
	return true
}
