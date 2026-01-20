package grpc

import (
	"github.com/KOMKZ/go-yogan-framework/governance"
)

// InstanceSelector instance selection interface
// Responsibility: Select one service instance from the list for connection
type InstanceSelector interface {
	// Select an instance from the list of instances
	// Return nil indicates no available instance
	Select(instances []*governance.ServiceInstance) *governance.ServiceInstance
}

// FirstHealthySelector selects the first healthy instance (default strategy)
// Applicable scenario: simple scenarios, quick response, stateless
type FirstHealthySelector struct{}

// Create the first healthy instance selector
func NewFirstHealthySelector() *FirstHealthySelector {
	return &FirstHealthySelector{}
}

// Select the first healthy instance
func (s *FirstHealthySelector) Select(instances []*governance.ServiceInstance) *governance.ServiceInstance {
	for _, inst := range instances {
		if inst.Healthy {
			return inst
		}
	}
	return nil
}

// LoadBalancerSelector Load Balancer Selector (Adapter Pattern)
// Responsibility: Reuse the algorithm implementation of governance.LoadBalancer
type LoadBalancerSelector struct {
	balancer governance.LoadBalancer
}

// Create load balancer selector
// strategy: "round_robin" | "random" | "weighted"
func NewLoadBalancerSelector(strategy string) *LoadBalancerSelector {
	return &LoadBalancerSelector{
		balancer: governance.NewLoadBalancer(strategy),
	}
}

// Select instances using load balancing algorithm
// Automatically filter unhealthy instances
func (s *LoadBalancerSelector) Select(instances []*governance.ServiceInstance) *governance.ServiceInstance {
	// Filter healthy instances
	healthy := make([]*governance.ServiceInstance, 0, len(instances))
	for _, inst := range instances {
		if inst.Healthy {
			healthy = append(healthy, inst)
		}
	}

	if len(healthy) == 0 {
		return nil
	}

	return s.balancer.Select(healthy)
}

// NewInstanceSelector creates selector based on strategy name (factory method)
func NewInstanceSelector(strategy string) InstanceSelector {
	switch strategy {
	case "first", "":
		return NewFirstHealthySelector()
	case "round_robin", "random", "weighted":
		return NewLoadBalancerSelector(strategy)
	default:
		// Unknown strategy, fallback to the first healthy instance
		return NewFirstHealthySelector()
	}
}

