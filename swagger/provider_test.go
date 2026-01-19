package swagger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/KOMKZ/go-yogan-framework/config"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvideManager_Disabled(t *testing.T) {
	// 创建临时配置目录
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// 写入禁用 Swagger 的配置
	configContent := `
swagger:
  enabled: false
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// 创建 Config Loader
	loader, err := config.NewLoaderBuilder().
		WithConfigPath(tmpDir).
		WithAppType("http").
		Build()
	require.NoError(t, err)

	// 创建 DI 容器
	injector := do.New()
	do.ProvideValue(injector, loader)
	do.ProvideValue(injector, logger.GetLogger("test"))

	// 测试 Provider
	mgr, err := ProvideManager(injector)
	assert.NoError(t, err)
	assert.Nil(t, mgr, "禁用时应返回 nil")
}

func TestProvideManager_Enabled(t *testing.T) {
	// 创建临时配置目录
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// 写入启用 Swagger 的配置
	configContent := `
swagger:
  enabled: true
  ui_path: "/docs/*any"
  spec_path: "/api-spec.json"
  info:
    title: "Test API"
    version: "2.0.0"
    base_path: "/api/v2"
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// 创建 Config Loader
	loader, err := config.NewLoaderBuilder().
		WithConfigPath(tmpDir).
		WithAppType("http").
		Build()
	require.NoError(t, err)

	// 创建 DI 容器
	injector := do.New()
	do.ProvideValue(injector, loader)
	do.ProvideValue(injector, logger.GetLogger("test"))

	// 测试 Provider
	mgr, err := ProvideManager(injector)
	assert.NoError(t, err)
	require.NotNil(t, mgr)
	assert.True(t, mgr.IsEnabled())
	assert.Equal(t, "/docs/*any", mgr.GetConfig().UIPath)
	assert.Equal(t, "/api-spec.json", mgr.GetConfig().SpecPath)
}

func TestProvideManager_NoConfig(t *testing.T) {
	// 创建临时配置目录（没有 swagger 配置）
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// 写入空配置
	configContent := `
app:
  name: "TestApp"
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// 创建 Config Loader
	loader, err := config.NewLoaderBuilder().
		WithConfigPath(tmpDir).
		WithAppType("http").
		Build()
	require.NoError(t, err)

	// 创建 DI 容器
	injector := do.New()
	do.ProvideValue(injector, loader)
	do.ProvideValue(injector, logger.GetLogger("test"))

	// 测试 Provider（无配置时使用默认值，默认禁用）
	mgr, err := ProvideManager(injector)
	assert.NoError(t, err)
	assert.Nil(t, mgr, "无配置时默认禁用，应返回 nil")
}

func TestProvideManager_InheritAppInfo(t *testing.T) {
	// 创建临时配置目录
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// 写入配置（Swagger 启用但 info 使用默认值，应从 app 继承）
	configContent := `
app:
  name: "MyApp"
  version: "3.0.0"
swagger:
  enabled: true
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// 创建 Config Loader
	loader, err := config.NewLoaderBuilder().
		WithConfigPath(tmpDir).
		WithAppType("http").
		Build()
	require.NoError(t, err)

	// 创建 DI 容器
	injector := do.New()
	do.ProvideValue(injector, loader)
	do.ProvideValue(injector, logger.GetLogger("test"))

	// 测试 Provider
	mgr, err := ProvideManager(injector)
	assert.NoError(t, err)
	require.NotNil(t, mgr)

	// 应从 app 配置继承标题和版本
	assert.Equal(t, "MyApp API", mgr.GetInfo().Title)
	assert.Equal(t, "3.0.0", mgr.GetInfo().Version)
}
