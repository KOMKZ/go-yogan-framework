// Package application 提供通用的应用启动框架
// GRPCApplication 是 gRPC 应用的专用封装（类似 CLIApplication、CronApplication）
package application

import (
	"context"
	"time"

	"github.com/KOMKZ/go-yogan-framework/governance"
	"go.uber.org/zap"
)

// GRPCApplication gRPC 应用（组合 BaseApplication + gRPC 专有功能）
type GRPCApplication struct {
	*BaseApplication // 组合核心框架（80% 通用逻辑）

	// 🎯 服务治理管理器（可选，如果启用会自动注册/注销服务）
	governanceManager *governance.Manager
}

// NewGRPC 创建 gRPC 应用实例
// configPath: 配置目录路径（如 ../configs/auth-service）
// configPrefix: 环境变量前缀（如 "APP"）
// flags: 命令行参数（可选，nil 表示不使用）
func NewGRPC(configPath, configPrefix string, flags interface{}) *GRPCApplication {
	if configPath == "" {
		configPath = "../configs"
	}
	if configPrefix == "" {
		configPrefix = "APP"
	}

	baseApp := NewBase(configPath, configPrefix, "grpc", flags)

	return &GRPCApplication{
		BaseApplication: baseApp,
	}
}

// NewGRPCWithDefaults 创建 gRPC 应用实例（使用默认配置）
// appName: 应用名称（如 auth-service），用于构建默认配置路径
func NewGRPCWithDefaults(appName string) *GRPCApplication {
	return NewGRPC("../configs/"+appName, "APP", nil)
}

// NewGRPCWithFlags 创建 gRPC 应用实例（支持命令行参数）
// configPath: 配置目录路径
// configPrefix: 环境变量前缀
// flags: 命令行参数（AppFlags 结构体）
func NewGRPCWithFlags(configPath, configPrefix string, flags interface{}) *GRPCApplication {
	return NewGRPC(configPath, configPrefix, flags)
}

// OnSetup 注册 Setup 阶段回调（链式调用）
func (g *GRPCApplication) OnSetup(fn func(*GRPCApplication) error) *GRPCApplication {
	g.BaseApplication.OnSetup(func(base *BaseApplication) error {
		return fn(g)
	})
	return g
}

// OnReady 注册启动完成回调（链式调用）
func (g *GRPCApplication) OnReady(fn func(*GRPCApplication) error) *GRPCApplication {
	g.BaseApplication.OnReady(func(base *BaseApplication) error {
		return fn(g)
	})
	return g
}

// OnShutdown 注册关闭前回调（链式调用）
func (g *GRPCApplication) OnShutdown(fn func(*GRPCApplication) error) *GRPCApplication {
	g.BaseApplication.onShutdown = func(ctx context.Context) error {
		return fn(g)
	}
	return g
}

// Run 启动 gRPC 应用（阻塞直到收到关闭信号）
func (g *GRPCApplication) Run() {
	logger := g.MustGetLogger()

	// 1. Setup 阶段（初始化所有组件）
	if err := g.Setup(); err != nil {
		logger.ErrorCtx(g.ctx, "Application start failed", zap.Error(err))
		panic(err)
	}

	// 2. 🎯 自动注册服务到治理中心（如果启用）
	if g.governanceManager != nil {
		if err := g.autoRegisterService(); err != nil {
			logger.WarnCtx(g.ctx, "⚠️  Service registration failed (does not affect app startup)", zap.Error(err))

		}
	}

	// 3. 触发 OnReady（应用自定义初始化）
	g.BaseApplication.setState(StateRunning)
	if g.BaseApplication.onReady != nil {
		if err := g.BaseApplication.onReady(g.BaseApplication); err != nil {
			logger.ErrorCtx(g.ctx, "OnReady 失败", zap.Error(err))
			panic(err)
		}
	}

	logger.InfoCtx(g.ctx, "✅ gRPC application started", zap.Duration("startup_time", g.GetStartDuration()))

	// 4. 等待关闭信号（阻塞）
	g.WaitShutdown()

	// 5. 🎯 自动注销服务（如果启用）
	if g.governanceManager != nil {
		if err := g.autoDeregisterService(); err != nil {
			logger.ErrorCtx(g.ctx, "Service deregistration failed", zap.Error(err))
		}
	}

	// 6. 优雅关闭
	if err := g.gracefulShutdown(); err != nil {
		logger.ErrorCtx(g.ctx, "Application close failed", zap.Error(err))
	}
}

// gracefulShutdown gRPC 应用优雅关闭
func (g *GRPCApplication) gracefulShutdown() error {
	logger := g.MustGetLogger()
	logger.DebugCtx(g.ctx, "Starting gRPC application graceful shutdown...")

	// 调用 Base 的通用关闭逻辑（30秒超时）
	return g.BaseApplication.Shutdown(30 * time.Second)
}

// SetGovernanceManager 设置服务治理管理器（可选，用于自动服务注册/注销）
func (g *GRPCApplication) SetGovernanceManager(manager *governance.Manager) *GRPCApplication {
	g.governanceManager = manager
	return g
}

// autoRegisterService 自动注册服务（从 gRPC 组件获取端口信息）
func (g *GRPCApplication) autoRegisterService() error {
	// TODO: 从 gRPC 组件获取实际监听端口并注册服务
	logger := g.MustGetLogger()
	logger.DebugCtx(g.ctx, "🎯 Service registration enabled (implementing...)")

	return nil
}

// autoDeregisterService 自动注销服务
func (g *GRPCApplication) autoDeregisterService() error {
	if g.governanceManager == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return g.governanceManager.Shutdown(ctx)
}
