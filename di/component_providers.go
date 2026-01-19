package di

import (
	"github.com/KOMKZ/go-yogan-framework/config"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/samber/do/v2"
)

// ============================================
// 基础组件 Provider（Config, Logger）
// 这些是最底层的依赖，其他组件都依赖它们
// ============================================

// ConfigOptions 配置组件选项
type ConfigOptions struct {
	ConfigPath   string      // 配置目录路径
	ConfigPrefix string      // 环境变量前缀
	AppType      string      // 应用类型：grpc, http, mixed
	Flags        interface{} // 命令行参数
}

// ProvideConfigLoader 创建 config.Loader 的 Provider
// 这是最基础的组件，无依赖
func ProvideConfigLoader(opts ConfigOptions) func(do.Injector) (*config.Loader, error) {
	return func(i do.Injector) (*config.Loader, error) {
		if opts.ConfigPath == "" {
			opts.ConfigPath = "../configs"
		}
		if opts.AppType == "" {
			opts.AppType = "grpc"
		}

		loader, err := config.NewLoaderBuilder().
			WithConfigPath(opts.ConfigPath).
			WithEnvPrefix(opts.ConfigPrefix).
			WithAppType(opts.AppType).
			WithFlags(opts.Flags).
			Build()
		if err != nil {
			return nil, err
		}
		return loader, nil
	}
}

// ProvideLoggerManager 创建 logger.Manager 的 Provider
// 依赖：config.Loader（从配置读取 logger 配置）
func ProvideLoggerManager(i do.Injector) (*logger.Manager, error) {
	// 尝试从配置加载 logger 配置
	loader, err := do.Invoke[*config.Loader](i)
	if err != nil {
		// 无配置时使用默认配置
		return logger.NewManager(logger.DefaultManagerConfig()), nil
	}

	var loggerCfg logger.ManagerConfig
	if err := loader.GetViper().UnmarshalKey("logger", &loggerCfg); err != nil {
		// 解析失败使用默认配置
		return logger.NewManager(logger.DefaultManagerConfig()), nil
	}

	loggerCfg.ApplyDefaults()
	return logger.NewManager(loggerCfg), nil
}

// ProvideCtxLogger 创建命名 CtxZapLogger 的 Provider 工厂
// 用于应用层获取特定模块的 logger
func ProvideCtxLogger(moduleName string) func(do.Injector) (*logger.CtxZapLogger, error) {
	return func(i do.Injector) (*logger.CtxZapLogger, error) {
		mgr, err := do.Invoke[*logger.Manager](i)
		if err != nil {
			// 回退到全局 logger
			return logger.GetLogger(moduleName), nil
		}
		return mgr.GetLogger(moduleName), nil
	}
}
