package application

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAppConfig_LoggerNil test uses default values when Logger is not configured
func TestAppConfig_LoggerNil(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	
	// The configuration file does not contain a logger section
	configContent := `
api_server:
  port: 8080
  mode: debug
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	app := New(tmpDir, "TEST", nil)

	app.OnReady(func(a *Application) error {
		// Verify that the logger has been initialized (using default configuration)
		a.MustGetLogger().Debug("Test log")
		
		go func() {
			time.Sleep(100 * time.Millisecond)
			a.Shutdown()
		}()
		return nil
	})

	err = app.Run()
	assert.NoError(t, err)
}

// TestAppConfig_LoggerConfigured test using user configuration when logger is configured
func TestAppConfig_LoggerConfigured(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	
	// 🎯 The configuration file includes a logger section, logging output to a temporary directory (to avoid polluting the source code directory)
	configContent := `
api_server:
  port: 8080
  mode: debug

logger:
  base_log_dir: ` + tmpDir + `/logs
  level: debug
  encoding: json
  stacktrace_level: error
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	app := New(tmpDir, "TEST", nil)

	app.OnReady(func(a *Application) error {
		a.MustGetLogger().Debug("English: Test log (user configuration)（English: Test log (user configuration)）")
		
		go func() {
			time.Sleep(100 * time.Millisecond)
			a.Shutdown()
		}()
		return nil
	})

	err = app.Run()
	assert.NoError(t, err)
}

// TestAppConfig_DatabaseNil test database not configured does not result in an error
func TestAppConfig_DatabaseNil(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	
	configContent := `
api_server:
  port: 8080
  mode: debug
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	app := New(tmpDir, "TEST", nil)

	app.OnSetup(func(a *Application) error {
		// Load configuration
		appCfg, err := a.LoadAppConfig()
		require.NoError(t, err)
		
		// Verify configuration loading success (business configurations such as Database/Redis are no longer part of AppConfig)
		assert.NotNil(t, appCfg)
		
		return nil
	})

	app.OnReady(func(a *Application) error {
		go func() {
			time.Sleep(100 * time.Millisecond)
			a.Shutdown()
		}()
		return nil
	})

	err = app.Run()
	assert.NoError(t, err)
}

// TestAppConfig_MiddlewareApplyDefaults test middleware default value application
func TestAppConfig_MiddlewareApplyDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	
	// Configuration includes middleware but does not include certain default values
	configContent := `
api_server:
  port: 8080
  mode: debug

middleware:
  cors:
    enabled: true
  trace_id:
    enabled: true
  request_log:
    enabled: true
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	app := New(tmpDir, "TEST", nil)

	app.OnSetup(func(a *Application) error {
		appCfg, err := a.LoadAppConfig()
		require.NoError(t, err)
		
		// Verify default values are applied
		assert.NotNil(t, appCfg.Middleware)
		if appCfg.Middleware != nil && appCfg.Middleware.CORS != nil {
			assert.NotEmpty(t, appCfg.Middleware.CORS.AllowOrigins)
		}
		if appCfg.Middleware != nil && appCfg.Middleware.TraceID != nil {
			assert.NotEmpty(t, appCfg.Middleware.TraceID.TraceIDHeader)
		}
		if appCfg.Middleware != nil && appCfg.Middleware.RequestLog != nil {
			assert.Greater(t, appCfg.Middleware.RequestLog.MaxBodySize, 0)
		}
		
		return nil
	})

	app.OnReady(func(a *Application) error {
		a.Shutdown()
		return nil
	})

	err = app.Run()
	assert.NoError(t, err)
}

// TestAppConfig_MiddlewareDefaultsEager regression: middleware defaults must
// be applied when the app is constructed (in NewBase), not only when the old
// shadowing Application.LoadAppConfig was called — the startup path
// (startHTTPServer) previously consumed the bare unmarshalled config.
func TestAppConfig_MiddlewareDefaultsEager(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `
api_server:
  port: 8080
  mode: debug

middleware:
  cors:
    enable: true
  trace_id:
    enable: true
  request_log:
    enable: true
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	app := New(tmpDir, "TEST", nil)

	// No Run/OnSetup needed: defaults are already filled at construction.
	cfg, err := app.LoadAppConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg.Middleware)
	require.NotNil(t, cfg.Middleware.CORS)
	require.NotNil(t, cfg.Middleware.TraceID)
	require.NotNil(t, cfg.Middleware.RequestLog)

	assert.NotEmpty(t, cfg.Middleware.CORS.AllowOrigins)
	assert.Equal(t, "X-Trace-ID", cfg.Middleware.TraceID.TraceIDHeader)
	assert.Equal(t, 4096, cfg.Middleware.RequestLog.MaxBodySize)
}
