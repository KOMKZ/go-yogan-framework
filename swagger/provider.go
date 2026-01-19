package swagger

import (
	"github.com/KOMKZ/go-yogan-framework/config"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/samber/do/v2"
)

// ProvideManager 创建 Swagger Manager 的 samber/do Provider
func ProvideManager(i do.Injector) (*Manager, error) {
	loader := do.MustInvoke[*config.Loader](i)
	log := do.MustInvoke[*logger.CtxZapLogger](i)

	// 读取配置
	var cfg Config
	if err := loader.GetViper().UnmarshalKey("swagger", &cfg); err != nil {
		// 配置不存在时使用默认值（禁用状态）
		cfg = DefaultConfig()
	}
	cfg.ApplyDefaults()

	// 读取 API 文档元信息
	var info SwaggerInfo
	if err := loader.GetViper().UnmarshalKey("swagger.info", &info); err != nil {
		info = DefaultSwaggerInfo()
	}

	// 从应用配置补充信息
	if info.Title == "" || info.Title == "API Documentation" {
		appName := loader.GetString("app.name")
		if appName != "" {
			info.Title = appName + " API"
		}
	}
	if info.Version == "" || info.Version == "1.0.0" {
		appVersion := loader.GetString("app.version")
		if appVersion != "" {
			info.Version = appVersion
		}
	}

	if !cfg.Enabled {
		log.Debug("Swagger is disabled")
		return nil, nil
	}

	return NewManager(cfg, info, log), nil
}
