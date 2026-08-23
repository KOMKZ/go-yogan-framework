package application

import (
	"context"
	"fmt"
	"time"

	"github.com/KOMKZ/go-yogan-framework/queue"
	"github.com/samber/do/v2"
	"go.uber.org/zap"
)

type WorkerApplication struct {
	*BaseApplication

	profile string
	server  *queue.Server
}

func NewWorker(configPath, configPrefix, profile string, flags interface{}) *WorkerApplication {
	if configPath == "" {
		configPath = "../configs"
	}
	if profile == "" {
		profile = queue.DefaultProfile
	}

	return &WorkerApplication{
		BaseApplication: NewBase(configPath, configPrefix, "worker", flags),
		profile:         profile,
	}
}

func NewWorkerWithDefaults(appName, profile string) *WorkerApplication {
	return NewWorker("../configs/"+appName, EnvPrefixFor(appName), profile, nil)
}

func (a *WorkerApplication) WithVersion(version string) *WorkerApplication {
	a.BaseApplication.WithVersion(version)
	return a
}

func (a *WorkerApplication) Run() error {
	if err := a.RunNonBlocking(); err != nil {
		return err
	}
	a.WaitShutdown()
	return a.gracefulShutdown()
}

func (a *WorkerApplication) RunNonBlocking() error {
	if err := a.Setup(); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}

	if err := a.startQueueServer(); err != nil {
		return err
	}

	a.BaseApplication.setState(StateRunning)
	if a.BaseApplication.onReady != nil {
		if err := a.BaseApplication.onReady(a.BaseApplication); err != nil {
			return fmt.Errorf("onReady failed: %w", err)
		}
	}

	logger := a.MustGetLogger()
	fields := []zap.Field{
		zap.String("state", a.GetState().String()),
		zap.String("profile", a.profile),
		zap.Int64("startup_time", a.GetStartupTimeMs()),
	}
	if version := a.GetVersion(); version != "" {
		fields = append(fields, zap.String("version", version))
	}
	logger.InfoCtx(a.ctx, "worker application started", fields...)
	return nil
}

func (a *WorkerApplication) Shutdown() error {
	a.Cancel()
	return a.gracefulShutdown()
}

func (a *WorkerApplication) OnSetup(fn func(*WorkerApplication) error) *WorkerApplication {
	a.BaseApplication.OnSetup(func(base *BaseApplication) error {
		return fn(a)
	})
	return a
}

func (a *WorkerApplication) OnReady(fn func(*WorkerApplication) error) *WorkerApplication {
	a.BaseApplication.OnReady(func(base *BaseApplication) error {
		return fn(a)
	})
	return a
}

func (a *WorkerApplication) OnShutdown(fn func(*WorkerApplication) error) *WorkerApplication {
	a.BaseApplication.OnShutdown(func(ctx context.Context) error {
		return fn(a)
	})
	return a
}

func (a *WorkerApplication) QueueRegistry() (*queue.Registry, error) {
	return do.Invoke[*queue.Registry](a.GetInjector())
}

func (a *WorkerApplication) QueueClient() (queue.Client, error) {
	return do.Invoke[queue.Client](a.GetInjector())
}

func (a *WorkerApplication) startQueueServer() error {
	server, err := do.Invoke[*queue.Server](a.GetInjector())
	if err != nil {
		return fmt.Errorf("queue server provider failed: %w", err)
	}
	if server == nil {
		return fmt.Errorf("queue server is not configured")
	}
	if err := server.Start(a.ctx, a.profile); err != nil {
		return fmt.Errorf("start queue server: %w", err)
	}
	a.server = server
	return nil
}

func (a *WorkerApplication) gracefulShutdown() error {
	logger := a.MustGetLogger()
	logger.DebugCtx(a.ctx, "starting worker application graceful shutdown")
	return a.BaseApplication.Shutdown(10 * time.Second)
}
