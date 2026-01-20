package application

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewGRPC 测试创建 gRPC 应用
func TestNewGRPC(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)
	require.NoError(t, err)

	app := NewGRPC(tmpDir, "TEST", nil)

	assert.NotNil(t, app)
	assert.NotNil(t, app.BaseApplication)
}

// TestNewGRPC_DefaultValues 测试默认值处理
func TestNewGRPC_DefaultValues(t *testing.T) {
	// 测试空配置路径使用默认值
	app := NewGRPC("", "", nil)
	assert.NotNil(t, app)
}

// TestNewGRPCWithDefaults 测试使用默认配置创建 gRPC 应用
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

// TestNewGRPCWithFlags 测试使用 Flags 创建 gRPC 应用
func TestNewGRPCWithFlags(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	flags := &AppFlags{Port: 9090}
	app := NewGRPCWithFlags(tmpDir, "TEST", flags)

	assert.NotNil(t, app)
}

// TestGRPCApplication_Callbacks 测试回调注册
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

// TestGRPCApplication_SetGovernanceManager 测试设置服务治理管理器
func TestGRPCApplication_SetGovernanceManager(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewGRPC(tmpDir, "TEST", nil)

	// SetGovernanceManager 接受 nil 也可以
	result := app.SetGovernanceManager(nil)
	assert.Equal(t, app, result)
	assert.Nil(t, app.governanceManager)
}

// TestGRPCApplication_Run 测试阻塞运行
func TestGRPCApplication_Run(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewGRPC(tmpDir, "TEST", nil)

	var readyCalled bool

	app.OnReady(func(g *GRPCApplication) error {
		readyCalled = true
		// 在 OnReady 中触发关闭
		go func() {
			time.Sleep(50 * time.Millisecond)
			g.Cancel()
		}()
		return nil
	})

	// 在 goroutine 中运行以避免阻塞测试
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

// TestGRPCApplication_GracefulShutdown 测试优雅关闭
func TestGRPCApplication_GracefulShutdown(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewGRPC(tmpDir, "TEST", nil)

	// 先 Setup
	err := app.Setup()
	require.NoError(t, err)

	// 测试 gracefulShutdown
	err = app.gracefulShutdown()
	assert.NoError(t, err)
}

// TestGRPCApplication_AutoDeregisterService_NilManager 测试自动注销服务（无管理器）
func TestGRPCApplication_AutoDeregisterService_NilManager(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewGRPC(tmpDir, "TEST", nil)

	// governanceManager 为 nil 时，autoDeregisterService 应返回 nil
	err := app.autoDeregisterService()
	assert.NoError(t, err)
}

// TestGRPCApplication_AutoRegisterService 测试自动注册服务
func TestGRPCApplication_AutoRegisterService(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("server:\n  port: 8080\n"), 0644)

	app := NewGRPC(tmpDir, "TEST", nil)
	err := app.Setup()
	require.NoError(t, err)

	// autoRegisterService 目前只记录日志，不会报错
	err = app.autoRegisterService()
	assert.NoError(t, err)
}

// TestGRPCApplication_Run_SetupError 测试 Run 启动失败
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

// TestGRPCApplication_Run_ReadyError 测试 OnReady 失败
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
