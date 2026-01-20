// Package application 提供通用的应用启动框架
// Application 是 HTTP 应用专用（组合 BaseApplication）
package application

import (
	"context"
	"fmt"
	"time"

	"github.com/KOMKZ/go-yogan-framework/limiter"
	"github.com/KOMKZ/go-yogan-framework/swagger"
	"github.com/KOMKZ/go-yogan-framework/telemetry"
	"github.com/samber/do/v2"
	"go.uber.org/zap"
)

// Application HTTP 应用（组合 BaseApplication + HTTP 专有功能）
type Application struct {
	*BaseApplication // 组合核心框架（80% 通用逻辑）

	// HTTP Server（HTTP 专有）
	httpServer      *HTTPServer
	routerRegistrar RouterRegistrar
	routerManager   *Manager // 路由管理器（内核组件）
}

// New 创建 HTTP 应用实例
// configPath: 配置目录路径（如 ../configs/user-api）
// configPrefix: 环境变量前缀（如 "APP"）
// flags: 命令行参数（可选，nil 表示不使用）
func New(configPath, configPrefix string, flags interface{}) *Application {
	// 默认值处理
	if configPath == "" {
		configPath = "../configs" // 不应该用，但防御性默认
	}
	if configPrefix == "" {
		configPrefix = "APP"
	}

	baseApp := NewBase(configPath, configPrefix, "http", flags)

	return &Application{
		BaseApplication: baseApp,
		routerManager:   NewManager(), // 初始化路由管理器
	}
}

// NewWithDefaults 创建 HTTP 应用实例（使用默认配置）
// appName: 应用名称（如 user-api），用于构建默认配置路径
func NewWithDefaults(appName string) *Application {
	return New("../configs/"+appName, "APP", nil)
}

// NewWithFlags 创建 HTTP 应用实例（支持命令行参数）
// configPath: 配置目录路径
// configPrefix: 环境变量前缀
// flags: 命令行参数（AppFlags 结构体）
func NewWithFlags(configPath, configPrefix string, flags interface{}) *Application {
	return New(configPath, configPrefix, flags)
}

// WithVersion 设置应用版本号（链式调用）
func (a *Application) WithVersion(version string) *Application {
	a.BaseApplication.WithVersion(version)
	return a
}

// Run 启动 HTTP 应用（阻塞直到收到关闭信号）
func (a *Application) Run() error {
	// 执行非阻塞启动
	if err := a.RunNonBlocking(); err != nil {
		return err
	}

	// 等待关闭信号
	a.WaitShutdown()

	// 优雅关闭
	return a.gracefulShutdown()
}

// RunNonBlocking 非阻塞启动 HTTP 应用（用于测试或需要手动控制生命周期的场景）
// 执行所有初始化和启动逻辑，但不等待关闭信号
func (a *Application) RunNonBlocking() error {
	// 1. Setup 阶段（初始化组件，触发 OnSetup 回调）
	if err := a.Setup(); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}

	// 2. 启动 HTTP Server（如果已注册路由）
	if err := a.startHTTPServer(); err != nil {
		return err
	}

	// 3. 触发 OnReady 回调（使用 BaseApplication 的统一回调）
	a.BaseApplication.setState(StateRunning)
	if a.BaseApplication.onReady != nil {
		if err := a.BaseApplication.onReady(a.BaseApplication); err != nil {
			return fmt.Errorf("onReady failed: %w", err)
		}
	}

	logger := a.MustGetLogger()
	fields := []zap.Field{
		zap.String("state", a.GetState().String()),
		zap.Duration("startup_time", a.GetStartDuration()),
	}
	if version := a.GetVersion(); version != "" {
		fields = append(fields, zap.String("version", version))
	}
	logger.InfoCtx(a.ctx, "✅ HTTP application started", fields...)

	return nil
}

// startHTTPServer 启动 HTTP Server（HTTP 专有逻辑）
func (a *Application) startHTTPServer() error {
	if a.routerRegistrar == nil {
		return nil
	}

	// 🎯 通过 DI 获取 Telemetry Manager（可选）
	var telemetryMgr *telemetry.Manager
	if mgr, err := do.Invoke[*telemetry.Manager](a.GetInjector()); err == nil && mgr != nil && mgr.IsEnabled() {
		telemetryMgr = mgr
	}

	// 🎯 通过 DI 获取 Limiter Manager（可选）
	var limiterMgr *limiter.Manager
	if mgr, err := do.Invoke[*limiter.Manager](a.GetInjector()); err == nil && mgr != nil {
		limiterMgr = mgr
	}

	// 创建 HTTP Server（传递中间件配置、httpx 配置、限流器和 telemetry）
	a.httpServer = NewHTTPServerWithTelemetry(
		a.appConfig.ApiServer,
		a.appConfig.Middleware,
		a.appConfig.Httpx,
		limiterMgr,
		telemetryMgr,
	)

	// 业务应用注册路由（传递 Application 依赖容器）
	a.routerRegistrar.RegisterRoutes(a.httpServer.GetEngine(), a)

	logger := a.MustGetLogger()
	logger.DebugCtx(a.ctx, "✅ Routes registered")

	// 🎯 自动挂载 Swagger 路由（如果已启用）
	if err := swagger.Setup(a.GetInjector(), a.httpServer.GetEngine()); err != nil {
		logger.WarnCtx(a.ctx, "Swagger setup failed", zap.Error(err))
	}

	// 启动 HTTP Server（非阻塞）
	if err := a.httpServer.Start(); err != nil {
		return fmt.Errorf("启动 HTTP Server 失败: %w", err)
	}

	return nil
}

// gracefulShutdown HTTP 应用优雅关闭
func (a *Application) gracefulShutdown() error {
	logger := a.MustGetLogger()
	logger.DebugCtx(a.ctx, "Starting HTTP application graceful shutdown...")

	// 1. 先关闭 HTTP Server（停止接收新请求）
	if a.httpServer != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := a.httpServer.Shutdown(shutdownCtx); err != nil {
			logger.ErrorCtx(a.ctx, "HTTP server close failed", zap.Error(err))
		}
	}

	// 2. 调用 Base 的通用关闭逻辑（触发 OnShutdown 回调 + 关闭组件）
	return a.BaseApplication.Shutdown(10 * time.Second)
}

// GetHTTPServer 获取 HTTP Server 实例（供测试使用）
func (a *Application) GetHTTPServer() *HTTPServer {
	return a.httpServer
}

// GetRouterManager 获取路由管理器（内核组件）
func (a *Application) GetRouterManager() *Manager {
	return a.routerManager
}

// Shutdown 手动触发关闭（用于测试或程序控制）
func (a *Application) Shutdown() {
	a.Cancel()
}

// OnSetup 注册 Setup 阶段回调（链式调用）
func (a *Application) OnSetup(fn func(*Application) error) *Application {
	a.BaseApplication.OnSetup(func(base *BaseApplication) error {
		return fn(a)
	})
	return a
}

// OnReady 注册启动完成回调（链式调用）
func (a *Application) OnReady(fn func(*Application) error) *Application {
	a.BaseApplication.OnReady(func(base *BaseApplication) error {
		return fn(a)
	})
	return a
}

// OnShutdown 注册关闭前回调（链式调用）
func (a *Application) OnShutdown(fn func(*Application) error) *Application {
	a.BaseApplication.OnShutdown(func(ctx context.Context) error {
		return fn(a)
	})
	return a
}

// RegisterRoutes 注册路由（HTTP 专有）
func (a *Application) RegisterRoutes(registrar RouterRegistrar) *Application {
	a.routerRegistrar = registrar
	return a
}
