// src/pkg/logger/ctx_zap_logger.go
package logger

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// CtxZapLogger Context-Aware 的 Zap Logger 包装器
// 设计思路：module 在创建时绑定，使用时只需传递 ctx
// 参考：docs/085-logger-context-integration-analysis.md 方案2.5
// 注意：不再提供 NewCtxZapLogger 导出函数，统一通过 GetLogger() 或 CreateLogger() 获取
type CtxZapLogger struct {
	base   *zap.Logger
	module string
	config *ManagerConfig // 保存配置，用于堆栈深度控制
}

// newCtxZapLogger 创建 Context-Aware Logger（内部使用，创建时绑定 module）
// 用法：
//
//	logger := logger.MustGetLogger("user")  // 应用层使用
//	logger := logger.MustGetLogger("yogan") // Yogan 内核统一使用
//	logger.InfoCtx(ctx, "Create user", zap.String("name", "张三"))
func NewCtxZapLogger(module string) *CtxZapLogger {
	base := GetLogger(module) // 从 Manager 获取（已包含 module 字段）

	// 注意：CallerSkip 已在 Manager.MustGetLogger 中设置，这里不需要再设置
	return base
}

// InfoCtx 记录 Info 级别日志（自动提取 TraceID）
func (l *CtxZapLogger) InfoCtx(ctx context.Context, msg string, fields ...zap.Field) {
	l.base.Info(msg, l.enrichFields(ctx, fields)...)
}

// Info 记录 Info 级别日志（不需要 context 的便捷方法）
func (l *CtxZapLogger) Info(msg string, fields ...zap.Field) {
	l.InfoCtx(context.Background(), msg, fields...)
}

// ErrorCtx 记录 Error 级别日志（自动提取 TraceID + 可选堆栈）
func (l *CtxZapLogger) ErrorCtx(ctx context.Context, msg string, fields ...zap.Field) {
	enriched := l.enrichFields(ctx, fields)

	// 如果配置启用堆栈且满足级别要求，自动添加受控深度的堆栈
	if l.config != nil && l.config.EnableStacktrace {
		if shouldCaptureStacktrace("error", *l.config) {
			depth := l.config.StacktraceDepth
			if depth <= 0 {
				depth = 10 // 默认 10 层
			}
			// skip=3: CaptureStacktrace(0) -> ErrorCtx(1) -> 实际调用者(2)
			stack := CaptureStacktrace(3, depth)
			if stack != "" {
				enriched = append(enriched, zap.String("stack", stack))
			}
		}
	}

	l.base.Error(msg, enriched...)
}

// Error 记录 Error 级别日志（不需要 context 的便捷方法）
func (l *CtxZapLogger) Error(msg string, fields ...zap.Field) {
	l.ErrorCtx(context.Background(), msg, fields...)
}

// DebugCtx 记录 Debug 级别日志（自动提取 TraceID）
func (l *CtxZapLogger) DebugCtx(ctx context.Context, msg string, fields ...zap.Field) {
	l.base.Debug(msg, l.enrichFields(ctx, fields)...)
}

// Debug 记录 Debug 级别日志（不需要 context 的便捷方法）
func (l *CtxZapLogger) Debug(msg string, fields ...zap.Field) {
	l.DebugCtx(context.Background(), msg, fields...)
}

// WarnCtx 记录 Warn 级别日志（自动提取 TraceID）
func (l *CtxZapLogger) WarnCtx(ctx context.Context, msg string, fields ...zap.Field) {
	l.base.Warn(msg, l.enrichFields(ctx, fields)...)
}

// Warn 记录 Warn 级别日志（不需要 context 的便捷方法）
func (l *CtxZapLogger) Warn(msg string, fields ...zap.Field) {
	l.WarnCtx(context.Background(), msg, fields...)
}

// With 返回带有预设字段的新 Logger（支持链式调用）
// 用法：
//
//	orderLogger := logger.With(zap.Int64("order_id", 123))
//	orderLogger.InfoCtx(ctx, "订单处理中")  // 自动包含 order_id
func (l *CtxZapLogger) With(fields ...zap.Field) *CtxZapLogger {
	return &CtxZapLogger{
		base:   l.base.With(fields...), // base 已经包含了 CallerSkip
		module: l.module,
		config: l.config,
	}
}

// GetZapLogger 获取底层的 *zap.Logger（用于第三方库集成）
// 例如：etcd client.WithLogger(logger.GetZapLogger())
func (l *CtxZapLogger) GetZapLogger() *zap.Logger {
	return l.base
}

// enrichFields 自动添加 TraceID 和 app_name
// 注意：module 字段已经在 Manager.GetLogger() 中添加，无需重复添加
func (l *CtxZapLogger) enrichFields(ctx context.Context, fields []zap.Field) []zap.Field {
	enriched := make([]zap.Field, 0, len(fields)+2)

	// 🎯 优先添加 app_name（始终注入，即使为空）
	if l.config != nil {
		enriched = append(enriched, zap.String("app_name", l.config.AppName))
	}

	// 检查是否启用 TraceID
	if l.config != nil && l.config.EnableTraceID {
		// 提取 TraceID
		traceID := extractTraceIDFromContext(ctx, l.config)
		if traceID != "" {
			// 获取字段名（支持自定义）
			fieldName := "trace_id"
			if l.config.TraceIDFieldName != "" {
				fieldName = l.config.TraceIDFieldName
			}
			enriched = append(enriched, zap.String(fieldName, traceID))
		}
	}

	// 添加原始字段
	enriched = append(enriched, fields...)

	return enriched
}

// extractTraceIDFromContext 从 Context 提取 TraceID
// 🎯 优先级：OpenTelemetry Span > 自定义 Context Key
// 支持多种 key（兼容不同场景）和自定义配置
func extractTraceIDFromContext(ctx context.Context, cfg *ManagerConfig) string {
	// 🎯 优先级 1: 从 OpenTelemetry Span Context 提取（如果启用）
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}

	// 🎯 优先级 2: 如果提供了配置，使用配置的 key
	if cfg != nil && cfg.TraceIDKey != "" {
		if val := ctx.Value(cfg.TraceIDKey); val != nil {
			if traceID, ok := val.(string); ok {
				return traceID
			}
		}
	}

	// 🎯 优先级 3: 尝试标准 key
	if val := ctx.Value("trace_id"); val != nil {
		if traceID, ok := val.(string); ok {
			return traceID
		}
	}

	// 🎯 优先级 4: 尝试其他可能的 key（兼容性）
	if val := ctx.Value("traceId"); val != nil {
		if traceID, ok := val.(string); ok {
			return traceID
		}
	}

	return ""
}
