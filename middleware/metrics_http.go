package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// HTTPMetricsConfig HTTP 指标配置
type HTTPMetricsConfig struct {
	Enabled            bool
	RecordRequestSize  bool
	RecordResponseSize bool
}

// HTTPMetrics HTTP 层指标收集器
// Implements component.MetricsProvider interface
type HTTPMetrics struct {
	config           HTTPMetricsConfig
	requestsTotal    metric.Int64Counter       // 请求总数
	requestDuration  metric.Float64Histogram   // 请求耗时
	requestsInFlight metric.Int64UpDownCounter // 正在处理的请求数
	requestSize      metric.Int64Histogram     // 请求大小（可选）
	responseSize     metric.Int64Histogram     // 响应大小（可选）
	registered       bool
}

// NewHTTPMetrics 创建 HTTP 指标收集器
func NewHTTPMetrics(cfg HTTPMetricsConfig) *HTTPMetrics {
	return &HTTPMetrics{
		config: cfg,
	}
}

// MetricsName returns the metrics group name
func (m *HTTPMetrics) MetricsName() string {
	return "http"
}

// IsMetricsEnabled returns whether metrics collection is enabled
func (m *HTTPMetrics) IsMetricsEnabled() bool {
	return m.config.Enabled
}

// RegisterMetrics registers all HTTP metrics with the provided Meter
func (m *HTTPMetrics) RegisterMetrics(meter metric.Meter) error {
	if m.registered {
		return nil
	}

	var err error

	m.requestsTotal, err = meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return err
	}

	m.requestDuration, err = meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("HTTP request duration distribution"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	m.requestsInFlight, err = meter.Int64UpDownCounter(
		"http_requests_in_flight",
		metric.WithDescription("Number of HTTP requests currently being processed"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return err
	}

	// Optional: record request size
	if m.config.RecordRequestSize {
		m.requestSize, err = meter.Int64Histogram(
			"http_request_size_bytes",
			metric.WithDescription("HTTP request body size distribution"),
			metric.WithUnit("By"),
		)
		if err != nil {
			return err
		}
	}

	// Optional: record response size
	if m.config.RecordResponseSize {
		m.responseSize, err = meter.Int64Histogram(
			"http_response_size_bytes",
			metric.WithDescription("HTTP response body size distribution"),
			metric.WithUnit("By"),
		)
		if err != nil {
			return err
		}
	}

	m.registered = true
	return nil
}

// IsRegistered returns whether metrics have been registered
func (m *HTTPMetrics) IsRegistered() bool {
	return m.registered
}

// Handler returns a Gin middleware for collecting HTTP metrics.
// Metrics must be registered via RegisterMetrics before calling this.
func (m *HTTPMetrics) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip if not registered
		if !m.registered {
			c.Next()
			return
		}

		start := time.Now()
		ctx := c.Request.Context()
		path := c.FullPath() // Use route pattern, not actual path (avoid high cardinality)
		if path == "" {
			path = "unknown" // 404 or unmatched route
		}

		// Request started, increment in-flight count
		m.requestsInFlight.Add(ctx, 1)
		defer m.requestsInFlight.Add(ctx, -1)

		// Record request size (optional)
		if m.config.RecordRequestSize && m.requestSize != nil {
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

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start).Seconds()
		statusCode := c.Writer.Status()

		// Common attributes
		attrs := []attribute.KeyValue{
			attribute.String("method", c.Request.Method),
			attribute.String("path", path),
			attribute.Int("status_code", statusCode),
			attribute.String("status_class", getStatusClass(statusCode)),
		}

		// Record request count
		m.requestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))

		// Record request duration
		m.requestDuration.Record(ctx, duration, metric.WithAttributes(attrs...))

		// Record response size (optional)
		if m.config.RecordResponseSize && m.responseSize != nil {
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

