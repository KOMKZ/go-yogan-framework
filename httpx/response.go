// The package httpx provides unified handling of HTTP requests/responses
package httpx

import (
	"errors"
	"net/http"

	"github.com/KOMKZ/go-yogan-framework/database"
	"github.com/KOMKZ/go-yogan-framework/errcode"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Unified response format
type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg,omitempty"`
	Data interface{} `json:"data,omitempty"`
}

// OkJson successful response
func OkJson(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code: 0,
		Msg:  "success",
		Data: data,
	})
}

// ErrorJson error response (400 Bad Request)
func ErrorJson(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, Response{
		Code: 400,
		Msg:  msg,
	})
}

// BadRequestJson 400 error response
func BadRequestJson(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, Response{
		Code: 400,
		Msg:  err.Error(),
	})
}

// NotFoundJson 404 error response
func NotFoundJson(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, Response{
		Code: 404,
		Msg:  msg,
	})
}

// InternalErrorJson 500 error response
func InternalErrorJson(c *gin.Context, msg string) {
	c.JSON(http.StatusInternalServerError, Response{
		Code: 500,
		Msg:  msg,
	})
}

// NoRouteHandler 404 route not found handler
// For registering engine.NoRoute(), returns a unified JSON response format
func NoRouteHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotFound, Response{
			Code: 404,
			Msg:  "路由不存在: " + c.Request.Method + " " + c.Request.URL.Path,
		})
	}
}

// NoMethodHandler 405 Method Not Allowed Handler
// Used for engine.NoMethod() registration, returns a uniformly formatted JSON response
func NoMethodHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, Response{
			Code: 405,
			Msg:  "方法不允许: " + c.Request.Method + " " + c.Request.URL.Path,
		})
	}
}

// HandleError intelligently handles errors (returning different status codes based on error type)
// According to Best Practices Article 039 + Configuration Logging Article 041:
// Return the error code and message of LayeredError to the frontend
// 2. Decide based on configuration whether to log the full error chain to backend logs (default is not to log)
// 3. Use errors.Is to check for business error codes
func HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	ctx := c.Request.Context()
	cfg := getErrorLoggingConfig(c) // Read configuration from Context

	// Try to extract LayeredError
	var layeredErr *errcode.LayeredError
	if errors.As(err, &layeredErr) {
		// 1.1 Decide whether to log based on configuration
		if shouldLogError(cfg, layeredErr) {
			fields := []zap.Field{
				zap.Int("error_code", layeredErr.Code()),
				zap.String("error_msg", layeredErr.Message()),
			}

			// If the full error chain recording is configured, add details
			if cfg.FullErrorChain {
				fields = append(fields,
					zap.String("error_chain", layeredErr.String()), // complete error chain
					zap.Error(err), // 原始错误（支持 errors.Unwrap）
				)
			}

			// Log according to the configured log level
			logMessage := "业务错误"
			switch cfg.LogLevel {
			case "warn":
				logger.WarnCtx(ctx, "httpx", logMessage, fields...)
			case "info":
				logger.DebugCtx(ctx, "httpx", logMessage, fields...)
			default: // "error"
				logger.ErrorCtx(ctx, "httpx", logMessage, fields...)
			}
		}

		// 1.2 Returns the HTTP status code, error code, and message of a LayeredError
		c.JSON(layeredErr.HTTPStatus(), Response{
			Code: layeredErr.Code(),
			Msg:  layeredErr.Message(), // Use dynamically modified message (WithMsgf)
			Data: layeredErr.Data(),    // Optional: return additional data
		})
		return
	}

	// 2. Compatibility for old error types: database record does not exist -> 404
	if errors.Is(err, database.ErrRecordNotFound) {
		if cfg.Enable {
			logger.WarnCtx(ctx, "httpx", "English: Resource does not exist", zap.Error(err))
		}
		NotFoundJson(c, err.Error())
		return
	}

	// 3. Unknown error (default) -> 500 (to avoid leaking internal information)
	if cfg.Enable {
		logger.ErrorCtx(ctx, "httpx", "general error",
			zap.Error(err),
			zap.String("error_chain", err.Error()),
		)
	}
	InternalErrorJson(c, err.Error())
}

// determine whether logging should occur based on configuration
func shouldLogError(cfg errorLoggingConfigInternal, err *errcode.LayeredError) bool {
	// Check master switch
	if !cfg.Enable {
		return false
	}

	// Check if in ignore list
	if cfg.IgnoreStatusMap[err.HTTPStatus()] {
		return false
	}

	return true
}
