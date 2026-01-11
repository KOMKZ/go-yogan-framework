package application

import (
	"fmt"

	"github.com/KOMKZ/go-yogan-framework/httpx"
	"github.com/KOMKZ/go-yogan-framework/logger"
)

// AppConfig 框架配置（只包含框架层配置）
//
// 注意：不再包含业务配置（如 Database, Redis）
// 业务组件应该自己从 ConfigLoader 读取配置
type AppConfig struct {
	// 必选配置 - 值类型（应用必须配置）
	ApiServer ApiServerConfig `mapstructure:"api_server"`

	// 可选配置 - 指针（有默认值或可不配置）
	Logger     *logger.ManagerConfig       `mapstructure:"logger,omitempty"`
	Middleware *MiddlewareConfig           `mapstructure:"middleware,omitempty"` // 中间件配置
	Httpx      *httpx.ErrorLoggingConfig   `mapstructure:"httpx,omitempty"`      // HTTP 错误处理配置
}

// ApiServerConfig HTTP API 服务器配置
type ApiServerConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Mode         string `mapstructure:"mode"` // debug, release, test
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
}

// MiddlewareConfig 中间件配置
type MiddlewareConfig struct {
	CORS       *CORSConfig       `mapstructure:"cors,omitempty"`
	TraceID    *TraceIDConfig    `mapstructure:"trace_id,omitempty"`
	RequestLog *RequestLogConfig `mapstructure:"request_log,omitempty"`
	RateLimit  *RateLimitConfig  `mapstructure:"rate_limit,omitempty"`
}

// TraceIDConfig TraceID 中间件配置
type TraceIDConfig struct {
	// Enable 是否启用 TraceID 中间件（默认 true）
	Enable bool `mapstructure:"enable"`

	// TraceIDKey Context 中存储的 Key（默认 "trace_id"）
	TraceIDKey string `mapstructure:"trace_id_key"`

	// TraceIDHeader HTTP Header 中的 Key（默认 "X-Trace-ID"）
	TraceIDHeader string `mapstructure:"trace_id_header"`

	// EnableResponseHeader 是否将 TraceID 写入 Response Header（默认 true）
	EnableResponseHeader bool `mapstructure:"enable_response_header"`
}

// RequestLogConfig HTTP 请求日志中间件配置
type RequestLogConfig struct {
	// Enable 是否启用请求日志中间件（默认 true）
	Enable bool `mapstructure:"enable"`

	// SkipPaths 跳过记录的路径列表（例如健康检查）
	SkipPaths []string `mapstructure:"skip_paths"`

	// EnableBody 是否记录请求体（默认 false，性能考虑）
	EnableBody bool `mapstructure:"enable_body"`

	// MaxBodySize 最大请求体记录大小（字节，默认 4KB）
	MaxBodySize int `mapstructure:"max_body_size"`
}

// CORSConfig CORS 跨域中间件配置
type CORSConfig struct {
	// Enable 是否启用 CORS 中间件（默认 false）
	Enable bool `mapstructure:"enable"`

	// AllowOrigins 允许的源列表（默认 ["*"]）
	// 示例：["https://example.com", "https://app.example.com"]
	AllowOrigins []string `mapstructure:"allow_origins"`

	// AllowMethods 允许的 HTTP 方法列表（默认 ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"]）
	AllowMethods []string `mapstructure:"allow_methods"`

	// AllowHeaders 允许的请求头列表（默认 ["Origin", "Content-Type", "Accept", "Authorization"]）
	AllowHeaders []string `mapstructure:"allow_headers"`

	// ExposeHeaders 暴露给客户端的响应头列表（默认 []）
	ExposeHeaders []string `mapstructure:"expose_headers"`

	// AllowCredentials 是否允许发送凭证（Cookie、HTTP认证等）（默认 false）
	// 注意：当为 true 时，AllowOrigins 不能使用 "*"
	AllowCredentials bool `mapstructure:"allow_credentials"`

	// MaxAge 预检请求缓存时间（秒）（默认 43200，即12小时）
	MaxAge int `mapstructure:"max_age"`
}

// RateLimitConfig 限流中间件配置
type RateLimitConfig struct {
	// Enable 是否启用限流中间件（默认 false）
	Enable bool `mapstructure:"enable"`

	// KeyFunc 资源键生成方式（默认 "path"）
	// 可选值：path, ip, user, path_ip, api_key
	KeyFunc string `mapstructure:"key_func"`

	// SkipPaths 跳过限流的路径列表（例如健康检查）
	SkipPaths []string `mapstructure:"skip_paths"`
}

// ApplyDefaults 应用默认值
func (c *MiddlewareConfig) ApplyDefaults() {
	if c == nil {
		return
	}

	// CORS 默认值
	if c.CORS != nil {
		if len(c.CORS.AllowOrigins) == 0 {
			c.CORS.AllowOrigins = []string{"*"}
		}
		if len(c.CORS.AllowMethods) == 0 {
			c.CORS.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
		}
		if len(c.CORS.AllowHeaders) == 0 {
			c.CORS.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
		}
		if c.CORS.MaxAge == 0 {
			c.CORS.MaxAge = 43200 // 12小时
		}
	}

	// TraceID 默认值
	if c.TraceID != nil {
		if c.TraceID.TraceIDKey == "" {
			c.TraceID.TraceIDKey = "trace_id"
		}
		if c.TraceID.TraceIDHeader == "" {
			c.TraceID.TraceIDHeader = "X-Trace-ID"
		}
	}

	// RequestLog 默认值
	if c.RequestLog != nil {
		if c.RequestLog.MaxBodySize == 0 {
			c.RequestLog.MaxBodySize = 4096 // 4KB
		}
	}
}

// LoadAppConfig 加载框架配置
func (a *Application) LoadAppConfig() (*AppConfig, error) {
	// 从注册中心获取 ConfigLoader
	loader := a.GetConfigLoader()
	if loader == nil {
		return nil, fmt.Errorf("配置加载器未初始化")
	}

	var cfg AppConfig
	if err := loader.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// 应用中间件配置默认值
	if cfg.Middleware != nil {
		cfg.Middleware.ApplyDefaults()
	}

	return &cfg, nil
}
