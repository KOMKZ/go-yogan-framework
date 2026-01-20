package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestErrorLoggingMiddleware 测试错误日志中间件
func TestErrorLoggingMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := ErrorLoggingConfig{
		Enable:           true,
		IgnoreHTTPStatus: []int{400, 404},
		FullErrorChain:   true,
		LogLevel:         "warn",
	}

	engine := gin.New()
	engine.Use(ErrorLoggingMiddleware(cfg))
	engine.GET("/test", func(c *gin.Context) {
		// 验证配置是否正确注入
		internalCfg := getErrorLoggingConfig(c)
		assert.True(t, internalCfg.Enable)
		assert.True(t, internalCfg.IgnoreStatusMap[400])
		assert.True(t, internalCfg.IgnoreStatusMap[404])
		assert.False(t, internalCfg.IgnoreStatusMap[500])
		assert.True(t, internalCfg.FullErrorChain)
		assert.Equal(t, "warn", internalCfg.LogLevel)
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
}

// TestErrorLoggingMiddleware_EmptyIgnoreList 测试空忽略列表
func TestErrorLoggingMiddleware_EmptyIgnoreList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := ErrorLoggingConfig{
		Enable:           true,
		IgnoreHTTPStatus: []int{},
		FullErrorChain:   false,
		LogLevel:         "error",
	}

	engine := gin.New()
	engine.Use(ErrorLoggingMiddleware(cfg))
	engine.GET("/test", func(c *gin.Context) {
		internalCfg := getErrorLoggingConfig(c)
		assert.True(t, internalCfg.Enable)
		assert.Empty(t, internalCfg.IgnoreStatusMap)
		assert.False(t, internalCfg.FullErrorChain)
		assert.Equal(t, "error", internalCfg.LogLevel)
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
}

// TestGetErrorLoggingConfig_NoMiddleware 测试没有中间件时的默认配置
func TestGetErrorLoggingConfig_NoMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	cfg := getErrorLoggingConfig(c)

	assert.False(t, cfg.Enable)
	assert.Empty(t, cfg.IgnoreStatusMap)
	assert.True(t, cfg.FullErrorChain)
	assert.Equal(t, "error", cfg.LogLevel)
}

// TestGetErrorLoggingConfig_InvalidType 测试配置类型错误
func TestGetErrorLoggingConfig_InvalidType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// 设置错误类型的配置
	c.Set(errorLoggingConfigKey, "invalid type")

	cfg := getErrorLoggingConfig(c)

	// 应该返回默认配置
	assert.False(t, cfg.Enable)
	assert.Empty(t, cfg.IgnoreStatusMap)
	assert.True(t, cfg.FullErrorChain)
	assert.Equal(t, "error", cfg.LogLevel)
}
