package application

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewCron 测试创建 Cron 应用
func TestNewCron(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)
	require.NoError(t, err)

	app, err := NewCron(tmpDir, "TEST")

	assert.NoError(t, err)
	assert.NotNil(t, app)
	assert.NotNil(t, app.BaseApplication)
	assert.NotNil(t, app.GetScheduler())
}

// TestNewCronWithDefaults 测试使用默认配置创建 Cron 应用
func TestNewCronWithDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "configs", "cron-app")
	err := os.MkdirAll(appDir, 0755)
	require.NoError(t, err)

	configFile := filepath.Join(appDir, "config.yaml")
	err = os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)
	require.NoError(t, err)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	app, err := NewCronWithDefaults("cron-app")
	assert.NoError(t, err)
	assert.NotNil(t, app)
}

// TestCronApplication_Callbacks 测试回调注册
func TestCronApplication_Callbacks(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app, err := NewCron(tmpDir, "TEST")
	require.NoError(t, err)

	var setupCalled, readyCalled, shutdownCalled bool

	result := app.
		OnSetup(func(c *CronApplication) error {
			setupCalled = true
			return nil
		}).
		OnReady(func(c *CronApplication) error {
			readyCalled = true
			return nil
		}).
		OnShutdown(func(c *CronApplication) error {
			shutdownCalled = true
			return nil
		})

	assert.Equal(t, app, result)
	assert.NotNil(t, app.cronOnSetup)
	assert.NotNil(t, app.cronOnReady)
	assert.NotNil(t, app.cronOnShutdown)

	_ = setupCalled
	_ = readyCalled
	_ = shutdownCalled
}

// TestCronApplication_RunNonBlocking 测试非阻塞运行
func TestCronApplication_RunNonBlocking(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app, err := NewCron(tmpDir, "TEST")
	require.NoError(t, err)

	var setupCalled, readyCalled bool

	app.OnSetup(func(c *CronApplication) error {
		setupCalled = true
		return nil
	})
	app.OnReady(func(c *CronApplication) error {
		readyCalled = true
		return nil
	})

	err = app.RunNonBlocking()
	assert.NoError(t, err)
	assert.True(t, setupCalled)
	assert.True(t, readyCalled)
	assert.Equal(t, StateRunning, app.GetState())

	// 关闭应用
	app.Shutdown()
	time.Sleep(100 * time.Millisecond) // 等待关闭完成
}

// TestCronApplication_RegisterTask 测试注册任务
func TestCronApplication_RegisterTask(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app, err := NewCron(tmpDir, "TEST")
	require.NoError(t, err)

	// 注册一个简单的任务
	job, err := app.RegisterTask("*/5 * * * *", func() {
		// 测试任务
	})

	assert.NoError(t, err)
	assert.NotNil(t, job)
}

// mockTaskRegistrar 模拟任务注册器
type mockTaskRegistrar struct {
	registered bool
}

func (m *mockTaskRegistrar) RegisterTasks(app *CronApplication) error {
	m.registered = true
	return nil
}

// TestCronApplication_RegisterTasks 测试注册任务注册器
func TestCronApplication_RegisterTasks(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app, err := NewCron(tmpDir, "TEST")
	require.NoError(t, err)

	registrar := &mockTaskRegistrar{}
	result := app.RegisterTasks(registrar)

	assert.Equal(t, app, result)

	// 运行应用，验证注册器被调用
	err = app.RunNonBlocking()
	assert.NoError(t, err)
	assert.True(t, registrar.registered)

	app.Shutdown()
	time.Sleep(100 * time.Millisecond)
}

// TestCronApplication_Shutdown 测试手动关闭
func TestCronApplication_Shutdown(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app, err := NewCron(tmpDir, "TEST")
	require.NoError(t, err)

	err = app.RunNonBlocking()
	assert.NoError(t, err)

	ctx := app.Context()
	app.Shutdown()

	select {
	case <-ctx.Done():
		// 预期行为
	case <-time.After(1 * time.Second):
		t.Fatal("context should be done after shutdown")
	}
}

// TestCronApplication_GracefulShutdown 测试优雅关闭
func TestCronApplication_GracefulShutdown(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("cron:\n  shutdown_timeout: 5\n"), 0644)

	var shutdownCalled bool

	app, err := NewCron(tmpDir, "TEST")
	require.NoError(t, err)

	app.OnShutdown(func(c *CronApplication) error {
		shutdownCalled = true
		return nil
	})

	err = app.RunNonBlocking()
	assert.NoError(t, err)

	// 手动调用 gracefulShutdown
	err = app.gracefulShutdown()
	assert.NoError(t, err)
	assert.True(t, shutdownCalled)
	assert.Equal(t, StateStopped, app.GetState())
}

// TestCronApplication_Run 测试阻塞运行
func TestCronApplication_Run(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app, err := NewCron(tmpDir, "TEST")
	require.NoError(t, err)

	var readyCalled bool

	app.OnReady(func(c *CronApplication) error {
		readyCalled = true
		// 在 OnReady 中触发关闭
		go func() {
			time.Sleep(50 * time.Millisecond)
			c.Shutdown()
		}()
		return nil
	})

	// 在 goroutine 中运行以避免阻塞测试
	done := make(chan error, 1)
	go func() {
		done <- app.Run()
	}()

	select {
	case err := <-done:
		assert.NoError(t, err)
		assert.True(t, readyCalled)
	case <-time.After(2 * time.Second):
		t.Fatal("Run should complete after shutdown")
	}
}

// TestCronApplication_Run_SetupError 测试 Run 启动失败
func TestCronApplication_Run_SetupError(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app, err := NewCron(tmpDir, "TEST")
	require.NoError(t, err)

	app.OnSetup(func(c *CronApplication) error {
		return assert.AnError
	})

	err = app.Run()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "onSetup failed")
}

// TestNewCron_DefaultValues 测试默认值处理
func TestNewCron_DefaultValues(t *testing.T) {
	// 测试空配置路径使用默认值
	app, err := NewCron("", "")
	assert.NoError(t, err)
	assert.NotNil(t, app)
}

// TestCronApplication_GracefulShutdown_WithShutdownError 测试关闭时回调返回错误
func TestCronApplication_GracefulShutdown_WithShutdownError(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("cron:\n  shutdown_timeout: 1\n"), 0644)

	app, err := NewCron(tmpDir, "TEST")
	require.NoError(t, err)

	app.OnShutdown(func(c *CronApplication) error {
		return assert.AnError // 返回错误
	})

	err = app.RunNonBlocking()
	assert.NoError(t, err)

	// 调用 gracefulShutdown，即使回调返回错误也应该继续执行
	err = app.gracefulShutdown()
	assert.NoError(t, err) // Base shutdown 成功
}

// TestCronApplication_Run_OnReadyError 测试 OnReady 返回错误
func TestCronApplication_Run_OnReadyError(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app, err := NewCron(tmpDir, "TEST")
	require.NoError(t, err)

	app.OnReady(func(c *CronApplication) error {
		return assert.AnError
	})

	err = app.Run()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "onReady failed")
}

// TestCronApplication_Run_TaskRegistrarError 测试任务注册器返回错误
func TestCronApplication_Run_TaskRegistrarError(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app, err := NewCron(tmpDir, "TEST")
	require.NoError(t, err)

	registrar := &errorTaskRegistrar{}
	app.RegisterTasks(registrar)

	err = app.Run()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "register tasks failed")
}

// errorTaskRegistrar 模拟返回错误的任务注册器
type errorTaskRegistrar struct{}

func (m *errorTaskRegistrar) RegisterTasks(app *CronApplication) error {
	return assert.AnError
}
