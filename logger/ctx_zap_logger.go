// src/pkg/logger/ctx_zap_logger.go
package logger

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// CtxZapLogger Context-aware Zap logger wrapper
// Design idea: module is bound during creation, only needs to pass ctx when used
// Reference: docs/085-logger-context-integration-analysis.md Solution 2.5
// Note: The NewCtxZapLogger export function is no longer provided; use GetLogger() or CreateLogger() uniformly to obtain it.
type CtxZapLogger struct {
	base   *zap.Logger
	module string
	config *ManagerConfig // Save configuration for stack depth control
}

// newCtxZapLogger creates a context-aware logger (internal use, binds module on creation)
// Usage:
//
// logger := logger.MustGetLogger("user")  // for use in application layer
// logger := logger.MustGetLogger("yogan") // Use uniformly in the Yogan kernel
// logger.InfoCtx(ctx, "Create user", zap.String("name", "Zhang San"))
func NewCtxZapLogger(module string) *CtxZapLogger {
	base := GetLogger(module) // Retrieve from Manager (including the module field)

	// Note: CallerSkip has already been set in Manager.MustGetLogger, no need to set it here
	return base
}

// InfoCtx records Info level logs (automatically extracts TraceID)
func (l *CtxZapLogger) InfoCtx(ctx context.Context, msg string, fields ...zap.Field) {
	l.base.Info(msg, l.enrichFields(ctx, fields)...)
}

// Info level logging (convenient method without needing context)
func (l *CtxZapLogger) Info(msg string, fields ...zap.Field) {
	l.InfoCtx(context.Background(), msg, fields...)
}

// ErrorCtx logs error level logs (automatically extracts TraceID + optional stack)
func (l *CtxZapLogger) ErrorCtx(ctx context.Context, msg string, fields ...zap.Field) {
	enriched := l.enrichFields(ctx, fields)

	// If configuration enables stack and meets level requirements, automatically add controlled depth stack
	if l.config != nil && l.config.EnableStacktrace {
		if shouldCaptureStacktrace("error", *l.config) {
			depth := l.config.StacktraceDepth
			if depth <= 0 {
				depth = 10 // Default 10 layers
			}
			// skip=3: CaptureStacktrace(0) -> ErrorCtx(1) -> actual caller(2)
			stack := CaptureStacktrace(3, depth)
			if stack != "" {
				enriched = append(enriched, zap.String("stack", stack))
			}
		}
	}

	l.base.Error(msg, enriched...)
}

// Records error level logs (a convenient method without needing context)
func (l *CtxZapLogger) Error(msg string, fields ...zap.Field) {
	l.ErrorCtx(context.Background(), msg, fields...)
}

// DebugCtx logs debug level logs (automatically extracts TraceID)
func (l *CtxZapLogger) DebugCtx(ctx context.Context, msg string, fields ...zap.Field) {
	l.base.Debug(msg, l.enrichFields(ctx, fields)...)
}

// Debug logging for level debug (convenient method without needing context)
func (l *CtxZapLogger) Debug(msg string, fields ...zap.Field) {
	l.DebugCtx(context.Background(), msg, fields...)
}

// WarnCtx logs warnings (automatically extracts TraceID)
func (l *CtxZapLogger) WarnCtx(ctx context.Context, msg string, fields ...zap.Field) {
	l.base.Warn(msg, l.enrichFields(ctx, fields)...)
}

// Warn record Warn level log (a convenient method without requiring context)
func (l *CtxZapLogger) Warn(msg string, fields ...zap.Field) {
	l.WarnCtx(context.Background(), msg, fields...)
}

// Returns a new Logger with preset fields (supports method chaining)
// Usage:
//
//	orderLogger := logger.With(zap.Int64("order_id", 123))
// orderLogger.InfoCtx(ctx, "Order processing")  // Automatically includes order_id
func (l *CtxZapLogger) With(fields ...zap.Field) *CtxZapLogger {
	return &CtxZapLogger{
		base:   l.base.With(fields...), // base already includes CallerSkip
		module: l.module,
		config: l.config,
	}
}

// GetZapLogger Obtain the underlying *zap.Logger (for integration with third-party libraries)
// For example: etcd client.WithLogger(logger.GetZapLogger())
func (l *CtxZapLogger) GetZapLogger() *zap.Logger {
	return l.base
}

// enrichFields automatically adds TraceID and app_name
// Note: The module field has already been added in Manager.GetLogger(), no need to add it again
func (l *CtxZapLogger) enrichFields(ctx context.Context, fields []zap.Field) []zap.Field {
	enriched := make([]zap.Field, 0, len(fields)+2)

	// 🎯 Prioritize adding app_name (always inject, even if empty)
	if l.config != nil {
		enriched = append(enriched, zap.String("app_name", l.config.AppName))
	}

	// Check if TraceID is enabled
	if l.config != nil && l.config.EnableTraceID {
		// Extract TraceID
		traceID := extractTraceIDFromContext(ctx, l.config)
		if traceID != "" {
			// Get field name (support custom)
			fieldName := "trace_id"
			if l.config.TraceIDFieldName != "" {
				fieldName = l.config.TraceIDFieldName
			}
			enriched = append(enriched, zap.String(fieldName, traceID))
		}
	}

	// Add original field
	enriched = append(enriched, fields...)

	return enriched
}

// extractTraceIDFromContext extracts TraceID from Context
// 🎯 Priority: OpenTelemetry Span > Custom Context Key
// Supports multiple keys (compatible with different scenarios) and custom configuration
func extractTraceIDFromContext(ctx context.Context, cfg *ManagerConfig) string {
	// 🎯 Priority 1: Extract from OpenTelemetry Span Context (if enabled)
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}

	// 🎯 Priority 2: Use the configured key if a configuration is provided
	if cfg != nil && cfg.TraceIDKey != "" {
		if val := ctx.Value(cfg.TraceIDKey); val != nil {
			if traceID, ok := val.(string); ok {
				return traceID
			}
		}
	}

	// 🎯 Priority 3: Try standard key
	if val := ctx.Value("trace_id"); val != nil {
		if traceID, ok := val.(string); ok {
			return traceID
		}
	}

	// 🎯 Priority 4: Try other possible keys (for compatibility)
	if val := ctx.Value("traceId"); val != nil {
		if traceID, ok := val.(string); ok {
			return traceID
		}
	}

	return ""
}
