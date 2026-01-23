// Package application provides a generic application startup framework
// Application is for HTTP application use only (extends BaseApplication)
package application

import (
	"context"
	"fmt"
	"time"

	"github.com/KOMKZ/go-yogan-framework/health"
	"github.com/KOMKZ/go-yogan-framework/limiter"
	"github.com/KOMKZ/go-yogan-framework/swagger"
	"github.com/KOMKZ/go-yogan-framework/telemetry"
	"github.com/samber/do/v2"
	"go.uber.org/zap"
)

// Application HTTP (combining BaseApplication with dedicated HTTP features)
type Application struct {
	*BaseApplication // Combines core framework (80% generic logic)

	// HTTP Server (HTTP proprietary)
	httpServer      *HTTPServer
	routerRegistrar RouterRegistrar
	routerManager   *Manager // Router manager (kernel component)
}

// Create a new HTTP application instance
// configPath: Configuration directory path (e.g., ../configs/user-api)
// configPrefix: Configuration prefix (e.g., "APP")
// flags: command-line arguments (optional, nil indicates not used)
func New(configPath, configPrefix string, flags interface{}) *Application {
	// default value handling
	if configPath == "" {
		configPath = "../configs" // Not recommended to use, but defensive default setting
	}
	if configPrefix == "" {
		configPrefix = "APP"
	}

	baseApp := NewBase(configPath, configPrefix, "http", flags)

	return &Application{
		BaseApplication: baseApp,
		routerManager:   NewManager(), // Initialize route manager
	}
}

// Create an HTTP application instance with default configuration
// appName: application name (e.g., user-api), used to construct default configuration paths
func NewWithDefaults(appName string) *Application {
	return New("../configs/"+appName, "APP", nil)
}

// NewWithFlags creates an HTTP application instance (supports command-line arguments)
// configPath: configuration directory path
// configPrefix: environment variable prefix
// flags: command-line arguments (AppFlags struct)
func NewWithFlags(configPath, configPrefix string, flags interface{}) *Application {
	return New(configPath, configPrefix, flags)
}

// WithVersion sets the application version number (chaining call)
func (a *Application) WithVersion(version string) *Application {
	a.BaseApplication.WithVersion(version)
	return a
}

// Run HTTP application (block until shutdown signal received)
func (a *Application) Run() error {
	// Execute non-blocking startup
	if err := a.RunNonBlocking(); err != nil {
		return err
	}

	// waiting for shutdown signal
	a.WaitShutdown()

	// graceful shutdown
	return a.gracefulShutdown()
}

// RunNonBlocking starts the HTTP application in a non-blocking manner (for testing or scenarios where manual lifecycle control is needed)
// Execute all initialization and startup logic but do not wait for shutdown signals
func (a *Application) RunNonBlocking() error {
	// 1. Setup stage (initialize components, trigger OnSetup callback)
	if err := a.Setup(); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}

	// 2. Start HTTP Server (if routes are registered)
	if err := a.startHTTPServer(); err != nil {
		return err
	}

	// 3. Trigger the OnReady callback (using the unified callback of BaseApplication)
	a.BaseApplication.setState(StateRunning)
	if a.BaseApplication.onReady != nil {
		if err := a.BaseApplication.onReady(a.BaseApplication); err != nil {
			return fmt.Errorf("onReady failed: %w", err)
		}
	}

	logger := a.MustGetLogger()
	fields := []zap.Field{
		zap.String("state", a.GetState().String()),
		zap.Int64("startup_time", a.GetStartupTimeMs()),
	}
	if version := a.GetVersion(); version != "" {
		fields = append(fields, zap.String("version", version))
	}
	logger.InfoCtx(a.ctx, "✅ HTTP application started", fields...)

	return nil
}

