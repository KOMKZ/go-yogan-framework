package grpc

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"
)

// GRPCMetrics gRPC layer metric collector
type GRPCMetrics struct {
	requestsTotal       metric.Int64Counter     // Total request count
	requestDuration     metric.Float64Histogram // request duration
	requestsInFlight    metric.Int64UpDownCounter // The number of requests being processed
	requestMessageSize  metric.Int64Histogram   // Request message size (optional)
	responseMessageSize metric.Int64Histogram   // Response message size (optional)
	streamsActive       metric.Int64UpDownCounter // Active stream count (optional)
	streamMessagesSent  metric.Int64Counter     // Optional stream message send count
	streamMessagesRecv  metric.Int64Counter     // Stream message reception count (optional)
	recordMessageSize   bool
	recordStreamMetrics bool
}

// NewGRPCMetrics creates a gRPC metric collector
func NewGRPCMetrics(recordMessageSize, recordStreamMetrics bool) (*GRPCMetrics, error) {
	meter := otel.Meter("grpc-server")

	requestsTotal, err := meter.Int64Counter(
		"grpc_requests_total",
		metric.WithDescription("gRPC 请求总数"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	requestDuration, err := meter.Float64Histogram(
		"grpc_request_duration_seconds",
		metric.WithDescription("gRPC 请求耗时分布"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	requestsInFlight, err := meter.Int64UpDownCounter(
		"grpc_requests_in_flight",
		metric.WithDescription("当前正在处理的 gRPC 请求数"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	m := &GRPCMetrics{
		requestsTotal:       requestsTotal,
		requestDuration:     requestDuration,
		requestsInFlight:    requestsInFlight,
		recordMessageSize:   recordMessageSize,
		recordStreamMetrics: recordStreamMetrics,
	}

	// Optional: Log message size
	if recordMessageSize {
		requestMessageSize, err := meter.Int64Histogram(
			"grpc_request_message_size_bytes",
			metric.WithDescription("gRPC 请求消息大小分布"),
			metric.WithUnit("By"),
		)
		if err != nil {
			return nil, err
		}
		m.requestMessageSize = requestMessageSize

		responseMessageSize, err := meter.Int64Histogram(
			"grpc_response_message_size_bytes",
			metric.WithDescription("gRPC 响应消息大小分布"),
			metric.WithUnit("By"),
		)
		if err != nil {
			return nil, err
		}
		m.responseMessageSize = responseMessageSize
	}

	// Optional: Log streaming metrics
	if recordStreamMetrics {
		streamsActive, err := meter.Int64UpDownCounter(
			"grpc_streams_active",
			metric.WithDescription("当前活跃的 gRPC 流数量"),
			metric.WithUnit("{stream}"),
		)
		if err != nil {
			return nil, err
		}
		m.streamsActive = streamsActive

		streamMessagesSent, err := meter.Int64Counter(
			"grpc_stream_messages_sent_total",
			metric.WithDescription("gRPC 流发送消息总数"),
			metric.WithUnit("{message}"),
		)
		if err != nil {
			return nil, err
		}
		m.streamMessagesSent = streamMessagesSent

		streamMessagesRecv, err := meter.Int64Counter(
			"grpc_stream_messages_received_total",
			metric.WithDescription("gRPC 流接收消息总数"),
			metric.WithUnit("{message}"),
		)
		if err != nil {
			return nil, err
		}
		m.streamMessagesRecv = streamMessagesRecv
	}

	return m, nil
}

// StatsHandler returns a gRPC stats.Handler (for Server)
func (m *GRPCMetrics) StatsHandler() stats.Handler {
	return &metricsStatsHandler{metrics: m}
}

// metricsStatsHandler gRPC stats.Handler implementation (for Metrics only)
type metricsStatsHandler struct {
	metrics *GRPCMetrics
}

type metricsContextKey struct{}

type metricsData struct {
	startTime time.Time
	service   string
	method    string
	isStream  bool
}

func (h *metricsStatsHandler) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	// Log RPC metadata
	data := &metricsData{
		startTime: time.Now(),
		service:   info.FullMethodName,
		method:    info.FullMethodName,
	}
	return context.WithValue(ctx, metricsContextKey{}, data)
}

func (h *metricsStatsHandler) HandleRPC(ctx context.Context, s stats.RPCStats) {
	data, ok := ctx.Value(metricsContextKey{}).(*metricsData)
	if !ok {
		return
	}

	switch s := s.(type) {
	case *stats.Begin:
		// RPC start
		h.metrics.requestsInFlight.Add(ctx, 1)
		
		// StreamRPC start
		if data.isStream && h.metrics.recordStreamMetrics && h.metrics.streamsActive != nil {
			h.metrics.streamsActive.Add(ctx, 1)
		}

	case *stats.End:
		// RPC end
		h.metrics.requestsInFlight.Add(ctx, -1)

		// calculate duration
		duration := time.Since(data.startTime).Seconds()

		// Get status code
		statusCode := "OK"
		if s.Error != nil {
			st, _ := status.FromError(s.Error)
			statusCode = st.Code().String()
		}

		// Build label
		attrs := []attribute.KeyValue{
			attribute.String("method", data.method),
			attribute.String("status_code", statusCode),
		}

		// Record the total number of requests
		h.metrics.requestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))

		// Log request duration
		h.metrics.requestDuration.Record(ctx, duration, metric.WithAttributes(attrs...))

		// Streaming RPC ends
		if data.isStream && h.metrics.recordStreamMetrics && h.metrics.streamsActive != nil {
			h.metrics.streamsActive.Add(ctx, -1)
		}

	case *stats.InPayload:
		// Received request message
		if h.metrics.recordMessageSize && h.metrics.requestMessageSize != nil {
			h.metrics.requestMessageSize.Record(ctx, int64(s.Length),
				metric.WithAttributes(
					attribute.String("method", data.method),
				),
			)
		}

		// Stream message reception count
		if data.isStream && h.metrics.recordStreamMetrics && h.metrics.streamMessagesRecv != nil {
			h.metrics.streamMessagesRecv.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("method", data.method),
				),
			)
		}

	case *stats.OutPayload:
		// Send response message
		if h.metrics.recordMessageSize && h.metrics.responseMessageSize != nil {
			h.metrics.responseMessageSize.Record(ctx, int64(s.Length),
				metric.WithAttributes(
					attribute.String("method", data.method),
				),
			)
		}

		// Streamed message count
		if data.isStream && h.metrics.recordStreamMetrics && h.metrics.streamMessagesSent != nil {
			h.metrics.streamMessagesSent.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("method", data.method),
				),
			)
		}
	}
}

func (h *metricsStatsHandler) TagConn(ctx context.Context, info *stats.ConnTagInfo) context.Context {
	return ctx
}

func (h *metricsStatsHandler) HandleConn(ctx context.Context, s stats.ConnStats) {
	// Connection level statistics (not yet implemented)
}

