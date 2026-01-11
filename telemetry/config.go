package telemetry

import (
	"fmt"
	"time"
)

// Config OpenTelemetry 配置
type Config struct {
	Enabled        bool                   `mapstructure:"enabled"`             // 是否启用
	ServiceName    string                 `mapstructure:"service_name"`        // 服务名称
	ServiceVersion string                 `mapstructure:"service_version"`     // 服务版本
	Exporter       ExporterConfig         `mapstructure:"exporter"`            // 导出器配置
	Sampler        SamplerConfig          `mapstructure:"sampler"`             // 采样配置
	ResourceAttrs  map[string]interface{} `mapstructure:"resource_attributes"` // 资源属性（支持嵌套）
	Span           SpanConfig             `mapstructure:"span"`                // Span 配置
	Batch          BatchConfig            `mapstructure:"batch"`               // 批处理配置
	CircuitBreaker CircuitBreakerConfig   `mapstructure:"circuit_breaker"`     // 熔断器配置
	Metrics        MetricsConfig          `mapstructure:"metrics"`             // 🎯 Metrics 配置
}

// ExporterConfig 导出器配置
type ExporterConfig struct {
	Type     string            `mapstructure:"type"`     // 导出类型：otlp, jaeger, stdout
	Endpoint string            `mapstructure:"endpoint"` // 导出端点
	Insecure bool              `mapstructure:"insecure"` // 是否使用不安全连接
	Timeout  time.Duration     `mapstructure:"timeout"`  // 导出超时
	Headers  map[string]string `mapstructure:"headers"`  // 🎯 自定义 Headers（用于认证等）
}

// SamplerConfig 采样配置
type SamplerConfig struct {
	Type  string  `mapstructure:"type"`  // 采样类型
	Ratio float64 `mapstructure:"ratio"` // 采样比例（仅 trace_id_ratio 时有效）
}

// SpanConfig Span 配置
type SpanConfig struct {
	MaxAttributes      int `mapstructure:"max_attributes"`       // Span 最大属性数
	MaxEvents          int `mapstructure:"max_events"`           // Span 最大事件数
	MaxLinks           int `mapstructure:"max_links"`            // Span 最大链接数
	MaxAttributeLength int `mapstructure:"max_attribute_length"` // 单个属性最大长度
}

// BatchConfig 批处理配置
type BatchConfig struct {
	Enabled            bool          `mapstructure:"enabled"`               // 是否启用批处理
	MaxQueueSize       int           `mapstructure:"max_queue_size"`        // 队列最大大小
	MaxExportBatchSize int           `mapstructure:"max_export_batch_size"` // 单次导出最大 Span 数
	ScheduleDelay      time.Duration `mapstructure:"schedule_delay"`        // 导出间隔
	ExportTimeout      time.Duration `mapstructure:"export_timeout"`        // 导出超时
}

// MetricsConfig Metrics 配置
type MetricsConfig struct {
	Enabled        bool          `mapstructure:"enabled"`         // 是否启用 Metrics
	ExportInterval time.Duration `mapstructure:"export_interval"` // 导出间隔
	ExportTimeout  time.Duration `mapstructure:"export_timeout"`  // 导出超时
	HTTP           HTTPMetrics   `mapstructure:"http"`            // HTTP 层指标配置
	Database       DBMetrics     `mapstructure:"database"`        // 数据库层指标配置
	GRPC           GRPCMetrics   `mapstructure:"grpc"`            // gRPC 层指标配置
}

// HTTPMetrics HTTP 层指标配置
type HTTPMetrics struct {
	Enabled           bool `mapstructure:"enabled"`             // 是否启用
	RecordRequestSize bool `mapstructure:"record_request_size"` // 是否记录请求大小
	RecordResponseSize bool `mapstructure:"record_response_size"` // 是否记录响应大小
}

// DBMetrics 数据库层指标配置
type DBMetrics struct {
	Enabled          bool          `mapstructure:"enabled"`           // 是否启用
	RecordSQLText    bool          `mapstructure:"record_sql_text"`   // 是否记录 SQL 文本（⚠️ 性能影响）
	SlowQuerySeconds float64       `mapstructure:"slow_query_seconds"` // 慢查询阈值（秒）
	PoolInterval     time.Duration `mapstructure:"pool_interval"`     // 连接池指标采集间隔
}

