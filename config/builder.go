package config

import (
	"os"
	"path/filepath"
)

// LoaderBuilder 配置加载器构建器
type LoaderBuilder struct {
	configPath string
	envPrefix  string
	appType    string      // grpc, http, mixed
	flags      interface{} // 命令行参数
}

// NewLoaderBuilder 创建加载器构建器
func NewLoaderBuilder() *LoaderBuilder {
	return &LoaderBuilder{
		appType: "grpc", // 默认
	}
}

// WithConfigPath 设置配置目录
func (b *LoaderBuilder) WithConfigPath(path string) *LoaderBuilder {
	b.configPath = path
	return b
}

// WithEnvPrefix 设置环境变量前缀
func (b *LoaderBuilder) WithEnvPrefix(prefix string) *LoaderBuilder {
	b.envPrefix = prefix
	return b
}

// WithAppType 设置应用类型
func (b *LoaderBuilder) WithAppType(appType string) *LoaderBuilder {
	b.appType = appType
	return b
}

// WithFlags 设置命令行参数
func (b *LoaderBuilder) WithFlags(flags interface{}) *LoaderBuilder {
	b.flags = flags
	return b
}

// Build 构建加载器
func (b *LoaderBuilder) Build() (*Loader, error) {
	loader := NewLoader()

	// 1. 基础配置文件（优先级 10）
	if b.configPath != "" {
		configFile := filepath.Join(b.configPath, "config.yaml")
		loader.AddSource(NewFileSource(configFile, 10))
	}

	// 2. 环境配置文件（优先级 20）
	if b.configPath != "" {
		env := GetEnv()
		if env != "" {
			envFile := filepath.Join(b.configPath, env+".yaml")
			loader.AddSource(NewFileSource(envFile, 20))
		}
	}

	// 3. 环境变量（优先级 50）
	if b.envPrefix != "" {
		loader.AddSource(NewEnvSource(b.envPrefix, 50))
	}

	// 4. 命令行参数（优先级 100）
	if b.flags != nil {
		loader.AddSource(NewFlagSource(b.flags, b.appType, 100))
	}

	// 加载所有数据源
	if err := loader.Load(); err != nil {
		return nil, err
	}

	return loader, nil
}

// GetEnv 获取环境变量（优先级：APP_ENV > ENV > 默认dev）
// 导出供其他包使用
func GetEnv() string {
	if env := os.Getenv("APP_ENV"); env != "" {
		return env
	}
	if env := os.Getenv("ENV"); env != "" {
		return env
	}
	return "dev" // 默认开发环境
}

