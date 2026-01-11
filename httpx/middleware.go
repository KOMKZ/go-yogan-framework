// Package httpx 提供 HTTP 请求/响应的统一处理
package httpx

import (
	"github.com/gin-gonic/gin"
)

const errorLoggingConfigKey = "httpx:error_logging_config"

// errorLoggingConfigInternal 内部配置结构（优化性能）
type errorLoggingConfigInternal struct {
	Enable          bool
	IgnoreStatusMap map[int]bool // 预处理为 map，加速查找
	FullErrorChain  bool
	LogLevel        string
}

// ErrorLoggingMiddleware 注入错误日志配置到 Context
// 使用此中间件后，HandleError 将根据配置决定是否记录日志
func ErrorLoggingMiddleware(cfg ErrorLoggingConfig) gin.HandlerFunc {
	// 预处理配置（避免每次请求重复处理）
	ignoreStatusMap := make(map[int]bool, len(cfg.IgnoreHTTPStatus))
	for _, status := range cfg.IgnoreHTTPStatus {
		ignoreStatusMap[status] = true
	}

	internalCfg := errorLoggingConfigInternal{
		Enable:          cfg.Enable,
		IgnoreStatusMap: ignoreStatusMap,
		FullErrorChain:  cfg.FullErrorChain,
		LogLevel:        cfg.LogLevel,
	}

	return func(c *gin.Context) {
		// 将预处理后的配置注入 Context
		c.Set(errorLoggingConfigKey, internalCfg)
		c.Next()
	}
}

// getErrorLoggingConfig 从 Context 读取配置
func getErrorLoggingConfig(c *gin.Context) errorLoggingConfigInternal {
	if val, exists := c.Get(errorLoggingConfigKey); exists {
		if cfg, ok := val.(errorLoggingConfigInternal); ok {
			return cfg
		}
	}

	// 默认配置：不记录日志
	return errorLoggingConfigInternal{
		Enable:          false,
		IgnoreStatusMap: make(map[int]bool),
		FullErrorChain:  true,
		LogLevel:        "error",
	}
}

