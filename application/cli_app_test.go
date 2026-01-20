package application

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewCLI test creating CLI application
func TestNewCLI(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)
	require.NoError(t, err)

	rootCmd := &cobra.Command{
		Use:   "test",
		Short: "Test CLI",
	}

	app := NewCLI(tmpDir, "TEST", rootCmd)

	assert.NotNil(t, app)
	assert.NotNil(t, app.BaseApplication)
	assert.Equal(t, rootCmd, app.GetRootCmd())
}

// TestNewCLIWithDefaults test creating CLI app with default configuration
func TestNewCLIWithDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "configs", "cli-app")
	err := os.MkdirAll(appDir, 0755)
	require.NoError(t, err)

	configFile := filepath.Join(appDir, "config.yaml")
	err = os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)
	require.NoError(t, err)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	rootCmd := &cobra.Command{Use: "test"}
	app := NewCLIWithDefaults("cli-app", rootCmd)
	assert.NotNil(t, app)
}

// TestCLIApplication_Callbacks test callback registration
func TestCLIApplication_Callbacks(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	rootCmd := &cobra.Command{Use: "test"}
	app := NewCLI(tmpDir, "TEST", rootCmd)

	var setupCalled, readyCalled, shutdownCalled bool

	result := app.
		OnSetup(func(c *CLIApplication) error {
			setupCalled = true
			return nil
		}).
		OnReady(func(c *CLIApplication) error {
			readyCalled = true
			return nil
		}).
		OnShutdown(func(c *CLIApplication) error {
			shutdownCalled = true
			return nil
		})

	assert.Equal(t, app, result)
	assert.NotNil(t, app.BaseApplication.onSetup)
	assert.NotNil(t, app.BaseApplication.onReady)
	assert.NotNil(t, app.BaseApplication.onShutdown)

	_ = setupCalled
	_ = readyCalled
	_ = shutdownCalled
}

// TestCLIApplication_AddCommand_TestAddingSubcommand
func TestCLIApplication_AddCommand(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	rootCmd := &cobra.Command{Use: "test"}
	app := NewCLI(tmpDir, "TEST", rootCmd)

	subCmd := &cobra.Command{
		Use:   "sub",
		Short: "Sub command",
	}

	result := app.AddCommand(subCmd)
	assert.Equal(t, app, result)
	assert.Len(t, rootCmd.Commands(), 1)
}

// TestCLIApplication_Execute test executing CLI command
func TestCLIApplication_Execute(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	executed := false
	rootCmd := &cobra.Command{
		Use: "test",
		Run: func(cmd *cobra.Command, args []string) {
			executed = true
		},
	}

	app := NewCLI(tmpDir, "TEST", rootCmd)

	err := app.Execute()
	assert.NoError(t, err)
	assert.True(t, executed)
	assert.Equal(t, StateStopped, app.GetState())
}

// TestNewCLI_DefaultValues test default value handling
func TestNewCLI_DefaultValues(t *testing.T) {
	rootCmd := &cobra.Command{Use: "test"}
	app := NewCLI("", "", rootCmd)
	assert.NotNil(t, app)
}

// TestCLIApplication_Execute_WithCallbacks tests that callbacks are called when executing a CLI command
func TestCLIApplication_Execute_WithCallbacks(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	var setupCalled, readyCalled, shutdownCalled bool

	rootCmd := &cobra.Command{
		Use: "test",
		Run: func(cmd *cobra.Command, args []string) {},
	}

	app := NewCLI(tmpDir, "TEST", rootCmd)
	app.OnSetup(func(c *CLIApplication) error {
		setupCalled = true
		return nil
	})
	app.OnReady(func(c *CLIApplication) error {
		readyCalled = true
		return nil
	})
	app.OnShutdown(func(c *CLIApplication) error {
		shutdownCalled = true
		return nil
	})

	err := app.Execute()
	assert.NoError(t, err)
	assert.True(t, setupCalled)
	assert.True(t, readyCalled)
	assert.True(t, shutdownCalled)
}

// TestCLIApplication_Execute_SetupError Setup failure test
func TestCLIApplication_Execute_SetupError(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	rootCmd := &cobra.Command{Use: "test"}
	app := NewCLI(tmpDir, "TEST", rootCmd)
	app.OnSetup(func(c *CLIApplication) error {
		return assert.AnError
	})

	err := app.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "setup failed")
}

// TestCLIApplication_Execute_ReadyError Test Ready failure
func TestCLIApplication_Execute_ReadyError(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	rootCmd := &cobra.Command{Use: "test"}
	app := NewCLI(tmpDir, "TEST", rootCmd)
	app.OnReady(func(c *CLIApplication) error {
		return assert.AnError
	})

	err := app.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "onReady failed")
}

// TestCLIApplication_Execute_CommandError_test_command_execution_failure
func TestCLIApplication_Execute_CommandError(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	rootCmd := &cobra.Command{
		Use: "test",
		RunE: func(cmd *cobra.Command, args []string) error {
			return assert.AnError
		},
	}

	app := NewCLI(tmpDir, "TEST", rootCmd)

	err := app.Execute()
	assert.Error(t, err)
}

// TestCLIApplication_GracefulShutdown test graceful shutdown
func TestCLIApplication_GracefulShutdown(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	var shutdownCalled bool

	rootCmd := &cobra.Command{
		Use: "test",
		Run: func(cmd *cobra.Command, args []string) {},
	}

	app := NewCLI(tmpDir, "TEST", rootCmd)
	app.OnShutdown(func(c *CLIApplication) error {
		shutdownCalled = true
		return nil
	})

	err := app.Execute()
	assert.NoError(t, err)
	assert.True(t, shutdownCalled)
}
