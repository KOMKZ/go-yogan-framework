package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

const (
	// TraceIDKeyDefault Context 中的 TraceID Key 默认值
	TraceIDKeyDefault = "trace_id"

	// TraceIDHeaderDefault HTTP Header 中的 TraceID Key 默认值
	TraceIDHeaderDefault = "X-Trace-ID"
)

// TraceConfig Trace 中间件配置
type TraceConfig struct {
	// TraceIDKey Context 中存储的 Key（默认 "trace_id"）
	TraceIDKey string

	// TraceIDHeader HTTP Header 中的 Key（默认 "X-Trace-ID"）
	TraceIDHeader string

	// EnableResponseHeader 是否将 TraceID 写入 Response Header（默认 true）
	EnableResponseHeader bool

	// Generator 自定义 TraceID 生成器（默认使用 UUID）
	Generator func() string
}

// DefaultTraceConfig 默认配置
func DefaultTraceConfig() TraceConfig {
	return TraceConfig{
		TraceIDKey:           TraceIDKeyDefault,
		TraceIDHeader:        TraceIDHeaderDefault,
		EnableResponseHeader: true,
		Generator:            func() string { return uuid.New().String() },
	}
}

// TraceID 创建 TraceID 中间件
// 
// 功能：
//   1. 从 Header 提取或生成 TraceID
//   2. 注入到 gin.Context 和 context.Context
//   3. 可选：将 TraceID 写入 Response Header
//   4. 🎯 智能切换：如果 OpenTelemetry 已启用，优先使用 OTel Trace ID
//
// 用法：
//   engine.Use(middleware.TraceID(middleware.DefaultTraceConfig()))
func TraceID(cfg TraceConfig) gin.HandlerFunc {
	// 应用默认值
	if cfg.TraceIDKey == "" {
		cfg.TraceIDKey = TraceIDKeyDefault
	}
	if cfg.TraceIDHeader == "" {
		cfg.TraceIDHeader = TraceIDHeaderDefault
	}
	if cfg.Generator == nil {
		cfg.Generator = func() string { return uuid.New().String() }
	}

	return func(c *gin.Context) {
		// ===========================
		// 🎯 1. 检查 OpenTelemetry 是否启用
		// ===========================
		span := trace.SpanFromContext(c.Request.Context())
		
		var traceID string
		if span.SpanContext().IsValid() {
			// OTel 已启用，使用 OTel Trace ID
			traceID = span.SpanContext().TraceID().String()
		} else {
			// OTel 未启用，使用自定义 TraceID 逻辑
			traceID = c.GetHeader(cfg.TraceIDHeader)
			if traceID == "" {
				traceID = cfg.Generator()
			}
			// 注入到 context（兼容旧逻辑）
			ctx := context.WithValue(c.Request.Context(), cfg.TraceIDKey, traceID)
			c.Request = c.Request.WithContext(ctx)
		}

		// ===========================
		// 2. 注入到 gin.Context（便于 Handler 直接获取）
		// ===========================
		c.Set(cfg.TraceIDKey, traceID)

		// ===========================
		// 3. 可选：将 TraceID 写入 Response Header
		// ===========================
		if cfg.EnableResponseHeader {
			c.Writer.Header().Set(cfg.TraceIDHeader, traceID)
		}

		// 处理请求
		c.Next()
	}
}

// GetTraceID 从 gin.Context 获取 TraceID（便捷方法）
// 使用默认 Key
func GetTraceID(c *gin.Context) string {
	return GetTraceIDWithKey(c, TraceIDKeyDefault)
}

// GetTraceIDWithKey 从 gin.Context 获取 TraceID（指定 Key）
func GetTraceIDWithKey(c *gin.Context, key string) string {
	traceID, exists := c.Get(key)
	if !exists {
		return ""
	}
	if id, ok := traceID.(string); ok {
		return id
	}
	return ""
}