// GRPCMetrics gRPC 层指标配置
type GRPCMetrics struct {
	Enabled            bool `mapstructure:"enabled"`              // 是否启用
	RecordMessageSize  bool `mapstructure:"record_message_size"`  // 是否记录消息大小
	RecordStreamMetrics bool `mapstructure:"record_stream_metrics"` // 是否记录流式传输指标
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		Enabled:        false,
		ServiceName:    "unknown-service",
		ServiceVersion: "1.0.0",
		Exporter: ExporterConfig{
			Type:     "otlp",
			Endpoint: "localhost:4317",
			Insecure: true,
			Timeout:  10 * time.Second,
		},
		Sampler: SamplerConfig{
			Type:  "parent_based_always_on",
			Ratio: 1.0,
		},
		ResourceAttrs: make(map[string]interface{}),
		Span: SpanConfig{
			MaxAttributes:      128,
			MaxEvents:          128,
			MaxLinks:           128,
			MaxAttributeLength: 1024,
		},
		Batch: BatchConfig{
			Enabled:            true,
			MaxQueueSize:       2048,
			MaxExportBatchSize: 512,
			ScheduleDelay:      5 * time.Second,
			ExportTimeout:      30 * time.Second,
		},
		CircuitBreaker: CircuitBreakerConfig{
			Enabled:              true,
			FailureThreshold:     5,
			SuccessThreshold:     2,
			Timeout:              60 * time.Second,
			HalfOpenMaxRequests:  3,
			FallbackExporterType: "stdout",
		},
		Metrics: MetricsConfig{
			Enabled:        false, // 默认关闭
			ExportInterval: 10 * time.Second,
			ExportTimeout:  5 * time.Second,
			HTTP: HTTPMetrics{
				Enabled:            false,
				RecordRequestSize:  false,
				RecordResponseSize: false,
			},
			Database: DBMetrics{
				Enabled:          false,
				RecordSQLText:    false,
				SlowQuerySeconds: 1.0,
				PoolInterval:     30 * time.Second,
			},
			GRPC: GRPCMetrics{
				Enabled:             false,
				RecordMessageSize:   false,
				RecordStreamMetrics: false,
			},
		},
	}
}

// Validate 验证配置
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil // 未启用，无需验证
	}

	// 验证服务名称
	if c.ServiceName == "" {
		return fmt.Errorf("service_name is required when telemetry is enabled")
	}

	// 验证导出器类型
	switch c.Exporter.Type {
	case "otlp", "stdout":
		// 支持的类型
	default:
		return fmt.Errorf("unsupported exporter type: %s (supported: otlp, stdout)", c.Exporter.Type)
	}

	// 验证 OTLP 导出器端点
	if c.Exporter.Type == "otlp" && c.Exporter.Endpoint == "" {
		return fmt.Errorf("exporter endpoint is required for otlp exporter")
	}

	// 验证采样类型
	switch c.Sampler.Type {
	case "always_on", "always_off", "trace_id_ratio", "parent_based_always_on":
		// 支持的类型
	default:
		return fmt.Errorf("unsupported sampler type: %s", c.Sampler.Type)
	}

	// 验证采样比例
	if c.Sampler.Type == "trace_id_ratio" {
		if c.Sampler.Ratio < 0 || c.Sampler.Ratio > 1 {
			return fmt.Errorf("sampler ratio must be between 0 and 1, got: %f", c.Sampler.Ratio)
		}
	}

	// 验证批处理配置
	if c.Batch.Enabled {
		if c.Batch.MaxQueueSize <= 0 {
			return fmt.Errorf("batch max_queue_size must be positive, got: %d", c.Batch.MaxQueueSize)
		}
		if c.Batch.MaxExportBatchSize <= 0 {
			return fmt.Errorf("batch max_export_batch_size must be positive, got: %d", c.Batch.MaxExportBatchSize)
		}
	}

	// 验证熔断器配置
	if c.CircuitBreaker.Enabled {
		if c.CircuitBreaker.FailureThreshold <= 0 {
			return fmt.Errorf("circuit_breaker failure_threshold must be positive, got: %d", c.CircuitBreaker.FailureThreshold)
		}
		if c.CircuitBreaker.SuccessThreshold <= 0 {
			return fmt.Errorf("circuit_breaker success_threshold must be positive, got: %d", c.CircuitBreaker.SuccessThreshold)
		}
		if c.CircuitBreaker.Timeout <= 0 {
			return fmt.Errorf("circuit_breaker timeout must be positive, got: %s", c.CircuitBreaker.Timeout)
		}
		switch c.CircuitBreaker.FallbackExporterType {
		case "stdout", "noop":
			// 支持的类型
		default:
			return fmt.Errorf("unsupported fallback exporter type: %s (supported: stdout, noop)", c.CircuitBreaker.FallbackExporterType)
		}
	}

	return nil
}
