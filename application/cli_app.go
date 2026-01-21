// Provides a generic application startup framework
// CLIApplication is for CLI applications specifically (composes BaseApplication)
package application

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// CLIApplication CLI application (combination of BaseApplication + CLI specific features)
type CLIApplication struct {
	*BaseApplication // Combines core framework (80% general logic)

	// CLI specific fields
	rootCmd *cobra.Command
}

// Create CLI application instance
// configPath: Configuration directory path (e.g., ../configs/cli-app)
// configPrefix: Configuration prefix (e.g., "APP")
// rootCmd: Cobra root command
func NewCLI(configPath, configPrefix string, rootCmd *cobra.Command) *CLIApplication {
	// Default value handling
	if configPath == "" {
		configPath = "../configs" // Not recommended to use, but defensive default
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

// Create CLI application instance with default configuration
// appName: Application name (e.g., cli-app), used for building default configuration paths
func NewCLIWithDefaults(appName string, rootCmd *cobra.Command) *CLIApplication {
	return NewCLI("../configs/"+appName, "APP", rootCmd)
}

// OnSetup registers the Setup stage callback (chained call)
func (c *CLIApplication) OnSetup(fn func(*CLIApplication) error) *CLIApplication {
	// Convert to BaseApplication callback
	c.BaseApplication.OnSetup(func(base *BaseApplication) error {
		return fn(c)
	})
	return c
}

// Register completion callback on ready (chained call)
func (c *CLIApplication) OnReady(fn func(*CLIApplication) error) *CLIApplication {
	// Convert to BaseApplication callback
	c.BaseApplication.OnReady(func(base *BaseApplication) error {
		return fn(c)
	})
	return c
}

// OnShutdown register shutdown callback (chained call)
func (c *CLIApplication) OnShutdown(fn func(*CLIApplication) error) *CLIApplication {
	// Convert to BaseApplication callback
	c.BaseApplication.onShutdown = func(ctx context.Context) error {
		return fn(c)
	}
	return c
}

// Execute CLI command (synchronous execution, exit after completion)
func (c *CLIApplication) Execute() error {
	// 1. Setup stage (initialize all components)
	if err := c.Setup(); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}

	// 2. Trigger OnReady (custom initialization for CLI application)
	c.BaseApplication.setState(StateRunning)
	if c.BaseApplication.onReady != nil {
		if err := c.BaseApplication.onReady(c.BaseApplication); err != nil {
			return fmt.Errorf("onReady failed: %w", err)
		}
	}

	logger := c.MustGetLogger()
	logger.DebugCtx(c.ctx, "✅ CLI application initialized", zap.Int64("startup_time", c.GetStartupTimeMs()))

	// Execute Cobra command (sync)
	err := c.rootCmd.Execute()

	// 4. Graceful shutdown (clean up resources whether successful or failed)
	shutdownErr := c.gracefulShutdown()

	if err != nil {
		return err
	}
	return shutdownErr
}

// graceful shutdown for CLI application
func (c *CLIApplication) gracefulShutdown() error {
	logger := c.MustGetLogger()
	logger.DebugCtx(c.ctx, "Starting CLI application graceful shutdown...")

	// Call the generic close logic in Base (5-second timeout, CLI applications typically finish quickly)
	return c.BaseApplication.Shutdown(5 * time.Second)
}

// GetRootCmd obtains the root command (for testing)
func (c *CLIApplication) GetRootCmd() *cobra.Command {
	return c.rootCmd
}

// AddCommand Adds subcommands (convenience method)
func (c *CLIApplication) AddCommand(cmds ...*cobra.Command) *CLIApplication {
	c.rootCmd.AddCommand(cmds...)
	return c
}
