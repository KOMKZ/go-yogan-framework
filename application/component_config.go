package application

import (
	"context"
	"fmt"

	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/KOMKZ/go-yogan-framework/config"
)

// ConfigComponent 配置组件（支持多数据源）
type ConfigComponent struct {
	configPath   string         // 配置目录路径
	configPrefix string         // 环境变量前缀
	appType      string         // 应用类型：grpc, http, mixed
	flags        interface{}    // 命令行参数
	loader       *config.Loader // 配置加载器
	appConfig    *AppConfig     // 缓存已加载的 AppConfig
}

// NewConfigComponent 创建配置组件
func NewConfigComponent(configPath, configPrefix, appType string, flags interface{}) *ConfigComponent {
	if configPath == "" {
		configPath = "../configs" // 默认配置目录
	}

	if appType == "" {
		appType = "grpc" // 默认 gRPC
	}

	return &ConfigComponent{
		configPath:   configPath,
		configPrefix: configPrefix,
		appType:      appType,
		flags:        flags,
	}
}

// Name 组件名称
func (c *ConfigComponent) Name() string {
	return component.ComponentConfig
}

// DependsOn 配置组件无依赖
func (c *ConfigComponent) DependsOn() []string {
	return []string{}
}

// Init 初始化配置加载器
func (c *ConfigComponent) Init(ctx context.Context, loader component.ConfigLoader) error {
	// 使用加载器构建器
	var err error
	c.loader, err = config.NewLoaderBuilder().
		WithConfigPath(c.configPath).
		WithEnvPrefix(c.configPrefix).
		WithAppType(c.appType).
		WithFlags(c.flags).
		Build()

	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	// 立即加载并缓存 AppConfig
	var appCfg AppConfig
	if err := c.loader.Unmarshal(&appCfg); err != nil {
		return fmt.Errorf("加载 AppConfig 失败: %w", err)
	}
	c.appConfig = &appCfg

	return nil
}

// Start 启动配置热更新监听
func (c *ConfigComponent) Start(ctx context.Context) error {
	// 配置组件不需要启动任何服务
	return nil
}

// Stop 停止配置监听
func (c *ConfigComponent) Stop(ctx context.Context) error {
	// 配置组件无需停止操作
	return nil
}

// GetLoader 获取配置加载器
func (c *ConfigComponent) GetLoader() *config.Loader {
	return c.loader
}

// SetLoader 设置配置加载器（用于 DI 模式复用已创建的 Loader）
func (c *ConfigComponent) SetLoader(loader *config.Loader) {
	c.loader = loader
}

// GetAppConfig 获取缓存的 AppConfig
func (c *ConfigComponent) GetAppConfig() *AppConfig {
	return c.appConfig
}

// GetAppConfigInterface 获取 AppConfig 作为 interface{}
func (c *ConfigComponent) GetAppConfigInterface() interface{} {
	return c.appConfig
}

// 实现 component.ConfigLoader 接口
// 委托给 Loader

// Get 获取配置项
func (c *ConfigComponent) Get(key string) interface{} {
	return c.loader.Get(key)
}

// Unmarshal 将配置反序列化到结构体
func (c *ConfigComponent) Unmarshal(key string, v interface{}) error {
	if key == "" {
		return c.loader.Unmarshal(v)
	}
	return c.loader.GetViper().UnmarshalKey(key, v)
}

// GetString 获取字符串配置
func (c *ConfigComponent) GetString(key string) string {
	return c.loader.GetString(key)
}

// GetInt 获取整数配置
func (c *ConfigComponent) GetInt(key string) int {
	return c.loader.GetInt(key)
}

// GetBool 获取布尔配置
func (c *ConfigComponent) GetBool(key string) bool {
	return c.loader.GetBool(key)
}

// IsSet 检查配置项是否存在
func (c *ConfigComponent) IsSet(key string) bool {
	return c.loader.GetViper().IsSet(key)
}
