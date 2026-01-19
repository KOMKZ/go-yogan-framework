// Package component 提供组件接口定义
// 这是最底层的包，不依赖任何业务包，避免循环依赖
package component

import "context"

// HealthChecker 健康检查接口
// 组件可选实现此接口，提供健康检查能力
type HealthChecker interface {
	// Check 执行健康检查
	// 返回 nil 表示健康，返回 error 表示不健康
	Check(ctx context.Context) error

	// Name 返回检查项名称（如 "database", "redis"）
	Name() string
}

// HealthCheckProvider 健康检查提供者接口
// 组件可选实现此接口，提供健康检查器
type HealthCheckProvider interface {
	GetHealthChecker() HealthChecker
}
