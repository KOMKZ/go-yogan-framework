// Package httpx provides unified handling for HTTP requests/responses
package httpx

import (
	"github.com/gin-gonic/gin"
)

const errorLoggingConfigKey = "httpx:error_logging_config"

// internal configuration structure for error logging (to optimize performance)
type errorLoggingConfigInternal struct {
	Enable          bool
	IgnoreStatusMap map[int]bool // Preprocess as map, accelerate lookup
	FullErrorChain  bool
	LogLevel        string
}

// Inject error logging configuration into Context
// After using this middleware, HandleError will decide whether to log based on the configuration.
func ErrorLoggingMiddleware(cfg ErrorLoggingConfig) gin.HandlerFunc {
	// Preprocess configuration (avoid redundant processing for each request)
	ignoreStatusMap := make(map[int]bool, len(cfg.IgnoreHTTPStatus))
	for _, status := range cfg.IgnoreHTTPStatus {
		ignoreStatusMap[status] = true
	}

	internalCfg := errorLoggingConfigInternal{
		Enable:          true,
		IgnoreStatusMap: ignoreStatusMap,
		FullErrorChain:  cfg.FullErrorChain,
		LogLevel:        cfg.LogLevel,
	}

	return func(c *gin.Context) {
		// Inject the preprocessed configuration into the Context
		c.Set(errorLoggingConfigKey, internalCfg)
		c.Next()
	}
}

// get error logging config from context
func getErrorLoggingConfig(c *gin.Context) errorLoggingConfigInternal {
	if val, exists := c.Get(errorLoggingConfigKey); exists {
		if cfg, ok := val.(errorLoggingConfigInternal); ok {
			return cfg
		}
	}

	// Default configuration: do not log
	return errorLoggingConfigInternal{
		Enable:          false,
		IgnoreStatusMap: make(map[int]bool),
		FullErrorChain:  true,
		LogLevel:        "error",
	}
}
