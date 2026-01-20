package governance

import (
	"context"
)

// ServiceRegistry service registration interface
// Define core capabilities such as service registration, deregistration, and update
type ServiceRegistry interface {
	// Register service (block until successful or timeout)
	// Successful execution will automatically start heartbeats to keep the connection alive
	Register(ctx context.Context, info *ServiceInfo) error

	// Unregister service
	// Stop heartbeats and remove service information from the registry
	Deregister(ctx context.Context) error

	// UpdateMetadata Update service metadata
	// Additional information for dynamically updating service (such as weight, version, etc.)
	UpdateMetadata(ctx context.Context, metadata map[string]string) error

	// Checks if the service is registered
	IsRegistered() bool
}

// ServiceInfo service registration information
type ServiceInfo struct {
	// Basic information
	ServiceName string `json:"service_name"` // service name (e.g., "auth-app")
	InstanceID  string `json:"instance_id"`  // Instance ID (unique identifier, e.g., "auth-app-192.168.1.100-9002")
	Address     string `json:"address"`      // Service address (e.g., "192.168.1.100")
	Port        int    `json:"port"`         // server port (e.g., 9002)

	// Protocol information
	Protocol string `json:"protocol"` // protocol type (grpc/http/https)
	Version  string `json:"version"`  // Service version (e.g., "v1.0.0")

	// metadata
	Metadata map[string]string `json:"metadata"` // Custom metadata (such as weights, regions, etc.)

	// Health check
	HealthCheck *HealthCheckConfig `json:"health_check,omitempty"` // health check configuration

	// Registry center configuration
	TTL int64 `json:"ttl"` // Heartbeat interval (seconds)
}

// HealthCheckConfig Health check configuration
type HealthCheckConfig struct {
	Enabled  bool   `json:"enabled"`  // Whether health checks are enabled
	Interval int    `json:"interval"` // Check interval (seconds)
	Timeout  int    `json:"timeout"`  // timeout in seconds
	Path     string `json:"path"`     // HTTP health check path (such as "/health")
}

// GetFullAddress Obtain full address (address:port)
func (s *ServiceInfo) GetFullAddress() string {
	return FormatServiceAddress(s.Address, s.Port)
}

// Validate service information
func (s *ServiceInfo) Validate() error {
	if s.ServiceName == "" {
		return ErrInvalidServiceName
	}
	if s.Address == "" {
		return ErrInvalidAddress
	}
	if s.Port <= 0 || s.Port > 65535 {
		return ErrInvalidPort
	}
	if s.TTL <= 0 {
		s.TTL = 10 // Default 10 seconds
	}
	if s.Protocol == "" {
		s.Protocol = "grpc" // Default gRPC
	}
	if s.InstanceID == "" {
		// Auto-generated instance ID
		s.InstanceID = GenerateInstanceID(s.ServiceName, s.Address, s.Port)
	}
	return nil
}

