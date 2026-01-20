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
	// Create temporary configuration directory
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Write the configuration to disable Swagger
	configContent := `
swagger:
  enabled: false
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// Create Config Loader
	loader, err := config.NewLoaderBuilder().
		WithConfigPath(tmpDir).
		WithAppType("http").
		Build()
	require.NoError(t, err)

	// Create DI container
	injector := do.New()
	do.ProvideValue(injector, loader)
	do.ProvideValue(injector, logger.GetLogger("test"))

	// Test Provider
	mgr, err := ProvideManager(injector)
	assert.NoError(t, err)
	assert.Nil(t, mgr, "禁用时应返回 nil")
}

func TestProvideManager_Enabled(t *testing.T) {
	// Create temporary configuration directory
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Write the configuration to enable Swagger
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

	// Create Config Loader
	loader, err := config.NewLoaderBuilder().
		WithConfigPath(tmpDir).
		WithAppType("http").
		Build()
	require.NoError(t, err)

	// Create DI container
	injector := do.New()
	do.ProvideValue(injector, loader)
	do.ProvideValue(injector, logger.GetLogger("test"))

	// Test Provider
	mgr, err := ProvideManager(injector)
	assert.NoError(t, err)
	require.NotNil(t, mgr)
	assert.True(t, mgr.IsEnabled())
	assert.Equal(t, "/docs/*any", mgr.GetConfig().UIPath)
	assert.Equal(t, "/api-spec.json", mgr.GetConfig().SpecPath)
}

func TestProvideManager_NoConfig(t *testing.T) {
	// Create a temporary configuration directory (without Swagger configuration)
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Write empty configuration
	configContent := `
app:
  name: "TestApp"
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// Create Config Loader
	loader, err := config.NewLoaderBuilder().
		WithConfigPath(tmpDir).
		WithAppType("http").
		Build()
	require.NoError(t, err)

	// Create DI container
	injector := do.New()
	do.ProvideValue(injector, loader)
	do.ProvideValue(injector, logger.GetLogger("test"))

	// Test Provider (uses default values when no configuration is provided, default is disabled)
	mgr, err := ProvideManager(injector)
	assert.NoError(t, err)
	assert.Nil(t, mgr, "无配置时默认禁用，应返回 nil")
}

func TestProvideManager_InheritAppInfo(t *testing.T) {
	// Create temporary configuration directory
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Write configuration ( Swagger enabled but info uses default values, should inherit from app)
	configContent := `
app:
  name: "MyApp"
  version: "3.0.0"
swagger:
  enabled: true
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// Create Config Loader
	loader, err := config.NewLoaderBuilder().
		WithConfigPath(tmpDir).
		WithAppType("http").
		Build()
	require.NoError(t, err)

	// Create DI container
	injector := do.New()
	do.ProvideValue(injector, loader)
	do.ProvideValue(injector, logger.GetLogger("test"))

	// Test Provider
	mgr, err := ProvideManager(injector)
	assert.NoError(t, err)
	require.NotNil(t, mgr)

	// The title and version should be inherited from the app configuration
	assert.Equal(t, "MyApp API", mgr.GetInfo().Title)
	assert.Equal(t, "3.0.0", mgr.GetInfo().Version)
}
