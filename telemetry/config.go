package telemetry

import (
	"fmt"
	"time"
)

// Configure OpenTelemetry
type Config struct {
	Enabled        bool                   `mapstructure:"enabled"`             // Is enabled
	ServiceName    string                 `mapstructure:"service_name"`        // service name
	ServiceVersion string                 `mapstructure:"service_version"`     // service version
	Exporter       ExporterConfig         `mapstructure:"exporter"`            // exporter configuration
	Sampler        SamplerConfig          `mapstructure:"sampler"`             // Sampling configuration
	ResourceAttrs  map[string]interface{} `mapstructure:"resource_attributes"` // Resource attributes (support nesting)
	Span           SpanConfig             `mapstructure:"span"`                // Span configuration
	Batch          BatchConfig            `mapstructure:"batch"`               // Batch processing configuration
	CircuitBreaker CircuitBreakerConfig   `mapstructure:"circuit_breaker"`     // circuit breaker configuration
	Metrics        MetricsConfig          `mapstructure:"metrics"`             // 🎯 Metrics configuration
}

// ExporterConfig exporter configuration
type ExporterConfig struct {
	Type     string            `mapstructure:"type"`     // Export type: otlp, jaeger, stdout
	Endpoint string            `mapstructure:"endpoint"` // Export endpoint
	Insecure bool              `mapstructure:"insecure"` // Whether to use an insecure connection
	Timeout  time.Duration     `mapstructure:"timeout"`  // Export timeout
	Headers  map[string]string `mapstructure:"headers"`  // 🎯 Custom Headers (for authentication etc.)
}

// SamplerConfig Sampling configuration
type SamplerConfig struct {
	Type  string  `mapstructure:"type"`  // sampling type
	Ratio float64 `mapstructure:"ratio"` // Sampling ratio (effective only when using trace_id_ratio)
}

// SpanConfig Span configuration
type SpanConfig struct {
	MaxAttributes      int `mapstructure:"max_attributes"`       // Maximum number of span attributes
	MaxEvents          int `mapstructure:"max_events"`           // Maximum number of events per span
	MaxLinks           int `mapstructure:"max_links"`            // Maximum number of connections per span
	MaxAttributeLength int `mapstructure:"max_attribute_length"` // Maximum length of a single attribute
}

// BatchConfig batch processing configuration
type BatchConfig struct {
	Enabled            bool          `mapstructure:"enabled"`               // Whether batch processing is enabled
	MaxQueueSize       int           `mapstructure:"max_queue_size"`        // maximum queue size
	MaxExportBatchSize int           `mapstructure:"max_export_batch_size"` // Maximum number of Spans per export
	ScheduleDelay      time.Duration `mapstructure:"schedule_delay"`        // export interval
	ExportTimeout      time.Duration `mapstructure:"export_timeout"`        // Export timeout
}

// Metrics Configuration
type MetricsConfig struct {
	Enabled        bool              `mapstructure:"enabled"`         // Whether Metrics is enabled
	ExportInterval time.Duration     `mapstructure:"export_interval"` // export interval
	ExportTimeout  time.Duration     `mapstructure:"export_timeout"`  // Export timeout
	Namespace      string            `mapstructure:"namespace"`       // metric namespace prefix
	Labels         map[string]string `mapstructure:"labels"`          // Global tags (env, region, etc.)
	HTTP           HTTPMetrics       `mapstructure:"http"`            // HTTP layer metric configuration
	Database       DBMetrics         `mapstructure:"database"`        // Database layer metric configuration
	GRPC           GRPCMetrics       `mapstructure:"grpc"`            // gRPC layer metric configuration
	Redis          RedisMetrics      `mapstructure:"redis"`           // Redis metric configuration
	Kafka          KafkaMetrics      `mapstructure:"kafka"`           // Kafka metric configuration
	Limiter        LimiterMetrics    `mapstructure:"limiter"`         // Rate limiting configuration indicators
	Breaker        BreakerMetrics    `mapstructure:"breaker"`         // circuit breaker metric configuration
	JWT            JWTMetrics        `mapstructure:"jwt"`             // JWT metric configuration
	Auth           AuthMetrics       `mapstructure:"auth"`            // Authentication metric configuration
	Event          EventMetrics      `mapstructure:"event"`           // Event metric configuration
}

// HTTP Metrics HTTP layer metric configuration
type HTTPMetrics struct {
	Enabled           bool `mapstructure:"enabled"`             // Whether to enable
	RecordRequestSize bool `mapstructure:"record_request_size"` // Whether to log request size
	RecordResponseSize bool `mapstructure:"record_response_size"` // Whether to log response size
}

// DBMetrics database layer metric configuration
type DBMetrics struct {
	Enabled          bool          `mapstructure:"enabled"`           // Is enabled
	RecordSQLText    bool          `mapstructure:"record_sql_text"`   // Whether to log SQL text (⚠ Performance impact)
	SlowQuerySeconds float64       `mapstructure:"slow_query_seconds"` // slow query threshold (seconds)
	PoolInterval     time.Duration `mapstructure:"pool_interval"`     // pool metrics collection interval
}

