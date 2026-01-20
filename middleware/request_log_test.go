package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRequestLog_Basic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestLog())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
}

func TestRequestLog_SkipPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := DefaultRequestLogConfig()
	cfg.SkipPaths = []string{"/health", "/metrics"}

	router := gin.New()
	router.Use(RequestLogWithConfig(cfg))
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	router.GET("/api", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": "test"})
	})

	// Test skipped paths
	req1 := httptest.NewRequest("GET", "/health", nil)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	assert.Equal(t, 200, w1.Code)

	// Test normal path
	req2 := httptest.NewRequest("GET", "/api", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, 200, w2.Code)
}

func TestRequestLog_ErrorResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestLog())
	router.GET("/error", func(c *gin.Context) {
		c.JSON(500, gin.H{"error": "internal error"})
	})

	req := httptest.NewRequest("GET", "/error", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 500, w.Code)
}

func TestRequestLog_WithTraceID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(TraceID(DefaultTraceConfig()))
	router.Use(RequestLog())
	router.GET("/test", func(c *gin.Context) {
		traceID := GetTraceID(c)
		assert.NotEmpty(t, traceID, "TraceID 应存在")
		c.JSON(200, gin.H{"trace_id": traceID})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.NotEmpty(t, w.Header().Get("X-Trace-ID"), "Response 应包含 TraceID")
}

