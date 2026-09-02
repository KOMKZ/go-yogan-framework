package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewLoaderBuilder test creating builder
func TestNewLoaderBuilder(t *testing.T) {
	builder := NewLoaderBuilder()

	assert.NotNil(t, builder)
	assert.Equal(t, "grpc", builder.appType) // Default value
}

// TestLoaderBuilder_WithConfigPath test configuration path
func TestLoaderBuilder_WithConfigPath(t *testing.T) {
	builder := NewLoaderBuilder().WithConfigPath("/path/to/config")

	assert.Equal(t, "/path/to/config", builder.configPath)
}

// TestLoaderBuilder_WithEnvPrefix test with environment variable prefix set
func TestLoaderBuilder_WithEnvPrefix(t *testing.T) {
	builder := NewLoaderBuilder().WithEnvPrefix("MY_APP")

	assert.Equal(t, "MY_APP", builder.envPrefix)
}

// TestLoaderBuilder_WithAppType test to set application type
func TestLoaderBuilder_WithAppType(t *testing.T) {
	builder := NewLoaderBuilder().WithAppType("http")

	assert.Equal(t, "http", builder.appType)
}

// TestLoaderBuilder_WithFlags test with command line arguments
func TestLoaderBuilder_WithFlags(t *testing.T) {
	type TestFlags struct {
		Port int
	}
	flags := &TestFlags{Port: 8080}

	builder := NewLoaderBuilder().WithFlags(flags)

	assert.Equal(t, flags, builder.flags)
}

// TestLoaderBuilder_Build test loader construction
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

// TestLoaderBuilder_Build_WithEnvConfig test build with environment configuration
func TestLoaderBuilder_Build_WithEnvConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Create base configuration
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("app:\n  port: 8080\n"), 0644)

	// Create development environment configuration
	devFile := filepath.Join(tmpDir, "dev.yaml")
	os.WriteFile(devFile, []byte("app:\n  port: 9090\n"), 0644)

	// Set environment to dev
	os.Setenv("APP_ENV", "dev")
	defer os.Unsetenv("APP_ENV")

	loader, err := NewLoaderBuilder().
		WithConfigPath(tmpDir).
		Build()

	require.NoError(t, err)
	assert.Equal(t, 9090, loader.GetInt("app.port")) // dev.yaml override
}

func TestLoaderBuilder_Build_WithRateLimiterConfig(t *testing.T) {
	tmpDir := t.TempDir()

	configFile := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte("app:\n  name: test\n"), 0644))
	rateLimiterFile := filepath.Join(tmpDir, "rate_limiter.yaml")
	require.NoError(t, os.WriteFile(rateLimiterFile, []byte("limiter:\n  enabled: true\n  store_type: memory\n  key_func: path_ip\n"), 0644))

	loader, err := NewLoaderBuilder().
		WithConfigPath(tmpDir).
		Build()

	require.NoError(t, err)
	assert.True(t, loader.GetBool("limiter.enabled"))
	assert.Equal(t, "memory", loader.GetString("limiter.store_type"))
	assert.Equal(t, "path_ip", loader.GetString("limiter.key_func"))
}

func TestLoaderBuilder_Build_WithManifestImports(t *testing.T) {
	tmpDir := t.TempDir()
	writeConfigFile(t, filepath.Join(tmpDir, "config.yaml"), `
yogan:
  config:
    mode: manifest
    imports:
      - runtime.yaml
      - database.yaml
`)
	writeConfigFile(t, filepath.Join(tmpDir, "runtime.yaml"), `
api_server:
  port: 8080
logger:
  level: info
`)
	writeConfigFile(t, filepath.Join(tmpDir, "database.yaml"), `
database:
  driver: mysql
`)
	require.NoError(t, os.Mkdir(filepath.Join(tmpDir, "test"), 0755))
	writeConfigFile(t, filepath.Join(tmpDir, "test", "database.yaml"), `
database:
  driver: sqlite
`)
	writeConfigFile(t, filepath.Join(tmpDir, "test.yaml"), `
database:
  driver: should-not-load
`)

	loader, err := NewLoaderBuilder().
		WithConfigPath(tmpDir).
		WithEnv("test").
		Build()

	require.NoError(t, err)
	assert.Equal(t, 8080, loader.GetInt("api_server.port"))
	assert.Equal(t, "sqlite", loader.GetString("database.driver"))
	assert.Equal(t, "manifest", loader.GetString("yogan.config.mode"))
	assert.Contains(t, loader.GetLoadedFiles(), filepath.Join(tmpDir, "runtime.yaml"))
	assert.Contains(t, loader.GetLoadedFiles(), filepath.Join(tmpDir, "test", "database.yaml"))
}

