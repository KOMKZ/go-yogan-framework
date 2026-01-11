package governance

import "errors"

// 服务注册相关错误
var (
	// ErrInvalidServiceName 无效的服务名称
	ErrInvalidServiceName = errors.New("invalid service name")

	// ErrInvalidAddress 无效的服务地址
	ErrInvalidAddress = errors.New("invalid service address")

	// ErrInvalidPort 无效的服务端口
	ErrInvalidPort = errors.New("invalid service port")

	// ErrNotRegistered 服务未注册
	ErrNotRegistered = errors.New("service not registered")

	// ErrAlreadyRegistered 服务已注册
	ErrAlreadyRegistered = errors.New("service already registered")

	// ErrRegistryUnavailable 注册中心不可用
	ErrRegistryUnavailable = errors.New("registry unavailable")

	// ErrHeartbeatFailed 心跳失败
	ErrHeartbeatFailed = errors.New("heartbeat failed")

	// ErrKeepAliveFailed 心跳保活失败
	ErrKeepAliveFailed = errors.New("keep alive failed")

	// ErrMaxRetriesExceeded 超过最大重试次数
	ErrMaxRetriesExceeded = errors.New("max retries exceeded")
)

// 健康检查相关错误
var (
	// ErrHealthCheckFailed 健康检查失败
	ErrHealthCheckFailed = errors.New("health check failed")

	// ErrHealthCheckTimeout 健康检查超时
	ErrHealthCheckTimeout = errors.New("health check timeout")
)

