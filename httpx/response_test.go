package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KOMKZ/go-yogan-framework/database"
	"github.com/KOMKZ/go-yogan-framework/errcode"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestOkJson 测试成功响应
func TestOkJson(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	data := map[string]string{"name": "test"}
	OkJson(c, data)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "success", resp.Msg)
	assert.NotNil(t, resp.Data)
}

// TestErrorJson 测试错误响应
func TestErrorJson(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	ErrorJson(c, "something went wrong")

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 400, resp.Code)
	assert.Equal(t, "something went wrong", resp.Msg)
}

// TestBadRequestJson 测试 400 错误响应
func TestBadRequestJson(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	BadRequestJson(c, errors.New("invalid parameter"))

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 400, resp.Code)
	assert.Equal(t, "invalid parameter", resp.Msg)
}

// TestNotFoundJson 测试 404 错误响应
func TestNotFoundJson(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	NotFoundJson(c, "resource not found")

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 404, resp.Code)
	assert.Equal(t, "resource not found", resp.Msg)
}

// TestInternalErrorJson 测试 500 错误响应
func TestInternalErrorJson(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	InternalErrorJson(c, "internal server error")

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 500, resp.Code)
	assert.Equal(t, "internal server error", resp.Msg)
}

// TestNoRouteHandler 测试 404 路由不存在处理器
func TestNoRouteHandler(t *testing.T) {
	engine := gin.New()
	engine.NoRoute(NoRouteHandler())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/nonexistent", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 404, resp.Code)
	assert.Contains(t, resp.Msg, "路由不存在")
}

// TestNoMethodHandler 测试 405 方法不允许处理器
func TestNoMethodHandler(t *testing.T) {
	engine := gin.New()
	engine.HandleMethodNotAllowed = true
	engine.NoMethod(NoMethodHandler())
	engine.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)

	var resp Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 405, resp.Code)
	assert.Contains(t, resp.Msg, "方法不允许")
}

// TestHandleError_NilError 测试 nil 错误
func TestHandleError_NilError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)

	HandleError(c, nil)

	// nil 错误不应该写入响应
	assert.Equal(t, 200, w.Code)
	assert.Empty(t, w.Body.String())
}

// TestHandleError_LayeredError 测试 LayeredError 处理
func TestHandleError_LayeredError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)

	// 使用 errcode.New: moduleCode=10, businessCode=1, module="test", msgKey="test.error", msg="参数错误", httpStatus=400
	layeredErr := errcode.New(10, 1, "test", "test.error", "参数错误", http.StatusBadRequest)
	HandleError(c, layeredErr)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 100001, resp.Code) // 10*10000 + 1 = 100001
	assert.Equal(t, "参数错误", resp.Msg)
}

// TestHandleError_LayeredError_WithLogging 测试 LayeredError 带日志配置
func TestHandleError_LayeredError_WithLogging(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)

	// 设置启用日志配置
	c.Set(errorLoggingConfigKey, errorLoggingConfigInternal{
		Enable:          true,
		IgnoreStatusMap: make(map[int]bool),
		FullErrorChain:  true,
		LogLevel:        "error",
	})

	layeredErr := errcode.New(10, 1, "test", "test.error", "参数错误", http.StatusBadRequest)
	HandleError(c, layeredErr)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestHandleError_LayeredError_WarnLevel 测试 warn 日志级别
func TestHandleError_LayeredError_WarnLevel(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)

	c.Set(errorLoggingConfigKey, errorLoggingConfigInternal{
		Enable:          true,
		IgnoreStatusMap: make(map[int]bool),
		FullErrorChain:  false,
		LogLevel:        "warn",
	})

	layeredErr := errcode.New(10, 1, "test", "test.error", "参数错误", http.StatusBadRequest)
	HandleError(c, layeredErr)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestHandleError_LayeredError_InfoLevel 测试 info 日志级别
func TestHandleError_LayeredError_InfoLevel(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)

	c.Set(errorLoggingConfigKey, errorLoggingConfigInternal{
		Enable:          true,
		IgnoreStatusMap: make(map[int]bool),
		FullErrorChain:  false,
		LogLevel:        "info",
	})

	layeredErr := errcode.New(10, 1, "test", "test.error", "参数错误", http.StatusBadRequest)
	HandleError(c, layeredErr)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestHandleError_DatabaseNotFound 测试数据库记录不存在错误
func TestHandleError_DatabaseNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)

	// 启用日志
	c.Set(errorLoggingConfigKey, errorLoggingConfigInternal{
		Enable:          true,
		IgnoreStatusMap: make(map[int]bool),
		FullErrorChain:  true,
		LogLevel:        "error",
	})

	HandleError(c, database.ErrRecordNotFound)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestHandleError_UnknownError 测试未知错误
func TestHandleError_UnknownError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)

	// 启用日志
	c.Set(errorLoggingConfigKey, errorLoggingConfigInternal{
		Enable:          true,
		IgnoreStatusMap: make(map[int]bool),
		FullErrorChain:  true,
		LogLevel:        "error",
	})

	HandleError(c, errors.New("unknown error"))

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 500, resp.Code)
	assert.Equal(t, "内部服务器错误", resp.Msg)
}

// TestShouldLogError 测试日志记录判断
func TestShouldLogError(t *testing.T) {
	tests := []struct {
		name     string
		cfg      errorLoggingConfigInternal
		err      *errcode.LayeredError
		expected bool
	}{
		{
			name: "日志关闭",
			cfg: errorLoggingConfigInternal{
				Enable:          false,
				IgnoreStatusMap: make(map[int]bool),
			},
			err:      errcode.New(10, 1, "test", "test.error", "test", http.StatusBadRequest),
			expected: false,
		},
		{
			name: "日志开启",
			cfg: errorLoggingConfigInternal{
				Enable:          true,
				IgnoreStatusMap: make(map[int]bool),
			},
			err:      errcode.New(10, 1, "test", "test.error", "test", http.StatusBadRequest),
			expected: true,
		},
		{
			name: "状态码在忽略列表中",
			cfg: errorLoggingConfigInternal{
				Enable:          true,
				IgnoreStatusMap: map[int]bool{http.StatusBadRequest: true},
			},
			err:      errcode.New(10, 1, "test", "test.error", "test", http.StatusBadRequest),
			expected: false,
		},
		{
			name: "状态码不在忽略列表中",
			cfg: errorLoggingConfigInternal{
				Enable:          true,
				IgnoreStatusMap: map[int]bool{http.StatusNotFound: true},
			},
			err:      errcode.New(10, 1, "test", "test.error", "test", http.StatusBadRequest),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldLogError(tt.cfg, tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
