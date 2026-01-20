package middleware

import (
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RequestLogConfig HTTP 请求日志配置
type RequestLogConfig struct {
	// SkipPaths 跳过记录的路径列表
	SkipPaths []string

	// EnableBody 是否记录请求体（性能考虑，默认 false）
	EnableBody bool

	// MaxBodySize 最大请求体记录大小（字节）
	MaxBodySize int
}

// DefaultRequestLogConfig 默认配置
func DefaultRequestLogConfig() RequestLogConfig {
	return RequestLogConfig{
		SkipPaths:   []string{},
		EnableBody:  false,
		MaxBodySize: 4096, // 4KB
	}
}

// RequestLog Gin HTTP 请求日志中间件（结构化日志）
// 替代 gin.Logger()，使用自定义 Logger 组件记录请求日志
//
// 功能：
//   - 结构化日志字段（状态码、耗时、客户端IP等）
//   - 按状态码自动分级（500+ Error、400+ Warn、200+ Info）
//   - 记录请求错误信息
//   - 支持 TraceID 自动关联（使用 Context API）
//   - 支持跳过指定路径
//
// 用法：
//
//	engine.Use(middleware.RequestLog())
//	// 或自定义配置
//	cfg := middleware.DefaultRequestLogConfig()
//	cfg.SkipPaths = []string{"/health", "/metrics"}
//	engine.Use(middleware.RequestLogWithConfig(cfg))
func RequestLog() gin.HandlerFunc {
	return RequestLogWithConfig(DefaultRequestLogConfig())
}

// RequestLogWithConfig 创建 HTTP 请求日志中间件（自定义配置）
func RequestLogWithConfig(cfg RequestLogConfig) gin.HandlerFunc {
	// 构建跳过路径的 map（提高查找性能）
	skipPathsMap := make(map[string]bool)
	for _, path := range cfg.SkipPaths {
		skipPathsMap[path] = true
	}

	return func(c *gin.Context) {
		// 检查是否跳过此路径
		if skipPathsMap[c.Request.URL.Path] {
			c.Next()
			return
		}

		// 记录开始时间
		startTime := time.Now()

		// 处理请求（调用后续中间件和 Handler）
		c.Next()

		// 计算请求耗时
		endTime := time.Now()
		latency := endTime.Sub(startTime)

		// 提取请求信息
		clientIP := c.ClientIP()
		method := c.Request.Method
		path := c.Request.URL.Path
		statusCode := c.Writer.Status()
		bodySize := c.Writer.Size()

		// 提取错误信息（如果有）
		errorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()

		// 构建结构化日志字段
		fields := []zap.Field{
			zap.Int("status", statusCode),
			zap.Duration("latency", latency),
			zap.String("client_ip", clientIP),
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("body_size", bodySize),
		}

		// 如果有错误信息，添加到字段中
		if errorMessage != "" {
			fields = append(fields, zap.String("error", errorMessage))
		}

		// ✅ 使用 Context API（支持 TraceID 自动关联）
		ctx := c.Request.Context()

		// 根据状态码选择日志级别
		// 500+: 服务器错误，Error 级别
		// 400+: 客户端错误，Warn 级别
		// 200+: 正常请求，Info 级别
		if statusCode >= 500 {
			logger.ErrorCtx(ctx, "yogan", "HTTP 请求", fields...)
		} else if statusCode >= 400 {
			logger.WarnCtx(ctx, "yogan", "HTTP 请求", fields...)
		} else {
			logger.InfoCtx(ctx, "yogan", "HTTP 请求", fields...)
		}
	}
}
