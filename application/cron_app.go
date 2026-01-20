package application

import (
	"fmt"
	"time"

	"github.com/go-co-op/gocron/v2"
	"go.uber.org/zap"
)

// Cron Application (combines BaseApplication with cron-specific features)
type CronApplication struct {
	*BaseApplication // Combine core framework

	// Cron dedicated
	scheduler      gocron.Scheduler
	cronOnSetup    func(*CronApplication) error
	cronOnReady    func(*CronApplication) error
	cronOnShutdown func(*CronApplication) error
	taskRegistrar  TaskRegistrar // Task registrar
}

// TaskRegistrar task registration interface
type TaskRegistrar interface {
	RegisterTasks(app *CronApplication) error
}

// Create Cron application instance
// configPath: Configuration directory path (e.g., ../configs/cron-app)
// configPrefix: Configuration prefix (e.g., "APP")
func NewCron(configPath, configPrefix string) (*CronApplication, error) {
	if configPath == "" {
		configPath = "../configs/cron-app"
	}
	if configPrefix == "" {
		configPrefix = "APP"
	}

	baseApp := NewBase(configPath, configPrefix, "cron", nil)

	// Create gocron scheduler
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		return nil, fmt.Errorf("Failed to create scheduler: %w: %w", err)
	}

	return &CronApplication{
		BaseApplication: baseApp,
		scheduler:       scheduler,
	}, nil
}

// Create Cron application instance with default configuration
func NewCronWithDefaults(appName string) (*CronApplication, error) {
	return NewCron("../configs/"+appName, "APP")
}

// Run the Cron application (block until shutdown signal received)
func (a *CronApplication) Run() error {
	return a.run(true)
}

// RunNonBlocking Start application non-blockingly (for testing environment)
func (a *CronApplication) RunNonBlocking() error {
	return a.run(false)
}

// run internal startup logic (uniform implementation)
func (a *CronApplication) run(blocking bool) error {
	// 1. Setup phase (configuration + logging + component initialization)
	if err := a.Setup(); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}

	// Trigger Cron dedicated setup callback
	if a.cronOnSetup != nil {
		if err := a.cronOnSetup(a); err != nil {
			return fmt.Errorf("cron onSetup failed: %w", err)
		}
	}

	// 3. Register task
	if a.taskRegistrar != nil {
		if err := a.taskRegistrar.RegisterTasks(a); err != nil {
			return fmt.Errorf("register tasks failed: %w", err)
		}
	}

	// 4. Start the scheduler
	a.scheduler.Start()

	// 5. Trigger OnReady callback
	a.BaseApplication.setState(StateRunning)
	if a.cronOnReady != nil {
		if err := a.cronOnReady(a); err != nil {
			return fmt.Errorf("onReady failed: %w", err)
		}
	}

	logger := a.MustGetLogger()
	logger.DebugCtx(a.ctx, "✅ Cron application started", zap.String("state", a.GetState().String()), zap.Duration("startup_time", a.GetStartDuration()))

	// If in blocking mode, wait for shutdown signal
	if blocking {
		a.WaitShutdown()
		return a.gracefulShutdown()
	}

	return nil
}

// graceful shutdown for Cron application
func (a *CronApplication) gracefulShutdown() error {
	logger := a.MustGetLogger()
	logger.DebugCtx(a.ctx, "Starting Cron application graceful shutdown...")

	// Trigger Cron dedicated shutdown callback (quick execution: release locks, etc.)
	if a.cronOnShutdown != nil {
		if err := a.cronOnShutdown(a); err != nil {
			logger.ErrorCtx(a.ctx, "Cron OnShutdown callback failed", zap.Error(err))
		}
	}

	// 2. Shutdown scheduler (with timeout control)
	if a.scheduler != nil {
		if err := a.shutdownSchedulerWithTimeout(); err != nil {
			if logger != nil {
				logger.ErrorCtx(a.ctx, "Scheduler close exception", zap.Error(err))
			}
		}
	}

	// Call Base's generic shutdown logic
	return a.BaseApplication.Shutdown(10 * time.Second)
}

// shutdownSchedulerWithTimeout Shutdown scheduler (with timeout control)
func (a *CronApplication) shutdownSchedulerWithTimeout() error {
	logger := a.MustGetLogger()

	// Default timeout of 30 seconds (can be adjusted via configuration)
	timeout := 30 * time.Second

	// Try to load timeout from configuration
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

	// Close the scheduler in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- a.scheduler.Shutdown()
	}()

	// wait for completion or timeout
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
		// ⚠️ Timeout, force exit
		if logger != nil {
			logger.WarnCtx(a.ctx, "⚠️  Scheduler close timeout, forcing exit",
				zap.Duration("timeout", timeout))
			logger.WarnCtx(a.ctx, "💡 Suggestion: Increase cron.shutdown_timeout or optimize task execution time")
		}
		return fmt.Errorf("Scheduling shutdown timeout (%v)（%v）", timeout)
	}
}

// Get scheduler instance
func (a *CronApplication) GetScheduler() gocron.Scheduler {
	return a.scheduler
}

// RegisterTask registers a single task (convenience method)
func (a *CronApplication) RegisterTask(cronExpr string, task interface{}, options ...gocron.JobOption) (gocron.Job, error) {
	return a.scheduler.NewJob(
		gocron.CronJob(cronExpr, false),
		gocron.NewTask(task),
		options...,
	)
}

// RegisterTasks registers the task registrar
func (a *CronApplication) RegisterTasks(registrar TaskRegistrar) *CronApplication {
	a.taskRegistrar = registrar
	return a
}

// OnSetup registers the callback for the Setup phase
func (a *CronApplication) OnSetup(fn func(*CronApplication) error) *CronApplication {
	a.cronOnSetup = fn
	// Set the callback for Base (conversion type) simultaneously
	a.BaseApplication.OnSetup(func(base *BaseApplication) error {
		return fn(a)
	})
	return a
}

// Register startup completion callback
func (a *CronApplication) OnReady(fn func(*CronApplication) error) *CronApplication {
	a.cronOnReady = fn
	// Set the callback for Base (conversion type) simultaneously
	a.BaseApplication.OnReady(func(base *BaseApplication) error {
		return fn(a)
	})
	return a
}

// Register shutdown callback
func (a *CronApplication) OnShutdown(fn func(*CronApplication) error) *CronApplication {
	a.cronOnShutdown = fn
	return a
}

// Shutdown manually triggered
func (a *CronApplication) Shutdown() {
	a.Cancel()
}
