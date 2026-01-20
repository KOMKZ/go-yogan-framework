package grpc

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	// TraceIDKey in Context (consistent with middleware)
	TraceIDKey = "trace_id"

	// TraceIDMetadataKey gRPC Metadata中的Trace ID键
	TraceIDMetadataKey = "x-trace-id"
)

// UnaryClientTraceInterceptor client TraceID interceptor (for backward compatibility)
//
// Function:
// 1. Extract custom TraceID from Context (backward compatible)
// Inject into outgoing metadata (for log correlation)
//
// Note:
// - OpenTelemetry trace propagation is automatically handled by StatsHandler (traceparent header)
// - This interceptor is for backward compatibility and log correlation (x-trace-id header)
// - The Logger will automatically extract the TraceID from the OTel Span Context, no manual injection is required
//
// Usage:
//
//	grpc.WithUnaryInterceptor(grpc.ChainUnaryInterceptor(
//	    UnaryClientTraceInterceptor(),
//	    UnaryClientLoggerInterceptor(logger),
//	))
func UnaryClientTraceInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

		// Extract TraceID from custom Context (backward compatible)
		traceID := extractTraceIDFromContext(ctx)

		// If there is a TraceID, inject it into the metadata (for log correlation only)
		if traceID != "" {
			ctx = injectTraceIDToMetadata(ctx, traceID)
		}

		// Call downstream
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// UnaryServerTraceInterceptor server-side TraceID interceptor (for backward compatibility)
//
// Function:
// 1. Extract custom TraceID from incoming metadata (backward compatible)
// Inject into Context (for logging purposes)
//
// Note:
// - OpenTelemetry trace propagation is automatically handled by StatsHandler
// - This interceptor is for backward compatibility only
// - The Logger will prioritize using the OTel Span Context's TraceID
//
// Usage:
//
//	grpc.UnaryInterceptor(grpc.ChainUnaryInterceptor(
//	    UnaryServerTraceInterceptor(),
//	    UnaryLoggerInterceptor(logger),
//	))
func UnaryServerTraceInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (interface{}, error) {

		// Extract custom TraceID from metadata (backward compatible)
		traceID := extractTraceIDFromMetadata(ctx)

		// If there is a TraceID, inject it into the Context
		if traceID != "" {
			ctx = context.WithValue(ctx, TraceIDKey, traceID)
		}

		// Call business logic
		return handler(ctx, req)
	}
}

// UnaryClientTraceLoggerInterceptor client trace ID and log interceptor (combined version)
//
// Function:
// 1. Extract TraceID from Context and inject into metadata
// Record logs with TraceID
//
// Usage:
//
//	grpc.WithUnaryInterceptor(UnaryClientTraceLoggerInterceptor(logger))
func UnaryClientTraceLoggerInterceptor(logger *zap.Logger) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

		// Extract TraceID from Context
		traceID := extractTraceIDFromContext(ctx)

		// If there is a TraceID, inject it into the metadata
		if traceID != "" {
			ctx = injectTraceIDToMetadata(ctx, traceID)
		}

		// Call downstream and log (with TraceID)
		start := logger.Sugar().Desugar().WithOptions(zap.AddCallerSkip(1))
		fields := []zap.Field{
			zap.String("method", method),
			zap.String("target", cc.Target()),
		}

		// Add TraceID to log field
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

// UnaryServerTraceLoggerInterceptor server-side TraceID and logging interceptor (combined version)
//
// Function:
// Extract TraceID from metadata and inject into Context
// Record logs with TraceID
//
// Usage:
//
//	grpc.UnaryInterceptor(UnaryServerTraceLoggerInterceptor(logger))
func UnaryServerTraceLoggerInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (interface{}, error) {

		// Extract TraceID from metadata
		traceID := extractTraceIDFromMetadata(ctx)

		// If there is a TraceID, inject it into the Context
		if traceID != "" {
			ctx = context.WithValue(ctx, TraceIDKey, traceID)
		}

		// 3. Log with TraceID
		start := logger.Sugar().Desugar().WithOptions(zap.AddCallerSkip(1))
		fields := []zap.Field{
			zap.String("method", info.FullMethod),
		}

		// Add TraceID to log field
		if traceID != "" {
			fields = append([]zap.Field{zap.String(TraceIDKey, traceID)}, fields...)
		}

		// 4. Call business logic
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
// Internal auxiliary function
// ============================================

// extractTraceIDFromContext extracts a custom TraceID from Context (backward compatible)
func extractTraceIDFromContext(ctx context.Context) string {
	if val := ctx.Value(TraceIDKey); val != nil {
		if traceID, ok := val.(string); ok {
			return traceID
		}
	}
	return ""
}

// extractTraceIDFromMetadata extracts TraceID from gRPC incoming metadata
func extractTraceIDFromMetadata(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	// Get TraceID from metadata (key case insensitive)
	if values := md.Get(TraceIDMetadataKey); len(values) > 0 {
		return values[0]
	}

	return ""
}

// InjectTraceIDToMetadata injects TraceID into gRPC outgoing metadata
func injectTraceIDToMetadata(ctx context.Context, traceID string) context.Context {
	// Get existing outgoing metadata (if any)
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}

	// Add TraceID
	md.Set(TraceIDMetadataKey, traceID)

	// Return new context
	return metadata.NewOutgoingContext(ctx, md)
}
