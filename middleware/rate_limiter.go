package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/KOMKZ/go-yogan-framework/limiter"
	"github.com/gin-gonic/gin"
)

// RateLimiterConfig rate limiting middleware configuration
type RateLimiterConfig struct {
	// Manager Rate Limiter Manager (required)
	Manager *limiter.Manager

	// KeyFunc custom resource key generation function (default: method:path)
	KeyFunc func(*gin.Context) string

	// ErrorHandler custom error handling function (default: log errors but proceed)
	ErrorHandler func(*gin.Context, error)

	// RateLimitHandler custom rate limiting response function (default: return 429)
	RateLimitHandler func(*gin.Context)

	// SkipFunc optional function to skip rate limiting conditions
	SkipFunc func(*gin.Context) bool

	// SkipPaths list of paths to bypass rate limiting (optional)
	SkipPaths []string
}

// DefaultRateLimiterConfig default rate limiting configuration
func DefaultRateLimiterConfig(manager *limiter.Manager) RateLimiterConfig {
	return RateLimiterConfig{
		Manager: manager,
		KeyFunc: func(c *gin.Context) string {
			return fmt.Sprintf("%s:%s", strings.ToLower(c.Request.Method), c.Request.URL.Path)
		},
		ErrorHandler: func(c *gin.Context, err error) {
			// Default: Allow requests through when the rate limiter encounters an internal error (degradation strategy)
			c.Next()
		},
		RateLimitHandler: func(c *gin.Context) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "Rate limit exceeded",
				"message": "请求过于频繁，请稍后再试",
			})
			c.Abort()
		},
		SkipFunc:  nil,
		SkipPaths: []string{},
	}
}

// Create rate limiting middleware
//
// Function:
// 1. Rate limiting for requests
// Supports multiple rate limiting algorithms (Token Bucket, Sliding Window, Concurrency, Adaptive)
// Supports rate limiting by dimensions such as path, IP, user etc.
// English: Automatically allow when rate limiter is not enabled
// English: Downgrade and proceed in case of rate limiter error
//
// Usage:
//
// // Basic usage
//	engine.Use(middleware.RateLimiter(limiterManager))
//
// // Custom configuration
//	cfg := middleware.DefaultRateLimiterConfig(limiterManager)
//	cfg.KeyFunc = middleware.RateLimiterKeyByIP
//	cfg.SkipPaths = []string{"/health", "/metrics"}
//	engine.Use(middleware.RateLimiterWithConfig(cfg))
func RateLimiter(manager *limiter.Manager) gin.HandlerFunc {
	return RateLimiterWithConfig(DefaultRateLimiterConfig(manager))
}

// Creates a rate limiter middleware with custom configuration
func RateLimiterWithConfig(cfg RateLimiterConfig) gin.HandlerFunc {
	// Validate required parameters
	if cfg.Manager == nil {
		panic("RateLimiterConfig.Manager cannot be nil")
	}

	// Apply default values
	if cfg.KeyFunc == nil {
		cfg.KeyFunc = func(c *gin.Context) string {
			return fmt.Sprintf("%s:%s", c.Request.Method, c.Request.URL.Path)
		}
	}

	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = func(c *gin.Context, err error) {
			c.Next()
		}
	}

	if cfg.RateLimitHandler == nil {
		cfg.RateLimitHandler = func(c *gin.Context) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "Rate limit exceeded",
				"message": "请求过于频繁，请稍后再试",
			})
			c.Abort()
		}
	}

	// Build a map for skipping paths (improve lookup performance)
	skipPathsMap := make(map[string]bool)
	for _, path := range cfg.SkipPaths {
		skipPathsMap[path] = true
	}

	return func(c *gin.Context) {
		// ===========================
		// Check if the rate limiter is enabled
		// ===========================
		if !cfg.Manager.IsEnabled() {
			c.Next()
			return
		}

		// ===========================
		// Check if skipping this path
		// ===========================
		if skipPathsMap[c.Request.URL.Path] {
			c.Next()
			return
		}

		// ===========================
		// 3. Check custom skip conditions
		// ===========================
		if cfg.SkipFunc != nil && cfg.SkipFunc(c) {
			c.Next()
			return
		}

		// ===========================
		// 4. Generate resource keys
		// ===========================
		resource := cfg.KeyFunc(c)

		// ===========================
		// 5. Perform rate limiting check
		// ===========================
		ctx := c.Request.Context()
		allowed, err := cfg.Manager.Allow(ctx, resource)

		if err != nil {
			// Rate limiter internal error, execute error handling
			cfg.ErrorHandler(c, err)
			return
		}

		if !allowed {
			// Throttling limit reached, execute throttling response
			cfg.RateLimitHandler(c)
			return
		}

		// ===========================
		// 6. Allow passage
		// ===========================
		c.Next()
	}
}

// RateLimiterKeyByIP generates resource keys based on client IP
// Used for IP rate limiting
func RateLimiterKeyByIP(c *gin.Context) string {
	return fmt.Sprintf("ip:%s", c.ClientIP())
}

// RateLimiterKeyByUser generates resource keys based on user ID
// For rate limiting by user (requires obtaining user information from context)
//
// Usage:
//
//	cfg.KeyFunc = middleware.RateLimiterKeyByUser("user_id")
func RateLimiterKeyByUser(userIDKey string) func(*gin.Context) string {
	return func(c *gin.Context) string {
		userID, exists := c.Get(userIDKey)
		if !exists {
			return "user:anonymous"
		}
		return fmt.Sprintf("user:%v", userID)
	}
}

// RateLimiterKeyByPathAndIP generates a resource key based on path and IP
// For rate limiting by path+IP combination
func RateLimiterKeyByPathAndIP(c *gin.Context) string {
	return fmt.Sprintf("%s:%s:%s", c.Request.Method, c.Request.URL.Path, c.ClientIP())
}

// RateLimiterKeyByAPIKey generates resource keys based on API key
// Used for rate limiting by API key (requires obtaining from Header or Query)
//
// Usage:
//
//	cfg.KeyFunc = middleware.RateLimiterKeyByAPIKey("X-API-Key")
func RateLimiterKeyByAPIKey(headerName string) func(*gin.Context) string {
	return func(c *gin.Context) string {
		apiKey := c.GetHeader(headerName)
		if apiKey == "" {
			apiKey = c.Query("api_key")
		}
		if apiKey == "" {
			return "apikey:anonymous"
		}
		return fmt.Sprintf("apikey:%s", apiKey)
	}
}
