// Package httpx 提供 HTTP 请求/响应的统一处理
package httpx

// ErrorLoggingConfig 错误日志记录配置
type ErrorLoggingConfig struct {
	// Enable 是否启用错误日志记录（默认 false）
	Enable bool `mapstructure:"enable" json:"enable"`

	// IgnoreHTTPStatus 忽略的 HTTP 状态码（不记录日志）
	// 例如：[]int{400, 404} 表示不记录 400 和 404 错误
	IgnoreHTTPStatus []int `mapstructure:"ignore_http_status" json:"ignore_http_status"`

	// FullErrorChain 是否记录完整错误链（默认 true）
	// false 则只记录 error_code 和 error_msg，不记录 error_chain
	FullErrorChain bool `mapstructure:"full_error_chain" json:"full_error_chain"`

	// LogLevel 日志级别：error, warn, info（默认 error）
	LogLevel string `mapstructure:"log_level" json:"log_level"`
}

// DefaultErrorLoggingConfig 返回默认配置（默认不记录日志）
func DefaultErrorLoggingConfig() ErrorLoggingConfig {
	return ErrorLoggingConfig{
		Enable:           false, // 默认关闭
		IgnoreHTTPStatus: []int{},
		FullErrorChain:   true,
		LogLevel:         "error",
	}
}

