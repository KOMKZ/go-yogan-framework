package application

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNew 测试创建应用实例
func TestNew(t *testing.T) {
	app := New("./configs", "APP", nil)

	assert.NotNil(t, app)
	assert.Equal(t, StateInit, app.GetState())
	assert.NotNil(t, app.Context())
}

// TestApplication_GetState 测试状态获取（线程安全）
func TestApplication_GetState(t *testing.T) {
	app := New("./configs", "APP", nil)

	assert.Equal(t, StateInit, app.GetState())

	app.setState(StateRunning)
	assert.Equal(t, StateRunning, app.GetState())
}

// TestApplication_ChainCall 测试链式调用
func TestApplication_ChainCall(t *testing.T) {
	app := New("./testdata", "TEST", nil).
		OnSetup(func(a *Application) error {
			return nil
		}).
		OnReady(func(a *Application) error {
			a.Shutdown() // 手动触发关闭
			return nil
		}).
		OnShutdown(func(a *Application) error {
			return nil
		})

	assert.NotNil(t, app)
	assert.NotNil(t, app.httpOnSetup)
	assert.NotNil(t, app.httpOnReady)
	assert.NotNil(t, app.httpOnShutdown)
}

// TestApplication_Run_WithConfig 测试完整启动流程（有配置文件）
func TestApplication_Run_WithConfig(t *testing.T) {
	// 创建临时配置文件
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

	// 注册回调
	app.OnSetup(func(a *Application) error {
		setupCalled = true
		assert.NotNil(t, a.GetConfigLoader())
		assert.NotNil(t, a.MustGetLogger())
		return nil
	})

	app.OnReady(func(a *Application) error {
		readyCalled = true
		assert.Equal(t, StateRunning, a.GetState())

		// 验证可以读取配置
		port := a.GetConfigLoader().GetViper().GetInt("server.port")
		assert.Equal(t, 8080, port)

		// 手动触发关闭
		go func() {
			time.Sleep(100 * time.Millisecond)
			a.Shutdown()
		}()
		return nil
	})

	app.OnConfigReload(func(loader *config.Loader) {
		atomic.AddInt32(&reloadCalled, 1)
	})

	// 运行应用
	err = app.Run()
	assert.NoError(t, err)

	// 验证回调被调用
	assert.True(t, setupCalled, "OnSetup should be called")
	assert.True(t, readyCalled, "OnReady should be called")
	assert.Equal(t, StateStopped, app.GetState())
}

// TestApplication_OnReady_Error 测试 OnReady 返回错误
func TestApplication_OnReady_Error(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)
	require.NoError(t, err)

	app := New(tmpDir, "TEST", nil)

	app.OnReady(func(a *Application) error {
		return assert.AnError // 返回错误
	})

	err = app.Run()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "onReady failed")
}

// TestApplication_OnShutdown 测试关闭回调
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

// TestApplication_Context 测试应用上下文
func TestApplication_Context(t *testing.T) {
	app := New("./testdata", "TEST", nil)

	ctx := app.Context()
	assert.NotNil(t, ctx)

	// 验证上下文未取消
	select {
	case <-ctx.Done():
		t.Fatal("context should not be done initially")
	default:
	}

	// 触发关闭
	app.Shutdown()

	// 验证上下文已取消
	select {
	case <-ctx.Done():
		// 预期行为
	case <-time.After(1 * time.Second):
		t.Fatal("context should be done after shutdown")
	}
}

// TestApplication_ConfigReload 测试配置热更新
func TestApplication_ConfigReload(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	initialConfig := `
server:
  port: 8080
`
	err := os.WriteFile(configFile, []byte(initialConfig), 0644)
	require.NoError(t, err)

	var reloadCount int32
	var lastPort int

	app := New(tmpDir, "TEST", nil)

	app.OnReady(func(a *Application) error {
		// 等待一会儿后修改配置
		go func() {
			time.Sleep(200 * time.Millisecond)

			newConfig := `
server:
  port: 9090
`
			os.WriteFile(configFile, []byte(newConfig), 0644)

			// 再等待一会儿后关闭
			time.Sleep(1 * time.Second)
			a.Shutdown()
		}()
		return nil
	})

	app.OnConfigReload(func(loader *config.Loader) {
		atomic.AddInt32(&reloadCount, 1)
		lastPort = loader.GetViper().GetInt("server.port")
	})

	err = app.Run()
	assert.NoError(t, err)

	// 验证配置热更新被触发
	count := atomic.LoadInt32(&reloadCount)
	assert.GreaterOrEqual(t, count, int32(1), "配置更新应该被触发至少1次")
	assert.Equal(t, 9090, lastPort, "应该收到最新的配置")
}

// TestAppState_String 测试状态字符串表示
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

// TestApplication_GetLogger 测试获取日志实例
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

// TestApplication_GetConfigLoader 测试获取配置加载器
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
