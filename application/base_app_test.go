package application

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewBase test creating a base application instance
func TestNewBase(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)
	require.NoError(t, err)

	app := NewBase(tmpDir, "TEST", "http", nil)

	assert.NotNil(t, app)
	assert.Equal(t, StateInit, app.GetState())
	assert.NotNil(t, app.Context())
	assert.NotNil(t, app.GetInjector())
	assert.NotNil(t, app.MustGetLogger())
	assert.NotNil(t, app.GetConfigLoader())
}

// TestNewBaseWithDefaults test creating an application using default configuration
func TestNewBaseWithDefaults(t *testing.T) {
	// Create temporary configuration directory
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "configs", "test-app")
	err := os.MkdirAll(appDir, 0755)
	require.NoError(t, err)

	configFile := filepath.Join(appDir, "config.yaml")
	err = os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)
	require.NoError(t, err)

	// Switch working directory
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	app := NewBaseWithDefaults("test-app", "http")
	assert.NotNil(t, app)
}

// TestBaseApplication_WithVersion test version settings
func TestBaseApplication_WithVersion(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewBase(tmpDir, "TEST", "http", nil)
	app.WithVersion("v1.2.3")

	assert.Equal(t, "v1.2.3", app.GetVersion())
}

// TestBaseApplication_GetStartDuration test startup duration
func TestBaseApplication_GetStartDuration(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewBase(tmpDir, "TEST", "http", nil)
	time.Sleep(10 * time.Millisecond)

	duration := app.GetStartDuration()
	assert.Greater(t, duration.Milliseconds(), int64(9))
}

// TestBaseApplication_Setup test setup process
func TestBaseApplication_Setup(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewBase(tmpDir, "TEST", "http", nil)

	var setupCalled bool
	app.OnSetup(func(b *BaseApplication) error {
		setupCalled = true
		return nil
	})

	err := app.Setup()
	assert.NoError(t, err)
	assert.True(t, setupCalled)
	assert.Equal(t, StateSetup, app.GetState())
}

// TestBaseApplication_Shutdown test shutdown process
func TestBaseApplication_Shutdown(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewBase(tmpDir, "TEST", "http", nil)

	var shutdownCalled bool
	app.OnShutdown(func(ctx context.Context) error {
		shutdownCalled = true
		return nil
	})

	err := app.Shutdown(5 * time.Second)
	assert.NoError(t, err)
	assert.True(t, shutdownCalled)
	assert.Equal(t, StateStopped, app.GetState())
}

// TestBaseApplication_Cancel test manual cancellation
func TestBaseApplication_Cancel(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewBase(tmpDir, "TEST", "http", nil)

	ctx := app.Context()
	select {
	case <-ctx.Done():
		t.Fatal("context should not be done initially")
	default:
	}

	app.Cancel()

	select {
	case <-ctx.Done():
		// Expected behavior
	case <-time.After(1 * time.Second):
		t.Fatal("context should be done after cancel")
	}
}

// TestBaseApplication_Callbacks test callback registration
func TestBaseApplication_Callbacks(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewBase(tmpDir, "TEST", "http", nil)

	// Test chain call
	result := app.
		OnSetup(func(b *BaseApplication) error { return nil }).
		OnReady(func(b *BaseApplication) error { return nil }).
		OnConfigReload(func(l *config.Loader) {}).
		OnShutdown(func(ctx context.Context) error { return nil })

	assert.Equal(t, app, result)
	assert.NotNil(t, app.onSetup)
	assert.NotNil(t, app.onReady)
	assert.NotNil(t, app.onConfigReload)
	assert.NotNil(t, app.onShutdown)
}

// TestBaseApplication_LoadAppConfig test to load application configuration
func TestBaseApplication_LoadAppConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("api_server:\n  port: 9090\n"), 0644)

	app := NewBase(tmpDir, "TEST", "http", nil)

	appCfg, err := app.LoadAppConfig()
	assert.NoError(t, err)
	assert.NotNil(t, appCfg)
}

// TestAppState_String test state string representation
func TestAppState_String_Base(t *testing.T) {
	tests := []struct {
		state    AppState
		expected string
	}{
		{StateInit, "Init"},
		{StateSetup, "Setup"},
		{StateRunning, "Running"},
		{StateStopping, "Stopping"},
		{StateStopped, "Stopped"},
		{AppState(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.state.String())
		})
	}
}

// TestBaseApplication_WaitShutdown test waiting for shutdown signal
func TestBaseApplication_WaitShutdown(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewBase(tmpDir, "TEST", "http", nil)
	err := app.Setup()
	require.NoError(t, err)

	// Call Cancel in another goroutine to trigger context cancellation
	go func() {
		time.Sleep(50 * time.Millisecond)
		app.Cancel()
	}()

	// WaitShutdown should return after Cancel
	done := make(chan struct{})
	go func() {
		app.WaitShutdown()
		close(done)
	}()

	select {
	case <-done:
		// Expected behavior
	case <-time.After(2 * time.Second):
		t.Fatal("WaitShutdown should complete after cancel")
	}
}

// TestBaseApplication_MustGetLogger_Panic Test for panic when getting logger before initialization
func TestBaseApplication_MustGetLogger_Panic(t *testing.T) {
	// Create an uninitialized app (skip normal initialization process)
	app := &BaseApplication{}

	assert.Panics(t, func() {
		app.MustGetLogger()
	})
}

// TestBaseApplication_GetConfigLoader_Panic test for config loader panic when not initialized
func TestBaseApplication_GetConfigLoader_Panic(t *testing.T) {
	// Create an uninitialized app
	app := &BaseApplication{}

	assert.Panics(t, func() {
		app.GetConfigLoader()
	})
}

// TestBaseApplication_LoadAppConfig_NotInitialized tests AppConfig is not initialized
func TestBaseApplication_LoadAppConfig_NotInitialized(t *testing.T) {
	// Create an uninitialized app
	app := &BaseApplication{}

	cfg, err := app.LoadAppConfig()
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "AppConfig not initialized")
}

// TestBaseApplication_Setup_Error setup callback returns error
func TestBaseApplication_Setup_Error(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewBase(tmpDir, "TEST", "http", nil)

	app.OnSetup(func(b *BaseApplication) error {
		return assert.AnError
	})

	err := app.Setup()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "onSetup failed")
}

// TestBaseApplication_Shutdown_CallsCallback Tests that Shutdown calls callback
func TestBaseApplication_Shutdown_CallsCallback(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewBase(tmpDir, "TEST", "http", nil)

	var called bool
	app.OnShutdown(func(ctx context.Context) error {
		called = true
		return nil
	})

	err := app.Shutdown(5 * time.Second)
	assert.NoError(t, err)
	assert.True(t, called)
}

// TestBaseApplication_SetState test state settings
func TestBaseApplication_SetState(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewBase(tmpDir, "TEST", "http", nil)

	// Initial state is Init
	assert.Equal(t, StateInit, app.GetState())

	// Set to Running
	app.setState(StateRunning)
	assert.Equal(t, StateRunning, app.GetState())

	// Set to Stopped
	app.setState(StateStopped)
	assert.Equal(t, StateStopped, app.GetState())
}
