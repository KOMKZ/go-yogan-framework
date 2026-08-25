package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	frameworkjwt "github.com/KOMKZ/go-yogan-framework/jwt"
	"github.com/KOMKZ/go-yogan-framework/limiter"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRateLimiterTokenManager struct {
	userID int64
}

func (m fakeRateLimiterTokenManager) GenerateAccessToken(context.Context, string, map[string]interface{}) (string, error) {
	return "", nil
}

func (m fakeRateLimiterTokenManager) GenerateRefreshToken(context.Context, string) (string, error) {
	return "", nil
}

func (m fakeRateLimiterTokenManager) VerifyToken(context.Context, string) (*frameworkjwt.Claims, error) {
	return &frameworkjwt.Claims{UserID: m.userID}, nil
}

func (m fakeRateLimiterTokenManager) RefreshToken(context.Context, string) (string, error) {
	return "", nil
}

func (m fakeRateLimiterTokenManager) RevokeToken(context.Context, string) error {
	return nil
}

func (m fakeRateLimiterTokenManager) RevokeUserTokens(context.Context, string) error {
	return nil
}

func setupRateLimiterTest() (*gin.Engine, *limiter.Manager) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Create rate limiter configuration
	// Note: 中间件 KeyFunc 生成的 key 格式是 "method:path"（小写），所以配置也需要用小写
	cfg := limiter.Config{
		Enabled:   true,
		StoreType: "memory",
		Default: limiter.ResourceConfig{
			Algorithm: "token_bucket",
			Rate:      10,
			Capacity:  10,
		},
		Resources: map[string]limiter.ResourceConfig{
			"get:/api/limited": {
				Algorithm:  "token_bucket",
				Rate:       2, // 2 req/s
				Capacity:   2, // Up to 2 requests
				InitTokens: 2, // Initial 2 tokens
			},
		},
	}

	log := logger.GetLogger("test")
	manager, _ := limiter.NewManagerWithLogger(cfg, log, nil, nil)

	return router, manager
}

func TestRateLimiter_Basic(t *testing.T) {
	router, manager := setupRateLimiterTest()
	defer manager.Close()

	// Add middleware
	router.Use(RateLimiter(manager))

	// Add test route
	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// The first request should succeed
	req := httptest.NewRequest("GET", "/api/test", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)

	// Subsequent requests should also succeed (default configuration limit is higher)
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

	// Add middleware
	router.Use(RateLimiter(manager))

	// Add test route
	router.GET("/api/limited", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// The first two requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/api/limited", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Code, "第%d个请求应该成功", i+1)
	}

	// The third request should be rate-limited
	req := httptest.NewRequest("GET", "/api/limited", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusTooManyRequests, resp.Code)
	assert.Contains(t, resp.Body.String(), "Rate limit exceeded")
}

