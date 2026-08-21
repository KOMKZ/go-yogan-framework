package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/KOMKZ/go-yogan-framework/httpx"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.uber.org/zap"
)

// Recovery from Gin panic (structured logging)
// Replace gin.Recovery() with a custom Logger component to log panic logs
// Function:
// - Catch panics in the handler to prevent program crashes
// - Log complete stack information
// - Return a unified 500 error response to the client
// - Do not expose sensitive stack information to clients
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Get stack information
				stack := string(debug.Stack())

				// Record Panic log (Error level)
				logger.Error("gin-error", "Panic recovered",
					zap.Any("error", err),
					zap.String("method", c.Request.Method),
					zap.String("path", c.Request.URL.Path),
					zap.String("client_ip", c.ClientIP()),
					zap.String("stack", stack),
				)

				// Return the site-wide unified response with a fixed default
				// message; panic details (err) stay in the log only to avoid
				// leaking internals (SQL, paths, variables) to clients.
				c.AbortWithStatusJSON(http.StatusInternalServerError, httpx.Response{
					Code: 500,
					Msg:  "内部服务器错误",
				})
			}
		}()

		// Handle request
		c.Next()
	}
}

