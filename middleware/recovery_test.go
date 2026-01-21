package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRecovery_NoPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.Use(Recovery())
	router.GET("/normal", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/normal", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "success")
}

func TestRecovery_WithPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 初始化 logger
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "recovery")
	logger.MustResetManager(logger.ManagerConfig{
		BaseLogDir:            logDir,
		Level:                 "debug",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
	})
	defer logger.CloseAll()

	router := gin.New()
	router.Use(Recovery())
	router.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Contains(t, resp.Body.String(), "Internal Server Error")
	assert.Contains(t, resp.Body.String(), "test panic")
}

func TestRecovery_WithPanicError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 初始化 logger
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "recovery_error")
	logger.MustResetManager(logger.ManagerConfig{
		BaseLogDir:            logDir,
		Level:                 "debug",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
	})
	defer logger.CloseAll()

	router := gin.New()
	router.Use(Recovery())
	router.GET("/panic-error", func(c *gin.Context) {
		panic("runtime error: index out of range")
	})

	req := httptest.NewRequest("GET", "/panic-error", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)

	// 验证日志文件存在
	_, err := os.Stat(filepath.Join(logDir, "gin-error"))
	assert.NoError(t, err)
}
