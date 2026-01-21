package governance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoundRobinBalancer_Select(t *testing.T) {
	lb := NewRoundRobinBalancer()
	assert.Equal(t, "round_robin", lb.Name())

	// Empty instances
	result := lb.Select(nil)
	assert.Nil(t, result)

	result = lb.Select([]*ServiceInstance{})
	assert.Nil(t, result)

	// Multiple instances - round robin
	instances := []*ServiceInstance{
		{Address: "192.168.1.1", Port: 8080},
		{Address: "192.168.1.2", Port: 8080},
		{Address: "192.168.1.3", Port: 8080},
	}

	// Should cycle through instances
	first := lb.Select(instances)
	second := lb.Select(instances)
	third := lb.Select(instances)
	fourth := lb.Select(instances)

	assert.NotNil(t, first)
	assert.NotNil(t, second)
	assert.NotNil(t, third)
	assert.NotNil(t, fourth)

	// Fourth should be same as first (round robin)
	assert.Equal(t, first.Address, fourth.Address)
}

func TestRandomBalancer_Select(t *testing.T) {
	lb := NewRandomBalancer()
	assert.Equal(t, "random", lb.Name())

	// Empty instances
	result := lb.Select(nil)
	assert.Nil(t, result)

	result = lb.Select([]*ServiceInstance{})
	assert.Nil(t, result)

	// Single instance
	instances := []*ServiceInstance{
		{Address: "192.168.1.1", Port: 8080},
	}
	result = lb.Select(instances)
	assert.NotNil(t, result)
	assert.Equal(t, "192.168.1.1", result.Address)

	// Multiple instances - should return valid instance
	instances = []*ServiceInstance{
		{Address: "192.168.1.1", Port: 8080},
		{Address: "192.168.1.2", Port: 8080},
		{Address: "192.168.1.3", Port: 8080},
	}

	for i := 0; i < 10; i++ {
		result := lb.Select(instances)
		assert.NotNil(t, result)
		assert.Contains(t, []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"}, result.Address)
	}
}

func TestWeightedBalancer_Select(t *testing.T) {
	lb := NewWeightedBalancer()
	assert.Equal(t, "weighted", lb.Name())

	// Empty instances
	result := lb.Select(nil)
	assert.Nil(t, result)

	result = lb.Select([]*ServiceInstance{})
	assert.Nil(t, result)

	// All unhealthy - fallback to round robin
	instances := []*ServiceInstance{
		{Address: "192.168.1.1", Port: 8080, Weight: 0, Healthy: false},
		{Address: "192.168.1.2", Port: 8080, Weight: 0, Healthy: false},
	}
	result = lb.Select(instances)
	assert.NotNil(t, result)

	// Mixed weights
	instances = []*ServiceInstance{
		{Address: "192.168.1.1", Port: 8080, Weight: 10, Healthy: true},
		{Address: "192.168.1.2", Port: 8080, Weight: 20, Healthy: true},
		{Address: "192.168.1.3", Port: 8080, Weight: 0, Healthy: true}, // Zero weight
	}

	counts := make(map[string]int)
	for i := 0; i < 30; i++ {
		result := lb.Select(instances)
		assert.NotNil(t, result)
		counts[result.Address]++
	}

	// Higher weight should get more selections
	assert.Greater(t, counts["192.168.1.2"], 0)
	assert.Greater(t, counts["192.168.1.1"], 0)
}

func TestNewLoadBalancer(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"round_robin", "round_robin"},
		{"random", "random"},
		{"weighted", "weighted"},
		{"", "round_robin"},      // Default
		{"unknown", "round_robin"}, // Unknown fallback
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lb := NewLoadBalancer(tt.name)
			assert.Equal(t, tt.expected, lb.Name())
		})
	}
}
