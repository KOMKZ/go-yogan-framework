package middleware

import (
	"strconv"
	"strings"

	"github.com/KOMKZ/go-yogan-framework/jwt"
	"github.com/KOMKZ/go-yogan-framework/permission"
	"github.com/gin-gonic/gin"
)

// PermissionGateConfig controls the global permission gate middleware behavior.
type PermissionGateConfig struct {
	PolicyEngine  permission.PolicyEngine
	TokenManager  jwt.TokenManager
	SkipPaths     []string
	DefaultPolicy permission.Decision
}

// PermissionGate performs global policy checks based on request metadata.
func PermissionGate(config PermissionGateConfig) gin.HandlerFunc {
	skipPrefixes := normalizeSkipPrefixes(config.SkipPaths)
	defaultAllow := config.DefaultPolicy != permission.Deny

	return func(c *gin.Context) {
		if shouldSkipPermission(c.Request.URL.Path, skipPrefixes) {
			c.Next()
			return
		}

		if config.PolicyEngine == nil {
			c.Next()
			return
		}

		userID, ok := resolveUserID(c, config.TokenManager)
		if !ok || userID <= 0 {
			// Authentication is still handled by JWT middleware.
			c.Next()
			return
		}

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		decision := config.PolicyEngine.Evaluate(c.Request.Context(), permission.RequestMeta{
			Method: c.Request.Method,
			Path:   path,
			UserID: userID,
		})

		switch decision {
		case permission.Allow:
			c.Next()
			return
		case permission.NotConfigured:
			if defaultAllow {
				c.Next()
				return
			}
		}

		c.JSON(403, gin.H{"code": 403, "msg": "权限不足", "data": nil})
		c.Abort()
	}
}

func resolveUserID(c *gin.Context, tokenManager jwt.TokenManager) (int64, bool) {
	if userID, ok := parseUserID(c.Get("user_id")); ok {
		return userID, true
	}

	if tokenManager == nil {
		return 0, false
	}

	token := extractBearerToken(c.GetHeader("Authorization"))
	if token == "" {
		return 0, false
	}

	claims, err := tokenManager.VerifyToken(c.Request.Context(), token)
	if err != nil || claims == nil || claims.UserID <= 0 || !isAllowedTokenType(claims.TokenType, DefaultJWTConfig.AllowedTokenTypes) {
		return 0, false
	}

	return claims.UserID, true
}

func normalizeSkipPrefixes(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}

	prefixes := make([]string, 0, len(paths))
	for _, path := range paths {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			continue
		}
		prefixes = append(prefixes, trimmed)
	}

	return prefixes
}

func shouldSkipPermission(path string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
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

func parseUserID(value interface{}, exists bool) (int64, bool) {
	if !exists || value == nil {
		return 0, false
	}

	if s, ok := value.(string); ok {
		parsed, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err == nil && parsed > 0 {
			return parsed, true
		}
	}

	switch v := value.(type) {
	case int64:
		return v, v > 0
	case int:
		return int64(v), v > 0
	case int32:
		return int64(v), v > 0
	case uint:
		if v == 0 {
			return 0, false
		}
		return int64(v), true
	case uint64:
		if v == 0 {
			return 0, false
		}
		return int64(v), true
	case uint32:
		if v == 0 {
			return 0, false
		}
		return int64(v), true
	case float64:
		if v <= 0 {
			return 0, false
		}
		return int64(v), true
	default:
		return 0, false
	}
}
