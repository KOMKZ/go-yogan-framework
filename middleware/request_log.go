package middleware

import (
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// HTTP request log configuration
type RequestLogConfig struct {
	// SkipPaths list of paths to skip recording
	SkipPaths []string

	// EnableBody Whether to log request body (considering performance, default is false)
	EnableBody bool

	// MaxBodySize maximum request body recording size (bytes)
	MaxBodySize int
}

// Default request log configuration
func DefaultRequestLogConfig() RequestLogConfig {
	return RequestLogConfig{
		SkipPaths:   []string{},
		EnableBody:  false,
		MaxBodySize: 4096, // 4KB
	}
}

// RequestLog Gin HTTP request logging middleware (structured logs)
// Replace gin.Logger() with a custom Logger component to log request logs
//
// Function:
// - Structured log fields (status code, duration, client IP, etc.)
// - Automatically classify by status code (500+ Error, 400+ Warning, 200+ Information)
// - Record request error information
// - Support for automatic association of TraceID (using Context API)
// - Support skipping specified paths
//
// Usage:
//
//	engine.Use(middleware.RequestLog())
// // or custom configuration
//	cfg := middleware.DefaultRequestLogConfig()
//	cfg.SkipPaths = []string{"/health", "/metrics"}
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

		// Calculate request duration
		endTime := time.Now()
		latency := endTime.Sub(startTime)

		// Extract request information
		clientIP := c.ClientIP()
		method := c.Request.Method
		path := c.Request.URL.Path
		statusCode := c.Writer.Status()
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
			logger.ErrorCtx(ctx, "yogan", "HTTP 请求", fields...)
		} else if statusCode >= 400 {
			logger.WarnCtx(ctx, "yogan", "HTTP 请求", fields...)
		} else {
			logger.InfoCtx(ctx, "yogan", "HTTP 请求", fields...)
		}
	}
}
