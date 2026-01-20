package governance

import (
	"context"
)

// Service Discovery interface
type ServiceDiscovery interface {
	// Discover service instance list
	Discover(ctx context.Context, serviceName string) ([]*ServiceInstance, error)

	// Watch for service changes
	Watch(ctx context.Context, serviceName string) (<-chan []*ServiceInstance, error)

	// Stop Stop listening
	Stop()
}

// ServiceInstance service instance information
type ServiceInstance struct {
	ID       string            `json:"id"`       // instance ID
	Service  string            `json:"service"`  // service name
	Address  string            `json:"address"`  // IP address
	Port     int               `json:"port"`     // Port
	Metadata map[string]string `json:"metadata"` // metadata
	Weight   int               `json:"weight"`   // weight (for load balancing)
	Healthy  bool              `json:"healthy"`  // health status
}

// GetAddress Retrieve full address
func (s *ServiceInstance) GetAddress() string {
	return FormatServiceAddress(s.Address, s.Port)
}

