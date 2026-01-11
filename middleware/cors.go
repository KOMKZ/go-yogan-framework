package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORSConfig CORS 中间件配置
type CORSConfig struct {
	// AllowOrigins 允许的源列表（默认 ["*"]）
	// 示例：["https://example.com", "https://app.example.com"]
	AllowOrigins []string

	// AllowMethods 允许的 HTTP 方法列表（默认 ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"]）
	AllowMethods []string

	// AllowHeaders 允许的请求头列表（默认 ["Origin", "Content-Type", "Accept", "Authorization"]）
	AllowHeaders []string

	// ExposeHeaders 暴露给客户端的响应头列表（默认 []）
	ExposeHeaders []string

	// AllowCredentials 是否允许发送凭证（Cookie、HTTP认证等）（默认 false）
	// 注意：当为 true 时，AllowOrigins 不能使用 "*"
	AllowCredentials bool

	// MaxAge 预检请求缓存时间（秒）（默认 43200，即12小时）
	MaxAge int
}

// DefaultCORSConfig 默认 CORS 配置
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{},
		AllowCredentials: false,
		MaxAge:           43200, // 12小时
	}
}

// CORS 创建 CORS 中间件（使用默认配置）
//
// 功能：
//   - 处理跨域资源共享（CORS）
//   - 自动响应 OPTIONS 预检请求
//   - 设置 CORS 相关响应头
//
// 用法：
//   engine.Use(middleware.CORS())
func CORS() gin.HandlerFunc {
	return CORSWithConfig(DefaultCORSConfig())
}

// CORSWithConfig 创建 CORS 中间件（自定义配置）
func CORSWithConfig(cfg CORSConfig) gin.HandlerFunc {
	// 应用默认值
	if len(cfg.AllowOrigins) == 0 {
		cfg.AllowOrigins = []string{"*"}
	}
	if len(cfg.AllowMethods) == 0 {
		cfg.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	}
	if len(cfg.AllowHeaders) == 0 {
		cfg.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	}
	if cfg.MaxAge == 0 {
		cfg.MaxAge = 43200
	}

	// 预处理配置（转换为字符串）
	allowMethodsStr := strings.Join(cfg.AllowMethods, ", ")
	allowHeadersStr := strings.Join(cfg.AllowHeaders, ", ")
	exposeHeadersStr := strings.Join(cfg.ExposeHeaders, ", ")

	return func(c *gin.Context) {
		// 获取请求的 Origin
		origin := c.Request.Header.Get("Origin")

		// ===========================
		// 1. 检查 Origin 是否允许
		// ===========================
		allowOrigin := ""
		if len(cfg.AllowOrigins) == 1 && cfg.AllowOrigins[0] == "*" {
			// 允许所有源
			allowOrigin = "*"
		} else if origin != "" {
			// 检查是否在允许列表中
			for _, allowedOrigin := range cfg.AllowOrigins {
				if allowedOrigin == origin {
					allowOrigin = origin
					break
				}
			}
		}

		// 如果 Origin 不允许，且不是通配符，跳过 CORS 处理
		if allowOrigin == "" && origin != "" {
			c.Next()
			return
		}

		// ===========================
		// 2. 设置 CORS 响应头
		// ===========================
		if allowOrigin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		}

		c.Writer.Header().Set("Access-Control-Allow-Methods", allowMethodsStr)
		c.Writer.Header().Set("Access-Control-Allow-Headers", allowHeadersStr)

		if len(cfg.ExposeHeaders) > 0 {
			c.Writer.Header().Set("Access-Control-Expose-Headers", exposeHeadersStr)
		}

		if cfg.AllowCredentials {
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// ===========================
		// 3. 处理 OPTIONS 预检请求
		// ===========================
		if c.Request.Method == "OPTIONS" {
			// 设置预检请求缓存时间
			c.Writer.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", cfg.MaxAge))
			
			// 直接返回 204 No Content
			c.AbortWithStatus(204)
			return
		}

		// 继续处理请求
		c.Next()
	}
}

