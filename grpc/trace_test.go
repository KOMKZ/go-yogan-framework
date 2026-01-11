package grpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/metadata"
)

// TestExtractTraceIDFromContext 测试从 Context 提取 TraceID
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

// TestInjectTraceIDToMetadata 测试将 TraceID 注入到 metadata
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
		// 创建已有 metadata
		md := metadata.New(map[string]string{"existing-key": "existing-value"})
		ctx := metadata.NewOutgoingContext(context.Background(), md)

		// 注入 TraceID
		ctx = injectTraceIDToMetadata(ctx, "test-trace-789")

		newMd, ok := metadata.FromOutgoingContext(ctx)
		assert.True(t, ok)
		assert.Contains(t, newMd, "existing-key")
		assert.Contains(t, newMd, TraceIDMetadataKey)
		assert.Equal(t, "test-trace-789", newMd.Get(TraceIDMetadataKey)[0])
	})
}

// TestExtractTraceIDFromMetadata 测试从 metadata 提取 TraceID
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

// TestTraceIDPropagation 测试 TraceID 完整传播流程
func TestTraceIDPropagation(t *testing.T) {
	// 模拟完整流程：
	// 1. HTTP 请求生成 TraceID 并注入到 Context
	// 2. 客户端拦截器提取并注入到 metadata
	// 3. 服务端拦截器从 metadata 提取并注入到 Context

	t.Run("客户端到服务端的TraceID传播", func(t *testing.T) {
		// ① 模拟 HTTP 请求生成的 TraceID
		originalTraceID := "http-req-trace-001"
		clientCtx := context.WithValue(context.Background(), TraceIDKey, originalTraceID)

		// ② 客户端拦截器：提取 TraceID 并注入到 metadata
		clientCtx = injectTraceIDToMetadata(clientCtx, originalTraceID)

		// ③ 模拟 gRPC 传输（将 outgoing metadata 转为 incoming metadata）
		outgoingMd, _ := metadata.FromOutgoingContext(clientCtx)
		serverCtx := metadata.NewIncomingContext(context.Background(), outgoingMd)

		// ④ 服务端拦截器：从 metadata 提取 TraceID
		extractedTraceID := extractTraceIDFromMetadata(serverCtx)
		assert.Equal(t, originalTraceID, extractedTraceID)

		// ⑤ 注入到服务端 Context
		serverCtx = context.WithValue(serverCtx, TraceIDKey, extractedTraceID)

		// ⑥ 验证服务端 Context 中的 TraceID
		finalTraceID := extractTraceIDFromContext(serverCtx)
		assert.Equal(t, originalTraceID, finalTraceID)
	})
}