func TestRateLimiter_RulePathIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	cfg := limiter.Config{
		Enabled:   true,
		StoreType: "memory",
		Rules: map[string]limiter.RuleConfig{
			"login": {
				KeyFunc: "path_ip",
				Match: []limiter.RuleMatcher{
					{Method: "POST", Path: "/api/auth/login"},
				},
				Limit: limiter.ResourceConfig{
					Algorithm:  "token_bucket",
					Rate:       1,
					Capacity:   1,
					InitTokens: 1,
				},
			},
		},
	}
	manager, err := limiter.NewManager(cfg)
	require.NoError(t, err)
	defer manager.Close()

	rateLimiterCfg := DefaultRateLimiterConfig(manager)
	rateLimiterCfg.Rules = manager.GetConfig().Rules
	router.Use(RateLimiterWithConfig(rateLimiterCfg))
	router.POST("/api/auth/login", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})
	router.POST("/api/auth/refresh", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("POST", "/api/auth/login", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	req = httptest.NewRequest("POST", "/api/auth/login", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusTooManyRequests, resp.Code)

	req = httptest.NewRequest("POST", "/api/auth/refresh", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestRateLimiter_RuleUserPathWithToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	cfg := limiter.Config{
		Enabled:   true,
		StoreType: "memory",
		Rules: map[string]limiter.RuleConfig{
			"profile": {
				KeyFunc:        "user_path",
				IdentitySource: "jwt_context_or_token",
				Match: []limiter.RuleMatcher{
					{Method: "GET", Path: "/api/profile"},
				},
				Limit: limiter.ResourceConfig{
					Algorithm:  "token_bucket",
					Rate:       1,
					Capacity:   1,
					InitTokens: 1,
				},
			},
		},
	}
	manager, err := limiter.NewManager(cfg)
	require.NoError(t, err)
	defer manager.Close()

	rateLimiterCfg := DefaultRateLimiterConfig(manager)
	rateLimiterCfg.Rules = manager.GetConfig().Rules
	rateLimiterCfg.TokenManager = fakeRateLimiterTokenManager{userID: 123}
	router.Use(RateLimiterWithConfig(rateLimiterCfg))
	router.GET("/api/profile", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/api/profile", nil)
	req.Header.Set("Authorization", "Bearer access-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)

	req = httptest.NewRequest("GET", "/api/profile", nil)
	req.Header.Set("Authorization", "Bearer access-token")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusTooManyRequests, resp.Code)
}

func TestRateLimiter_Disabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Create disabled rate limiter
	cfg := limiter.Config{
		Enabled:   false,
		StoreType: "memory",
	}

	log := logger.GetLogger("test")
	manager, _ := limiter.NewManagerWithLogger(cfg, log, nil, nil)
	defer manager.Close()

	// Add middleware
	router.Use(RateLimiter(manager))

	// Add test route
	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// All requests should succeed (rate limiter is disabled)
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

	// Custom configuration
	cfg := DefaultRateLimiterConfig(manager)
	cfg.KeyFunc = RateLimiterKeyByIP
	cfg.SkipPaths = []string{"/health"}
	cfg.RateLimitHandler = func(c *gin.Context) {
		c.JSON(http.StatusTooManyRequests, gin.H{"custom": "rate limited"})
		c.Abort()
	}

	router.Use(RateLimiterWithConfig(cfg))

	// Add test route
	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Test skip path
	req := httptest.NewRequest("GET", "/health", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)

	// Test normal path
	req = httptest.NewRequest("GET", "/api/test", nil)
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestRateLimiter_SkipFunc(t *testing.T) {
	router, manager := setupRateLimiterTest()
	defer manager.Close()

	skipCalled := false

	// Configure skip function
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

	// test skip
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

	// Use IP rate limiting
	cfg := DefaultRateLimiterConfig(manager)
	cfg.KeyFunc = RateLimiterKeyByIP
	router.Use(RateLimiterWithConfig(cfg))

	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Different IPs should have independent limits
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

	// Configure user rate limiting
	cfg := DefaultRateLimiterConfig(manager)
	cfg.KeyFunc = RateLimiterKeyByUser("user_id")

	// Set user ID first
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

	// Configure rate limiting per user (but do not set user ID)
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

	// Use key function of path + IP
	cfg := DefaultRateLimiterConfig(manager)
	cfg.KeyFunc = RateLimiterKeyByPathAndIP
	router.Use(RateLimiterWithConfig(cfg))

	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Different IPs should have independent limits
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

	// Configure rate limiting by API key
	cfg := DefaultRateLimiterConfig(manager)
	cfg.KeyFunc = RateLimiterKeyByAPIKey("X-API-Key")
	router.Use(RateLimiterWithConfig(cfg))

	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Test request with API key
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-API-Key", "test-key-123")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)

	// Test request without API key (anonymous)
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

	// Consume all tokens
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/api/limited", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		assert.Equal(t, http.StatusOK, resp.Code)
	}

	// The third request should be rate-limited
	req := httptest.NewRequest("GET", "/api/limited", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusTooManyRequests, resp.Code)

	// wait for token replenishment (2 req/s, waiting 600ms should replenish at least 1)
	time.Sleep(600 * time.Millisecond)

	// Now it should be able to make the request again
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

// TestRateLimiter_DefaultKeyFuncConsistency regression: the default config
// and the fallback path must build the same canonical key (previously one
// lowercased the method and the other did not, splitting one rate-limit
// resource into two key formats).
func TestRateLimiter_DefaultKeyFuncConsistency(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/orders", nil)

	want := "get:/api/orders"
	assert.Equal(t, want, defaultRateLimiterKeyFunc(c))
	assert.Equal(t, want, DefaultRateLimiterConfig(nil).KeyFunc(c))
}
