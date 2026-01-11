package grpc

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	// TraceIDKey Context 中的 TraceID Key（与 middleware 保持一致）
	TraceIDKey = "trace_id"

	// TraceIDMetadataKey gRPC Metadata 中的 TraceID Key
	TraceIDMetadataKey = "x-trace-id"
)

// UnaryClientTraceInterceptor 客户端 TraceID 拦截器（用于向后兼容）
//
// 功能：
//  1. 从 Context 提取自定义 TraceID（向后兼容）
//  2. 注入到 outgoing metadata（用于日志关联）
//
// 注意：
//   - OpenTelemetry trace 传播已由 StatsHandler 自动处理（traceparent header）
//   - 此拦截器仅用于向后兼容和日志关联（x-trace-id header）
//   - Logger 会自动从 OTel Span Context 提取 TraceID，无需手动注入
//
// 用法：
//
//	grpc.WithUnaryInterceptor(grpc.ChainUnaryInterceptor(
//	    UnaryClientTraceInterceptor(),
//	    UnaryClientLoggerInterceptor(logger),
//	))
func UnaryClientTraceInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

		// 从自定义 Context 提取 TraceID（向后兼容）
		traceID := extractTraceIDFromContext(ctx)

		// 如果有 TraceID，注入到 metadata（仅用于日志关联）
		if traceID != "" {
			ctx = injectTraceIDToMetadata(ctx, traceID)
		}

		// 调用下游
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// UnaryServerTraceInterceptor 服务端 TraceID 拦截器（用于向后兼容）
//
// 功能：
//  1. 从 incoming metadata 提取自定义 TraceID（向后兼容）
//  2. 注入到 Context（供日志使用）
//
// 注意：
//   - OpenTelemetry trace 传播已由 StatsHandler 自动处理
//   - 此拦截器仅用于向后兼容
//   - Logger 会优先使用 OTel Span Context 的 TraceID
//
// 用法：
//
//	grpc.UnaryInterceptor(grpc.ChainUnaryInterceptor(
//	    UnaryServerTraceInterceptor(),
//	    UnaryLoggerInterceptor(logger),
//	))
func UnaryServerTraceInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (interface{}, error) {

		// 从 metadata 提取自定义 TraceID（向后兼容）
		traceID := extractTraceIDFromMetadata(ctx)

		// 如果有 TraceID，注入到 Context
		if traceID != "" {
			ctx = context.WithValue(ctx, TraceIDKey, traceID)
		}

		// 调用业务逻辑
		return handler(ctx, req)
	}
}

// UnaryClientTraceLoggerInterceptor 客户端 TraceID + 日志拦截器（组合版本）
//
// 功能：
//  1. 从 Context 提取 TraceID 并注入到 metadata
//  2. 记录带 TraceID 的日志
//
// 用法：
//
//	grpc.WithUnaryInterceptor(UnaryClientTraceLoggerInterceptor(logger))
func UnaryClientTraceLoggerInterceptor(logger *zap.Logger) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

		// 1. 从 Context 提取 TraceID
		traceID := extractTraceIDFromContext(ctx)

		// 2. 如果有 TraceID，注入到 metadata
		if traceID != "" {
			ctx = injectTraceIDToMetadata(ctx, traceID)
		}

		// 3. 调用下游并记录日志（带 TraceID）
		start := logger.Sugar().Desugar().WithOptions(zap.AddCallerSkip(1))
		fields := []zap.Field{
			zap.String("method", method),
			zap.String("target", cc.Target()),
		}

		// 添加 TraceID 到日志字段
		if traceID != "" {
			fields = append([]zap.Field{zap.String(TraceIDKey, traceID)}, fields...)
		}

		err := invoker(ctx, method, req, reply, cc, opts...)

		if err != nil {
			start.Error("gRPC call failed", append(fields, zap.Error(err))...)
		} else {
			start.Debug("gRPC 调用成功", fields...)
		}

		return err
	}
}

// UnaryServerTraceLoggerInterceptor 服务端 TraceID + 日志拦截器（组合版本）
//
// 功能：
//  1. 从 metadata 提取 TraceID 并注入到 Context
//  2. 记录带 TraceID 的日志
//
// 用法：
//
//	grpc.UnaryInterceptor(UnaryServerTraceLoggerInterceptor(logger))
func UnaryServerTraceLoggerInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (interface{}, error) {

		// 1. 从 metadata 提取 TraceID
		traceID := extractTraceIDFromMetadata(ctx)

		// 2. 如果有 TraceID，注入到 Context
		if traceID != "" {
			ctx = context.WithValue(ctx, TraceIDKey, traceID)
		}

		// 3. 记录日志（带 TraceID）
		start := logger.Sugar().Desugar().WithOptions(zap.AddCallerSkip(1))
		fields := []zap.Field{
			zap.String("method", info.FullMethod),
		}

		// 添加 TraceID 到日志字段
		if traceID != "" {
			fields = append([]zap.Field{zap.String(TraceIDKey, traceID)}, fields...)
		}

		// 4. 调用业务逻辑
		resp, err := handler(ctx, req)

		if err != nil {
			start.Error("gRPC 请求失败", append(fields, zap.Error(err))...)
		} else {
			start.Debug("gRPC 请求成功", fields...)
		}

		return resp, err
	}
}

// ============================================
// 内部辅助函数
// ============================================

// extractTraceIDFromContext 从 Context 提取自定义 TraceID（向后兼容）
func extractTraceIDFromContext(ctx context.Context) string {
	if val := ctx.Value(TraceIDKey); val != nil {
		if traceID, ok := val.(string); ok {
			return traceID
		}
	}
	return ""
}

// extractTraceIDFromMetadata 从 gRPC incoming metadata 提取 TraceID
func extractTraceIDFromMetadata(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	// 从 metadata 中获取 TraceID（key 不区分大小写）
	if values := md.Get(TraceIDMetadataKey); len(values) > 0 {
		return values[0]
	}

	return ""
}

// injectTraceIDToMetadata 将 TraceID 注入到 gRPC outgoing metadata
func injectTraceIDToMetadata(ctx context.Context, traceID string) context.Context {
	// 获取现有的 outgoing metadata（如果有）
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}

	// 添加 TraceID
	md.Set(TraceIDMetadataKey, traceID)

	// 返回新的 Context
	return metadata.NewOutgoingContext(ctx, md)
}
