package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// HTTPMetrics HTTP 层指标收集器
type HTTPMetrics struct {
	requestsTotal      metric.Int64Counter      // 请求总数
	requestDuration    metric.Float64Histogram  // 请求耗时
	requestsInFlight   metric.Int64UpDownCounter // 正在处理的请求数
	requestSize        metric.Int64Histogram    // 请求大小（可选）
	responseSize       metric.Int64Histogram    // 响应大小（可选）
	recordRequestSize  bool
	recordResponseSize bool
}

// NewHTTPMetrics 创建 HTTP 指标收集器
func NewHTTPMetrics(recordRequestSize, recordResponseSize bool) (*HTTPMetrics, error) {
	meter := otel.Meter("http-server")

	requestsTotal, err := meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("HTTP 请求总数"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	requestDuration, err := meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("HTTP 请求耗时分布"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	requestsInFlight, err := meter.Int64UpDownCounter(
		"http_requests_in_flight",
		metric.WithDescription("当前正在处理的 HTTP 请求数"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	m := &HTTPMetrics{
		requestsTotal:      requestsTotal,
		requestDuration:    requestDuration,
		requestsInFlight:   requestsInFlight,
		recordRequestSize:  recordRequestSize,
		recordResponseSize: recordResponseSize,
	}

	// 可选：记录请求大小
	if recordRequestSize {
		requestSize, err := meter.Int64Histogram(
			"http_request_size_bytes",
			metric.WithDescription("HTTP 请求体大小分布"),
			metric.WithUnit("By"),
		)
		if err != nil {
			return nil, err
		}
		m.requestSize = requestSize
	}

	// 可选：记录响应大小
	if recordResponseSize {
		responseSize, err := meter.Int64Histogram(
			"http_response_size_bytes",
			metric.WithDescription("HTTP 响应体大小分布"),
			metric.WithUnit("By"),
		)
		if err != nil {
			return nil, err
		}
		m.responseSize = responseSize
	}

	return m, nil
}

// Handler 返回 Gin 中间件
func (m *HTTPMetrics) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		ctx := c.Request.Context()
		path := c.FullPath() // 使用路由模式而非实际路径（避免高基数）
		if path == "" {
			path = "unknown" // 404 或未匹配路由
		}

		// 请求开始，增加处理中计数
		m.requestsInFlight.Add(ctx, 1)
		defer m.requestsInFlight.Add(ctx, -1)

		// 记录请求大小（可选）
		if m.recordRequestSize && m.requestSize != nil {
			requestSize := c.Request.ContentLength
			if requestSize > 0 {
				m.requestSize.Record(ctx, requestSize,
					metric.WithAttributes(
						attribute.String("method", c.Request.Method),
						attribute.String("path", path),
					),
				)
			}
		}

		// 处理请求
		c.Next()

		// 计算耗时
		duration := time.Since(start).Seconds()
		statusCode := c.Writer.Status()

		// 通用标签
		attrs := []attribute.KeyValue{
			attribute.String("method", c.Request.Method),
			attribute.String("path", path),
			attribute.Int("status_code", statusCode),
			attribute.String("status_class", getStatusClass(statusCode)), // 2xx, 3xx, 4xx, 5xx
		}

		// 记录请求总数
		m.requestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))

		// 记录请求耗时
		m.requestDuration.Record(ctx, duration, metric.WithAttributes(attrs...))

		// 记录响应大小（可选）
		if m.recordResponseSize && m.responseSize != nil {
			responseSize := int64(c.Writer.Size())
			if responseSize > 0 {
				m.responseSize.Record(ctx, responseSize,
					metric.WithAttributes(
						attribute.String("method", c.Request.Method),
						attribute.String("path", path),
					),
				)
			}
		}
	}
}

// getStatusClass 获取状态码类别（降低基数）
func getStatusClass(statusCode int) string {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return "2xx"
	case statusCode >= 300 && statusCode < 400:
		return "3xx"
	case statusCode >= 400 && statusCode < 500:
		return "4xx"
	case statusCode >= 500:
		return "5xx"
	default:
		return "unknown"
	}
}

// CalculateQPS 辅助函数：通过 rate(http_requests_total[1m]) 计算 QPS（在查询侧使用）
// CalculateErrorRate 辅助函数：错误率 = rate(http_requests_total{status_code=~"5.."}[5m]) / rate(http_requests_total[5m])

