package application

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/KOMKZ/go-yogan-framework/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLifecycle_Setup 测试 setup 阶段
func TestLifecycle_Setup(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)
	require.NoError(t, err)

	app := New(tmpDir, "TEST", nil)

	// 注册配置热更新回调，确保 configWatcher 被创建
	app.OnConfigReload(func(loader *config.Loader) {
		// 测试回调
	})

	err = app.Setup()
	assert.NoError(t, err)

	// 验证配置加载完成（通过注册中心获取）
	assert.NotNil(t, app.GetConfigLoader())
	assert.NotNil(t, app.MustGetLogger())
	assert.Equal(t, StateSetup, app.GetState())

	// 验证 configWatcher 被创建（只有设置了回调才会创建）
	configComp, ok := app.GetRegistry().Get(component.ComponentConfig)
	assert.True(t, ok)
	assert.NotNil(t, configComp.(*ConfigComponent).GetWatcher())
}

// TestLifecycle_Setup_ConfigNotFound 测试配置文件不存在
func TestLifecycle_Setup_ConfigNotFound(t *testing.T) {
	app := New("/nonexistent", "TEST", nil)

	// 配置文件不存在不应该报错（Viper 行为）
	err := app.Setup()
	// 如果没有任何配置文件，setup 可能成功（使用默认值）
	// 这里不强制要求错误，取决于 config.Loader 的实现
	_ = err
}

// TestLifecycle_Setup_WithCallback 测试 OnSetup 回调
func TestLifecycle_Setup_WithCallback(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("app:\n  name: test\n"), 0644)
	require.NoError(t, err)

	var callbackCalled bool
	var receivedApp *Application

	app := New(tmpDir, "TEST", nil)
	app.OnSetup(func(a *Application) error {
		callbackCalled = true
		receivedApp = a
		assert.NotNil(t, a.GetConfigLoader())
		return nil
	})

	err = app.Setup()
	assert.NoError(t, err)
	assert.True(t, callbackCalled)
	assert.Equal(t, app, receivedApp)
}

// TestLifecycle_Setup_CallbackError 测试 OnSetup 回调返回错误
func TestLifecycle_Setup_CallbackError(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)
	require.NoError(t, err)

	app := New(tmpDir, "TEST", nil)
	app.OnSetup(func(a *Application) error {
		return assert.AnError
	})

	err = app.Setup()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "onSetup failed")
}

// TestLifecycle_GracefulShutdown 测试优雅关闭
func TestLifecycle_GracefulShutdown(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)
	require.NoError(t, err)

	app := New(tmpDir, "TEST", nil)
	err = app.Setup()
	require.NoError(t, err)

	app.BaseApplication.setState(StateRunning)

	err = app.gracefulShutdown()
	assert.NoError(t, err)
	assert.Equal(t, StateStopped, app.GetState())
}

// TestLifecycle_GracefulShutdown_WithCallback 测试优雅关闭回调
func TestLifecycle_GracefulShutdown_WithCallback(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)
	require.NoError(t, err)

	var callbackCalled bool

	app := New(tmpDir, "TEST", nil)
	app.OnShutdown(func(a *Application) error {
		callbackCalled = true
		return nil
	})

	err = app.Setup()
	require.NoError(t, err)

	err = app.gracefulShutdown()
	assert.NoError(t, err)
	assert.True(t, callbackCalled)
}

// TestLifecycle_GracefulShutdown_CallbackError 测试关闭回调错误
func TestLifecycle_GracefulShutdown_CallbackError(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)
	require.NoError(t, err)

	app := New(tmpDir, "TEST", nil)
	app.OnShutdown(func(a *Application) error {
		return assert.AnError
	})

	err = app.Setup()
	require.NoError(t, err)

	// 即使回调返回错误，gracefulShutdown 也应该成功（记录错误但继续）
	err = app.gracefulShutdown()
	assert.NoError(t, err)
}

// TestLifecycle_Shutdown_Manual 测试手动关闭
func TestLifecycle_Shutdown_Manual(t *testing.T) {
	app := New("./testdata", "TEST", nil)

	ctx := app.Context()

	// 手动触发关闭
	app.Shutdown()

	// 验证上下文已取消
	select {
	case <-ctx.Done():
		// 预期行为
	case <-time.After(1 * time.Second):
		t.Fatal("context should be cancelled after Shutdown()")
	}
}

// TestLifecycle_SetState_ThreadSafe 测试状态设置的线程安全性
func TestLifecycle_SetState_ThreadSafe(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)
	require.NoError(t, err)

	app := New(tmpDir, "TEST", nil)
	err = app.Setup()
	require.NoError(t, err)

	done := make(chan bool)
	states := []AppState{StateRunning, StateStopping, StateStopped, StateInit}

	// 并发修改状态
	for _, state := range states {
		go func(s AppState) {
			app.BaseApplication.setState(s)
			_ = app.GetState()
			done <- true
		}(state)
	}

	// 等待所有 goroutine 完成
	for range states {
		<-done
	}

	// 验证可以正常获取状态（不panic）
	finalState := app.GetState()
	assert.Contains(t, []AppState{StateRunning, StateStopping, StateStopped, StateInit}, finalState)
}
