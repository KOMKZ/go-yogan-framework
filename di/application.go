// Package di provides dependency injection support based on samber/do
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

// Application state
type AppState int

const (
	StateInit AppState = iota
	StateSetup
	StateRunning
	StateStopping
	StateStopped
)

// String represents textual status
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

// DoApplication based on samber/do framework
// Replace the original BaseApplication, use samber/do for managing component lifecycles
type DoApplication struct {
	// ═══════════════════════════════════════════════════════════
	// Core: sabmer/do injector
	// ═══════════════════════════════════════════════════════════
	injector *do.RootScope

	// configuration management
	configPath   string
	configPrefix string
	configLoader *config.Loader

	// Log
	logger *logger.CtxZapLogger

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	state  AppState
	mu     sync.RWMutex

	// Apply metadata
	name    string
	version string

	// callback function
	onSetup        func(*DoApplication) error
	onReady        func(*DoApplication) error
	onConfigReload func(*config.Loader)
	onShutdown     func(context.Context) error
}

// DoAppOption application option function
type DoAppOption func(*DoApplication)

// WithConfigPath set configuration path
func WithConfigPath(path string) DoAppOption {
	return func(app *DoApplication) {
		app.configPath = path
	}
}

// WithConfigPrefix set configuration prefix
func WithConfigPrefix(prefix string) DoAppOption {
	return func(app *DoApplication) {
		app.configPrefix = prefix
	}
}

// Set application name
func WithName(name string) DoAppOption {
	return func(app *DoApplication) {
		app.name = name
	}
}

// WithVersion sets the application version
func WithVersion(version string) DoAppOption {
	return func(app *DoApplication) {
		app.version = version
	}
}

// WithOnSetup sets up the Setup callback
func WithOnSetup(fn func(*DoApplication) error) DoAppOption {
	return func(app *DoApplication) {
		app.onSetup = fn
	}
}

// Set Ready callback
func WithOnReady(fn func(*DoApplication) error) DoAppOption {
	return func(app *DoApplication) {
		app.onReady = fn
	}
}

// WithOnShutdown sets the Shutdown callback
func WithOnShutdown(fn func(context.Context) error) DoAppOption {
	return func(app *DoApplication) {
		app.onShutdown = fn
	}
}

// Create an application instance based on samber/do
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

	// Apply options
	for _, opt := range opts {
		opt(app)
	}

	return app
}

// Injector retrieves do.Injector
func (app *DoApplication) Injector() *do.RootScope {
	return app.injector
}

// Get log instance
func (app *DoApplication) Logger() *logger.CtxZapLogger {
	return app.logger
}

// ConfigLoader obtain configuration loader
func (app *DoApplication) ConfigLoader() *config.Loader {
	return app.configLoader
}

// Get current state
func (app *DoApplication) State() AppState {
	app.mu.RLock()
	defer app.mu.RUnlock()
	return app.state
}

// set state to update state
func (app *DoApplication) setState(state AppState) {
	app.mu.Lock()
	defer app.mu.Unlock()
	app.state = state
}

// Setup initialization phase
// 1. Load configuration
// Initialize log
// 3. Register core Provider
func (app *DoApplication) Setup() error {
	app.setState(StateSetup)

	// Initialize configuration
	opts := ConfigOptions{
		ConfigPath:   app.configPath,
		ConfigPrefix: app.configPrefix,
		AppType:      "http",
	}
	do.Provide(app.injector, ProvideConfigLoader(opts))

	loader, err := do.Invoke[*config.Loader](app.injector)
	if err != nil {
		return fmt.Errorf("Initialization configuration failed: %w: %w", err)
	}
	app.configLoader = loader

	// Initialize log
	do.Provide(app.injector, ProvideLoggerManager)
	do.Provide(app.injector, ProvideCtxLogger(app.name))

	appLogger, err := do.Invoke[*logger.CtxZapLogger](app.injector)
	if err != nil {
		return fmt.Errorf("Log initialization failed: %w: %w", err)
	}
	app.logger = appLogger

	app.logger.Info("🔧 🔧 Application initialization in progress......",
		zap.String("name", app.name),
		zap.String("version", app.version),
		zap.String("config_path", app.configPath),
	)

	// Call Setup callback
	if app.onSetup != nil {
		if err := app.onSetup(app); err != nil {
			return fmt.Errorf("setup setup callback failed: %w: %w", err)
		}
	}

	return nil
}

// Start Application Initialization
func (app *DoApplication) Start() error {
	app.setState(StateRunning)

	app.logger.Info("✅ English: Application startup completed successfully",
		zap.String("name", app.name),
		zap.String("version", app.version),
		zap.String("state", app.State().String()),
	)

	// Call Ready callback
	if app.onReady != nil {
		if err := app.onReady(app); err != nil {
			return fmt.Errorf("ready Ready callback failed: %w: %w", err)
		}
	}

	return nil
}

// Run the application (block and wait for signals)
func (app *DoApplication) Run() error {
	// Setup
	if err := app.Setup(); err != nil {
		return err
	}

	// Start
	if err := app.Start(); err != nil {
		return err
	}

	// waiting for exit signal
	app.waitForSignal()

	return nil
}

// wait for exit signal
func (app *DoApplication) waitForSignal() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	sig := <-quit
	app.logger.Info("📥 English: Received exit signal", zap.String("signal", sig.String()))

	// graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := app.Shutdown(ctx); err != nil {
		app.logger.Error("English: Close failed", zap.Error(err))
	}
}

// Shut Down Gracefully
// samber/do will automatically shut down in reverse order based on dependencies
func (app *DoApplication) Shutdown(ctx context.Context) error {
	app.setState(StateStopping)
	app.logger.Info("🔄 🔄 Starting graceful shutdown...... 🔁: 🔁 Starting graceful shutdown......")

	// Call the user-defined close callback
	if app.onShutdown != nil {
		if err := app.onShutdown(ctx); err != nil {
			app.logger.Warn("shutdown shutdown callback failed", zap.Error(err))
		}
	}

	// 2. Cancel context
	app.cancel()

	// 3. Close the samber/do container (automatically shut down in dependency order)
	if err := app.injector.Shutdown(); err != nil {
		app.logger.Warn("injector shutdown English: injector shutdown failed", zap.Error(err))
	}

	app.setState(StateStopped)
	app.logger.Info("✅ English: The application has been closed")

	return nil
}

// HealthCheck Health check
func (app *DoApplication) HealthCheck() map[string]error {
	return app.injector.HealthCheck()
}

// IsHealthy whether healthy
func (app *DoApplication) IsHealthy() bool {
	checks := app.HealthCheck()
	for _, err := range checks {
		if err != nil {
			return false
		}
	}
	return true
}
