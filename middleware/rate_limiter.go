package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/KOMKZ/go-yogan-framework/limiter"
	"github.com/gin-gonic/gin"
)

// RateLimiterConfig 限流中间件配置
type RateLimiterConfig struct {
	// Manager 限流器管理器（必需）
	Manager *limiter.Manager

	// KeyFunc 自定义资源键生成函数（默认：method:path）
	KeyFunc func(*gin.Context) string

	// ErrorHandler 自定义错误处理函数（默认：记录错误但放行）
	ErrorHandler func(*gin.Context, error)

	// RateLimitHandler 自定义限流响应函数（默认：返回 429）
	RateLimitHandler func(*gin.Context)

	// SkipFunc 跳过限流的条件函数（可选）
	SkipFunc func(*gin.Context) bool

	// SkipPaths 跳过限流的路径列表（可选）
	SkipPaths []string
}

// DefaultRateLimiterConfig 默认限流配置
func DefaultRateLimiterConfig(manager *limiter.Manager) RateLimiterConfig {
	return RateLimiterConfig{
		Manager: manager,
		KeyFunc: func(c *gin.Context) string {
			return fmt.Sprintf("%s:%s", strings.ToLower(c.Request.Method), c.Request.URL.Path)
		},
		ErrorHandler: func(c *gin.Context, err error) {
			// 默认：限流器内部错误时放行请求（降级策略）
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

// RateLimiter 创建限流中间件
//
// 功能：
//  1. 对请求进行限流控制
//  2. 支持多种限流算法（Token Bucket、Sliding Window、Concurrency、Adaptive）
//  3. 支持按路径、IP、用户等维度限流
//  4. 限流器未启用时自动放行
//  5. 限流器错误时降级放行
//
// 用法：
//
//	// 基本用法
//	engine.Use(middleware.RateLimiter(limiterManager))
//
//	// 自定义配置
//	cfg := middleware.DefaultRateLimiterConfig(limiterManager)
//	cfg.KeyFunc = middleware.RateLimiterKeyByIP
//	cfg.SkipPaths = []string{"/health", "/metrics"}
//	engine.Use(middleware.RateLimiterWithConfig(cfg))
func RateLimiter(manager *limiter.Manager) gin.HandlerFunc {
	return RateLimiterWithConfig(DefaultRateLimiterConfig(manager))
}

// RateLimiterWithConfig 创建自定义配置的限流中间件
func RateLimiterWithConfig(cfg RateLimiterConfig) gin.HandlerFunc {
	// 验证必需参数
	if cfg.Manager == nil {
		panic("RateLimiterConfig.Manager cannot be nil")
	}

	// 应用默认值
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

	// 构建跳过路径的 map（提高查找性能）
	skipPathsMap := make(map[string]bool)
	for _, path := range cfg.SkipPaths {
		skipPathsMap[path] = true
	}

	return func(c *gin.Context) {
		// ===========================
		// 1. 检查限流器是否启用
		// ===========================
		if !cfg.Manager.IsEnabled() {
			c.Next()
			return
		}

		// ===========================
		// 2. 检查是否跳过此路径
		// ===========================
		if skipPathsMap[c.Request.URL.Path] {
			c.Next()
			return
		}

		// ===========================
		// 3. 检查自定义跳过条件
		// ===========================
		if cfg.SkipFunc != nil && cfg.SkipFunc(c) {
			c.Next()
			return
		}

		// ===========================
		// 4. 生成资源键
		// ===========================
		resource := cfg.KeyFunc(c)

		// ===========================
		// 5. 执行限流检查
		// ===========================
		ctx := c.Request.Context()
		allowed, err := cfg.Manager.Allow(ctx, resource)

		if err != nil {
			// 限流器内部错误，执行错误处理
			cfg.ErrorHandler(c, err)
			return
		}

		if !allowed {
			// 被限流，执行限流响应
			cfg.RateLimitHandler(c)
			return
		}

		// ===========================
		// 6. 允许通过
		// ===========================
		c.Next()
	}
}

// RateLimiterKeyByIP 根据客户端IP生成资源键
// 用于按IP限流
func RateLimiterKeyByIP(c *gin.Context) string {
	return fmt.Sprintf("ip:%s", c.ClientIP())
}

// RateLimiterKeyByUser 根据用户ID生成资源键
// 用于按用户限流（需要从上下文获取用户信息）
//
// 用法：
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

// RateLimiterKeyByPathAndIP 根据路径和IP生成资源键
// 用于按路径+IP组合限流
func RateLimiterKeyByPathAndIP(c *gin.Context) string {
	return fmt.Sprintf("%s:%s:%s", c.Request.Method, c.Request.URL.Path, c.ClientIP())
}

// RateLimiterKeyByAPIKey 根据API Key生成资源键
// 用于按API Key限流（需要从Header或Query获取）
//
// 用法：
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
