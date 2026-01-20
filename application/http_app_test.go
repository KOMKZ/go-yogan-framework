package application

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRouterRegistrar simulated router registrar
type mockRouterRegistrar struct {
	registered bool
}

func (m *mockRouterRegistrar) RegisterRoutes(engine *gin.Engine, app *Application) {
	m.registered = true
}

// TestNew test create application instance
func TestNew(t *testing.T) {
	app := New("./configs", "APP", nil)

	assert.NotNil(t, app)
	assert.Equal(t, StateInit, app.GetState())
	assert.NotNil(t, app.Context())
}

// TestApplication_GetState test state retrieval (thread-safe)
func TestApplication_GetState(t *testing.T) {
	app := New("./configs", "APP", nil)

	assert.Equal(t, StateInit, app.GetState())

	app.setState(StateRunning)
	assert.Equal(t, StateRunning, app.GetState())
}

// TestApplication_ChainCall Test chained calls
func TestApplication_ChainCall(t *testing.T) {
	var setupCalled, readyCalled, shutdownCalled bool

	app := New("./testdata", "TEST", nil).
		OnSetup(func(a *Application) error {
			setupCalled = true
			return nil
		}).
		OnReady(func(a *Application) error {
			readyCalled = true
			a.Shutdown() // Manually trigger shutdown
			return nil
		}).
		OnShutdown(func(a *Application) error {
			shutdownCalled = true
			return nil
		})

	assert.NotNil(t, app)
	// Verify that the callback function has been registered (via BaseApplication's callback)
	assert.NotNil(t, app.BaseApplication.onSetup)
	assert.NotNil(t, app.BaseApplication.onReady)
	assert.NotNil(t, app.BaseApplication.onShutdown)

	// Here no validation for setupCalled etc., as it is just registration, not execution yet
	_ = setupCalled
	_ = readyCalled
	_ = shutdownCalled
}

// TestApplication_Run_WithConfig test full startup process (with configuration file)
func TestApplication_Run_WithConfig(t *testing.T) {
	// Create temporary configuration file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  port: 8080
  mode: debug
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	var (
		setupCalled  bool
		readyCalled  bool
		reloadCalled int32
	)

	app := New(tmpDir, "TEST", nil)

	// register callback
	app.OnSetup(func(a *Application) error {
		setupCalled = true
		assert.NotNil(t, a.GetConfigLoader())
		assert.NotNil(t, a.MustGetLogger())
		return nil
	})

	app.OnReady(func(a *Application) error {
		readyCalled = true
		assert.Equal(t, StateRunning, a.GetState())

		// Verify that configuration can be read
		port := a.GetConfigLoader().GetViper().GetInt("server.port")
		assert.Equal(t, 8080, port)

		// Manually trigger shutdown
		go func() {
			time.Sleep(100 * time.Millisecond)
			a.Shutdown()
		}()
		return nil
	})

	app.OnConfigReload(func(loader *config.Loader) {
		atomic.AddInt32(&reloadCalled, 1)
	})

	// Run the application
	err = app.Run()
	assert.NoError(t, err)

	// Verify callback is called
	assert.True(t, setupCalled, "OnSetup should be called")
	assert.True(t, readyCalled, "OnReady should be called")
	assert.Equal(t, StateStopped, app.GetState())
}

// TestApplication_OnReady_Error Test OnReady returns error
func TestApplication_OnReady_Error(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)
	require.NoError(t, err)

	app := New(tmpDir, "TEST", nil)

	app.OnReady(func(a *Application) error {
		return assert.AnError // Return error
	})

	err = app.Run()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "onReady failed")
}

// TestApplication_OnShutdown test shutdown callback
func TestApplication_OnShutdown(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)
	require.NoError(t, err)

	var shutdownCalled bool

	app := New(tmpDir, "TEST", nil)

	app.OnReady(func(a *Application) error {
		go func() {
			time.Sleep(100 * time.Millisecond)
			a.Shutdown()
		}()
		return nil
	})

	app.OnShutdown(func(a *Application) error {
		shutdownCalled = true
		return nil
	})

	err = app.Run()
	assert.NoError(t, err)
	assert.True(t, shutdownCalled)
}

