package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

const (
	// TraceIDKeyDefault TraceID key default value in the Context
	TraceIDKeyDefault = "trace_id"

	// The default value for the TraceID key in the TraceIDHeaderDefault HTTP header
	TraceIDHeaderDefault = "X-Trace-ID"
)

// TraceConfig Trace middleware configuration
type TraceConfig struct {
	// TraceIDKey stored in Context (default "trace_id")
	TraceIDKey string

	// TraceIDHeader is the key in the HTTP Header (default "X-Trace-ID")
	TraceIDHeader string

	// EnableResponseHeader whether to write TraceID into Response Header (default true)
	EnableResponseHeader bool

	// Generator custom TraceID generator (default uses UUID)
	Generator func() string
}

// Default configuration for DefaultTraceConfig
func DefaultTraceConfig() TraceConfig {
	return TraceConfig{
		TraceIDKey:           TraceIDKeyDefault,
		TraceIDHeader:        TraceIDHeaderDefault,
		EnableResponseHeader: true,
		Generator:            func() string { return uuid.New().String() },
	}
}

// Create TraceID middleware
// 
// Function:
// 1. Extract or generate TraceID from Header
// Inject into gin.Context and context.Context
// Optional: Write TraceID to Response Header
// 4. 🎯 Intelligent switch: If OpenTelemetry is enabled, prioritize using the OTel Trace ID
//
// Usage:
//   engine.Use(middleware.TraceID(middleware.DefaultTraceConfig()))
func TraceID(cfg TraceConfig) gin.HandlerFunc {
	// Apply default values
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
		// 🎯 1. Check if OpenTelemetry is enabled
		// ===========================
		span := trace.SpanFromContext(c.Request.Context())
		
		var traceID string
		if span.SpanContext().IsValid() {
			// Otel is enabled, using Otel Trace ID
			traceID = span.SpanContext().TraceID().String()
		} else {
			// OTel not enabled, using custom TraceID logic
			traceID = c.GetHeader(cfg.TraceIDHeader)
			if traceID == "" {
				traceID = cfg.Generator()
			}
			// Inject into context (compatible with old logic)
			ctx := context.WithValue(c.Request.Context(), cfg.TraceIDKey, traceID)
			c.Request = c.Request.WithContext(ctx)
		}

		// ===========================
		// 2. Inject into gin.Context (for easy direct access by Handler)
		// ===========================
		c.Set(cfg.TraceIDKey, traceID)

		// ===========================
		// Optional: Write TraceID to Response Header
		// ===========================
		if cfg.EnableResponseHeader {
			c.Writer.Header().Set(cfg.TraceIDHeader, traceID)
		}

		// Handle request
		c.Next()
	}
}

// GetTraceID retrieves the TraceID from gin.Context (convenience method)
// Use default key
func GetTraceID(c *gin.Context) string {
	return GetTraceIDWithKey(c, TraceIDKeyDefault)
}

// GetTraceIDWithKey retrieves the TraceID from gin.Context (specified by Key)
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

