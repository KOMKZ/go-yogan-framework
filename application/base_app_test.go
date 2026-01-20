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

// TestNewBase 测试创建基础应用实例
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

// TestNewBaseWithDefaults 测试使用默认配置创建应用
func TestNewBaseWithDefaults(t *testing.T) {
	// 创建临时配置目录
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "configs", "test-app")
	err := os.MkdirAll(appDir, 0755)
	require.NoError(t, err)

	configFile := filepath.Join(appDir, "config.yaml")
	err = os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)
	require.NoError(t, err)

	// 切换工作目录
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	app := NewBaseWithDefaults("test-app", "http")
	assert.NotNil(t, app)
}

// TestBaseApplication_WithVersion 测试版本设置
func TestBaseApplication_WithVersion(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewBase(tmpDir, "TEST", "http", nil)
	app.WithVersion("v1.2.3")

	assert.Equal(t, "v1.2.3", app.GetVersion())
}

// TestBaseApplication_GetStartDuration 测试启动耗时
func TestBaseApplication_GetStartDuration(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewBase(tmpDir, "TEST", "http", nil)
	time.Sleep(10 * time.Millisecond)

	duration := app.GetStartDuration()
	assert.Greater(t, duration.Milliseconds(), int64(9))
}

// TestBaseApplication_Setup 测试 Setup 流程
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

// TestBaseApplication_Shutdown 测试 Shutdown 流程
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

// TestBaseApplication_Cancel 测试手动取消
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
		// 预期行为
	case <-time.After(1 * time.Second):
		t.Fatal("context should be done after cancel")
	}
}

// TestBaseApplication_Callbacks 测试回调注册
func TestBaseApplication_Callbacks(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewBase(tmpDir, "TEST", "http", nil)

	// 测试链式调用
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

// TestBaseApplication_LoadAppConfig 测试加载应用配置
func TestBaseApplication_LoadAppConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("api_server:\n  port: 9090\n"), 0644)

	app := NewBase(tmpDir, "TEST", "http", nil)

	appCfg, err := app.LoadAppConfig()
	assert.NoError(t, err)
	assert.NotNil(t, appCfg)
}

// TestAppState_String 测试状态字符串表示
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

// TestBaseApplication_WaitShutdown 测试等待关闭信号
func TestBaseApplication_WaitShutdown(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewBase(tmpDir, "TEST", "http", nil)
	err := app.Setup()
	require.NoError(t, err)

	// 在另一个 goroutine 中调用 Cancel 来触发 context 取消
	go func() {
		time.Sleep(50 * time.Millisecond)
		app.Cancel()
	}()

	// WaitShutdown 应该在 Cancel 后返回
	done := make(chan struct{})
	go func() {
		app.WaitShutdown()
		close(done)
	}()

	select {
	case <-done:
		// 预期行为
	case <-time.After(2 * time.Second):
		t.Fatal("WaitShutdown should complete after cancel")
	}
}

// TestBaseApplication_MustGetLogger_Panic 测试未初始化时获取日志 panic
func TestBaseApplication_MustGetLogger_Panic(t *testing.T) {
	// 创建一个未完全初始化的 app（跳过正常初始化流程）
	app := &BaseApplication{}

	assert.Panics(t, func() {
		app.MustGetLogger()
	})
}

// TestBaseApplication_GetConfigLoader_Panic 测试未初始化时获取配置加载器 panic
func TestBaseApplication_GetConfigLoader_Panic(t *testing.T) {
	// 创建一个未完全初始化的 app
	app := &BaseApplication{}

	assert.Panics(t, func() {
		app.GetConfigLoader()
	})
}

// TestBaseApplication_LoadAppConfig_NotInitialized 测试 AppConfig 未初始化
func TestBaseApplication_LoadAppConfig_NotInitialized(t *testing.T) {
	// 创建一个未完全初始化的 app
	app := &BaseApplication{}

	cfg, err := app.LoadAppConfig()
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "AppConfig 未初始化")
}

// TestBaseApplication_Setup_Error 测试 Setup 回调返回错误
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

// TestBaseApplication_Shutdown_CallsCallback 测试 Shutdown 调用回调
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

// TestBaseApplication_SetState 测试状态设置
func TestBaseApplication_SetState(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewBase(tmpDir, "TEST", "http", nil)

	// 初始状态是 Init
	assert.Equal(t, StateInit, app.GetState())

	// 设置为 Running
	app.setState(StateRunning)
	assert.Equal(t, StateRunning, app.GetState())

	// 设置为 Stopped
	app.setState(StateStopped)
	assert.Equal(t, StateStopped, app.GetState())
}
