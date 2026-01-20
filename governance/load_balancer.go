package governance

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// LoadBalancer interface
type LoadBalancer interface {
	// Select a service instance
	Select(instances []*ServiceInstance) *ServiceInstance

	// Name Load balancer name
	Name() string
}

// RoundRobinBalancer round-robin load balancer
type RoundRobinBalancer struct {
	counter uint64
}

// Create round-robin load balancer
func NewRoundRobinBalancer() *RoundRobinBalancer {
	return &RoundRobinBalancer{}
}

// Select poll for instance choice
func (b *RoundRobinBalancer) Select(instances []*ServiceInstance) *ServiceInstance {
	if len(instances) == 0 {
		return nil
	}

	// Atomic increment counter
	idx := atomic.AddUint64(&b.counter, 1) - 1
	return instances[int(idx)%len(instances)]
}

// Name Load Balancer Name
func (b *RoundRobinBalancer) Name() string {
	return "round_robin"
}

// RandomBalancer random load balancer
type RandomBalancer struct {
	rand *rand.Rand
	mu   sync.Mutex
}

// Create random load balancer
func NewRandomBalancer() *RandomBalancer {
	return &RandomBalancer{
		rand: rand.New(rand.NewSource(int64(time.Now().UnixNano()))),
	}
}

// Select randomly choose instance
func (b *RandomBalancer) Select(instances []*ServiceInstance) *ServiceInstance {
	if len(instances) == 0 {
		return nil
	}

	b.mu.Lock()
	idx := b.rand.Intn(len(instances))
	b.mu.Unlock()

	return instances[idx]
}

// Name Load Balancer Name
func (b *RandomBalancer) Name() string {
	return "random"
}

// Weighted Load Balancer
type WeightedBalancer struct {
	counter uint64
}

// Create weighted load balancer
func NewWeightedBalancer() *WeightedBalancer {
	return &WeightedBalancer{}
}

// Select an instance based on weight
func (b *WeightedBalancer) Select(instances []*ServiceInstance) *ServiceInstance {
	if len(instances) == 0 {
		return nil
	}

	// Calculate total weight
	totalWeight := 0
	for _, inst := range instances {
		if inst.Healthy && inst.Weight > 0 {
			totalWeight += inst.Weight
		}
	}

	if totalWeight == 0 {
		// fallback to polling
		idx := atomic.AddUint64(&b.counter, 1) - 1
		return instances[int(idx)%len(instances)]
	}

	// weight-based selection
	idx := atomic.AddUint64(&b.counter, 1) - 1
	target := int(idx) % totalWeight

	current := 0
	for _, inst := range instances {
		if !inst.Healthy || inst.Weight <= 0 {
			continue
		}

		current += inst.Weight
		if current > target {
			return inst
		}
	}

	// Fallback: return the first one
	return instances[0]
}

// Name Load Balancer Name
func (b *WeightedBalancer) Name() string {
	return "weighted"
}

// Create load balancer according to name
func NewLoadBalancer(name string) LoadBalancer {
	switch name {
	case "random":
		return NewRandomBalancer()
	case "weighted":
		return NewWeightedBalancer()
	case "round_robin", "":
		return NewRoundRobinBalancer()
	default:
		return NewRoundRobinBalancer()
	}
}