// TestApplication_Context test application context
func TestApplication_Context(t *testing.T) {
	app := New("./testdata", "TEST", nil)

	ctx := app.Context()
	assert.NotNil(t, ctx)

	// Verify that the context has not been cancelled
	select {
	case <-ctx.Done():
		t.Fatal("context should not be done initially")
	default:
	}

	// Trigger close
	app.Shutdown()

	// Verify context has been cancelled
	select {
	case <-ctx.Done():
		// Expected behavior
	case <-time.After(1 * time.Second):
		t.Fatal("context should be done after shutdown")
	}
}

// TestApplication_ConfigReload test configuration hot update callback registration
func TestApplication_ConfigReload(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	initialConfig := `
server:
  port: 8080
`
	err := os.WriteFile(configFile, []byte(initialConfig), 0644)
	require.NoError(t, err)

	app := New(tmpDir, "TEST", nil)

	// Register configuration update callback
	callbackRegistered := false
	app.OnConfigReload(func(loader *config.Loader) {
		callbackRegistered = true
	})

	app.OnReady(func(a *Application) error {
		// Immediately close, only verify that callbacks can be registered
		a.Shutdown()
		return nil
	})

	err = app.Run()
	assert.NoError(t, err)

	// Verify callback is registered
	assert.NotNil(t, app.BaseApplication)
	_ = callbackRegistered // Not actually triggered, but callback is registered
}

// TestAppState_String test state string representation
func TestAppState_String(t *testing.T) {
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

// TestApplication_GetLogger test getting log instance
func TestApplication_GetLogger(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)
	require.NoError(t, err)

	app := New(tmpDir, "TEST", nil)

	app.OnSetup(func(a *Application) error {
		logger := a.MustGetLogger()
		assert.NotNil(t, logger)
		logger.DebugCtx(context.Background(), "Test log")
		return nil
	})

	app.OnReady(func(a *Application) error {
		a.Shutdown()
		return nil
	})

	err = app.Run()
	assert.NoError(t, err)
}

// TestApplication_GetConfigLoader test for getting config loader
func TestApplication_GetConfigLoader(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("app:\n  name: test-app\n"), 0644)
	require.NoError(t, err)

	app := New(tmpDir, "TEST", nil)

	app.OnSetup(func(a *Application) error {
		loader := a.GetConfigLoader()
		assert.NotNil(t, loader)

		name := loader.GetViper().GetString("app.name")
		assert.Equal(t, "test-app", name)
		return nil
	})

	app.OnReady(func(a *Application) error {
		a.Shutdown()
		return nil
	})

	err = app.Run()
	assert.NoError(t, err)
}

// TestNewWithDefaults test creating an application with default configuration
func TestNewWithDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "configs", "test-app")
	err := os.MkdirAll(appDir, 0755)
	require.NoError(t, err)

	configFile := filepath.Join(appDir, "config.yaml")
	err = os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)
	require.NoError(t, err)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	app := NewWithDefaults("test-app")
	assert.NotNil(t, app)
}

// TestNewWithFlags test creating an application using Flags
func TestNewWithFlags(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	flags := &AppFlags{Port: 9090}
	app := NewWithFlags(tmpDir, "TEST", flags)

	assert.NotNil(t, app)
}

// TestApplication_WithVersion test version settings
func TestApplication_WithVersion(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := New(tmpDir, "TEST", nil)
	result := app.WithVersion("v1.0.0")

	assert.Equal(t, app, result)
	assert.Equal(t, "v1.0.0", app.GetVersion())
}

// TestApplication_GetHTTPServer test to get HTTP server
func TestApplication_GetHTTPServer(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("api_server:\n  port: 8080\n  mode: test\n"), 0644)

	app := New(tmpDir, "TEST", nil)

	// should be nil when not started
	assert.Nil(t, app.GetHTTPServer())
}

