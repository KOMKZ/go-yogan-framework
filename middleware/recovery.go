package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.uber.org/zap"
)

// Recovery Gin Panic 恢复中间件（结构化日志）
// 替代 gin.Recovery()，使用自定义 Logger 组件记录 Panic 日志
// 功能：
//   - 捕获 Handler 中的 panic，防止程序崩溃
//   - 记录完整的堆栈信息到日志
//   - 返回统一的 500 错误响应给客户端
//   - 不暴露敏感的堆栈信息给客户端
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 获取堆栈信息
				stack := string(debug.Stack())

				// 记录 Panic 日志（Error 级别）
				logger.Error("gin-error", "Panic recovered",
					zap.Any("error", err),
					zap.String("method", c.Request.Method),
					zap.String("path", c.Request.URL.Path),
					zap.String("client_ip", c.ClientIP()),
					zap.String("stack", stack),
				)

				// 返回统一的错误响应（不暴露堆栈信息）
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error":   "Internal Server Error",
					"message": fmt.Sprintf("%v", err),
				})
			}
		}()

		// 处理请求
		c.Next()
	}
}

