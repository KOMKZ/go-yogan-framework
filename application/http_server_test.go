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

// TestNewHTTPServer 测试创建 HTTP Server
func TestNewHTTPServer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := ApiServerConfig{
		Host:         "127.0.0.1",
		Port:         0, // 使用随机端口
		Mode:         "test",
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	server := NewHTTPServer(cfg, nil, nil, nil)

	assert.NotNil(t, server)
	assert.NotNil(t, server.GetEngine())
}

// TestNewHTTPServer_WithMiddleware 测试带中间件的 HTTP Server
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

// TestHTTPServer_GetEngine 测试获取 Gin Engine
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

	// 注册一个测试路由
	engine.GET("/test", func(c *gin.Context) {
		c.String(200, "OK")
	})

	// 使用 httptest 测试
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

// TestHTTPServer_Shutdown 测试关闭 Server
func TestHTTPServer_Shutdown(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := ApiServerConfig{
		Host: "127.0.0.1",
		Port: 0,
		Mode: "test",
	}

	server := NewHTTPServer(cfg, nil, nil, nil)

	// 关闭未启动的 server（应该不会 panic）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := server.Shutdown(ctx)
	// 关闭未启动的 server 可能返回错误或 nil，取决于实现
	_ = err
}

// TestHTTPServer_ShutdownWithTimeout 测试带超时的关闭
func TestHTTPServer_ShutdownWithTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := ApiServerConfig{
		Host: "127.0.0.1",
		Port: 0,
		Mode: "test",
	}

	server := NewHTTPServer(cfg, nil, nil, nil)

	err := server.ShutdownWithTimeout(5 * time.Second)
	// 关闭未启动的 server
	_ = err
}

// TestNewHTTPServerWithTelemetry 测试带遥测的 HTTP Server
func TestNewHTTPServerWithTelemetry(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := ApiServerConfig{
		Host: "127.0.0.1",
		Port: 0,
		Mode: "test",
	}

	// 不传递 telemetry（nil）
	server := NewHTTPServerWithTelemetry(cfg, nil, nil, nil, nil)

	assert.NotNil(t, server)
	assert.NotNil(t, server.GetEngine())
}

// TestNewHTTPServerWithTelemetryAndHealth 测试带遥测和健康检查的 HTTP Server
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

// TestNewHTTPServer_AllMiddleware 测试所有中间件
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

	// 测试请求
	engine := server.GetEngine()
	engine.GET("/test", func(c *gin.Context) {
		c.String(200, "OK")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
}

// TestHTTPServer_Start 测试启动服务器
func TestHTTPServer_Start(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := ApiServerConfig{
		Host: "127.0.0.1",
		Port: 0, // 使用随机端口
		Mode: "test",
	}

	server := NewHTTPServer(cfg, nil, nil, nil)

	// 启动服务器
	err := server.Start()
	assert.NoError(t, err)

	// 等待一会儿
	time.Sleep(100 * time.Millisecond)

	// 关闭服务器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(ctx)
}

// TestNewHTTPServer_WithHttpxConfig 测试带 httpx 配置的 HTTP Server
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

	// 测试 404 处理
	engine := server.GetEngine()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/not-exist", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)
}

// TestNewHTTPServer_ReleaseMode 测试 release 模式
func TestNewHTTPServer_ReleaseMode(t *testing.T) {
	cfg := ApiServerConfig{
		Host: "127.0.0.1",
		Port: 0,
		Mode: "release",
	}

	server := NewHTTPServer(cfg, nil, nil, nil)
	assert.NotNil(t, server)
}

// TestNewHTTPServer_DebugMode 测试 debug 模式
func TestNewHTTPServer_DebugMode(t *testing.T) {
	cfg := ApiServerConfig{
		Host: "127.0.0.1",
		Port: 0,
		Mode: "debug",
	}

	server := NewHTTPServer(cfg, nil, nil, nil)
	assert.NotNil(t, server)

	// 还原为 test 模式
	gin.SetMode(gin.TestMode)
}
