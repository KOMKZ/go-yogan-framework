package grpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/metadata"
)

// TestExtractTraceIDFromContext test extracting TraceID from Context
func TestExtractTraceIDFromContext(t *testing.T) {
	t.Run("有TraceID", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), TraceIDKey, "test-trace-123")
		traceID := extractTraceIDFromContext(ctx)
		assert.Equal(t, "test-trace-123", traceID)
	})

	t.Run("无TraceID", func(t *testing.T) {
		ctx := context.Background()
		traceID := extractTraceIDFromContext(ctx)
		assert.Empty(t, traceID)
	})

	t.Run("错误类型", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), TraceIDKey, 12345)
		traceID := extractTraceIDFromContext(ctx)
		assert.Empty(t, traceID)
	})
}

// TestInjectTraceIDToMetadata test inject TraceID into metadata
func TestInjectTraceIDToMetadata(t *testing.T) {
	t.Run("新建metadata", func(t *testing.T) {
		ctx := context.Background()
		ctx = injectTraceIDToMetadata(ctx, "test-trace-456")

		md, ok := metadata.FromOutgoingContext(ctx)
		assert.True(t, ok)
		assert.Contains(t, md, TraceIDMetadataKey)
		assert.Equal(t, "test-trace-456", md.Get(TraceIDMetadataKey)[0])
	})

	t.Run("追加到已有metadata", func(t *testing.T) {
		// Create existing metadata
		md := metadata.New(map[string]string{"existing-key": "existing-value"})
		ctx := metadata.NewOutgoingContext(context.Background(), md)

		// Inject TraceID
		ctx = injectTraceIDToMetadata(ctx, "test-trace-789")

		newMd, ok := metadata.FromOutgoingContext(ctx)
		assert.True(t, ok)
		assert.Contains(t, newMd, "existing-key")
		assert.Contains(t, newMd, TraceIDMetadataKey)
		assert.Equal(t, "test-trace-789", newMd.Get(TraceIDMetadataKey)[0])
	})
}

// TestExtractTraceIDFromMetadata test extracting TraceID from metadata
func TestExtractTraceIDFromMetadata(t *testing.T) {
	t.Run("有TraceID", func(t *testing.T) {
		md := metadata.New(map[string]string{TraceIDMetadataKey: "test-trace-abc"})
		ctx := metadata.NewIncomingContext(context.Background(), md)

		traceID := extractTraceIDFromMetadata(ctx)
		assert.Equal(t, "test-trace-abc", traceID)
	})

	t.Run("无metadata", func(t *testing.T) {
		ctx := context.Background()
		traceID := extractTraceIDFromMetadata(ctx)
		assert.Empty(t, traceID)
	})

	t.Run("无TraceID字段", func(t *testing.T) {
		md := metadata.New(map[string]string{"other-key": "other-value"})
		ctx := metadata.NewIncomingContext(context.Background(), md)

		traceID := extractTraceIDFromMetadata(ctx)
		assert.Empty(t, traceID)
	})
}

// TestTraceIDPropagation test complete TraceID propagation flow
func TestTraceIDPropagation(t *testing.T) {
	// Simulate complete process:
	// 1. Generate TraceID for HTTP request and inject into Context
	// 2. The client interceptor extracts and injects into metadata
	// 3. The server interceptor extracts metadata and injects it into the context

	t.Run("客户端到服务端的TraceID传播", func(t *testing.T) {
		// Generate a TraceID for simulated HTTP requests
		originalTraceID := "http-req-trace-001"
		clientCtx := context.WithValue(context.Background(), TraceIDKey, originalTraceID)

		// Client interceptor: extract TraceID and inject into metadata
		clientCtx = injectTraceIDToMetadata(clientCtx, originalTraceID)

		// Simulate gRPC transmission (convert outgoing metadata to incoming metadata)
		outgoingMd, _ := metadata.FromOutgoingContext(clientCtx)
		serverCtx := metadata.NewIncomingContext(context.Background(), outgoingMd)

		// English: ④ Server interceptor: Extract TraceID from metadata
		extractedTraceID := extractTraceIDFromMetadata(serverCtx)
		assert.Equal(t, originalTraceID, extractedTraceID)

		// ⑤ Inject into server-side Context
		serverCtx = context.WithValue(serverCtx, TraceIDKey, extractedTraceID)

		// Verify the TraceID in the server-side Context
		finalTraceID := extractTraceIDFromContext(serverCtx)
		assert.Equal(t, originalTraceID, finalTraceID)
	})
}

