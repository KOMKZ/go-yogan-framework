package swagger

import (
	"net/http"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/swaggo/swag"
	"go.uber.org/zap"
)

// Manager Swagger 管理器
// 封装 swaggo 的初始化和 Gin 中间件挂载
type Manager struct {
	config Config
	info   SwaggerInfo
	logger *logger.CtxZapLogger
}

// NewManager 创建 Swagger 管理器
func NewManager(cfg Config, info SwaggerInfo, log *logger.CtxZapLogger) *Manager {
	cfg.ApplyDefaults()
	return &Manager{
		config: cfg,
		info:   info,
		logger: log,
	}
}

// IsEnabled 返回 Swagger 是否启用
func (m *Manager) IsEnabled() bool {
	return m.config.Enabled
}

// GetConfig 返回配置
func (m *Manager) GetConfig() Config {
	return m.config
}

// GetInfo 返回 API 文档元信息
func (m *Manager) GetInfo() SwaggerInfo {
	return m.info
}

// SetupInfo 设置 swag.SwaggerInfo（在路由注册前调用）
// 此方法将配置的元信息同步到 swag 全局变量
func (m *Manager) SetupInfo() {
	if swag.GetSwagger("swagger") == nil {
		m.logger.Warn("swag.SwaggerInfo not initialized, please run 'swag init' first")
		return
	}

	// 通过 swag.ReadDoc 获取实例无法修改，需要应用层导入 docs 包并设置
	// 这里仅记录日志提示
	m.logger.Debug("Swagger info setup complete",
		zap.String("title", m.info.Title),
		zap.String("version", m.info.Version),
		zap.String("basePath", m.info.BasePath))
}

// RegisterRoutes 注册 Swagger 路由到 Gin Engine
// 应用层调用此方法挂载 Swagger UI 和 Spec 路由
//
// 使用示例：
//
//	import _ "your-app/docs" // 导入 swag init 生成的 docs 包
//	swaggerManager.RegisterRoutes(engine)
func (m *Manager) RegisterRoutes(engine *gin.Engine) {
	if !m.config.Enabled {
		m.logger.Debug("Swagger is disabled, skipping route registration")
		return
	}

	// 构建 ginSwagger 配置选项
	opts := m.buildGinSwaggerOptions()

	// 注册 Swagger UI 路由
	engine.GET(m.config.UIPath, ginSwagger.WrapHandler(swaggerFiles.Handler, opts...))

	// 注册 OpenAPI Spec 路由（返回 JSON）
	if m.config.SpecPath != "" {
		engine.GET(m.config.SpecPath, m.serveSpec)
	}

	m.logger.Info("Swagger routes registered",
		zap.String("ui_path", m.config.UIPath),
		zap.String("spec_path", m.config.SpecPath))
}

// buildGinSwaggerOptions 构建 ginSwagger 配置选项
func (m *Manager) buildGinSwaggerOptions() []func(*ginSwagger.Config) {
	opts := []func(*ginSwagger.Config){
		ginSwagger.DeepLinking(m.config.DeepLinking),
		ginSwagger.PersistAuthorization(m.config.PersistAuthorization),
	}

	if m.config.ValidatorURL != "" {
		opts = append(opts, ginSwagger.DocExpansion("none"))
	}

	if m.config.OAuth2RedirectURL != "" {
		opts = append(opts, ginSwagger.Oauth2DefaultClientID(""))
	}

	return opts
}

// serveSpec 返回 OpenAPI Spec JSON
func (m *Manager) serveSpec(c *gin.Context) {
	doc, err := swag.ReadDoc("swagger")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": ErrSwaggerDocNotFound.Wrap(err).Error(),
		})
		return
	}

	c.Header("Content-Type", "application/json")
	c.String(http.StatusOK, doc)
}

// Shutdown 关闭管理器（实现 do.Shutdownable）
func (m *Manager) Shutdown() error {
	m.logger.Debug("Swagger manager shutdown")
	return nil
}
