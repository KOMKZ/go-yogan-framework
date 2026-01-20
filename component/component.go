// Package component provides interface definitions for components
// This is the lowest level package, which does not depend on any business packages to avoid circular dependencies.
package component

import "context"

// HealthChecker health check interface
// Components optionally implement this interface to provide health check capabilities
type HealthChecker interface {
	// Check execution health status
	// Return nil indicates healthy, return error indicates unhealthy
	Check(ctx context.Context) error

	// Name returns the check item name (e.g., "database", "redis")
	Name() string
}

// HealthCheckProvider health check provider interface
// Components optionally implement this interface to provide health checkers
type HealthCheckProvider interface {
	GetHealthChecker() HealthChecker
}
