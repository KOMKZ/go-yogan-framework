// The package httpx provides unified handling of HTTP requests/responses
package httpx

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/KOMKZ/go-yogan-framework/database"
	"github.com/KOMKZ/go-yogan-framework/errcode"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Unified response format
type Response struct {
	Code    int         `json:"code"`
	Msg     string      `json:"msg,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	TraceID string      `json:"trace_id,omitempty"`
}

func traceIDFromContext(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if c.Request != nil {
		if traceID, ok := c.Request.Context().Value("trace_id").(string); ok && traceID != "" {
			return traceID
		}
	}
	if traceID, ok := c.Get("trace_id"); ok {
		if value, ok := traceID.(string); ok {
			return value
		}
	}
	return ""
}

// requestContextFields 把 request_id / user_id / client_ip 从 gin ctx 抽成 zap.Field 数组。
// 治理方案 ticket 000105 的 11 字段 schema 配套：保证错误日志能定位到具体请求上下文。
// user_id 来自中间件 c.Set("user_id", ...)（如 jwt.go:145）。
// client_ip 来自 c.ClientIP()（gin 标准）。
// request_id 与 traceIDFromContext 同源。
func requestContextFields(c *gin.Context) []zap.Field {
	if c == nil {
		return nil
	}
	fields := make([]zap.Field, 0, 3)
	if traceID := traceIDFromContext(c); traceID != "" {
		fields = append(fields, zap.String("request_id", traceID))
	}
	if v, ok := c.Get("user_id"); ok && v != nil {
		fields = append(fields, zap.Any("user_id", v))
	}
	if ip := c.ClientIP(); ip != "" {
		fields = append(fields, zap.String("client_ip", ip))
	}
	return fields
}

// OkJson successful response
func OkJson(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Msg:     "success",
		Data:    data,
		TraceID: traceIDFromContext(c),
	})
}

// ErrorJson error response (400 Bad Request)
func ErrorJson(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, Response{
		Code:    400,
		Msg:     msg,
		TraceID: traceIDFromContext(c),
	})
}

// BadRequestJson 400 error response
func BadRequestJson(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, Response{
		Code:    400,
		Msg:     err.Error(),
		TraceID: traceIDFromContext(c),
	})
}

// NotFoundJson 404 error response
func NotFoundJson(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, Response{
		Code:    404,
		Msg:     msg,
		TraceID: traceIDFromContext(c),
	})
}

// InternalErrorJson 500 error response
func InternalErrorJson(c *gin.Context, msg string) {
	c.JSON(http.StatusInternalServerError, Response{
		Code:    500,
		Msg:     msg,
		TraceID: traceIDFromContext(c),
	})
}

// NoRouteHandler 404 route not found handler
// For registering engine.NoRoute(), returns a unified JSON response format
func NoRouteHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotFound, Response{
			Code:    404,
			Msg:     "路由不存在: " + c.Request.Method + " " + c.Request.URL.Path,
			TraceID: traceIDFromContext(c),
		})
	}
}

// NoMethodHandler 405 Method Not Allowed Handler
// Used for engine.NoMethod() registration, returns a uniformly formatted JSON response
func NoMethodHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, Response{
			Code:    405,
			Msg:     "方法不允许: " + c.Request.Method + " " + c.Request.URL.Path,
			TraceID: traceIDFromContext(c),
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
	if errors.As(err, &layeredErr) && layeredErr.Code() > 0 {
		// 1.1 Decide whether to log based on configuration
		if shouldLogError(cfg, layeredErr) {
			fields := []zap.Field{
				zap.Int("error_code", layeredErr.Code()),
				zap.String("error_msg", layeredErr.Message()),
			}
			if data := layeredErr.Data(); len(data) > 0 {
				fields = append(fields, zap.Any("error_data", data))
			}
			if originStack := layeredErr.OriginStack(); originStack != "" {
				fields = append(fields, zap.String("error_origin_stack", originStack))
			}
			if operation := layeredErr.Operation(); operation != "" {
				fields = append(fields, zap.String("error_operation", operation))
			}
			diagnosticCause := layeredErr.DiagnosticCause()
			if diagnosticCause != nil {
				fields = append(fields,
					zap.String("error_cause_type", fmt.Sprintf("%T", diagnosticCause)),
					zap.String("error_cause_message", diagnosticCause.Error()),
				)
			}
			rootCause := layeredErr.RootCause()
			if rootCause != nil {
				fields = append(fields,
					zap.String("error_root_type", fmt.Sprintf("%T", rootCause)),
					zap.String("error_root_message", rootCause.Error()),
				)
			}
			if diagnosticCause != nil || rootCause != nil {
				fields = append(fields, zap.String("error_chain", layeredErr.String()))
			}

			// If the full error chain recording is configured, add details
			if cfg.FullErrorChain {
				fields = append(fields,
					zap.Error(err), // 原始错误（支持 errors.Unwrap）
				)
			}

			// 治理 ticket 000105：把 request_id / user_id / client_ip 接进来
			// 让 11 字段 schema 在线日志完整可见。
			fields = append(fields, requestContextFields(c)...)

			// Log according to the configured log level
			logMessage := "业务错误"
			switch cfg.LogLevel {
			case "warn":
				logger.WarnCtx(ctx, "httpx", logMessage, fields...)
			case "info":
				logger.InfoCtx(ctx, "httpx", logMessage, fields...)
			default: // "error"
				logger.ErrorCtx(ctx, "httpx", logMessage, fields...)
			}
		}

		// 1.2 Returns the HTTP status code, error code, and message of a LayeredError
		c.JSON(layeredErr.HTTPStatus(), Response{
			Code:    layeredErr.Code(),
			Msg:     layeredErr.Message(), // Use dynamically modified message (WithMsgf)
			Data:    layeredErr.Data(),    // Optional: return additional data
			TraceID: traceIDFromContext(c),
		})
		return
	}

	// 2. Compatibility for old error types: database record does not exist -> 404
	if errors.Is(err, database.ErrRecordNotFound) {
		if cfg.Enable {
			logger.WarnCtx(ctx, "httpx", "English: Resource does not exist", zap.Error(err))
		}
		// Fixed default message: err.Error() carries gorm internals and must
		// not be forwarded to clients (same leak pattern as branch 3).
		NotFoundJson(c, "资源不存在")
		return
	}

	// 3. Unknown error (default) -> 500 (to avoid leaking internal information)
	if cfg.Enable {
		fields := []zap.Field{
			zap.Error(err),
			zap.String("error_chain", err.Error()),
		}
		if errors.As(err, &layeredErr) {
			if originStack := layeredErr.OriginStack(); originStack != "" {
				fields = append(fields, zap.String("error_origin_stack", originStack))
			}
			if operation := layeredErr.Operation(); operation != "" {
				fields = append(fields, zap.String("error_operation", operation))
			}
			if data := layeredErr.Data(); len(data) > 0 {
				fields = append(fields, zap.Any("error_data", data))
			}
		}
		// 治理 ticket 000105：兜底分支同样带 request 上下文字段。
		fields = append(fields, requestContextFields(c)...)
		logger.ErrorCtx(ctx, "httpx", "general error", fields...)
	}
	// Return a fixed default message instead of err.Error() to avoid leaking
	// internal error details to clients (regression from f9a954c).
	InternalErrorJson(c, "内部服务器错误")
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