// startHTTPServer Start HTTP Server (HTTP proprietary logic)
func (a *Application) startHTTPServer() error {
	if a.routerRegistrar == nil {
		return nil
	}

	// 🎯 Obtain Telemetry Manager via DI (optional)
	var telemetryMgr *telemetry.Manager
	if mgr, err := do.Invoke[*telemetry.Manager](a.GetInjector()); err == nil && mgr != nil && mgr.IsEnabled() {
		telemetryMgr = mgr
	}

	// 🎯 Obtain Limiter Manager via DI (optional)
	var limiterMgr *limiter.Manager
	if mgr, err := do.Invoke[*limiter.Manager](a.GetInjector()); err == nil && mgr != nil {
		limiterMgr = mgr
	}

	// 🎯 Obtain Health Aggregator via DI (optional)
	var healthAgg *health.Aggregator
	if agg, err := do.Invoke[*health.Aggregator](a.GetInjector()); err == nil && agg != nil {
		healthAgg = agg
	}

	// Create HTTP Server (pass middleware configuration, httpx configuration, rate limiter, telemetry, and health)
	a.httpServer = NewHTTPServerWithTelemetryAndHealth(
		a.appConfig.ApiServer,
		a.appConfig.Middleware,
		a.appConfig.Httpx,
		limiterMgr,
		telemetryMgr,
		healthAgg,
	)

	// Register route for business application (passing Application dependencies container)
	a.routerRegistrar.RegisterRoutes(a.httpServer.GetEngine(), a)

	logger := a.MustGetLogger()
	logger.DebugCtx(a.ctx, "✅ Routes registered")

	// 🎯 Automatically mount Swagger routes (if enabled)
	if err := swagger.Setup(a.GetInjector(), a.httpServer.GetEngine()); err != nil {
		logger.WarnCtx(a.ctx, "Swagger setup failed", zap.Error(err))
	}

	// Start HTTP Server (non-blocking)
	if err := a.httpServer.Start(); err != nil {
		return fmt.Errorf("Failed to start HTTP Server: %w", err)
	}

	return nil
}

// graceful shutdown for HTTP application
func (a *Application) gracefulShutdown() error {
	logger := a.MustGetLogger()
	logger.DebugCtx(a.ctx, "Starting HTTP application graceful shutdown...")

	// 1. Shut down the HTTP Server (stop accepting new requests)
	if a.httpServer != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := a.httpServer.Shutdown(shutdownCtx); err != nil {
			logger.ErrorCtx(a.ctx, "HTTP server close failed", zap.Error(err))
		}
	}

	// Call Base's generic shutdown logic (trigger OnShutdown callback + shut down components)
	return a.BaseApplication.Shutdown(10 * time.Second)
}

// GetHTTPServer Get HTTP server instance (for testing purposes)
func (a *Application) GetHTTPServer() *HTTPServer {
	return a.httpServer
}

// GetRouterManager Get router manager (kernel component)
func (a *Application) GetRouterManager() *Manager {
	return a.routerManager
}

// Shutdown manually triggered (for testing or program control)
func (a *Application) Shutdown() {
	a.Cancel()
}

// OnSetup registers the callback for the Setup stage (chained call)
func (a *Application) OnSetup(fn func(*Application) error) *Application {
	a.BaseApplication.OnSetup(func(base *BaseApplication) error {
		return fn(a)
	})
	return a
}

// Register start completion callback (chained call)
func (a *Application) OnReady(fn func(*Application) error) *Application {
	a.BaseApplication.OnReady(func(base *BaseApplication) error {
		return fn(a)
	})
	return a
}

// OnShutdown register pre-shutdown callback (chained call)
func (a *Application) OnShutdown(fn func(*Application) error) *Application {
	a.BaseApplication.OnShutdown(func(ctx context.Context) error {
		return fn(a)
	})
	return a
}

// RegisterRoutes Register HTTP routes
func (a *Application) RegisterRoutes(registrar RouterRegistrar) *Application {
	a.routerRegistrar = registrar
	return a
}
