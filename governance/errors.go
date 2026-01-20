package governance

import "errors"

// Service registration related errors
var (
	// ErrInvalidServiceName Invalid service name
	ErrInvalidServiceName = errors.New("invalid service name")

	// ErrInvalidAddress Invalid service address
	ErrInvalidAddress = errors.New("invalid service address")

	// ErrInvalidPort Invalid service port
	ErrInvalidPort = errors.New("invalid service port")

	// ErrNotRegistered service not registered
	ErrNotRegistered = errors.New("service not registered")

	// ErrAlreadyRegistered Service already registered
	ErrAlreadyRegistered = errors.New("service already registered")

	// ErrRegistryUnavailable Registry unavailable
	ErrRegistryUnavailable = errors.New("registry unavailable")

	// ErrHeartbeatFailed Heartbeat failed
	ErrHeartbeatFailed = errors.New("heartbeat failed")

	// Heartbeat keep-alive failed
	ErrKeepAliveFailed = errors.New("keep alive failed")

	// ErrMaxRetriesExceeded Maximum retries exceeded
	ErrMaxRetriesExceeded = errors.New("max retries exceeded")
)

// Health check related errors
var (
	// ErrHealthCheckFailed Health check failed
	ErrHealthCheckFailed = errors.New("health check failed")

	// ErrHealthCheckTimeout Health check timeout
	ErrHealthCheckTimeout = errors.New("health check timeout")
)

