package application

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/di"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewGRPC test creating gRPC application
func TestNewGRPC(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)
	require.NoError(t, err)

	app := NewGRPC(tmpDir, "TEST", nil)

	assert.NotNil(t, app)
	assert.NotNil(t, app.BaseApplication)
}

// TestNewGRPC_DefaultValues test default value handling
func TestNewGRPC_DefaultValues(t *testing.T) {
	// Test empty configuration path using default values
	app := NewGRPC("", "", nil)
	assert.NotNil(t, app)
}

// TestNewGRPCWithDefaults test creating gRPC application with default configuration
func TestNewGRPCWithDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "configs", "grpc-app")
	err := os.MkdirAll(appDir, 0755)
	require.NoError(t, err)

	configFile := filepath.Join(appDir, "config.yaml")
	err = os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)
	require.NoError(t, err)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	app := NewGRPCWithDefaults("grpc-app")
	assert.NotNil(t, app)
}

// TestNewGRPCWithFlags test creating gRPC application using flags
func TestNewGRPCWithFlags(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	flags := &AppFlags{Port: 9090}
	app := NewGRPCWithFlags(tmpDir, "TEST", flags)

	assert.NotNil(t, app)
}

// TestGRPCApplication_Callbacks test callback registration
func TestGRPCApplication_Callbacks(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewGRPC(tmpDir, "TEST", nil)

	var setupCalled, readyCalled, shutdownCalled bool

	result := app.
		OnSetup(func(g *GRPCApplication) error {
			setupCalled = true
			return nil
		}).
		OnReady(func(g *GRPCApplication) error {
			readyCalled = true
			return nil
		}).
		OnShutdown(func(g *GRPCApplication) error {
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

// TestGRPCApplication_Run test blocking run
func TestGRPCApplication_Run(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewGRPC(tmpDir, "TEST", nil)

	var readyCalled bool

	app.OnReady(func(g *GRPCApplication) error {
		readyCalled = true
		// Trigger close in OnReady
		go func() {
			time.Sleep(50 * time.Millisecond)
			g.Cancel()
		}()
		return nil
	})

	// Run in a goroutine to avoid blocking tests
	done := make(chan error, 1)
	go func() {
		done <- app.Run()
	}()

	select {
	case err := <-done:
		assert.NoError(t, err)
		assert.True(t, readyCalled)
	case <-time.After(2 * time.Second):
		t.Fatal("Run should complete after cancel")
	}
}

// TestGRPCApplication_GracefulShutdown tests graceful shutdown
func TestGRPCApplication_GracefulShutdown(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewGRPC(tmpDir, "TEST", nil)

	// First setup
	err := app.Setup()
	require.NoError(t, err)

	// Test graceful shutdown
	err = app.gracefulShutdown()
	assert.NoError(t, err)
}

// TestGRPCApplication_Run_SetupError Run startup failed due to setup error
// 🎯 Run() must return the error instead of panicking (unified with HTTP/Cron)
func TestGRPCApplication_Run_SetupError(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewGRPC(tmpDir, "TEST", nil)

	app.OnSetup(func(g *GRPCApplication) error {
		return assert.AnError
	})

	err := app.Run()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "setup failed")
}

// TestGRPCApplication_Run_ReadyError OnReady failed
// 🎯 Run() must return the error instead of panicking (unified with HTTP/Cron)
func TestGRPCApplication_Run_ReadyError(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewGRPC(tmpDir, "TEST", nil)

	app.OnReady(func(g *GRPCApplication) error {
		return assert.AnError
	})

	err := app.Run()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "onReady failed")
}

// TestGRPCApplication_RunNonBlocking_AutoStartServer regression: with a
// *grpc.Server provider registered in DI and grpc.server.enabled, the framework
// must start the server automatically (instead of leaving it to handwritten
// template code) and stop it on Shutdown.
func TestGRPCApplication_RunNonBlocking_AutoStartServer(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("grpc:\n  server:\n    enabled: true\n    port: 0\n"), 0644)
	require.NoError(t, err)

	app := NewGRPC(tmpDir, "TEST", nil)
	do.Provide(app.GetInjector(), di.ProvideGRPCServer)

	var readyCalled bool
	app.OnReady(func(g *GRPCApplication) error {
		readyCalled = true
		return nil
	})

	err = app.RunNonBlocking()
	require.NoError(t, err)
	assert.True(t, readyCalled)

	// Server must have been started automatically with a real port (port 0 → auto-assigned)
	require.NotNil(t, app.GetGRPCServer())
	assert.Greater(t, app.GetGRPCServer().Port, 0)

	// Shutdown must stop the server and the container
	err = app.Shutdown()
	require.NoError(t, err)
	assert.Equal(t, StateStopped, app.GetState())
}
