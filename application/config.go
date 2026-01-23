package application

import (
	"fmt"

	"github.com/KOMKZ/go-yogan-framework/httpx"
	"github.com/KOMKZ/go-yogan-framework/logger"
)

// AppConfig framework configuration (contains only framework-level configurations)
//
// Note: Business configurations (such as Database, Redis) are no longer included.
// The business component should read configurations from ConfigLoader itself.
type AppConfig struct {
	// Required configuration - value type (the application must be configured)
	ApiServer ApiServerConfig `mapstructure:"api_server"`

	// Optional configuration - pointer (has default value or can be left unconfigured)
	Logger     *logger.ManagerConfig       `mapstructure:"logger,omitempty"`
	Middleware *MiddlewareConfig           `mapstructure:"middleware,omitempty"` // middleware configuration
	Httpx      *httpx.ErrorLoggingConfig   `mapstructure:"httpx,omitempty"`      // HTTP error handling configuration
}

// ApiServerConfig HTTP API server configuration
type ApiServerConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Mode         string `mapstructure:"mode"` // debug, release, test
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
}

// MiddlewareConfig middleware configuration
type MiddlewareConfig struct {
	CORS       *CORSConfig            `mapstructure:"cors,omitempty"`
	TraceID    *TraceIDConfig         `mapstructure:"trace_id,omitempty"`
	RequestLog *RequestLogConfig      `mapstructure:"request_log,omitempty"`
	Metrics    *MiddlewareMetricsConfig `mapstructure:"metrics,omitempty"`
}

// MiddlewareMetricsConfig HTTP metrics middleware configuration
type MiddlewareMetricsConfig struct {
	// Enabled whether to enable HTTP metrics collection (default false)
	Enabled bool `mapstructure:"enabled"`
	// RecordRequestSize whether to record request body size
	RecordRequestSize bool `mapstructure:"record_request_size"`
	// RecordResponseSize whether to record response body size
	RecordResponseSize bool `mapstructure:"record_response_size"`
}

// TraceIDConfig Trace ID middleware configuration
type TraceIDConfig struct {
	// Enable whether to use the TraceID middleware (default true)
	Enable bool `mapstructure:"enable"`

	// TraceIDKey stored in Context (default "trace_id")
	TraceIDKey string `mapstructure:"trace_id_key"`

	// TraceIDHeader is the key in the HTTP Header (default "X-Trace-ID")
	TraceIDHeader string `mapstructure:"trace_id_header"`

	// EnableResponseHeader whether to write TraceID into Response Header (default true)
	EnableResponseHeader bool `mapstructure:"enable_response_header"`
}

// RequestLogConfig HTTP request log middleware configuration
type RequestLogConfig struct {
	// Enable request log middleware (default true)
	Enable bool `mapstructure:"enable"`

	// SkipPaths list of paths to skip recording (e.g., health checks)
	SkipPaths []string `mapstructure:"skip_paths"`

	// EnableBody whether to log request body (default false, for performance considerations)
	EnableBody bool `mapstructure:"enable_body"`

	// MaxBodySize maximum request body recording size (bytes, default 4KB)
	MaxBodySize int `mapstructure:"max_body_size"`
}

// CORSConfig CORS cross-domain middleware configuration
type CORSConfig struct {
	// Enable whether to use the CORS middleware (default false)
	Enable bool `mapstructure:"enable"`

	// AllowOrigins list of allowed sources (default ["*"])
	// Example: ["https://example.com", "https://app.example.com"]
	AllowOrigins []string `mapstructure:"allow_origins"`

	// AllowMethods list of allowed HTTP methods (default ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"])
	AllowMethods []string `mapstructure:"allow_methods"`

	// AllowHeaders list of allowed request headers (default ["Origin", "Content-Type", "Accept", "Authorization"])
	AllowHeaders []string `mapstructure:"allow_headers"`

	// ExposeHeaders list of response headers exposed to the client (default [])
	ExposeHeaders []string `mapstructure:"expose_headers"`

	// Whether to allow sending credentials (such as Cookies, HTTP authentication, etc.) (default is false)
	// Note: When set to true, AllowOrigins cannot use "*"
	AllowCredentials bool `mapstructure:"allow_credentials"`

	// MaxAge pre-check request cache time (seconds) (default 43200, i.e., 12 hours)
	MaxAge int `mapstructure:"max_age"`
}


// ApplyDefaults Apply default values
func (c *MiddlewareConfig) ApplyDefaults() {
	if c == nil {
		return
	}

	// CORS default values
	if c.CORS != nil {
		if len(c.CORS.AllowOrigins) == 0 {
			c.CORS.AllowOrigins = []string{"*"}
		}
		if len(c.CORS.AllowMethods) == 0 {
			c.CORS.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
		}
		if len(c.CORS.AllowHeaders) == 0 {
			c.CORS.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
		}
		if c.CORS.MaxAge == 0 {
			c.CORS.MaxAge = 43200 // 12 hours
		}
	}

	// Trace ID default value
	if c.TraceID != nil {
		if c.TraceID.TraceIDKey == "" {
			c.TraceID.TraceIDKey = "trace_id"
		}
		if c.TraceID.TraceIDHeader == "" {
			c.TraceID.TraceIDHeader = "X-Trace-ID"
		}
	}

	// Default value for RequestLog
	if c.RequestLog != nil {
		if c.RequestLog.MaxBodySize == 0 {
			c.RequestLog.MaxBodySize = 4096 // 4KB
		}
	}
}

// LoadAppConfig load framework configuration
func (a *Application) LoadAppConfig() (*AppConfig, error) {
	// Retrieve ConfigLoader from the registry center
	loader := a.GetConfigLoader()
	if loader == nil {
		return nil, fmt.Errorf("Configuration loader uninitialized")
	}

	var cfg AppConfig
	if err := loader.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// Apply middleware configuration default values
	if cfg.Middleware != nil {
		cfg.Middleware.ApplyDefaults()
	}

	return &cfg, nil
}
