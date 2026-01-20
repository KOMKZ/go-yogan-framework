package application

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/httpx"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestNewHTTPServer test creating HTTP server
func TestNewHTTPServer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := ApiServerConfig{
		Host:         "127.0.0.1",
		Port:         0, // Use random port
		Mode:         "test",
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	server := NewHTTPServer(cfg, nil, nil, nil)

	assert.NotNil(t, server)
	assert.NotNil(t, server.GetEngine())
}

// TestNewHTTPServer_WithMiddleware test HTTP server with middleware
func TestNewHTTPServer_WithMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := ApiServerConfig{
		Host: "127.0.0.1",
		Port: 0,
		Mode: "test",
	}

	middlewareCfg := &MiddlewareConfig{
		CORS: &CORSConfig{
			Enable:       true,
			AllowOrigins: []string{"*"},
		},
		TraceID: &TraceIDConfig{
			Enable:        true,
			TraceIDKey:    "trace_id",
			TraceIDHeader: "X-Trace-ID",
		},
		RequestLog: &RequestLogConfig{
			Enable:      true,
			SkipPaths:   []string{"/health"},
			MaxBodySize: 4096,
		},
	}

	server := NewHTTPServer(cfg, middlewareCfg, nil, nil)

	assert.NotNil(t, server)
	assert.NotNil(t, server.GetEngine())
}

// TestHTTPServer_GetEngine test getting Gin Engine
func TestHTTPServer_GetEngine(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := ApiServerConfig{
		Host: "127.0.0.1",
		Port: 0,
		Mode: "test",
	}

	server := NewHTTPServer(cfg, nil, nil, nil)
	engine := server.GetEngine()

	assert.NotNil(t, engine)

	// Register a test route
	engine.GET("/test", func(c *gin.Context) {
		c.String(200, "OK")
	})

	// Use httptest for testing
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

// TestHTTPServer_Shutdown test server shutdown
func TestHTTPServer_Shutdown(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := ApiServerConfig{
		Host: "127.0.0.1",
		Port: 0,
		Mode: "test",
	}

	server := NewHTTPServer(cfg, nil, nil, nil)

	// shut down unstarted server (should not cause panic)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := server.Shutdown(ctx)
	// Shutting down an uninitialized server may return an error or nil, depending on the implementation
	_ = err
}

// TestHTTPServer_ShutdownWithTimeout Tests shutdown with timeout
func TestHTTPServer_ShutdownWithTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := ApiServerConfig{
		Host: "127.0.0.1",
		Port: 0,
		Mode: "test",
	}

	server := NewHTTPServer(cfg, nil, nil, nil)

	err := server.ShutdownWithTimeout(5 * time.Second)
	// shut down unstarted server
	_ = err
}

// TestNewHTTPServerWithTelemetry test new HTTP server with telemetry
func TestNewHTTPServerWithTelemetry(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := ApiServerConfig{
		Host: "127.0.0.1",
		Port: 0,
		Mode: "test",
	}

	// Do not pass telemetry (nil)
	server := NewHTTPServerWithTelemetry(cfg, nil, nil, nil, nil)

	assert.NotNil(t, server)
	assert.NotNil(t, server.GetEngine())
}

// TestNewHTTPServerWithTelemetryAndHealth test new HTTP server with telemetry and health checks
func TestNewHTTPServerWithTelemetryAndHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := ApiServerConfig{
		Host: "127.0.0.1",
		Port: 0,
		Mode: "test",
	}

	server := NewHTTPServerWithTelemetryAndHealth(cfg, nil, nil, nil, nil, nil)

	assert.NotNil(t, server)
	assert.NotNil(t, server.GetEngine())
}

// TestNewHTTPServer_AllMiddleware_TestingAllMiddlewares
func TestNewHTTPServer_AllMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := ApiServerConfig{
		Host: "127.0.0.1",
		Port: 0,
		Mode: "test",
	}

	middlewareCfg := &MiddlewareConfig{
		CORS: &CORSConfig{
			Enable:           true,
			AllowOrigins:     []string{"https://example.com"},
			AllowMethods:     []string{"GET", "POST"},
			AllowHeaders:     []string{"Authorization"},
			AllowCredentials: true,
		},
		TraceID: &TraceIDConfig{
			Enable:               true,
			TraceIDKey:           "trace_id",
			TraceIDHeader:        "X-Trace-ID",
			EnableResponseHeader: true,
		},
		RequestLog: &RequestLogConfig{
			Enable:      true,
			SkipPaths:   []string{"/health", "/metrics"},
			EnableBody:  true,
			MaxBodySize: 8192,
		},
	}

	server := NewHTTPServer(cfg, middlewareCfg, nil, nil)

	assert.NotNil(t, server)

	// Test request
	engine := server.GetEngine()
	engine.GET("/test", func(c *gin.Context) {
		c.String(200, "OK")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
}

// TestHTTPServer_Start Test server startup
func TestHTTPServer_Start(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := ApiServerConfig{
		Host: "127.0.0.1",
		Port: 0, // Use random port
		Mode: "test",
	}

	server := NewHTTPServer(cfg, nil, nil, nil)

	// Start server
	err := server.Start()
	assert.NoError(t, err)

	// wait for a moment
	time.Sleep(100 * time.Millisecond)

	// Shut down server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(ctx)
}

// TestNewHTTPServer_WithHttpxConfig test HTTP Server with httpx configuration
func TestNewHTTPServer_WithHttpxConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := ApiServerConfig{
		Host: "127.0.0.1",
		Port: 0,
		Mode: "test",
	}

	middlewareCfg := &MiddlewareConfig{
		RequestLog: &RequestLogConfig{
			Enable:      true,
			EnableBody:  true,
			MaxBodySize: 1024,
		},
	}

	httpxCfg := &httpx.ErrorLoggingConfig{
		Enable: true,
	}

	server := NewHTTPServer(cfg, middlewareCfg, httpxCfg, nil)

	assert.NotNil(t, server)
	assert.NotNil(t, server.GetEngine())

	// Test 404 handling
	engine := server.GetEngine()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/not-exist", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)
}

// TestNewHTTPServer_ReleaseMode test release mode
func TestNewHTTPServer_ReleaseMode(t *testing.T) {
	cfg := ApiServerConfig{
		Host: "127.0.0.1",
		Port: 0,
		Mode: "release",
	}

	server := NewHTTPServer(cfg, nil, nil, nil)
	assert.NotNil(t, server)
}

// TestNewHTTPServer_DebugMode test debug mode
func TestNewHTTPServer_DebugMode(t *testing.T) {
	cfg := ApiServerConfig{
		Host: "127.0.0.1",
		Port: 0,
		Mode: "debug",
	}

	server := NewHTTPServer(cfg, nil, nil, nil)
	assert.NotNil(t, server)

	// Revert to test mode
	gin.SetMode(gin.TestMode)
}
