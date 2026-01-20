package grpc

import (
	"errors"
	"math/rand"
	"sync"
)

// LoadBalancer load balancer interface
type LoadBalancer interface {
	// Select an address from the address list
	Select(addresses []string) (string, error)
	// Update address list (when service instance changes)
	Update(addresses []string)
}

// RoundRobinBalancer round-robin load balancer
type RoundRobinBalancer struct {
	mu        sync.Mutex
	current   int
	addresses []string
}

// Create new round-robin load balancer
func NewRoundRobinBalancer() *RoundRobinBalancer {
	return &RoundRobinBalancer{
		addresses: make([]string, 0),
	}
}

// Update address list
func (b *RoundRobinBalancer) Update(addresses []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	b.addresses = make([]string, len(addresses))
	copy(b.addresses, addresses)
	
	// Reset index (avoid out-of-bounds)
	if b.current >= len(b.addresses) {
		b.current = 0
	}
}

// Select the next address
func (b *RoundRobinBalancer) Select(addresses []string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Give priority to the address list passed as parameters
	list := addresses
	if len(list) == 0 {
		list = b.addresses
	}
	
	if len(list) == 0 {
		return "", errors.New("no available addresses")
	}
	
	addr := list[b.current%len(list)]
	b.current++
	
	return addr, nil
}

// RandomBalancer random load balancer
type RandomBalancer struct {
	mu        sync.Mutex
	addresses []string
}

// Create random load balancer
func NewRandomBalancer() *RandomBalancer {
	return &RandomBalancer{
		addresses: make([]string, 0),
	}
}

// Update address list
func (b *RandomBalancer) Update(addresses []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	b.addresses = make([]string, len(addresses))
	copy(b.addresses, addresses)
}

// Select a random address
func (b *RandomBalancer) Select(addresses []string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// Give priority to the address list passed as parameters
	list := addresses
	if len(list) == 0 {
		list = b.addresses
	}
	
	if len(list) == 0 {
		return "", errors.New("no available addresses")
	}
	
	idx := rand.Intn(len(list))
	return list[idx], nil
}

// Create load balancer according to policy
func NewLoadBalancer(strategy string) LoadBalancer {
	switch strategy {
	case "random":
		return NewRandomBalancer()
	case "round_robin", "":
		return NewRoundRobinBalancer()
	default:
		return NewRoundRobinBalancer()
	}
}

