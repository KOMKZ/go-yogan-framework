package limiter

import "errors"

var (
	// ErrLimitExceeded 超过限流阈值
	ErrLimitExceeded = errors.New("rate limit exceeded")

	// ErrWaitTimeout 等待超时
	ErrWaitTimeout = errors.New("wait timeout")

	// ErrKeyNotFound 键不存在
	ErrKeyNotFound = errors.New("key not found")

	// ErrInvalidConfig 配置无效
	ErrInvalidConfig = errors.New("invalid config")

	// ErrStoreNotSupported 存储不支持
	ErrStoreNotSupported = errors.New("store operation not supported")

	// ErrResourceNotFound 资源不存在
	ErrResourceNotFound = errors.New("resource not found")
)

// ValidationError 配置验证错误
type ValidationError struct {
	Resource string
	Field    string
	Message  string
	Err      error
}

func (e *ValidationError) Error() string {
	if e.Resource != "" {
		if e.Err != nil {
			return "limiter config validation failed for resource '" + e.Resource + "': " + e.Err.Error()
		}
		return "limiter config validation failed for resource '" + e.Resource + "." + e.Field + "': " + e.Message
	}

	if e.Field != "" {
		return "limiter config validation failed for field '" + e.Field + "': " + e.Message
	}

	if e.Err != nil {
		return "limiter config validation failed: " + e.Err.Error()
	}

	return "limiter config validation failed"
}

