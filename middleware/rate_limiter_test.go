package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/limiter"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRateLimiterTest() (*gin.Engine, *limiter.Manager) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// 创建限流器配置
	cfg := limiter.Config{
		Enabled:   true,
		StoreType: "memory",
		Default: limiter.ResourceConfig{
			Algorithm: "token_bucket",
			Rate:      10,
			Capacity:  10,
		},
		Resources: map[string]limiter.ResourceConfig{
			"GET:/api/limited": {
				Algorithm:  "token_bucket",
				Rate:       2, // 2 req/s
				Capacity:   2, // 最多2个请求
				InitTokens: 2, // 初始2个令牌
			},
		},
	}

	log := logger.GetLogger("test")
	manager, _ := limiter.NewManagerWithLogger(cfg, log, nil)

	return router, manager
}

func TestRateLimiter_Basic(t *testing.T) {
	router, manager := setupRateLimiterTest()
	defer manager.Close()

	// 添加中间件
	router.Use(RateLimiter(manager))

	// 添加测试路由
	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// 第一个请求应该成功
	req := httptest.NewRequest("GET", "/api/test", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)

	// 后续请求也应该成功（默认配置限额较大）
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/api/test", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		assert.Equal(t, http.StatusOK, resp.Code)
	}
}

func TestRateLimiter_RateLimited(t *testing.T) {
	router, manager := setupRateLimiterTest()
	defer manager.Close()

	// 添加中间件
	router.Use(RateLimiter(manager))

	// 添加测试路由
	router.GET("/api/limited", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// 前2个请求应该成功
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/api/limited", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code, "第%d个请求应该成功", i+1)
	}

	// 第3个请求应该被限流
	req := httptest.NewRequest("GET", "/api/limited", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusTooManyRequests, resp.Code)
	assert.Contains(t, resp.Body.String(), "Rate limit exceeded")
}

func TestRateLimiter_Disabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// 创建禁用的限流器
	cfg := limiter.Config{
		Enabled:   false,
		StoreType: "memory",
	}

	log := logger.GetLogger("test")
	manager, _ := limiter.NewManagerWithLogger(cfg, log, nil)
	defer manager.Close()

	// 添加中间件
	router.Use(RateLimiter(manager))

	// 添加测试路由
	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// 所有请求都应该成功（限流器已禁用）
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("GET", "/api/test", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		assert.Equal(t, http.StatusOK, resp.Code)
	}
}

func TestRateLimiter_WithConfig(t *testing.T) {
	router, manager := setupRateLimiterTest()
	defer manager.Close()

	// 自定义配置
	cfg := DefaultRateLimiterConfig(manager)
	cfg.KeyFunc = RateLimiterKeyByIP
	cfg.SkipPaths = []string{"/health"}
	cfg.RateLimitHandler = func(c *gin.Context) {
		c.JSON(http.StatusTooManyRequests, gin.H{"custom": "rate limited"})
		c.Abort()
	}

	router.Use(RateLimiterWithConfig(cfg))

	// 添加测试路由
	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 测试跳过路径
	req := httptest.NewRequest("GET", "/health", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)

	// 测试正常路径
	req = httptest.NewRequest("GET", "/api/test", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestRateLimiter_SkipFunc(t *testing.T) {
	router, manager := setupRateLimiterTest()
	defer manager.Close()

	skipCalled := false

	// 配置跳过函数
	cfg := DefaultRateLimiterConfig(manager)
	cfg.SkipFunc = func(c *gin.Context) bool {
		if c.GetHeader("X-Skip-Rate-Limit") == "true" {
			skipCalled = true
			return true
		}
		return false
	}

	router.Use(RateLimiterWithConfig(cfg))

	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// 测试跳过
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-Skip-Rate-Limit", "true")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.True(t, skipCalled)
}

func TestRateLimiter_KeyByIP(t *testing.T) {
	router, manager := setupRateLimiterTest()
	defer manager.Close()

	// 使用按IP限流
	cfg := DefaultRateLimiterConfig(manager)
	cfg.KeyFunc = RateLimiterKeyByIP
	router.Use(RateLimiterWithConfig(cfg))

	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// 不同IP应该有独立的限额
	ips := []string{"192.168.1.1:12345", "192.168.1.2:12345", "192.168.1.3:12345"}
	for _, ip := range ips {
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.RemoteAddr = ip
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		assert.Equal(t, http.StatusOK, resp.Code)
	}
}

func TestRateLimiter_KeyByUser(t *testing.T) {
	router, manager := setupRateLimiterTest()
	defer manager.Close()

	// 配置按用户限流
	cfg := DefaultRateLimiterConfig(manager)
	cfg.KeyFunc = RateLimiterKeyByUser("user_id")

	// 先设置用户ID
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "user123")
		c.Next()
	})
	router.Use(RateLimiterWithConfig(cfg))

	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestRateLimiter_KeyByUser_Anonymous(t *testing.T) {
	router, manager := setupRateLimiterTest()
	defer manager.Close()

	// 配置按用户限流（但不设置用户ID）
	cfg := DefaultRateLimiterConfig(manager)
	cfg.KeyFunc = RateLimiterKeyByUser("user_id")
	router.Use(RateLimiterWithConfig(cfg))

	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestRateLimiter_KeyByPathAndIP(t *testing.T) {
	router, manager := setupRateLimiterTest()
	defer manager.Close()

	// 使用路径+IP的键函数
	cfg := DefaultRateLimiterConfig(manager)
	cfg.KeyFunc = RateLimiterKeyByPathAndIP
	router.Use(RateLimiterWithConfig(cfg))

	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// 不同IP应该有独立的限额
	ips := []string{"192.168.1.1:12345", "192.168.1.2:12345", "192.168.1.3:12345"}
	for _, ip := range ips {
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.RemoteAddr = ip
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		assert.Equal(t, http.StatusOK, resp.Code)
	}
}

func TestRateLimiter_KeyByAPIKey(t *testing.T) {
	router, manager := setupRateLimiterTest()
	defer manager.Close()

	// 配置按API Key限流
	cfg := DefaultRateLimiterConfig(manager)
	cfg.KeyFunc = RateLimiterKeyByAPIKey("X-API-Key")
	router.Use(RateLimiterWithConfig(cfg))

	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// 测试带API Key的请求
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-API-Key", "test-key-123")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)

	// 测试不带API Key的请求（匿名）
	req = httptest.NewRequest("GET", "/api/test", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestRateLimiter_RefillTokens(t *testing.T) {
	router, manager := setupRateLimiterTest()
	defer manager.Close()

	router.Use(RateLimiter(manager))

	router.GET("/api/limited", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// 消耗所有令牌
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/api/limited", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		assert.Equal(t, http.StatusOK, resp.Code)
	}

	// 第3个请求应该被限流
	req := httptest.NewRequest("GET", "/api/limited", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusTooManyRequests, resp.Code)

	// 等待令牌补充（2 req/s，等待600ms应该补充至少1个）
	time.Sleep(600 * time.Millisecond)

	// 现在应该可以再次请求
	req = httptest.NewRequest("GET", "/api/limited", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestRateLimiter_PanicOnNilManager(t *testing.T) {
	cfg := RateLimiterConfig{
		Manager: nil,
	}

	assert.Panics(t, func() {
		RateLimiterWithConfig(cfg)
	})
}

