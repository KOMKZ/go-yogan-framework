package config

import (
	"os"
	"path/filepath"
)

// LoaderBuilder configuration loader builder
type LoaderBuilder struct {
	configPath string
	envPrefix  string
	appType    string      // grpc, http, mixed
	flags      interface{} // command line arguments
}

// NewLoaderBuilder creates a loader builder
func NewLoaderBuilder() *LoaderBuilder {
	return &LoaderBuilder{
		appType: "grpc", // DefaultValueHandling.DEFAULT
	}
}

// WithConfigPath set configuration directory
func (b *LoaderBuilder) WithConfigPath(path string) *LoaderBuilder {
	b.configPath = path
	return b
}

// Set environment variable prefix
func (b *LoaderBuilder) WithEnvPrefix(prefix string) *LoaderBuilder {
	b.envPrefix = prefix
	return b
}

// Set application type
func (b *LoaderBuilder) WithAppType(appType string) *LoaderBuilder {
	b.appType = appType
	return b
}

// WithFlags set command line arguments
func (b *LoaderBuilder) WithFlags(flags interface{}) *LoaderBuilder {
	b.flags = flags
	return b
}

// Build loader
func (b *LoaderBuilder) Build() (*Loader, error) {
	loader := NewLoader()

	// 1. Basic configuration file (priority 10)
	if b.configPath != "" {
		configFile := filepath.Join(b.configPath, "config.yaml")
		loader.AddSource(NewFileSource(configFile, 10))
	}

	// Environment configuration file (priority 20)
	if b.configPath != "" {
		env := GetEnv()
		if env != "" {
			envFile := filepath.Join(b.configPath, env+".yaml")
			loader.AddSource(NewFileSource(envFile, 20))
		}
	}

	// 3. Environment variables (priority 50)
	if b.envPrefix != "" {
		loader.AddSource(NewEnvSource(b.envPrefix, 50))
	}

	// 4. Command line arguments (priority 100)
	if b.flags != nil {
		loader.AddSource(NewFlagSource(b.flags, b.appType, 100))
	}

	// Load all data sources
	if err := loader.Load(); err != nil {
		return nil, err
	}

	return loader, nil
}

// GetEnv retrieves environment variables (priority: APP_ENV > ENV > default dev)
// Exported for use by other packages
func GetEnv() string {
	if env := os.Getenv("APP_ENV"); env != "" {
		return env
	}
	if env := os.Getenv("ENV"); env != "" {
		return env
	}
	return "dev" // Default development environment
}

