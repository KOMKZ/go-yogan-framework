package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewLoaderBuilder 测试创建构建器
func TestNewLoaderBuilder(t *testing.T) {
	builder := NewLoaderBuilder()

	assert.NotNil(t, builder)
	assert.Equal(t, "grpc", builder.appType) // 默认值
}

// TestLoaderBuilder_WithConfigPath 测试设置配置路径
func TestLoaderBuilder_WithConfigPath(t *testing.T) {
	builder := NewLoaderBuilder().WithConfigPath("/path/to/config")

	assert.Equal(t, "/path/to/config", builder.configPath)
}

// TestLoaderBuilder_WithEnvPrefix 测试设置环境变量前缀
func TestLoaderBuilder_WithEnvPrefix(t *testing.T) {
	builder := NewLoaderBuilder().WithEnvPrefix("MY_APP")

	assert.Equal(t, "MY_APP", builder.envPrefix)
}

// TestLoaderBuilder_WithAppType 测试设置应用类型
func TestLoaderBuilder_WithAppType(t *testing.T) {
	builder := NewLoaderBuilder().WithAppType("http")

	assert.Equal(t, "http", builder.appType)
}

// TestLoaderBuilder_WithFlags 测试设置命令行参数
func TestLoaderBuilder_WithFlags(t *testing.T) {
	type TestFlags struct {
		Port int
	}
	flags := &TestFlags{Port: 8080}

	builder := NewLoaderBuilder().WithFlags(flags)

	assert.Equal(t, flags, builder.flags)
}

// TestLoaderBuilder_Build 测试构建加载器
func TestLoaderBuilder_Build(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("app:\n  name: test\n"), 0644)

	loader, err := NewLoaderBuilder().
		WithConfigPath(tmpDir).
		Build()

	require.NoError(t, err)
	assert.NotNil(t, loader)
	assert.Equal(t, "test", loader.GetString("app.name"))
}

// TestLoaderBuilder_Build_WithEnvConfig 测试使用环境配置构建
func TestLoaderBuilder_Build_WithEnvConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建基础配置
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("app:\n  port: 8080\n"), 0644)

	// 创建 dev 环境配置
	devFile := filepath.Join(tmpDir, "dev.yaml")
	os.WriteFile(devFile, []byte("app:\n  port: 9090\n"), 0644)

	// 设置环境为 dev
	os.Setenv("APP_ENV", "dev")
	defer os.Unsetenv("APP_ENV")

	loader, err := NewLoaderBuilder().
		WithConfigPath(tmpDir).
		Build()

	require.NoError(t, err)
	assert.Equal(t, 9090, loader.GetInt("app.port")) // dev.yaml 覆盖
}

// TestLoaderBuilder_Build_WithEnvSource 测试环境变量数据源
func TestLoaderBuilder_Build_WithEnvSource(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("app:\n  port: 8080\n"), 0644)

	// 设置环境变量
	os.Setenv("TEST_APP_PORT", "7777")
	defer os.Unsetenv("TEST_APP_PORT")

	loader, err := NewLoaderBuilder().
		WithConfigPath(tmpDir).
		WithEnvPrefix("TEST").
		Build()

	require.NoError(t, err)
	assert.NotNil(t, loader)
}

// TestLoaderBuilder_Build_WithFlags 测试命令行参数
func TestLoaderBuilder_Build_WithFlags(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("grpc:\n  server:\n    port: 8080\n"), 0644)

	type TestFlags struct {
		Port int
	}

	loader, err := NewLoaderBuilder().
		WithConfigPath(tmpDir).
		WithAppType("grpc").
		WithFlags(&TestFlags{Port: 9999}).
		Build()

	require.NoError(t, err)
	assert.Equal(t, 9999, loader.GetInt("grpc.server.port"))
}

// TestLoaderBuilder_Build_NoConfigPath 测试无配置路径
func TestLoaderBuilder_Build_NoConfigPath(t *testing.T) {
	loader, err := NewLoaderBuilder().Build()

	// 没有配置路径也不应该报错
	require.NoError(t, err)
	assert.NotNil(t, loader)
}

// TestGetEnv 测试获取环境变量
func TestGetEnv(t *testing.T) {
	// 测试 APP_ENV 优先级
	os.Setenv("APP_ENV", "production")
	os.Setenv("ENV", "staging")
	defer func() {
		os.Unsetenv("APP_ENV")
		os.Unsetenv("ENV")
	}()

	env := GetEnv()
	assert.Equal(t, "production", env) // APP_ENV 优先

	// 测试 ENV 优先级
	os.Unsetenv("APP_ENV")
	env = GetEnv()
	assert.Equal(t, "staging", env) // 使用 ENV

	// 测试默认值
	os.Unsetenv("ENV")
	env = GetEnv()
	assert.Equal(t, "dev", env) // 默认 dev
}

// TestLoaderBuilder_ChainCall 测试链式调用
func TestLoaderBuilder_ChainCall(t *testing.T) {
	type TestFlags struct {
		Port int
	}

	builder := NewLoaderBuilder().
		WithConfigPath("/path").
		WithEnvPrefix("APP").
		WithAppType("http").
		WithFlags(&TestFlags{Port: 8080})

	assert.Equal(t, "/path", builder.configPath)
	assert.Equal(t, "APP", builder.envPrefix)
	assert.Equal(t, "http", builder.appType)
	assert.NotNil(t, builder.flags)
}
