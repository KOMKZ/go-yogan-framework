// Package httpx 提供 HTTP 请求/响应的统一处理
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

// Response 统一响应格式
type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg,omitempty"`
	Data interface{} `json:"data,omitempty"`
}

// OkJson 成功响应
func OkJson(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code: 0,
		Msg:  "success",
		Data: data,
	})
}

// ErrorJson 错误响应（400 Bad Request）
func ErrorJson(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, Response{
		Code: 400,
		Msg:  msg,
	})
}

// BadRequestJson 400 错误响应
func BadRequestJson(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, Response{
		Code: 400,
		Msg:  err.Error(),
	})
}

// NotFoundJson 404 错误响应
func NotFoundJson(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, Response{
		Code: 404,
		Msg:  msg,
	})
}

// InternalErrorJson 500 错误响应
func InternalErrorJson(c *gin.Context, msg string) {
	c.JSON(http.StatusInternalServerError, Response{
		Code: 500,
		Msg:  msg,
	})
}

// NoRouteHandler 404 路由不存在处理器
// 用于 engine.NoRoute() 注册，返回统一格式的 JSON 响应
func NoRouteHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotFound, Response{
			Code: 404,
			Msg:  "路由不存在: " + c.Request.Method + " " + c.Request.URL.Path,
		})
	}
}

// NoMethodHandler 405 方法不允许处理器
// 用于 engine.NoMethod() 注册，返回统一格式的 JSON 响应
func NoMethodHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, Response{
			Code: 405,
			Msg:  "方法不允许: " + c.Request.Method + " " + c.Request.URL.Path,
		})
	}
}

// HandleError 智能处理错误（根据错误类型返回不同状态码）
// 根据文章039最佳实践 + 文章041配置化日志：
// 1. 提取 LayeredError 的错误码和消息返回给前端
// 2. 根据配置决定是否记录完整错误链到后端日志（默认不记录）
// 3. 使用 errors.Is 判断业务错误码
func HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	ctx := c.Request.Context()
	cfg := getErrorLoggingConfig(c) // 从 Context 读取配置

	// 1. 尝试提取 LayeredError
	var layeredErr *errcode.LayeredError
	if errors.As(err, &layeredErr) {
		// 1.1 根据配置决定是否记录日志
		if shouldLogError(cfg, layeredErr) {
			fields := []zap.Field{
				zap.Int("error_code", layeredErr.Code()),
				zap.String("error_msg", layeredErr.Message()),
			}

			// 如果配置了记录完整错误链，则添加详细信息
			if cfg.FullErrorChain {
				fields = append(fields,
					zap.String("error_chain", layeredErr.String()), // 完整错误链
					zap.Error(err), // 原始错误（支持 errors.Unwrap）
				)
			}

			// 根据配置的日志级别记录
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

		// 1.2 返回 LayeredError 的 HTTP 状态码、错误码、消息
		c.JSON(layeredErr.HTTPStatus(), Response{
			Code: layeredErr.Code(),
			Msg:  layeredErr.Message(), // 使用动态修改后的消息（WithMsgf）
			Data: layeredErr.Data(),    // 可选：返回附加数据
		})
		return
	}

	// 2. 兼容旧的错误类型：数据库记录不存在 → 404
	if errors.Is(err, database.ErrRecordNotFound) {
		if cfg.Enable {
			logger.WarnCtx(ctx, "httpx", "资源不存在", zap.Error(err))
		}
		NotFoundJson(c, err.Error())
		return
	}

	// 3. 未知错误（默认） → 500（避免泄露内部信息）
	if cfg.Enable {
		logger.ErrorCtx(ctx, "httpx", "未知错误",
			zap.Error(err),
			zap.String("error_chain", err.Error()),
		)
	}
	InternalErrorJson(c, "内部服务器错误")
}

// shouldLogError 根据配置判断是否应该记录日志
func shouldLogError(cfg errorLoggingConfigInternal, err *errcode.LayeredError) bool {
	// 1. 检查总开关
	if !cfg.Enable {
		return false
	}

	// 2. 检查是否在忽略列表中
	if cfg.IgnoreStatusMap[err.HTTPStatus()] {
		return false
	}

	return true
}