// GRPCMetrics gRPC layer metric configuration
type GRPCMetrics struct {
	Enabled             bool `mapstructure:"enabled"`               // Is enabled
	RecordMessageSize   bool `mapstructure:"record_message_size"`   // Whether to log message size
	RecordStreamMetrics bool `mapstructure:"record_stream_metrics"` // Whether to log streaming metrics
}

// RedisMetrics Redis metric configuration
type RedisMetrics struct {
	Enabled          bool `mapstructure:"enabled"`            // Is Enabled
	RecordHitMiss    bool `mapstructure:"record_hit_miss"`    // Whether to log cache hits/misses
	RecordPoolStats  bool `mapstructure:"record_pool_stats"`  // Whether to log connection pool status
	RecordLatencyP99 bool `mapstructure:"record_latency_p99"` // Whether to log P99 latency
}

// Kafka Metrics Kafka metric configuration
type KafkaMetrics struct {
	Enabled        bool `mapstructure:"enabled"`         // Whether to enable
	RecordLag      bool `mapstructure:"record_lag"`      // Whether to record consumption delay
	RecordBatchSize bool `mapstructure:"record_batch_size"` // Whether to log batch size
}

// LimiterMetrics rate limiting metrics configuration
type LimiterMetrics struct {
	Enabled         bool `mapstructure:"enabled"`          // Whether to enable
	RecordTokens    bool `mapstructure:"record_tokens"`    // Whether to log token count
	RecordRejectRate bool `mapstructure:"record_reject_rate"` // Whether to record rejection rate
}

// CircuitBreaker metrics configuration
type BreakerMetrics struct {
	Enabled           bool `mapstructure:"enabled"`             // Is enabled
	RecordState       bool `mapstructure:"record_state"`        // Whether to log state changes
	RecordSuccessRate bool `mapstructure:"record_success_rate"` // Whether to log success rate
}

// JWT Metrics JWT metric configuration
type JWTMetrics struct {
	Enabled bool `mapstructure:"enabled"` // Whether to enable
}

// AuthMetrics authentication metrics configuration
type AuthMetrics struct {
	Enabled bool `mapstructure:"enabled"` // Is enabled
}

// EventMetrics event metric configuration
type EventMetrics struct {
	Enabled         bool `mapstructure:"enabled"`          // Is enabled
	RecordQueueSize bool `mapstructure:"record_queue_size"` // Whether to record queue size
}

// Return default configuration
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
			Enabled:        false, // Default off
			ExportInterval: 10 * time.Second,
			ExportTimeout:  5 * time.Second,
			Namespace:      "yogan",
			Labels:         make(map[string]string),
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
			Redis: RedisMetrics{
				Enabled:          false,
				RecordHitMiss:    true,
				RecordPoolStats:  true,
				RecordLatencyP99: true,
			},
			Kafka: KafkaMetrics{
				Enabled:         false,
				RecordLag:       true,
				RecordBatchSize: true,
			},
			Limiter: LimiterMetrics{
				Enabled:          false,
				RecordTokens:     true,
				RecordRejectRate: true,
			},
			Breaker: BreakerMetrics{
				Enabled:           false,
				RecordState:       true,
				RecordSuccessRate: true,
			},
			JWT: JWTMetrics{
				Enabled: false,
			},
			Auth: AuthMetrics{
				Enabled: false,
			},
			Event: EventMetrics{
				Enabled:         false,
				RecordQueueSize: true,
			},
		},
	}
}

// Validate configuration
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil // not enabled, verification not required
	}

	// Validate service name
	if c.ServiceName == "" {
		return fmt.Errorf("service_name is required when telemetry is enabled")
	}

	// Validate exporter type
	switch c.Exporter.Type {
	case "otlp", "stdout":
		// Supported types
	default:
		return fmt.Errorf("unsupported exporter type: %s (supported: otlp, stdout)", c.Exporter.Type)
	}

	// Verify OTLP exporter endpoint
	if c.Exporter.Type == "otlp" && c.Exporter.Endpoint == "" {
		return fmt.Errorf("exporter endpoint is required for otlp exporter")
	}

	// Validate sampling type
	switch c.Sampler.Type {
	case "always_on", "always_off", "trace_id_ratio", "parent_based_always_on":
		// Supported types
	default:
		return fmt.Errorf("unsupported sampler type: %s", c.Sampler.Type)
	}

	// Validate sampling ratio
	if c.Sampler.Type == "trace_id_ratio" {
		if c.Sampler.Ratio < 0 || c.Sampler.Ratio > 1 {
			return fmt.Errorf("sampler ratio must be between 0 and 1, got: %f", c.Sampler.Ratio)
		}
	}

	// Validate batch processing configuration
	if c.Batch.Enabled {
		if c.Batch.MaxQueueSize <= 0 {
			return fmt.Errorf("batch max_queue_size must be positive, got: %d", c.Batch.MaxQueueSize)
		}
		if c.Batch.MaxExportBatchSize <= 0 {
			return fmt.Errorf("batch max_export_batch_size must be positive, got: %d", c.Batch.MaxExportBatchSize)
		}
	}

	// Verify circuit breaker configuration
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
			// Supported types
		default:
			return fmt.Errorf("unsupported fallback exporter type: %s (supported: stdout, noop)", c.CircuitBreaker.FallbackExporterType)
		}
	}

	return nil
}
