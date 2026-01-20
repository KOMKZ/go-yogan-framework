// Package httpx provides unified handling of HTTP requests/responses
package httpx

// ErrorLoggingConfig Error logging configuration
type ErrorLoggingConfig struct {
	// Enable error log recording (default false)
	Enable bool `mapstructure:"enable" json:"enable"`

	// IgnoreHTTPStatus Ignored HTTP status codes (do not log)
	// For example: []int{400, 404} indicates that errors 400 and 404 are not recorded
	IgnoreHTTPStatus []int `mapstructure:"ignore_http_status" json:"ignore_http_status"`

	// Whether to record the full error chain (default true)
	// if false, only record error_code and error_msg, do not record error_chain
	FullErrorChain bool `mapstructure:"full_error_chain" json:"full_error_chain"`

	// LogLevel log level: error, warn, info (default is error)
	LogLevel string `mapstructure:"log_level" json:"log_level"`
}

// DefaultErrorLoggingConfig returns the default configuration (logging disabled by default)
func DefaultErrorLoggingConfig() ErrorLoggingConfig {
	return ErrorLoggingConfig{
		Enable:           false, // Default off
		IgnoreHTTPStatus: []int{},
		FullErrorChain:   true,
		LogLevel:         "error",
	}
}

