package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORSConfig CORS middleware configuration
type CORSConfig struct {
	// AllowOrigins list of allowed sources (default ["*"])
	// Example: ["https://example.com", "https://app.example.com"]
	AllowOrigins []string

	// AllowMethods list of allowed HTTP methods (default ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"])
	AllowMethods []string

	// AllowHeaders list of allowed request headers (default ["Origin", "Content-Type", "Accept", "Authorization"])
	AllowHeaders []string

	// ExposeHeaders list of response headers exposed to the client (default [])
	ExposeHeaders []string

	// AllowCredentials whether to allow sending credentials (such as Cookies, HTTP authentication, etc.) (default false)
	// Note: When set to true, AllowOrigins cannot use "*"
	AllowCredentials bool

	// MaxAge preflight request cache time (seconds) (default 43200, i.e., 12 hours)
	MaxAge int
}

// DefaultCORSConfig default CORS configuration
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{},
		AllowCredentials: false,
		MaxAge:           43200, // 12 hours
	}
}

// Create CORS middleware (using default configuration)
//
// Function:
// - Handle Cross-Origin Resource Sharing (CORS)
// - Automatically respond to OPTIONS preflight requests
// - Set CORS related response headers
//
// Usage:
//   engine.Use(middleware.CORS())
func CORS() gin.HandlerFunc {
	return CORSWithConfig(DefaultCORSConfig())
}

// CORSWithConfig creates CORS middleware (custom configuration)
func CORSWithConfig(cfg CORSConfig) gin.HandlerFunc {
	// Apply default values
	if len(cfg.AllowOrigins) == 0 {
		cfg.AllowOrigins = []string{"*"}
	}
	if len(cfg.AllowMethods) == 0 {
		cfg.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	}
	if len(cfg.AllowHeaders) == 0 {
		cfg.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	}
	if cfg.MaxAge == 0 {
		cfg.MaxAge = 43200
	}

	// Preprocess configuration (convert to string)
	allowMethodsStr := strings.Join(cfg.AllowMethods, ", ")
	allowHeadersStr := strings.Join(cfg.AllowHeaders, ", ")
	exposeHeadersStr := strings.Join(cfg.ExposeHeaders, ", ")

	return func(c *gin.Context) {
		// Get the request's Origin
		origin := c.Request.Header.Get("Origin")

		// ===========================
		// Check if Origin is allowed
		// ===========================
		allowOrigin := ""
		if len(cfg.AllowOrigins) == 1 && cfg.AllowOrigins[0] == "*" {
			// Allow all sources
			allowOrigin = "*"
		} else if origin != "" {
			// Check if in allowed list
			for _, allowedOrigin := range cfg.AllowOrigins {
				if allowedOrigin == origin {
					allowOrigin = origin
					break
				}
			}
		}

		// If Origin is not allowed and is not a wildcard, skip CORS handling
		if allowOrigin == "" && origin != "" {
			c.Next()
			return
		}

		// ===========================
		// Set CORS response headers
		// ===========================
		if allowOrigin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		}

		c.Writer.Header().Set("Access-Control-Allow-Methods", allowMethodsStr)
		c.Writer.Header().Set("Access-Control-Allow-Headers", allowHeadersStr)

		if len(cfg.ExposeHeaders) > 0 {
			c.Writer.Header().Set("Access-Control-Expose-Headers", exposeHeadersStr)
		}

		if cfg.AllowCredentials {
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// ===========================
		// Handle OPTIONS preflight requests
		// ===========================
		if c.Request.Method == "OPTIONS" {
			// Set preflight request cache time
			c.Writer.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", cfg.MaxAge))
			
			// directly return 204 No Content
			c.AbortWithStatus(204)
			return
		}

		// Proceed to handle the request
		c.Next()
	}
}

