package middleware

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initRequestLogTestLogger 把全局 logger 重定向到临时目录，供测试断言
// “某条访问日志是否被输出”。单目录 + 按级别命名文件，方便 glob 读取。
func initRequestLogTestLogger(t *testing.T) string {
	t.Helper()
	logDir := t.TempDir()
	fileEnabled := true
	logger.MustResetManager(logger.ManagerConfig{
		BaseLogDir:            logDir,
		Level:                 "info",
		Encoding:              "json",
		EnableConsole:         false,
		EnableFile:            &fileEnabled,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
	})
	t.Cleanup(logger.CloseAll)
	return logDir
}

// requestLogLines 读取临时目录下全部日志行。
func requestLogLines(t *testing.T, logDir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(logDir, "*.log"))
	require.NoError(t, err)
	var lines []string
	for _, file := range matches {
		data, err := os.ReadFile(file)
		require.NoError(t, err)
		lines = append(lines, strings.Split(strings.TrimSpace(string(data)), "\n")...)
	}
	return lines
}

// hasRequestLogEntry 判断是否存在同时命中 method+path+status 的访问日志行。
func hasRequestLogEntry(lines []string, method, path string, status int) bool {
	methodField := `"method":"` + method + `"`
	pathField := `"path":"` + path + `"`
	statusField := `"status":` + strconv.Itoa(status)
	for _, line := range lines {
		if strings.Contains(line, methodField) &&
			strings.Contains(line, pathField) &&
			strings.Contains(line, statusField) {
			return true
		}
	}
	return false
}

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

// TestRequestLogSkipRules_SuppressOnlyExactSuccessResponses 锁定条件跳过规则
// 的语义合同：只有 method+path 精确相等且最终状态码命中 statuses 的成功响应才
// 被抑制；同路径的其他方法、其他状态码（含 200/401/500）和相近路径照常记录。
func TestRequestLogSkipRules_SuppressOnlyExactSuccessResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logDir := initRequestLogTestLogger(t)

	cfg := DefaultRequestLogConfig()
	cfg.SkipRules = []RequestLogSkipRule{
		// 非 2xx 即使误配进 statuses 也必须保留，避免把降噪能力变成
		// 通用日志屏蔽开关。
		{Method: "POST", Path: "/api/poll", Statuses: []int{204, 302, 401, 500}},
	}

	router := gin.New()
	router.Use(RequestLogWithConfig(cfg))
	router.GET("/api/poll", func(c *gin.Context) { c.Status(204) })
	router.POST("/api/poll", func(c *gin.Context) {
		switch c.Query("result") {
		case "empty":
			c.Status(204)
		case "claimed":
			c.JSON(200, gin.H{"task": "t1"})
		case "denied":
			c.JSON(401, gin.H{"error": "unauthorized"})
		case "redirect":
			c.Status(302)
		default:
			c.JSON(500, gin.H{"error": "boom"})
		}
	})
	router.POST("/api/poll/extra", func(c *gin.Context) { c.Status(204) })

	perform := func(method, target string) {
		req := httptest.NewRequest(method, target, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}

	// 规则命中：POST /api/poll = 204（空轮询成功）——唯一的抑制目标
	perform("POST", "/api/poll?result=empty")
	// 同路径其他最终状态码不被成功规则吞掉
	perform("POST", "/api/poll?result=claimed")
	perform("POST", "/api/poll?result=denied")
	perform("POST", "/api/poll?result=redirect")
	perform("POST", "/api/poll?result=fail")
	// 同路径不同 method、相近不同 path 仍记录
	perform("GET", "/api/poll")
	perform("POST", "/api/poll/extra")

	lines := requestLogLines(t, logDir)
	assert.False(t, hasRequestLogEntry(lines, "POST", "/api/poll", 204),
		"命中精确成功规则的响应不应输出访问日志")
	assert.True(t, hasRequestLogEntry(lines, "POST", "/api/poll", 200),
		"同路径非规则状态码（如 claim=200）必须保留")
	assert.True(t, hasRequestLogEntry(lines, "POST", "/api/poll", 401),
		"认证失败不能被成功规则吞掉")
	assert.True(t, hasRequestLogEntry(lines, "POST", "/api/poll", 302),
		"重定向不能被成功规则吞掉")
	assert.True(t, hasRequestLogEntry(lines, "POST", "/api/poll", 500),
		"服务端错误不能被成功规则吞掉")
	assert.True(t, hasRequestLogEntry(lines, "GET", "/api/poll", 204),
		"同路径不同 method 即使状态码相同也要记录")
	assert.True(t, hasRequestLogEntry(lines, "POST", "/api/poll/extra", 204),
		"规则只做精确路径匹配，相近路径不继承抑制")
}

// TestRequestLogSkipRules_MethodComparisonIsCaseInsensitive 锁定方法比较不区分
// 大小写：规则声明为小写 get，实际 GET 请求同样命中。
func TestRequestLogSkipRules_MethodComparisonIsCaseInsensitive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logDir := initRequestLogTestLogger(t)

	cfg := DefaultRequestLogConfig()
	cfg.SkipRules = []RequestLogSkipRule{
		{Method: "get", Path: "/api/poll", Statuses: []int{200}},
	}

	router := gin.New()
	router.Use(RequestLogWithConfig(cfg))
	router.GET("/api/poll", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	req := httptest.NewRequest("GET", "/api/poll", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	lines := requestLogLines(t, logDir)
	assert.False(t, hasRequestLogEntry(lines, "GET", "/api/poll", 200),
		"方法比较不区分大小写，小写规则也应命中")
}

// TestRequestLogSkipPaths_StillSkipsEntirePath 锁定旧 skip_paths 语义不回归：
// 在请求执行前整路径跳过，无论最终状态码（含 5xx）都不输出访问日志。
func TestRequestLogSkipPaths_StillSkipsEntirePath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logDir := initRequestLogTestLogger(t)

	cfg := DefaultRequestLogConfig()
	cfg.SkipPaths = []string{"/health"}

	router := gin.New()
	router.Use(RequestLogWithConfig(cfg))
	router.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	router.GET("/api/data", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	perform := func(target string) {
		req := httptest.NewRequest("GET", target, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
	perform("/health")
	perform("/api/data")

	lines := requestLogLines(t, logDir)
	assert.False(t, hasRequestLogEntry(lines, "GET", "/health", 200),
		"skip_paths 整路径跳过语义不变")
	assert.True(t, hasRequestLogEntry(lines, "GET", "/api/data", 200),
		"未配置路径照常记录")
}
