package middleware

import (
	"strings"

	"github.com/KOMKZ/go-yogan-framework/jwt"
	"github.com/gin-gonic/gin"
)

// JWTConfig JWT 中间件配置
type JWTConfig struct {
	// Skipper 跳过中间件的函数
	Skipper func(*gin.Context) bool
	
	// TokenLookup Token 查找位置（格式：header:Authorization）
	TokenLookup string
	
	// TokenHeadName Token 前缀（如 "Bearer"）
	TokenHeadName string
	
	// ErrorHandler 错误处理函数
	ErrorHandler func(*gin.Context, error)
}

// DefaultJWTConfig 默认配置
var DefaultJWTConfig = JWTConfig{
	Skipper:       nil,
	TokenLookup:   "header:Authorization",
	TokenHeadName: "Bearer",
	ErrorHandler:  defaultJWTErrorHandler,
}

// JWT 创建 JWT 中间件（使用默认配置）
func JWT(tokenManager jwt.TokenManager) gin.HandlerFunc {
	return JWTWithConfig(tokenManager, DefaultJWTConfig)
}

// JWTWithConfig 创建 JWT 中间件（自定义配置）
func JWTWithConfig(tokenManager jwt.TokenManager, config JWTConfig) gin.HandlerFunc {
	// 设置默认值
	if config.TokenLookup == "" {
		config.TokenLookup = DefaultJWTConfig.TokenLookup
	}
	if config.TokenHeadName == "" {
		config.TokenHeadName = DefaultJWTConfig.TokenHeadName
	}
	if config.ErrorHandler == nil {
		config.ErrorHandler = DefaultJWTConfig.ErrorHandler
	}

	return func(c *gin.Context) {
		// 检查是否跳过
		if config.Skipper != nil && config.Skipper(c) {
			c.Next()
			return
		}

		// 提取 Token
		token, err := extractToken(c, config.TokenLookup, config.TokenHeadName)
		if err != nil {
			config.ErrorHandler(c, err)
			return
		}

		// 验证 Token
		ctx := c.Request.Context()
		claims, err := tokenManager.VerifyToken(ctx, token)
		if err != nil {
			config.ErrorHandler(c, err)
			return
		}

		// 将 Claims 注入到 Context
		c.Set("jwt_claims", claims)
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("roles", claims.Roles)

		c.Next()
	}
}

// extractToken 从请求中提取 Token
func extractToken(c *gin.Context, tokenLookup, tokenHeadName string) (string, error) {
	// 解析 TokenLookup（格式：header:Authorization）
	parts := strings.Split(tokenLookup, ":")
	if len(parts) != 2 {
		return "", jwt.ErrTokenMissing
	}

	source := parts[0]
	name := parts[1]

	var token string
	switch source {
	case "header":
		token = c.GetHeader(name)
	case "query":
		token = c.Query(name)
	case "cookie":
		token, _ = c.Cookie(name)
	default:
		return "", jwt.ErrTokenMissing
	}

	if token == "" {
		return "", jwt.ErrTokenMissing
	}

	// 移除 Token 前缀（如 "Bearer "）
	if tokenHeadName != "" {
		prefix := tokenHeadName + " "
		if strings.HasPrefix(token, prefix) {
			token = strings.TrimPrefix(token, prefix)
		}
	}

	return token, nil
}

// defaultJWTErrorHandler 默认错误处理
func defaultJWTErrorHandler(c *gin.Context, err error) {
	// 返回 401 错误
	c.JSON(401, gin.H{
		"code":    401,
		"message": "Unauthorized",
		"error":   err.Error(),
	})
	c.Abort()
}

// GetClaims 从 Context 获取 JWT Claims
func GetClaims(c *gin.Context) (*jwt.Claims, bool) {
	claims, exists := c.Get("jwt_claims")
	if !exists {
		return nil, false
	}
	jwtClaims, ok := claims.(*jwt.Claims)
	return jwtClaims, ok
}

// GetUserID 从 Context 获取用户 ID
func GetUserID(c *gin.Context) (int64, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}
	id, ok := userID.(int64)
	return id, ok
}

// GetUsername 从 Context 获取用户名
func GetUsername(c *gin.Context) (string, bool) {
	username, exists := c.Get("username")
	if !exists {
		return "", false
	}
	name, ok := username.(string)
	return name, ok
}

// HasRole 检查用户是否拥有指定角色
func HasRole(c *gin.Context, role string) bool {
	roles, exists := c.Get("roles")
	if !exists {
		return false
	}
	roleList, ok := roles.([]string)
	if !ok {
		return false
	}
	for _, r := range roleList {
		if r == role {
			return true
		}
	}
	return false
}

