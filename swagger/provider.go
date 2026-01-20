package swagger

import (
	"github.com/KOMKZ/go-yogan-framework/config"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/samber/do/v2"
)

// ProvideManager creates the Swagger Manager's samber/do provider
func ProvideManager(i do.Injector) (*Manager, error) {
	loader := do.MustInvoke[*config.Loader](i)
	log := do.MustInvoke[*logger.CtxZapLogger](i)

	// Read configuration
	var cfg Config
	if err := loader.GetViper().UnmarshalKey("swagger", &cfg); err != nil {
		// Use default values (disabled state) when configuration does not exist
		cfg = DefaultConfig()
	}
	cfg.ApplyDefaults()

	// Read API documentation metadata
	var info SwaggerInfo
	if err := loader.GetViper().UnmarshalKey("swagger.info", &info); err != nil {
		info = DefaultSwaggerInfo()
	}

	// Supplement information from application configuration
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
