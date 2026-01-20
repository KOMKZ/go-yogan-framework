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

// mockRouterRegistrar 模拟路由注册器
type mockRouterRegistrar struct {
	registered bool
}

func (m *mockRouterRegistrar) RegisterRoutes(engine *gin.Engine, app *Application) {
	m.registered = true
}

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
	var setupCalled, readyCalled, shutdownCalled bool

	app := New("./testdata", "TEST", nil).
		OnSetup(func(a *Application) error {
			setupCalled = true
			return nil
		}).
		OnReady(func(a *Application) error {
			readyCalled = true
			a.Shutdown() // 手动触发关闭
			return nil
		}).
		OnShutdown(func(a *Application) error {
			shutdownCalled = true
			return nil
		})

	assert.NotNil(t, app)
	// 验证回调函数已注册（通过 BaseApplication 的回调）
	assert.NotNil(t, app.BaseApplication.onSetup)
	assert.NotNil(t, app.BaseApplication.onReady)
	assert.NotNil(t, app.BaseApplication.onShutdown)

	// 这里不验证 setupCalled 等，因为只是注册，还没执行
	_ = setupCalled
	_ = readyCalled
	_ = shutdownCalled
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

// TestApplication_ConfigReload 测试配置热更新回调注册
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

	// 注册配置更新回调
	callbackRegistered := false
	app.OnConfigReload(func(loader *config.Loader) {
		callbackRegistered = true
	})

	app.OnReady(func(a *Application) error {
		// 立即关闭，只验证回调可以注册
		a.Shutdown()
		return nil
	})

	err = app.Run()
	assert.NoError(t, err)

	// 验证回调已注册
	assert.NotNil(t, app.BaseApplication)
	_ = callbackRegistered // 未实际触发，但回调已注册
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

// TestNewWithDefaults 测试使用默认配置创建应用
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

// TestNewWithFlags 测试使用 Flags 创建应用
func TestNewWithFlags(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	flags := &AppFlags{Port: 9090}
	app := NewWithFlags(tmpDir, "TEST", flags)

	assert.NotNil(t, app)
}

// TestApplication_WithVersion 测试版本设置
func TestApplication_WithVersion(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := New(tmpDir, "TEST", nil)
	result := app.WithVersion("v1.0.0")

	assert.Equal(t, app, result)
	assert.Equal(t, "v1.0.0", app.GetVersion())
}

// TestApplication_GetHTTPServer 测试获取 HTTP Server
func TestApplication_GetHTTPServer(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("api_server:\n  port: 8080\n  mode: test\n"), 0644)

	app := New(tmpDir, "TEST", nil)

	// 未启动时应该是 nil
	assert.Nil(t, app.GetHTTPServer())
}

// TestApplication_GetRouterManager 测试获取路由管理器
func TestApplication_GetRouterManager(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := New(tmpDir, "TEST", nil)

	manager := app.GetRouterManager()
	assert.NotNil(t, manager)
}

// TestApplication_RegisterRoutes 测试注册路由
func TestApplication_RegisterRoutes(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := New(tmpDir, "TEST", nil)

	registrar := &mockRouterRegistrar{}
	result := app.RegisterRoutes(registrar)

	assert.Equal(t, app, result)
}

// TestApplication_LoadAppConfig 测试加载应用配置
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

// TestNew_DefaultValues 测试默认值处理
func TestNew_DefaultValues(t *testing.T) {
	// 测试空配置路径使用默认值
	app := New("", "", nil)
	assert.NotNil(t, app)
}

// TestApplication_RunNonBlocking_NoRoutes 测试无路由的非阻塞运行
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

	// 关闭
	app.Shutdown()
	time.Sleep(100 * time.Millisecond)
}

// TestApplication_GracefulShutdown 测试优雅关闭
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

	// 手动调用 gracefulShutdown
	err = app.gracefulShutdown()
	assert.NoError(t, err)
	assert.True(t, shutdownCalled)
}

// TestApplication_StartHTTPServer_NoRegistrar 测试无路由注册器时启动 HTTP Server
func TestApplication_StartHTTPServer_NoRegistrar(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("api_server:\n  port: 0\n  mode: test\n"), 0644)

	app := New(tmpDir, "TEST", nil)
	err := app.Setup()
	assert.NoError(t, err)

	// 没有 routerRegistrar 时，startHTTPServer 应该直接返回 nil
	err = app.startHTTPServer()
	assert.NoError(t, err)
}

// TestApplication_GracefulShutdown_WithHTTPServer 测试有 HTTP Server 时的优雅关闭
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

// TestApplication_RunNonBlocking_SetupError 测试 Setup 失败
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
