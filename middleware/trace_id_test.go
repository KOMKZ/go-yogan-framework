package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestTraceID_GenerateNew(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建测试路由
	router := gin.New()
	router.Use(TraceID(DefaultTraceConfig()))
	router.GET("/test", func(c *gin.Context) {
		// 从 gin.Context 获取 TraceID
		traceID := GetTraceID(c)
		assert.NotEmpty(t, traceID, "TraceID 不应为空")

		// 从 context.Context 获取 TraceID
		ctxTraceID := c.Request.Context().Value(TraceIDKeyDefault)
		assert.NotNil(t, ctxTraceID, "Context 中应包含 TraceID")
		assert.Equal(t, traceID, ctxTraceID, "gin.Context 和 context.Context 中的 TraceID 应一致")

		c.JSON(200, gin.H{"trace_id": traceID})
	})

	// 发起请求（不携带 TraceID Header）
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 验证响应
	assert.Equal(t, 200, w.Code)
	assert.NotEmpty(t, w.Header().Get(TraceIDHeaderDefault), "Response Header 应包含 TraceID")
}

func TestTraceID_FromHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建测试路由
	router := gin.New()
	router.Use(TraceID(DefaultTraceConfig()))
	router.GET("/test", func(c *gin.Context) {
		traceID := GetTraceID(c)
		c.JSON(200, gin.H{"trace_id": traceID})
	})

	// 发起请求（携带自定义 TraceID）
	customTraceID := "custom-trace-id-12345"
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(TraceIDHeaderDefault, customTraceID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 验证响应
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, customTraceID, w.Header().Get(TraceIDHeaderDefault), "应使用客户端传入的 TraceID")
}

func TestTraceID_DisableResponseHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建测试路由（禁用 Response Header）
	cfg := DefaultTraceConfig()
	cfg.EnableResponseHeader = false

	router := gin.New()
	router.Use(TraceID(cfg))
	router.GET("/test", func(c *gin.Context) {
		traceID := GetTraceID(c)
		c.JSON(200, gin.H{"trace_id": traceID})
	})

	// 发起请求
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 验证响应（不应包含 TraceID Header）
	assert.Equal(t, 200, w.Code)
	assert.Empty(t, w.Header().Get(TraceIDHeaderDefault), "Response Header 不应包含 TraceID")
}

func TestTraceID_CustomGenerator(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 自定义生成器
	customID := "custom-generated-id"
	cfg := DefaultTraceConfig()
	cfg.Generator = func() string {
		return customID
	}

	router := gin.New()
	router.Use(TraceID(cfg))
	router.GET("/test", func(c *gin.Context) {
		traceID := GetTraceID(c)
		assert.Equal(t, customID, traceID, "应使用自定义生成器")
		c.JSON(200, gin.H{"trace_id": traceID})
	})

	// 发起请求
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, customID, w.Header().Get(TraceIDHeaderDefault))
}

func TestTraceID_CustomKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 自定义 Key
	customKey := "request_id"
	customHeader := "X-Request-ID"
	cfg := DefaultTraceConfig()
	cfg.TraceIDKey = customKey
	cfg.TraceIDHeader = customHeader

	router := gin.New()
	router.Use(TraceID(cfg))
	router.GET("/test", func(c *gin.Context) {
		traceID := GetTraceIDWithKey(c, customKey)
		assert.NotEmpty(t, traceID, "应能用自定义 Key 获取 TraceID")
		c.JSON(200, gin.H{"trace_id": traceID})
	})

	// 发起请求
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.NotEmpty(t, w.Header().Get(customHeader), "应使用自定义 Header")
}

func TestGetTraceID_NotExists(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建未使用 TraceID 中间件的路由
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		traceID := GetTraceID(c)
		assert.Empty(t, traceID, "未使用中间件时，TraceID 应为空")
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
}

func TestTraceID_ContextPropagation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(TraceID(DefaultTraceConfig()))

	var capturedTraceID string

	router.GET("/test", func(c *gin.Context) {
		// 从 Context 获取
		ctx := c.Request.Context()
		if val := ctx.Value(TraceIDKeyDefault); val != nil {
			capturedTraceID = val.(string)
		}
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.NotEmpty(t, capturedTraceID, "Context 中应能获取到 TraceID")
}

func BenchmarkTraceID(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(TraceID(DefaultTraceConfig()))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

