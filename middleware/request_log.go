// Package middleware 通用 Gin HTTP 中间件集合。
// 本文件是请求日志中间件：为所有 HTTP 请求输出结构化访问日志（status、latency、
// client_ip、method、path 等），并按最终状态码分级（5xx=Error、4xx=Warn、其余=Info）。
// 上游入口：application/http_server.go 按 application.RequestLogConfig 装配；
// 下游依赖：logger 组件。消费方应只依赖 RequestLogConfig / RequestLogSkipRule
// 这两个配置契约，不要依赖内部查找结构。
package middleware

import (
	"strings"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RequestLogSkipRule 一条精确的“方法 + 路径 + 最终状态码”日志跳过规则。
// 只用于抑制确定成功的轮询类请求产生的访问日志噪声：三个字段都必须精确匹配
// （path 是全等比较、不展开前缀/通配符/正则；method 忽略大小写；status 必须命中
// statuses 列表之一），任何一条不满足就照常记录，因此认证失败、4xx、5xx 永远
// 不会被成功规则吞掉。
type RequestLogSkipRule struct {
	// Method HTTP 方法（如 GET、POST），比较时不区分大小写
	Method string

	// Path 精确匹配的请求路径（不含 query string）
	Path string

	// Statuses 允许跳过的最终响应状态码集合（必须显式列出，通常是 2xx）
	Statuses []int
}

// HTTP request log configuration
type RequestLogConfig struct {
	// SkipPaths list of paths to skip recording
	SkipPaths []string

	// SkipRules list of method+path+status precise skip rules; evaluated
	// after the request completes so error responses are never suppressed
	SkipRules []RequestLogSkipRule

	// EnableBody Whether to log request body (considering performance, default is false)
	EnableBody bool

	// MaxBodySize maximum request body recording size (bytes)
	MaxBodySize int
}

// Default request log configuration
func DefaultRequestLogConfig() RequestLogConfig {
	return RequestLogConfig{
		SkipPaths:   []string{},
		SkipRules:   []RequestLogSkipRule{},
		EnableBody:  false,
		MaxBodySize: 4096, // 4KB
	}
}

// RequestLog Gin HTTP request logging middleware (structured logs)
// Replace gin.Logger() with a custom Logger component to log request logs
//
// Function:
//   - Structured log fields (status code, duration, client IP, etc.)
//   - Automatically classify by status code (500+ Error, 400+ Warning, 200+ Information)
//   - Record request error information
//   - Support for automatic association of TraceID (using Context API)
//   - Support skipping specified paths entirely (SkipPaths, before the request)
//   - Support skipping precise success responses by method+path+status
//     (SkipRules, evaluated after the request so errors are always logged)
//
// Usage:
//
//	engine.Use(middleware.RequestLog())
//
// // or custom configuration
//
//	cfg := middleware.DefaultRequestLogConfig()
//	cfg.SkipPaths = []string{"/health", "/metrics"}
//	cfg.SkipRules = []middleware.RequestLogSkipRule{
//		{Method: "POST", Path: "/api/v1/poll", Statuses: []int{204}},
//	}
//	engine.Use(middleware.RequestLogWithConfig(cfg))
func RequestLog() gin.HandlerFunc {
	return RequestLogWithConfig(DefaultRequestLogConfig())
}

// Create HTTP request log middleware with custom configuration
func RequestLogWithConfig(cfg RequestLogConfig) gin.HandlerFunc {
	// Build a map for skipping paths (improve lookup performance)
	skipPathsMap := make(map[string]bool)
	for _, path := range cfg.SkipPaths {
		skipPathsMap[path] = true
	}

	// Precompute the skip-rule lookup: "METHOD path" -> set of skippable
	// statuses. Rules are matched exactly after the request completes, so
	// error responses on the same route still get logged.
	skipRuleStatuses := make(map[string]map[int]bool)
	for _, rule := range cfg.SkipRules {
		key := strings.ToUpper(rule.Method) + " " + rule.Path
		statuses, ok := skipRuleStatuses[key]
		if !ok {
			statuses = make(map[int]bool)
			skipRuleStatuses[key] = statuses
		}
		for _, status := range rule.Statuses {
			// skip_rules is a success-log denoising contract, not a general
			// log suppression mechanism. Ignore misconfigured redirects and
			// errors so 3xx/4xx/5xx always remain observable.
			if status < 200 || status >= 300 {
				continue
			}
			statuses[status] = true
		}
	}

	return func(c *gin.Context) {
		// Check if skip this path
		if skipPathsMap[c.Request.URL.Path] {
			c.Next()
			return
		}

		// Record start time
		startTime := time.Now()

		// Handle request (invoke subsequent middleware and Handler)
		c.Next()

		// Extract request information
		method := c.Request.Method
		path := c.Request.URL.Path
		statusCode := c.Writer.Status()

		// Skip rules are evaluated only after the request completes: a rule
		// suppresses the log when method+path match exactly AND the final
		// status is one of the configured success statuses.
		if statuses, ok := skipRuleStatuses[strings.ToUpper(method)+" "+path]; ok && statuses[statusCode] {
			return
		}

		// Calculate request duration
		endTime := time.Now()
		latency := endTime.Sub(startTime)

		// Extract request information
		clientIP := c.ClientIP()
		bodySize := c.Writer.Size()

		// Extract error message if any
		errorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()

		// Build structured log fields
		fields := []zap.Field{
			zap.Int("status", statusCode),
			zap.Duration("latency", latency),
			zap.String("client_ip", clientIP),
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("body_size", bodySize),
		}

		// If there is an error message, add it to the field
		if errorMessage != "" {
			fields = append(fields, zap.String("error", errorMessage))
		}

		// ✅ Uses Context API (supports automatic association of TraceID)
		ctx := c.Request.Context()

		// Select log level based on status code
		// 500+: Server error, Error level
		// 400+: Client error, Warn level
		// 200+: Normal request, Info level
		if statusCode >= 500 {
			logger.ErrorCtx(ctx, "yogan", "HTTP HTTP request", fields...)
		} else if statusCode >= 400 {
			logger.WarnCtx(ctx, "yogan", "HTTP HTTP request", fields...)
		} else {
			logger.InfoCtx(ctx, "yogan", "HTTP HTTP request", fields...)
		}
	}
}
