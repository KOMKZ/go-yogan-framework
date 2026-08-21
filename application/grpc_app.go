// Package application provides a generic application startup framework
// GRPCApplication is a dedicated wrapper for gRPC applications (similar to CLIApplication, CronApplication)
package application

import (
	"context"
	"fmt"
	"time"

	"github.com/KOMKZ/go-yogan-framework/grpc"
	"github.com/samber/do/v2"
	"go.uber.org/zap"
)

// GRPCApplication gRPC application (combination of BaseApplication and gRPC specific features)
type GRPCApplication struct {
	*BaseApplication // Combines core framework (80% generic logic)

	// gRPC Server (started automatically when a *grpc.Server provider is
	// registered in DI and grpc.server.enabled is true; nil in manual mode)
	grpcServer *grpc.Server
}

// Create gRPC application instance using NewGRPC
// configPath: Configuration directory path (e.g., ../configs/auth-service)
// configPrefix: Configuration prefix (e.g., "AUTH_SERVICE"); empty disables the env source
// flags: command-line arguments (optional, nil indicates not used)
func NewGRPC(configPath, configPrefix string, flags interface{}) *GRPCApplication {
	if configPath == "" {
		configPath = "../configs"
	}

	baseApp := NewBase(configPath, configPrefix, "grpc", flags)

	return &GRPCApplication{
		BaseApplication: baseApp,
	}
}

// Create gRPC application instance with default configuration
// appName: Application name (e.g., auth-service), used to construct default configuration paths
// and the per-application environment prefix (AUTH_SERVICE)
func NewGRPCWithDefaults(appName string) *GRPCApplication {
	return NewGRPC("../configs/"+appName, EnvPrefixFor(appName), nil)
}

// Create gRPC application instance (supporting command-line arguments)
// configPath: configuration directory path
// configPrefix: environment variable prefix
// flags: command-line arguments (AppFlags struct)
func NewGRPCWithFlags(configPath, configPrefix string, flags interface{}) *GRPCApplication {
	return NewGRPC(configPath, configPrefix, flags)
}

// OnSetup registers the callback for the Setup phase (chained call)
func (g *GRPCApplication) OnSetup(fn func(*GRPCApplication) error) *GRPCApplication {
	g.BaseApplication.OnSetup(func(base *BaseApplication) error {
		return fn(g)
	})
	return g
}

// Register completion callback on ready (chained call)
func (g *GRPCApplication) OnReady(fn func(*GRPCApplication) error) *GRPCApplication {
	g.BaseApplication.OnReady(func(base *BaseApplication) error {
		return fn(g)
	})
	return g
}

// OnShutdown register pre-shutdown callback (chained call)
func (g *GRPCApplication) OnShutdown(fn func(*GRPCApplication) error) *GRPCApplication {
	g.BaseApplication.onShutdown = func(ctx context.Context) error {
		return fn(g)
	}
	return g
}

// Run the gRPC application (block until shutdown signal received)
// 🎯 Same signature and flow as HTTP/Cron: RunNonBlocking -> WaitShutdown -> gracefulShutdown
func (g *GRPCApplication) Run() error {
	// Execute non-blocking startup
	if err := g.RunNonBlocking(); err != nil {
		return err
	}

	// wait for shutdown signal (blocking)
	g.WaitShutdown()

	// graceful shutdown
	return g.gracefulShutdown()
}

// RunNonBlocking starts the gRPC application in a non-blocking manner
// (for testing or scenarios where manual lifecycle control is needed)
func (g *GRPCApplication) RunNonBlocking() error {
	// 1. Setup phase (initialize all components)
	if err := g.Setup(); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}

	// 2. Start gRPC server (if a *grpc.Server is provided via DI and enabled)
	if err := g.startGRPCServer(); err != nil {
		return err
	}

	// 3. Trigger OnReady (application custom initialization)
	g.BaseApplication.setState(StateRunning)
	if g.BaseApplication.onReady != nil {
		if err := g.BaseApplication.onReady(g.BaseApplication); err != nil {
			return fmt.Errorf("onReady failed: %w", err)
		}
	}

	logger := g.MustGetLogger()
	logger.InfoCtx(g.ctx, "✅ gRPC application started", zap.Int64("startup_time", g.GetStartupTimeMs()))

	return nil
}

// startGRPCServer obtains the *grpc.Server from DI and starts it (optional)
// 🎯 Lazy resolution: when no *grpc.Server provider is registered (manual mode)
// or grpc.server.enabled is false, nothing is started here.
func (g *GRPCApplication) startGRPCServer() error {
	server, err := do.Invoke[*grpc.Server](g.GetInjector())
	if err != nil {
		return nil // No provider registered: manual mode
	}
	if server == nil {
		return nil // gRPC server not enabled in config
	}

	if err := server.Start(g.ctx); err != nil {
		return fmt.Errorf("failed to start gRPC server: %w", err)
	}

	g.grpcServer = server
	return nil
}

// graceful shutdown for gRPC application
func (g *GRPCApplication) gracefulShutdown() error {
	logger := g.MustGetLogger()
	logger.DebugCtx(g.ctx, "Starting gRPC application graceful shutdown...")

	// 1. Stop the gRPC server (stop accepting new requests)
	if g.grpcServer != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		g.grpcServer.Stop(shutdownCtx)
	}

	// 2. Call Base's generic shutdown logic (trigger OnShutdown callback + shut down components)
	return g.BaseApplication.Shutdown(30 * time.Second)
}

// GetGRPCServer get gRPC server instance (for testing purposes)
func (g *GRPCApplication) GetGRPCServer() *grpc.Server {
	return g.grpcServer
}

// Shutdown manually triggers graceful shutdown (for testing or program control)
// Cancels the context (unblocking a blocking Run) and performs full cleanup.
func (g *GRPCApplication) Shutdown() error {
	g.Cancel()
	return g.gracefulShutdown()
}
