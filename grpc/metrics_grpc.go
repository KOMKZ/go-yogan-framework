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

// GRPCMetrics gRPC 层指标收集器
type GRPCMetrics struct {
	requestsTotal       metric.Int64Counter     // 请求总数
	requestDuration     metric.Float64Histogram // 请求耗时
	requestsInFlight    metric.Int64UpDownCounter // 正在处理的请求数
	requestMessageSize  metric.Int64Histogram   // 请求消息大小（可选）
	responseMessageSize metric.Int64Histogram   // 响应消息大小（可选）
	streamsActive       metric.Int64UpDownCounter // 活跃流数量（可选）
	streamMessagesSent  metric.Int64Counter     // 流发送消息数（可选）
	streamMessagesRecv  metric.Int64Counter     // 流接收消息数（可选）
	recordMessageSize   bool
	recordStreamMetrics bool
}

// NewGRPCMetrics 创建 gRPC 指标收集器
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

	// 可选：记录消息大小
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

	// 可选：记录流式传输指标
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

// StatsHandler 返回 gRPC stats.Handler（用于 Server）
func (m *GRPCMetrics) StatsHandler() stats.Handler {
	return &metricsStatsHandler{metrics: m}
}

// metricsStatsHandler gRPC stats.Handler 实现（仅用于 Metrics）
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
	// 记录 RPC 元数据
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
		// RPC 开始
		h.metrics.requestsInFlight.Add(ctx, 1)
		
		// 流式 RPC 开始
		if data.isStream && h.metrics.recordStreamMetrics && h.metrics.streamsActive != nil {
			h.metrics.streamsActive.Add(ctx, 1)
		}

	case *stats.End:
		// RPC 结束
		h.metrics.requestsInFlight.Add(ctx, -1)

		// 计算耗时
		duration := time.Since(data.startTime).Seconds()

		// 获取状态码
		statusCode := "OK"
		if s.Error != nil {
			st, _ := status.FromError(s.Error)
			statusCode = st.Code().String()
		}

		// 构建标签
		attrs := []attribute.KeyValue{
			attribute.String("method", data.method),
			attribute.String("status_code", statusCode),
		}

		// 记录请求总数
		h.metrics.requestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))

		// 记录请求耗时
		h.metrics.requestDuration.Record(ctx, duration, metric.WithAttributes(attrs...))

		// 流式 RPC 结束
		if data.isStream && h.metrics.recordStreamMetrics && h.metrics.streamsActive != nil {
			h.metrics.streamsActive.Add(ctx, -1)
		}

	case *stats.InPayload:
		// 接收到请求消息
		if h.metrics.recordMessageSize && h.metrics.requestMessageSize != nil {
			h.metrics.requestMessageSize.Record(ctx, int64(s.Length),
				metric.WithAttributes(
					attribute.String("method", data.method),
				),
			)
		}

		// 流式接收消息计数
		if data.isStream && h.metrics.recordStreamMetrics && h.metrics.streamMessagesRecv != nil {
			h.metrics.streamMessagesRecv.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("method", data.method),
				),
			)
		}

	case *stats.OutPayload:
		// 发送响应消息
		if h.metrics.recordMessageSize && h.metrics.responseMessageSize != nil {
			h.metrics.responseMessageSize.Record(ctx, int64(s.Length),
				metric.WithAttributes(
					attribute.String("method", data.method),
				),
			)
		}

		// 流式发送消息计数
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
	// 连接级别的统计（暂不实现）
}

