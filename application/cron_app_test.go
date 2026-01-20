package application

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewCron test creating Cron application
func TestNewCron(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)
	require.NoError(t, err)

	app, err := NewCron(tmpDir, "TEST")

	assert.NoError(t, err)
	assert.NotNil(t, app)
	assert.NotNil(t, app.BaseApplication)
	assert.NotNil(t, app.GetScheduler())
}

// TestNewCronWithDefaults tests creating a Cron application using default configuration
func TestNewCronWithDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "configs", "cron-app")
	err := os.MkdirAll(appDir, 0755)
	require.NoError(t, err)

	configFile := filepath.Join(appDir, "config.yaml")
	err = os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)
	require.NoError(t, err)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	app, err := NewCronWithDefaults("cron-app")
	assert.NoError(t, err)
	assert.NotNil(t, app)
}

// TestCronApplication_Callbacks test callback registration
func TestCronApplication_Callbacks(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app, err := NewCron(tmpDir, "TEST")
	require.NoError(t, err)

	var setupCalled, readyCalled, shutdownCalled bool

	result := app.
		OnSetup(func(c *CronApplication) error {
			setupCalled = true
			return nil
		}).
		OnReady(func(c *CronApplication) error {
			readyCalled = true
			return nil
		}).
		OnShutdown(func(c *CronApplication) error {
			shutdownCalled = true
			return nil
		})

	assert.Equal(t, app, result)
	assert.NotNil(t, app.cronOnSetup)
	assert.NotNil(t, app.cronOnReady)
	assert.NotNil(t, app.cronOnShutdown)

	_ = setupCalled
	_ = readyCalled
	_ = shutdownCalled
}

// TestCronApplication_RunNonBlocking test non-blocking execution
func TestCronApplication_RunNonBlocking(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app, err := NewCron(tmpDir, "TEST")
	require.NoError(t, err)

	var setupCalled, readyCalled bool

	app.OnSetup(func(c *CronApplication) error {
		setupCalled = true
		return nil
	})
	app.OnReady(func(c *CronApplication) error {
		readyCalled = true
		return nil
	})

	err = app.RunNonBlocking()
	assert.NoError(t, err)
	assert.True(t, setupCalled)
	assert.True(t, readyCalled)
	assert.Equal(t, StateRunning, app.GetState())

	// Close the application
	app.Shutdown()
	time.Sleep(100 * time.Millisecond) // wait for close to complete
}

// TestCronApplication_RegisterTask_test_task_registration
func TestCronApplication_RegisterTask(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app, err := NewCron(tmpDir, "TEST")
	require.NoError(t, err)

	// Register a simple task
	job, err := app.RegisterTask("*/5 * * * *", func() {
		// test task
	})

	assert.NoError(t, err)
	assert.NotNil(t, job)
}

// mockTaskRegistrar simulated task registrar
type mockTaskRegistrar struct {
	registered bool
}

func (m *mockTaskRegistrar) RegisterTasks(app *CronApplication) error {
	m.registered = true
	return nil
}

// TestCronApplication_RegisterTasks test task registrar registration
func TestCronApplication_RegisterTasks(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app, err := NewCron(tmpDir, "TEST")
	require.NoError(t, err)

	registrar := &mockTaskRegistrar{}
	result := app.RegisterTasks(registrar)

	assert.Equal(t, app, result)

	// Run the application, verify that the registry is called
	err = app.RunNonBlocking()
	assert.NoError(t, err)
	assert.True(t, registrar.registered)

	app.Shutdown()
	time.Sleep(100 * time.Millisecond)
}

// TestCronApplicationShutdown Test manual shutdown
func TestCronApplication_Shutdown(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app, err := NewCron(tmpDir, "TEST")
	require.NoError(t, err)

	err = app.RunNonBlocking()
	assert.NoError(t, err)

	ctx := app.Context()
	app.Shutdown()

	select {
	case <-ctx.Done():
		// Expected behavior
	case <-time.After(1 * time.Second):
		t.Fatal("context should be done after shutdown")
	}
}

// TestCronApplication_GracefulShutdown graceful shutdown test
func TestCronApplication_GracefulShutdown(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("cron:\n  shutdown_timeout: 5\n"), 0644)

	var shutdownCalled bool

	app, err := NewCron(tmpDir, "TEST")
	require.NoError(t, err)

	app.OnShutdown(func(c *CronApplication) error {
		shutdownCalled = true
		return nil
	})

	err = app.RunNonBlocking()
	assert.NoError(t, err)

	// Manually call gracefulShutdown
	err = app.gracefulShutdown()
	assert.NoError(t, err)
	assert.True(t, shutdownCalled)
	assert.Equal(t, StateStopped, app.GetState())
}

// TestCronApplication_Run test blocking run
func TestCronApplication_Run(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app, err := NewCron(tmpDir, "TEST")
	require.NoError(t, err)

	var readyCalled bool

	app.OnReady(func(c *CronApplication) error {
		readyCalled = true
		// Trigger close in OnReady
		go func() {
			time.Sleep(50 * time.Millisecond)
			c.Shutdown()
		}()
		return nil
	})

	// Run in a goroutine to avoid blocking the test
	done := make(chan error, 1)
	go func() {
		done <- app.Run()
	}()

	select {
	case err := <-done:
		assert.NoError(t, err)
		assert.True(t, readyCalled)
	case <-time.After(2 * time.Second):
		t.Fatal("Run should complete after shutdown")
	}
}

// TestCronApplication_Run_SetupError Run startup failed due to setup error
func TestCronApplication_Run_SetupError(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app, err := NewCron(tmpDir, "TEST")
	require.NoError(t, err)

	app.OnSetup(func(c *CronApplication) error {
		return assert.AnError
	})

	err = app.Run()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "onSetup failed")
}

// TestNewCron_DefaultValues test default value handling
func TestNewCron_DefaultValues(t *testing.T) {
	// Test empty configuration path using default values
	app, err := NewCron("", "")
	assert.NoError(t, err)
	assert.NotNil(t, app)
}

// TestCronApplication_GracefulShutdown_WithShutdownError Test graceful shutdown with shutdown error returned
func TestCronApplication_GracefulShutdown_WithShutdownError(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("cron:\n  shutdown_timeout: 1\n"), 0644)

	app, err := NewCron(tmpDir, "TEST")
	require.NoError(t, err)

	app.OnShutdown(func(c *CronApplication) error {
		return assert.AnError // Return error
	})

	err = app.RunNonBlocking()
	assert.NoError(t, err)

	// Call gracefulShutdown, continue execution even if the callback returns an error
	err = app.gracefulShutdown()
	assert.NoError(t, err) // Base shutdown successful
}

// TestCronApplication_OnReady_Returns_Error
func TestCronApplication_Run_OnReadyError(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app, err := NewCron(tmpDir, "TEST")
	require.NoError(t, err)

	app.OnReady(func(c *CronApplication) error {
		return assert.AnError
	})

	err = app.Run()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "onReady failed")
}

// TestCronApplication_Run_TaskRegistrarError Test task registrar returns error
func TestCronApplication_Run_TaskRegistrarError(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app, err := NewCron(tmpDir, "TEST")
	require.NoError(t, err)

	registrar := &errorTaskRegistrar{}
	app.RegisterTasks(registrar)

	err = app.Run()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "register tasks failed")
}

// errorTaskRegistrar simulates returning an erroneous task registrar
type errorTaskRegistrar struct{}

func (m *errorTaskRegistrar) RegisterTasks(app *CronApplication) error {
	return assert.AnError
}
