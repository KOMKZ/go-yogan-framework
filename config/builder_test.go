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