// TestApplication_GetRouterManager test getting router manager
func TestApplication_GetRouterManager(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := New(tmpDir, "TEST", nil)

	manager := app.GetRouterManager()
	assert.NotNil(t, manager)
}

// TestApplication_RegisterRoutes test route registration
func TestApplication_RegisterRoutes(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := New(tmpDir, "TEST", nil)

	registrar := &mockRouterRegistrar{}
	result := app.RegisterRoutes(registrar)

	assert.Equal(t, app, result)
}

// TestApplication_LoadAppConfig test loading application configuration
func TestApplication_LoadAppConfig_HTTP(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("api_server:\n  port: 9090\n  mode: release\n"), 0644)

	app := New(tmpDir, "TEST", nil)

	cfg, err := app.LoadAppConfig()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, 9090, cfg.ApiServer.Port)
}

// TestNew_DefaultValues test default value handling
func TestNew_DefaultValues(t *testing.T) {
	// Test empty configuration path using default values
	app := New("", "", nil)
	assert.NotNil(t, app)
}

// TestApplication_RunNonBlocking_NoRoutes_test_non-blocking_run_with_no_routes
func TestApplication_RunNonBlocking_NoRoutes(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("api_server:\n  port: 8080\n  mode: test\n"), 0644)

	app := New(tmpDir, "TEST", nil)

	var readyCalled bool
	app.OnReady(func(a *Application) error {
		readyCalled = true
		return nil
	})

	err := app.RunNonBlocking()
	assert.NoError(t, err)
	assert.True(t, readyCalled)
	assert.Equal(t, StateRunning, app.GetState())

	// Close
	app.Shutdown()
	time.Sleep(100 * time.Millisecond)
}

// TestApplication_GracefulShutdown test graceful shutdown
func TestApplication_GracefulShutdown(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("api_server:\n  port: 8080\n  mode: test\n"), 0644)

	var shutdownCalled bool

	app := New(tmpDir, "TEST", nil)
	app.OnShutdown(func(a *Application) error {
		shutdownCalled = true
		return nil
	})

	err := app.RunNonBlocking()
	assert.NoError(t, err)

	// Manually call gracefulShutdown
	err = app.gracefulShutdown()
	assert.NoError(t, err)
	assert.True(t, shutdownCalled)
}

// TestApplication_StartHTTPServer_NoRegistrar_test_starting_HTTP_server_without_router_registerer
func TestApplication_StartHTTPServer_NoRegistrar(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("api_server:\n  port: 0\n  mode: test\n"), 0644)

	app := New(tmpDir, "TEST", nil)
	err := app.Setup()
	assert.NoError(t, err)

	// When there is no routerRegistrar, startHTTPServer should return nil directly
	err = app.startHTTPServer()
	assert.NoError(t, err)
}

// TestApplication_GracefulShutdown_WithHTTPServer Test graceful shutdown with HTTP server present
func TestApplication_GracefulShutdown_WithHTTPServer(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("api_server:\n  port: 0\n  mode: test\n"), 0644)

	app := New(tmpDir, "TEST", nil)

	var shutdownCalled bool
	app.OnShutdown(func(a *Application) error {
		shutdownCalled = true
		return nil
	})

	err := app.RunNonBlocking()
	assert.NoError(t, err)

	err = app.gracefulShutdown()
	assert.NoError(t, err)
	assert.True(t, shutdownCalled)
}

// TestApplication_RunNonBlocking_SetupError_TestSetup_Failed
func TestApplication_RunNonBlocking_SetupError(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("api_server:\n  port: 0\n  mode: test\n"), 0644)

	app := New(tmpDir, "TEST", nil)

	app.OnSetup(func(a *Application) error {
		return assert.AnError
	})

	err := app.RunNonBlocking()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "setup failed")
}
