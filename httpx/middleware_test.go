package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestErrorLoggingMiddleware tests error logging middleware
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
		// Verify that the configuration has been correctly injected
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

// TestErrorLoggingMiddleware_EmptyIgnoreList_test empty ignore list
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

// TestGetErrorLoggingConfig_NoMiddleware test default configuration without middleware
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

// TestGetErrorLoggingConfig_InvalidType test configuration type error
func TestGetErrorLoggingConfig_InvalidType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Set error type configuration
	c.Set(errorLoggingConfigKey, "invalid type")

	cfg := getErrorLoggingConfig(c)

	// Should return default configuration
	assert.False(t, cfg.Enable)
	assert.Empty(t, cfg.IgnoreStatusMap)
	assert.True(t, cfg.FullErrorChain)
	assert.Equal(t, "error", cfg.LogLevel)
}
