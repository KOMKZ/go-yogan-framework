package application

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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

// TestGRPCApplication_SetGovernanceManager test setting service governance manager
func TestGRPCApplication_SetGovernanceManager(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewGRPC(tmpDir, "TEST", nil)

	// SetGovernanceManager can also accept null
	result := app.SetGovernanceManager(nil)
	assert.Equal(t, app, result)
	assert.Nil(t, app.governanceManager)
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
	done := make(chan struct{})
	go func() {
		app.Run()
		close(done)
	}()

	select {
	case <-done:
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

// TestGRPCApplication_AutoDeregisterService_NilManager Test auto-deregistration service (no manager)
func TestGRPCApplication_AutoDeregisterService_NilManager(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewGRPC(tmpDir, "TEST", nil)

	// When governanceManager is nil, autoDeregisterService should return nil
	err := app.autoDeregisterService()
	assert.NoError(t, err)
}

// TestGRPCApplication_AutoRegisterService test auto register service
func TestGRPCApplication_AutoRegisterService(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewGRPC(tmpDir, "TEST", nil)
	err := app.Setup()
	require.NoError(t, err)

	// autoRegisterService currently only logs, does not throw errors
	err = app.autoRegisterService()
	assert.NoError(t, err)
}

// TestGRPCApplication_Run_SetupError Run startup failed due to setup error
func TestGRPCApplication_Run_SetupError(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewGRPC(tmpDir, "TEST", nil)

	app.OnSetup(func(g *GRPCApplication) error {
		return assert.AnError
	})

	assert.Panics(t, func() {
		app.Run()
	})
}

// TestGRPCApplication_OnReady_Error test failed
func TestGRPCApplication_Run_ReadyError(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewGRPC(tmpDir, "TEST", nil)

	app.OnReady(func(g *GRPCApplication) error {
		return assert.AnError
	})

	assert.Panics(t, func() {
		app.Run()
	})
}
