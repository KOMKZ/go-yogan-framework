package di

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewDoApplication test for creating DoApplication
func TestNewDoApplication(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		app := NewDoApplication()
		require.NotNil(t, app)
		assert.NotNil(t, app.Injector())
		assert.Equal(t, StateInit, app.State())
		assert.Equal(t, "yogan-app", app.name)
		assert.Equal(t, "0.0.1", app.version)
	})

	t.Run("with custom options", func(t *testing.T) {
		app := NewDoApplication(
			WithName("test-app"),
			WithVersion("1.0.0"),
			WithConfigPath("./testdata"),
			WithConfigPrefix("TEST"),
		)
		require.NotNil(t, app)
		assert.Equal(t, "test-app", app.name)
		assert.Equal(t, "1.0.0", app.version)
		assert.Equal(t, "./testdata", app.configPath)
		assert.Equal(t, "TEST", app.configPrefix)
	})
}

// TestDoApplication_Setup test Setup phase
func TestDoApplication_Setup(t *testing.T) {
	t.Run("successful setup", func(t *testing.T) {
		setupCalled := false
		app := NewDoApplication(
			WithConfigPath("./testdata"),
			WithName("test-app"),
			WithOnSetup(func(app *DoApplication) error {
				setupCalled = true
				return nil
			}),
		)

		err := app.Setup()
		require.NoError(t, err)
		assert.True(t, setupCalled)
		assert.Equal(t, StateSetup, app.State())
		assert.NotNil(t, app.ConfigLoader())
		assert.NotNil(t, app.Logger())
	})
}

// TestDoApplication_Start test Start phase
func TestDoApplication_Start(t *testing.T) {
	t.Run("successful start", func(t *testing.T) {
		readyCalled := false
		app := NewDoApplication(
			WithConfigPath("./testdata"),
			WithName("test-app"),
			WithOnReady(func(app *DoApplication) error {
				readyCalled = true
				return nil
			}),
		)

		err := app.Setup()
		require.NoError(t, err)

		err = app.Start()
		require.NoError(t, err)
		assert.True(t, readyCalled)
		assert.Equal(t, StateRunning, app.State())
	})
}

// TestDoApplication_GracefulShutdown testing graceful shutdown
func TestDoApplication_Shutdown(t *testing.T) {
	t.Run("successful shutdown", func(t *testing.T) {
		shutdownCalled := false
		app := NewDoApplication(
			WithConfigPath("./testdata"),
			WithName("test-app"),
			WithOnShutdown(func(ctx context.Context) error {
				shutdownCalled = true
				return nil
			}),
		)

		err := app.Setup()
		require.NoError(t, err)

		err = app.Start()
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = app.Shutdown(ctx)
		require.NoError(t, err)
		assert.True(t, shutdownCalled)
		assert.Equal(t, StateStopped, app.State())
	})
}

// TestDoApplication_HealthCheck health check test
func TestDoApplication_HealthCheck(t *testing.T) {
	t.Run("healthy app", func(t *testing.T) {
		app := NewDoApplication(
			WithConfigPath("./testdata"),
			WithName("test-app"),
		)

		err := app.Setup()
		require.NoError(t, err)

		checks := app.HealthCheck()
		assert.NotNil(t, checks)

		assert.True(t, app.IsHealthy())
	})
}

// TestAppState_String test state string
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
