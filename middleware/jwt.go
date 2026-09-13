package middleware

import (
	"fmt"
	"strings"

	"github.com/KOMKZ/go-yogan-framework/jwt"
	"github.com/gin-gonic/gin"
)

// JWTConfig JWT middleware configuration
type JWTConfig struct {
	// Skip function for middleware
	Skipper func(*gin.Context) bool

	// Token lookup position (format: header:Authorization)
	TokenLookup string

	// TokenHeadName Token prefix (such as "Bearer")
	TokenHeadName string

	// Error handler function
	ErrorHandler func(*gin.Context, error)

	// AllowedTokenTypes limits which token_type values can pass this middleware.
	// Empty uses the framework default: access tokens only.
	AllowedTokenTypes []string
}

// Default JWT Configuration
var DefaultJWTConfig = JWTConfig{
	Skipper:       nil,
	TokenLookup:   "header:Authorization",
	TokenHeadName: "Bearer",
	ErrorHandler:  defaultJWTErrorHandler,
	AllowedTokenTypes: []string{
		"access",
	},
}

// Create JWT middleware (using default configuration)
func JWT(tokenManager jwt.TokenManager) gin.HandlerFunc {
	return JWTWithConfig(tokenManager, DefaultJWTConfig)
}

// CookieTokenLookup builds a JWT token lookup expression for a cookie name.
func CookieTokenLookup(cookieName string) string {
	return "cookie:" + strings.TrimSpace(cookieName)
}

// CookieJWTConfig creates a JWT config that reads access tokens from an HttpOnly cookie.
func CookieJWTConfig(cookieName string) JWTConfig {
	return JWTConfig{
		TokenLookup:   CookieTokenLookup(cookieName),
		TokenHeadName: "",
	}
}

// CookieJWT requires a valid access token from the named cookie.
func CookieJWT(tokenManager jwt.TokenManager, cookieName string) gin.HandlerFunc {
	return JWTWithConfig(tokenManager, CookieJWTConfig(cookieName))
}

// OptionalCookieJWT injects claims from the named cookie when present.
func OptionalCookieJWT(tokenManager jwt.TokenManager, cookieName string) gin.HandlerFunc {
	return OptionalJWT(tokenManager, CookieJWTConfig(cookieName))
}

// OptionalJWT validates a token when present and injects claims into context.
// Missing or invalid tokens are ignored so downstream auth middleware can decide.
func OptionalJWT(tokenManager jwt.TokenManager, config JWTConfig) gin.HandlerFunc {
	return jwtMiddleware(tokenManager, config, true)
}

// Create JWT middleware with custom configuration
func JWTWithConfig(tokenManager jwt.TokenManager, config JWTConfig) gin.HandlerFunc {
	return jwtMiddleware(tokenManager, config, false)
}

func jwtMiddleware(tokenManager jwt.TokenManager, config JWTConfig, optional bool) gin.HandlerFunc {
	// Set default values
	if config.TokenLookup == "" {
		config.TokenLookup = DefaultJWTConfig.TokenLookup
	}
	if config.TokenHeadName == "" {
		config.TokenHeadName = DefaultJWTConfig.TokenHeadName
	}
	if config.ErrorHandler == nil {
		config.ErrorHandler = DefaultJWTConfig.ErrorHandler
	}
	if len(config.AllowedTokenTypes) == 0 {
		config.AllowedTokenTypes = DefaultJWTConfig.AllowedTokenTypes
	}

	return func(c *gin.Context) {
		// Check if skip
		if config.Skipper != nil && config.Skipper(c) {
			c.Next()
			return
		}

		// Extract Token
		token, err := extractToken(c, config.TokenLookup, config.TokenHeadName)
		if err != nil {
			if optional {
				c.Next()
				return
			}
			config.ErrorHandler(c, err)
			return
		}

		// Validate Token
		ctx := c.Request.Context()
		claims, err := tokenManager.VerifyToken(ctx, token)
		if err != nil {
			if optional {
				c.Next()
				return
			}
			config.ErrorHandler(c, err)
			return
		}
		if !isAllowedTokenType(claims.TokenType, config.AllowedTokenTypes) {
			if optional {
				c.Next()
				return
			}
			config.ErrorHandler(c, fmt.Errorf("jwt: token type %q is not allowed", claims.TokenType))
			return
		}

		InjectJWTClaims(c, claims)

		c.Next()
	}
}

// InjectJWTClaims stores verified JWT claims in gin.Context using framework keys.
func InjectJWTClaims(c *gin.Context, claims *jwt.Claims) {
	if claims == nil {
		return
	}
	c.Set("jwt_claims", claims)
	c.Set("user_id", claims.UserID)
	c.Set("username", claims.Username)
	c.Set("roles", claims.Roles)
	c.Set("sid", claims.SID)
}

func isAllowedTokenType(tokenType string, allowed []string) bool {
	for _, item := range allowed {
		if strings.EqualFold(strings.TrimSpace(item), tokenType) {
			return true
		}
	}
	return false
}

// extractToken: Extract the Token from the request
func extractToken(c *gin.Context, tokenLookup, tokenHeadName string) (string, error) {
	// Parse TokenLookup (format: header:Authorization)
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

	// Remove token prefix (e.g., "Bearer ")
	if tokenHeadName != "" {
		prefix := tokenHeadName + " "
		if strings.HasPrefix(token, prefix) {
			token = strings.TrimPrefix(token, prefix)
		}
	}

	return token, nil
}

func extractBearerToken(header string) string {
	value := strings.TrimSpace(header)
	if value == "" {
		return ""
	}

	parts := strings.SplitN(value, " ", 2)
	if len(parts) != 2 {
		return value
	}

	if strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}

	return value
}

// defaultJWTErrorHandler default error handling
func defaultJWTErrorHandler(c *gin.Context, err error) {
	// Return 401 error
	c.JSON(401, gin.H{
		"code":    401,
		"message": "Unauthorized",
		"error":   err.Error(),
	})
	c.Abort()
}

// GetClaims retrieves JWT Claims from Context
func GetClaims(c *gin.Context) (*jwt.Claims, bool) {
	claims, exists := c.Get("jwt_claims")
	if !exists {
		return nil, false
	}
	jwtClaims, ok := claims.(*jwt.Claims)
	return jwtClaims, ok
}

// Get user ID from context
func GetUserID(c *gin.Context) (int64, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}
	id, ok := userID.(int64)
	return id, ok
}

// Get username from context
func GetUsername(c *gin.Context) (string, bool) {
	username, exists := c.Get("username")
	if !exists {
		return "", false
	}
	name, ok := username.(string)
	return name, ok
}

// Check if the user has the specified role
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