func TestLoaderBuilder_Build_ManifestEnvAndFlagsOverrideFiles(t *testing.T) {
	tmpDir := t.TempDir()
	writeConfigFile(t, filepath.Join(tmpDir, "config.yaml"), `
yogan:
  config:
    mode: manifest
    imports:
      - runtime.yaml
`)
	writeConfigFile(t, filepath.Join(tmpDir, "runtime.yaml"), `
api_server:
  port: 8080
  host: 127.0.0.1
`)
	require.NoError(t, os.Mkdir(filepath.Join(tmpDir, "test"), 0755))
	writeConfigFile(t, filepath.Join(tmpDir, "test", "runtime.yaml"), `
api_server:
  port: 8081
`)

	os.Setenv("TEST_ADDRESS", "127.0.0.9")
	defer os.Unsetenv("TEST_ADDRESS")

	type TestFlags struct {
		Port int
	}

	loader, err := NewLoaderBuilder().
		WithConfigPath(tmpDir).
		WithEnv("test").
		WithEnvPrefix("TEST").
		WithAppType("http").
		WithFlags(&TestFlags{Port: 8082}).
		Build()

	require.NoError(t, err)
	assert.Equal(t, 8082, loader.GetInt("api_server.port"))
	assert.Equal(t, "127.0.0.9", loader.GetString("api_server.host"))
}

func TestLoaderBuilder_Build_ManifestDoesNotAutoLoadRateLimiter(t *testing.T) {
	tmpDir := t.TempDir()
	writeConfigFile(t, filepath.Join(tmpDir, "config.yaml"), `
yogan:
  config:
    mode: manifest
    imports:
      - runtime.yaml
`)
	writeConfigFile(t, filepath.Join(tmpDir, "runtime.yaml"), `
logger:
  level: info
`)
	writeConfigFile(t, filepath.Join(tmpDir, "rate_limiter.yaml"), `
limiter:
  enabled: true
`)

	loader, err := NewLoaderBuilder().
		WithConfigPath(tmpDir).
		Build()

	require.NoError(t, err)
	assert.False(t, loader.IsSet("limiter"))
}

// TestLoaderBuilder_Build_WithEnvSource test environment variable data source
func TestLoaderBuilder_Build_WithEnvSource(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("app:\n  port: 8080\n"), 0644)

	// Set environment variables
	os.Setenv("TEST_APP_PORT", "7777")
	defer os.Unsetenv("TEST_APP_PORT")

	loader, err := NewLoaderBuilder().
		WithConfigPath(tmpDir).
		WithEnvPrefix("TEST").
		Build()

	require.NoError(t, err)
	assert.NotNil(t, loader)
}

// TestLoaderBuilder_Build_WithFlags test command line arguments
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

// TestLoaderBuilder_Build_EnvPortMapping regression: {PREFIX}_PORT must map to
// the appType-aware config key (api_server.port for http) instead of a
// meaningless top-level "port" key from the generic prefix scan.
func TestLoaderBuilder_Build_EnvPortMapping(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("api_server:\n  port: 8080\n"), 0644)

	os.Setenv("TEST_PORT", "8081")
	os.Setenv("TEST_ADDRESS", "127.0.0.9")
	defer func() {
		os.Unsetenv("TEST_PORT")
		os.Unsetenv("TEST_ADDRESS")
	}()

	loader, err := NewLoaderBuilder().
		WithConfigPath(tmpDir).
		WithEnvPrefix("TEST").
		WithAppType("http").
		Build()

	require.NoError(t, err)
	assert.Equal(t, 8081, loader.GetInt("api_server.port"), "{PREFIX}_PORT must map to api_server.port")
	assert.Equal(t, "127.0.0.9", loader.GetString("api_server.host"))
	assert.Equal(t, 0, loader.GetInt("port"), "no stray top-level port key")
}

// TestLoaderBuilder_Build_EnvPortMappingGRPC same mapping for gRPC app type
func TestLoaderBuilder_Build_EnvPortMappingGRPC(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configFile, []byte("grpc:\n  server:\n    port: 9000\n"), 0644)

	os.Setenv("TEST_PORT", "9001")
	defer os.Unsetenv("TEST_PORT")

	loader, err := NewLoaderBuilder().
		WithConfigPath(tmpDir).
		WithEnvPrefix("TEST").
		WithAppType("grpc").
		Build()

	require.NoError(t, err)
	assert.Equal(t, 9001, loader.GetInt("grpc.server.port"))
}

// TestLoaderBuilder_Build_NoConfigPath test without configuration path
func TestLoaderBuilder_Build_NoConfigPath(t *testing.T) {
	loader, err := NewLoaderBuilder().Build()

	// No error should be reported if the path is not configured
	require.NoError(t, err)
	assert.NotNil(t, loader)
}

// TestGetEnv tests getting environment variables
func TestGetEnv(t *testing.T) {
	// Test the priority of APP_ENV
	os.Setenv("APP_ENV", "production")
	os.Setenv("ENV", "staging")
	defer func() {
		os.Unsetenv("APP_ENV")
		os.Unsetenv("ENV")
	}()

	env := GetEnv()
	assert.Equal(t, "production", env) // PREFER_APP_ENV

	// Test ENV priority
	os.Unsetenv("APP_ENV")
	env = GetEnv()
	assert.Equal(t, "staging", env) // Use ENV

	// Test default values
	os.Unsetenv("ENV")
	env = GetEnv()
	assert.Equal(t, "dev", env) // Default dev
}

// TestLoaderBuilder_ChainCall test chain call
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
