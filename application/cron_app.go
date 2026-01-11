package application

import (
	"fmt"
	"time"

	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/go-co-op/gocron/v2"
	"go.uber.org/zap"
)

// CronApplication Cron 应用（组合 BaseApplication + Cron 专有功能）
type CronApplication struct {
	*BaseApplication // 组合核心框架

	// Cron 专有
	scheduler      gocron.Scheduler
	cronOnSetup    func(*CronApplication) error
	cronOnReady    func(*CronApplication) error
	cronOnShutdown func(*CronApplication) error
	taskRegistrar  TaskRegistrar // 任务注册器
}

// TaskRegistrar 任务注册接口
type TaskRegistrar interface {
	RegisterTasks(app *CronApplication) error
}

// NewCron 创建 Cron 应用实例
// configPath: 配置目录路径（如 ../configs/cron-app）
// configPrefix: 环境变量前缀（如 "APP"）
func NewCron(configPath, configPrefix string) (*CronApplication, error) {
	if configPath == "" {
		configPath = "../configs/cron-app"
	}
	if configPrefix == "" {
		configPrefix = "APP"
	}

	baseApp := NewBase(configPath, configPrefix, "cron", nil)

	// 创建 gocron 调度器
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, fmt.Errorf("创建调度器失败: %w", err)
	}

	return &CronApplication{
		BaseApplication: baseApp,
		scheduler:       scheduler,
	}, nil
}

// NewCronWithDefaults 创建 Cron 应用实例（使用默认配置）
func NewCronWithDefaults(appName string) (*CronApplication, error) {
	return NewCron("../configs/"+appName, "APP")
}

// Register 注册组件（链式调用，重写以返回 *CronApplication）
func (a *CronApplication) Register(components ...component.Component) *CronApplication {
	a.BaseApplication.Register(components...)
	return a
}

// Run 启动 Cron 应用（阻塞直到收到关闭信号）
func (a *CronApplication) Run() error {
	return a.run(true)
}

// RunNonBlocking 非阻塞启动应用（用于测试环境）
func (a *CronApplication) RunNonBlocking() error {
	return a.run(false)
}

// run 内部启动逻辑（统一实现）
func (a *CronApplication) run(blocking bool) error {
	// 1. Setup 阶段（配置 + 日志 + 组件初始化）
	if err := a.Setup(); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}

	// 2. 触发 Cron 专有 Setup 回调
	if a.cronOnSetup != nil {
		if err := a.cronOnSetup(a); err != nil {
			return fmt.Errorf("cron onSetup failed: %w", err)
		}
	}

	// 3. 注册任务
	if a.taskRegistrar != nil {
		if err := a.taskRegistrar.RegisterTasks(a); err != nil {
			return fmt.Errorf("register tasks failed: %w", err)
		}
	}

	// 4. 启动调度器
	a.scheduler.Start()

	// 5. 触发 OnReady 回调
	a.BaseApplication.setState(StateRunning)
	if a.cronOnReady != nil {
		if err := a.cronOnReady(a); err != nil {
			return fmt.Errorf("onReady failed: %w", err)
		}
	}

	logger := a.MustGetLogger()
	logger.DebugCtx(a.ctx, "✅ Cron application started", zap.String("state", a.GetState().String()))

	// 6. 如果是阻塞模式，等待关闭信号
	if blocking {
		a.WaitShutdown()
		return a.gracefulShutdown()
	}

	return nil
}

// gracefulShutdown Cron 应用优雅关闭
func (a *CronApplication) gracefulShutdown() error {
	logger := a.MustGetLogger()
	logger.DebugCtx(a.ctx, "Starting Cron application graceful shutdown...")

	// 1. 触发 Cron 专有关闭回调（快速执行：释放锁等）
	if a.cronOnShutdown != nil {
		if err := a.cronOnShutdown(a); err != nil {
			logger.ErrorCtx(a.ctx, "Cron OnShutdown callback failed", zap.Error(err))
		}
	}

	// 2. 关闭调度器（带超时控制）
	if a.scheduler != nil {
		if err := a.shutdownSchedulerWithTimeout(); err != nil {
			if logger != nil {
				logger.ErrorCtx(a.ctx, "Scheduler close exception", zap.Error(err))
			}
		}
	}

	// 3. 调用 Base 的通用关闭逻辑
	return a.BaseApplication.Shutdown(10 * time.Second)
}

