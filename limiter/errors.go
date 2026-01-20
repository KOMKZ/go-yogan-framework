package limiter

import "errors"

var (
	// ErrLimitExceeded Exceeds rate limiting threshold
	ErrLimitExceeded = errors.New("rate limit exceeded")

	// ErrWaitTimeout timeout waiting
	ErrWaitTimeout = errors.New("wait timeout")

	// ErrKeyNotFound Key does not exist
	ErrKeyNotFound = errors.New("key not found")

	// ErrInvalidConfig Invalid configuration
	ErrInvalidConfig = errors.New("invalid config")

	// ErrStoreNotSupported Storage Not Supported
	ErrStoreNotSupported = errors.New("store operation not supported")

	// ErrResourceNotFound Resource not found
	ErrResourceNotFound = errors.New("resource not found")
)

// ValidationError configuration validation error
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

