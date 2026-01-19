package config

import (
	"fmt"

	"github.com/samber/do/v2"
)

// ProvideLoaderOptions 创建 Loader 的选项
type ProvideLoaderOptions struct {
	ConfigPath   string      // 配置目录路径
	ConfigPrefix string      // 环境变量前缀
	AppType      string      // 应用类型：grpc, http, mixed
	Flags        interface{} // 命令行参数
}

// ProvideLoader 创建 Config Loader Provider
// Config 是最底层组件，无任何依赖
//
// 使用示例：
//
//	do.Provide(injector, config.ProvideLoader(config.ProvideLoaderOptions{
//	    ConfigPath: "../configs/admin-api",
//	    ConfigPrefix: "ADMIN_API",
//	    AppType: "http",
//	}))
//	loader := do.MustInvoke[*config.Loader](injector)
func ProvideLoader(opts ProvideLoaderOptions) func(do.Injector) (*Loader, error) {
	return func(i do.Injector) (*Loader, error) {
		if opts.ConfigPath == "" {
			opts.ConfigPath = "../configs"
		}
		if opts.AppType == "" {
			opts.AppType = "http"
		}

		loader, err := NewLoaderBuilder().
			WithConfigPath(opts.ConfigPath).
			WithEnvPrefix(opts.ConfigPrefix).
			WithAppType(opts.AppType).
			WithFlags(opts.Flags).
			Build()

		if err != nil {
			return nil, fmt.Errorf("config loader build failed: %w", err)
		}

		return loader, nil
	}
}

// ProvideLoaderValue 直接注册已创建的 Loader（用于测试或特殊场景）
//
// 使用示例：
//
//	loader, _ := config.NewLoaderBuilder().Build()
//	do.ProvideValue(injector, loader)
func ProvideLoaderValue(loader *Loader) func(do.Injector) (*Loader, error) {
	return func(i do.Injector) (*Loader, error) {
		return loader, nil
	}
}