// shutdownSchedulerWithTimeout 关闭调度器（带超时控制）
func (a *CronApplication) shutdownSchedulerWithTimeout() error {
	logger := a.MustGetLogger()

	// 默认超时时间 30 秒（可通过配置调整）
	timeout := 30 * time.Second

	// 尝试从配置加载超时时间
	configLoader := a.GetConfigLoader()
	if configLoader != nil {
		var cfg struct {
			Cron struct {
				ShutdownTimeout int `mapstructure:"shutdown_timeout"`
			} `mapstructure:"cron"`
		}
		if err := configLoader.Unmarshal(&cfg); err == nil && cfg.Cron.ShutdownTimeout > 0 {
			timeout = time.Duration(cfg.Cron.ShutdownTimeout) * time.Second
		}
	}

	if logger != nil {
		logger.DebugCtx(a.ctx, "Shutting down scheduler, waiting for tasks to complete...",
			zap.Duration("timeout", timeout))
	}

	// 在 goroutine 中关闭调度器
	done := make(chan error, 1)
	go func() {
		done <- a.scheduler.Shutdown()
	}()

	// 等待完成或超时
	select {
	case err := <-done:
		if err != nil {
			if logger != nil {
				logger.ErrorCtx(a.ctx, "Scheduler close failed", zap.Error(err))
			}
			return err
		}
		if logger != nil {
			logger.DebugCtx(a.ctx, "✅ Scheduler closed, all tasks completed")
		}
		return nil

	case <-time.After(timeout):
		// ⚠️ 超时，强制退出
		if logger != nil {
			logger.WarnCtx(a.ctx, "⚠️  Scheduler close timeout, forcing exit",
				zap.Duration("timeout", timeout))
			logger.WarnCtx(a.ctx, "💡 Suggestion: Increase cron.shutdown_timeout or optimize task execution time")
		}
		return fmt.Errorf("调度器关闭超时（%v）", timeout)
	}
}

// GetScheduler 获取调度器实例
func (a *CronApplication) GetScheduler() gocron.Scheduler {
	return a.scheduler
}

// RegisterTask 注册单个任务（便捷方法）
func (a *CronApplication) RegisterTask(cronExpr string, task interface{}, options ...gocron.JobOption) (gocron.Job, error) {
	return a.scheduler.NewJob(
		gocron.CronJob(cronExpr, false),
		gocron.NewTask(task),
		options...,
	)
}

// RegisterTasks 注册任务注册器
func (a *CronApplication) RegisterTasks(registrar TaskRegistrar) *CronApplication {
	a.taskRegistrar = registrar
	return a
}

// OnSetup 注册 Setup 阶段回调
func (a *CronApplication) OnSetup(fn func(*CronApplication) error) *CronApplication {
	a.cronOnSetup = fn
	// 同时设置 Base 的回调（转换类型）
	a.BaseApplication.OnSetup(func(base *BaseApplication) error {
		return fn(a)
	})
	return a
}

// OnReady 注册启动完成回调
func (a *CronApplication) OnReady(fn func(*CronApplication) error) *CronApplication {
	a.cronOnReady = fn
	// 同时设置 Base 的回调（转换类型）
	a.BaseApplication.OnReady(func(base *BaseApplication) error {
		return fn(a)
	})
	return a
}

// OnShutdown 注册关闭前回调
func (a *CronApplication) OnShutdown(fn func(*CronApplication) error) *CronApplication {
	a.cronOnShutdown = fn
	return a
}

// Shutdown 手动触发关闭
func (a *CronApplication) Shutdown() {
	a.Cancel()
}
