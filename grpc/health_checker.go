package grpc

import (
	"context"
	"fmt"
)

// HealthChecker gRPC health checker
type HealthChecker struct {
	server        *Server
	clientManager *ClientManager
}

// Create gRPC health checker
func NewHealthChecker(server *Server, clientManager *ClientManager) *HealthChecker {
	return &HealthChecker{
		server:        server,
		clientManager: clientManager,
	}
}

// Name Check item name
func (h *HealthChecker) Name() string {
	return "grpc"
}

// Check execution health check
func (h *HealthChecker) Check(ctx context.Context) error {
	// The gRPC component mainly checks if it has been properly initialized.
	// The Server and ClientManager may be nil (depending on the configuration)
	
	if h.server == nil && h.clientManager == nil {
		return fmt.Errorf("grpc component not initialized")
	}

	// The health status of the gRPC Server is determined by whether it successfully starts up
	// gRPC client health status is determined by whether a connection can be successfully established
	// Here only basic checks are performed; actual RPC calls are not executed.

	return nil
}

