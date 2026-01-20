package application

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAppConfig_LoggerNil 测试 Logger 未配置时使用默认值
func TestAppConfig_LoggerNil(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	
	// 配置文件不包含 logger 段
	configContent := `
api_server:
  port: 8080
  mode: debug
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	app := New(tmpDir, "TEST", nil)

	app.OnReady(func(a *Application) error {
		// 验证 logger 已被初始化（使用默认配置）
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

// TestAppConfig_LoggerConfigured 测试 Logger 已配置时使用用户配置
func TestAppConfig_LoggerConfigured(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	
	// 🎯 配置文件包含 logger 段，日志输出到临时目录（避免污染源码目录）
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
		a.MustGetLogger().Debug("测试日志（用户配置）")
		
		go func() {
			time.Sleep(100 * time.Millisecond)
			a.Shutdown()
		}()
		return nil
	})

	err = app.Run()
	assert.NoError(t, err)
}

// TestAppConfig_DatabaseNil 测试 Database 未配置不报错
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
		// 加载配置
		appCfg, err := a.LoadAppConfig()
		require.NoError(t, err)
		
		// 验证配置加载成功（Database/Redis 等业务配置不再属于 AppConfig）
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

// TestAppConfig_MiddlewareApplyDefaults 测试中间件默认值应用
func TestAppConfig_MiddlewareApplyDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	
	// 配置包含中间件但不包含某些默认值
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
		
		// 验证默认值已应用
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
