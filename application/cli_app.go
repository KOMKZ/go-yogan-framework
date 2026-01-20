// Package application 提供通用的应用启动框架
// CLIApplication 是 CLI 应用专用（组合 BaseApplication）
package application

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// CLIApplication CLI 应用（组合 BaseApplication + CLI 专有功能）
type CLIApplication struct {
	*BaseApplication // 组合核心框架（80% 通用逻辑）

	// CLI 专有字段
	rootCmd *cobra.Command
}

// NewCLI 创建 CLI 应用实例
// configPath: 配置目录路径（如 ../configs/cli-app）
// configPrefix: 环境变量前缀（如 "APP"）
// rootCmd: Cobra 根命令
func NewCLI(configPath, configPrefix string, rootCmd *cobra.Command) *CLIApplication {
	// 默认值处理
	if configPath == "" {
		configPath = "../configs" // 不应该用，但防御性默认
	}
	if configPrefix == "" {
		configPrefix = "APP"
	}

	baseApp := NewBase(configPath, configPrefix, "cli", nil)

	return &CLIApplication{
		BaseApplication: baseApp,
		rootCmd:         rootCmd,
	}
}

// NewCLIWithDefaults 创建 CLI 应用实例（使用默认配置）
// appName: 应用名称（如 cli-app），用于构建默认配置路径
func NewCLIWithDefaults(appName string, rootCmd *cobra.Command) *CLIApplication {
	return NewCLI("../configs/"+appName, "APP", rootCmd)
}

// OnSetup 注册 Setup 阶段回调（链式调用）
func (c *CLIApplication) OnSetup(fn func(*CLIApplication) error) *CLIApplication {
	// 转换为 BaseApplication 回调
	c.BaseApplication.OnSetup(func(base *BaseApplication) error {
		return fn(c)
	})
	return c
}

// OnReady 注册启动完成回调（链式调用）
func (c *CLIApplication) OnReady(fn func(*CLIApplication) error) *CLIApplication {
	// 转换为 BaseApplication 回调
	c.BaseApplication.OnReady(func(base *BaseApplication) error {
		return fn(c)
	})
	return c
}

// OnShutdown 注册关闭前回调（链式调用）
func (c *CLIApplication) OnShutdown(fn func(*CLIApplication) error) *CLIApplication {
	// 转换为 BaseApplication 回调
	c.BaseApplication.onShutdown = func(ctx context.Context) error {
		return fn(c)
	}
	return c
}

// Execute 执行 CLI 命令（同步执行，完成后退出）
func (c *CLIApplication) Execute() error {
	// 1. Setup 阶段（初始化所有组件）
	if err := c.Setup(); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}

	// 2. 触发 OnReady（CLI 应用自定义初始化）
	c.BaseApplication.setState(StateRunning)
	if c.BaseApplication.onReady != nil {
		if err := c.BaseApplication.onReady(c.BaseApplication); err != nil {
			return fmt.Errorf("onReady failed: %w", err)
		}
	}

	logger := c.MustGetLogger()
	logger.DebugCtx(c.ctx, "✅ CLI application initialized", zap.Duration("startup_time", c.GetStartDuration()))

	// 3. 执行 Cobra 命令（同步）
	err := c.rootCmd.Execute()

	// 4. 优雅关闭（无论成功失败都要清理资源）
	shutdownErr := c.gracefulShutdown()

	if err != nil {
		return err
	}
	return shutdownErr
}

// gracefulShutdown CLI 应用优雅关闭
func (c *CLIApplication) gracefulShutdown() error {
	logger := c.MustGetLogger()
	logger.DebugCtx(c.ctx, "Starting CLI application graceful shutdown...")

	// 调用 Base 的通用关闭逻辑（5秒超时，CLI 应用通常很快）
	return c.BaseApplication.Shutdown(5 * time.Second)
}

// GetRootCmd 获取根命令（供测试使用）
func (c *CLIApplication) GetRootCmd() *cobra.Command {
	return c.rootCmd
}

// AddCommand 添加子命令（便捷方法）
func (c *CLIApplication) AddCommand(cmds ...*cobra.Command) *CLIApplication {
	c.rootCmd.AddCommand(cmds...)
	return c
}
