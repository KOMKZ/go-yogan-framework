// Package application provides a generic application startup framework
// GRPCApplication is a dedicated wrapper for gRPC applications (similar to CLIApplication, CronApplication)
package application

import (
	"context"
	"time"

	"github.com/KOMKZ/go-yogan-framework/governance"
	"go.uber.org/zap"
)

// GRPCApplication gRPC application (combination of BaseApplication and gRPC specific features)
type GRPCApplication struct {
	*BaseApplication // Combines core framework (80% generic logic)

	// 🎯 Service Governance Manager (optional, automatically registers/unregisters services if enabled)
	governanceManager *governance.Manager
}

// Create gRPC application instance using NewGRPC
// configPath: Configuration directory path (e.g., ../configs/auth-service)
// configPrefix: Configuration prefix (e.g., "APP")
// flags: command-line arguments (optional, nil indicates not used)
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

// Create gRPC application instance with default configuration
// appName: Application name (e.g., auth-service), used to construct default configuration paths
func NewGRPCWithDefaults(appName string) *GRPCApplication {
	return NewGRPC("../configs/"+appName, "APP", nil)
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
func (g *GRPCApplication) Run() {
	logger := g.MustGetLogger()

	// 1. Setup phase (initialize all components)
	if err := g.Setup(); err != nil {
		logger.ErrorCtx(g.ctx, "Application start failed", zap.Error(err))
		panic(err)
	}

	// 2. 🎯 Automatically register services to the governance center (if enabled)
	if g.governanceManager != nil {
		if err := g.autoRegisterService(); err != nil {
			logger.WarnCtx(g.ctx, "⚠️  Service registration failed (does not affect app startup)", zap.Error(err))

		}
	}

	// 3. Trigger OnReady (application custom initialization)
	g.BaseApplication.setState(StateRunning)
	if g.BaseApplication.onReady != nil {
		if err := g.BaseApplication.onReady(g.BaseApplication); err != nil {
			logger.ErrorCtx(g.ctx, "OnReady OnReady failed", zap.Error(err))
			panic(err)
		}
	}

	logger.InfoCtx(g.ctx, "✅ gRPC application started", zap.Int64("startup_time", g.GetStartupTimeMs()))

	// wait for shutdown signal (blocking)
	g.WaitShutdown()

	// 5. 🎯 Automatically log out service (if enabled)
	if g.governanceManager != nil {
		if err := g.autoDeregisterService(); err != nil {
			logger.ErrorCtx(g.ctx, "Service deregistration failed", zap.Error(err))
		}
	}

	// Elegant shutdown
	if err := g.gracefulShutdown(); err != nil {
		logger.ErrorCtx(g.ctx, "Application close failed", zap.Error(err))
	}
}

// graceful shutdown for gRPC application
func (g *GRPCApplication) gracefulShutdown() error {
	logger := g.MustGetLogger()
	logger.DebugCtx(g.ctx, "Starting gRPC application graceful shutdown...")

	// Call Base's generic shutdown logic (30-second timeout)
	return g.BaseApplication.Shutdown(30 * time.Second)
}

// SetGovernanceManager set service governance manager (optional, for automatic service registration/unregistration)
func (g *GRPCApplication) SetGovernanceManager(manager *governance.Manager) *GRPCApplication {
	g.governanceManager = manager
	return g
}

// autoRegisterService Automatically register service (retrieve port information from gRPC components)
func (g *GRPCApplication) autoRegisterService() error {
	// TODO: Retrieve actual listening port from gRPC component and register service
	logger := g.MustGetLogger()
	logger.DebugCtx(g.ctx, "🎯 Service registration enabled (implementing...)")

	return nil
}

// autoDeregisterService Automatically deregister service
func (g *GRPCApplication) autoDeregisterService() error {
	if g.governanceManager == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return g.governanceManager.Shutdown(ctx)
}
