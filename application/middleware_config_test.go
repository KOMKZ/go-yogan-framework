package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMiddlewareConfig_ApplyDefaults_test_default_values_for_middleware_config
func TestMiddlewareConfig_ApplyDefaults(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		var cfg *MiddlewareConfig
		cfg.ApplyDefaults() // Should not panic
	})

	t.Run("empty CORS config", func(t *testing.T) {
		cfg := &MiddlewareConfig{
			CORS: &CORSConfig{},
		}
		cfg.ApplyDefaults()

		assert.Equal(t, []string{"*"}, cfg.CORS.AllowOrigins)
		assert.Contains(t, cfg.CORS.AllowMethods, "GET")
		assert.Contains(t, cfg.CORS.AllowMethods, "POST")
		assert.Contains(t, cfg.CORS.AllowHeaders, "Authorization")
		assert.Equal(t, 43200, cfg.CORS.MaxAge)
	})

	t.Run("empty TraceID config", func(t *testing.T) {
		cfg := &MiddlewareConfig{
			TraceID: &TraceIDConfig{},
		}
		cfg.ApplyDefaults()

		assert.Equal(t, "trace_id", cfg.TraceID.TraceIDKey)
		assert.Equal(t, "X-Trace-ID", cfg.TraceID.TraceIDHeader)
	})

	t.Run("empty RequestLog config", func(t *testing.T) {
		cfg := &MiddlewareConfig{
			RequestLog: &RequestLogConfig{},
		}
		cfg.ApplyDefaults()

		assert.Equal(t, 4096, cfg.RequestLog.MaxBodySize)
	})

	t.Run("full config with defaults", func(t *testing.T) {
		cfg := &MiddlewareConfig{
			CORS: &CORSConfig{
				Enable: true,
			},
			TraceID: &TraceIDConfig{
				Enable: true,
			},
			RequestLog: &RequestLogConfig{
				Enable: true,
			},
		}
		cfg.ApplyDefaults()

		assert.True(t, cfg.CORS.Enable)
		assert.Equal(t, []string{"*"}, cfg.CORS.AllowOrigins)
		assert.True(t, cfg.TraceID.Enable)
		assert.Equal(t, "trace_id", cfg.TraceID.TraceIDKey)
		assert.True(t, cfg.RequestLog.Enable)
		assert.Equal(t, 4096, cfg.RequestLog.MaxBodySize)
	})

	t.Run("custom values not overwritten", func(t *testing.T) {
		cfg := &MiddlewareConfig{
			CORS: &CORSConfig{
				AllowOrigins: []string{"https://example.com"},
				AllowMethods: []string{"GET"},
				AllowHeaders: []string{"X-Custom"},
				MaxAge:       3600,
			},
			TraceID: &TraceIDConfig{
				TraceIDKey:    "request_id",
				TraceIDHeader: "X-Request-ID",
			},
			RequestLog: &RequestLogConfig{
				MaxBodySize: 8192,
			},
		}
		cfg.ApplyDefaults()

		// Custom values should not be overwritten
		assert.Equal(t, []string{"https://example.com"}, cfg.CORS.AllowOrigins)
		assert.Equal(t, []string{"GET"}, cfg.CORS.AllowMethods)
		assert.Equal(t, []string{"X-Custom"}, cfg.CORS.AllowHeaders)
		assert.Equal(t, 3600, cfg.CORS.MaxAge)
		assert.Equal(t, "request_id", cfg.TraceID.TraceIDKey)
		assert.Equal(t, "X-Request-ID", cfg.TraceID.TraceIDHeader)
		assert.Equal(t, 8192, cfg.RequestLog.MaxBodySize)
	})
}

// TestCORSConfig tests the CORS configuration struct
func TestCORSConfig(t *testing.T) {
	cfg := CORSConfig{
		Enable:           true,
		AllowOrigins:     []string{"https://example.com"},
		AllowMethods:     []string{"GET", "POST"},
		AllowHeaders:     []string{"Authorization"},
		ExposeHeaders:    []string{"X-Custom-Header"},
		AllowCredentials: true,
		MaxAge:           86400,
	}

	assert.True(t, cfg.Enable)
	assert.Contains(t, cfg.AllowOrigins, "https://example.com")
	assert.True(t, cfg.AllowCredentials)
	assert.Equal(t, 86400, cfg.MaxAge)
}

// TestApiServerConfig Test API server configuration
func TestApiServerConfig(t *testing.T) {
	cfg := ApiServerConfig{
		Host:         "0.0.0.0",
		Port:         8080,
		Mode:         "release",
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	assert.Equal(t, "0.0.0.0", cfg.Host)
	assert.Equal(t, 8080, cfg.Port)
	assert.Equal(t, "release", cfg.Mode)
	assert.Equal(t, 30, cfg.ReadTimeout)
	assert.Equal(t, 30, cfg.WriteTimeout)
}
