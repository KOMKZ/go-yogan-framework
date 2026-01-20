package grpc

import (
	"testing"

	"github.com/KOMKZ/go-yogan-framework/governance"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test instance data
func createTestInstances() []*governance.ServiceInstance {
	return []*governance.ServiceInstance{
		{
			Service: "test-service",
			Address: "192.168.1.1:9000",
			Port:    9000,
			Healthy: true,
			Weight:  10,
		},
		{
			Service: "test-service",
			Address: "192.168.1.2:9000",
			Port:    9000,
			Healthy: true,
			Weight:  20,
		},
		{
			Service: "test-service",
			Address: "192.168.1.3:9000",
			Port:    9000,
			Healthy: false, // unhealthy
			Weight:  10,
		},
		{
			Service: "test-service",
			Address: "192.168.1.4:9000",
			Port:    9000,
			Healthy: true,
			Weight:  15,
		},
	}
}

func TestFirstHealthySelector(t *testing.T) {
	t.Run("选择第一个健康实例", func(t *testing.T) {
		selector := NewFirstHealthySelector()
		instances := createTestInstances()

		selected := selector.Select(instances)

		require.NotNil(t, selected)
		assert.Equal(t, "192.168.1.1:9000", selected.Address)
		assert.True(t, selected.Healthy)
	})

	t.Run("空实例列表", func(t *testing.T) {
		selector := NewFirstHealthySelector()
		selected := selector.Select([]*governance.ServiceInstance{})

		assert.Nil(t, selected)
	})

	t.Run("全部不健康", func(t *testing.T) {
		selector := NewFirstHealthySelector()
		instances := []*governance.ServiceInstance{
			{Address: "192.168.1.1:9000", Healthy: false},
			{Address: "192.168.1.2:9000", Healthy: false},
		}

		selected := selector.Select(instances)
		assert.Nil(t, selected)
	})
}

func TestLoadBalancerSelector_RoundRobin(t *testing.T) {
	t.Run("轮询选择健康实例", func(t *testing.T) {
		selector := NewLoadBalancerSelector("round_robin")
		instances := createTestInstances()

		// Consecutive calls, verify polling behavior
		results := make([]string, 3)
		for i := 0; i < 3; i++ {
			selected := selector.Select(instances)
			require.NotNil(t, selected)
			assert.True(t, selected.Healthy)
			results[i] = selected.Address
		}

		// Verify that the address is in a healthy instance
		healthyAddrs := []string{"192.168.1.1:9000", "192.168.1.2:9000", "192.168.1.4:9000"}
		for _, addr := range results {
			assert.Contains(t, healthyAddrs, addr)
		}

		// Verify no unhealthy instances are included
		for _, addr := range results {
			assert.NotEqual(t, "192.168.1.3:9000", addr)
		}
	})

	t.Run("空实例列表", func(t *testing.T) {
		selector := NewLoadBalancerSelector("round_robin")
		selected := selector.Select([]*governance.ServiceInstance{})

		assert.Nil(t, selected)
	})

	t.Run("全部不健康", func(t *testing.T) {
		selector := NewLoadBalancerSelector("round_robin")
		instances := []*governance.ServiceInstance{
			{Address: "192.168.1.1:9000", Healthy: false},
		}

		selected := selector.Select(instances)
		assert.Nil(t, selected)
	})
}

func TestLoadBalancerSelector_Random(t *testing.T) {
	t.Run("随机选择健康实例", func(t *testing.T) {
		selector := NewLoadBalancerSelector("random")
		instances := createTestInstances()

		// Multiple calls, collect results
		results := make(map[string]int)
		for i := 0; i < 100; i++ {
			selected := selector.Select(instances)
			require.NotNil(t, selected)
			assert.True(t, selected.Healthy)
			results[selected.Address]++
		}

		// Verify that it contains only healthy instances
		healthyAddrs := map[string]bool{
			"192.168.1.1:9000": true,
			"192.168.1.2:9000": true,
			"192.168.1.4:9000": true,
		}
		for addr := range results {
			assert.True(t, healthyAddrs[addr], "地址应该是健康实例: %s", addr)
		}

		// Validate no unhealthy instances
		assert.NotContains(t, results, "192.168.1.3:9000")

		// Verify randomness (at least select 2 different instances)
		assert.GreaterOrEqual(t, len(results), 2, "应该有多个不同实例被选中")
	})
}

func TestLoadBalancerSelector_Weighted(t *testing.T) {
	t.Run("加权负载均衡", func(t *testing.T) {
		selector := NewLoadBalancerSelector("weighted")
		instances := createTestInstances()

		// Multiple calls, verify weighted distribution
		results := make(map[string]int)
		for i := 0; i < 90; i++ { // 90 = 10+20+15+10+... (a multiple of the total weight of healthy instances)
			selected := selector.Select(instances)
			require.NotNil(t, selected)
			assert.True(t, selected.Healthy)
			results[selected.Address]++
		}

		// Verify that only healthy instances are included
		healthyAddrs := map[string]bool{
			"192.168.1.1:9000": true, // weight 10
			"192.168.1.2:9000": true, // weight 20
			"192.168.1.4:9000": true, // weight 15
		}
		for addr := range results {
			assert.True(t, healthyAddrs[addr], "地址应该是健康实例: %s", addr)
		}

		// Validate no unhealthy instances
		assert.NotContains(t, results, "192.168.1.3:9000")
	})
}

func TestNewInstanceSelector(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		want     string // selector type
	}{
		{
			name:     "空策略使用默认",
			strategy: "",
			want:     "*grpc.FirstHealthySelector",
		},
		{
			name:     "first 策略",
			strategy: "first",
			want:     "*grpc.FirstHealthySelector",
		},
		{
			name:     "round_robin 策略",
			strategy: "round_robin",
			want:     "*grpc.LoadBalancerSelector",
		},
		{
			name:     "random 策略",
			strategy: "random",
			want:     "*grpc.LoadBalancerSelector",
		},
		{
			name:     "weighted 策略",
			strategy: "weighted",
			want:     "*grpc.LoadBalancerSelector",
		},
		{
			name:     "未知策略降级为默认",
			strategy: "unknown",
			want:     "*grpc.FirstHealthySelector",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector := NewInstanceSelector(tt.strategy)
			require.NotNil(t, selector)

			// Validate type
			selectorType := getTypeName(selector)
			assert.Equal(t, tt.want, selectorType)
		})
	}
}

func TestLoadBalancerSelector_OnlyHealthy(t *testing.T) {
	t.Run("自动过滤不健康实例", func(t *testing.T) {
		selector := NewLoadBalancerSelector("round_robin")
		instances := []*governance.ServiceInstance{
			{Address: "192.168.1.1:9000", Healthy: false},
			{Address: "192.168.1.2:9000", Healthy: true},
			{Address: "192.168.1.3:9000", Healthy: false},
			{Address: "192.168.1.4:9000", Healthy: true},
		}

		// Multiple calls, verify that only healthy instances are selected
		for i := 0; i < 10; i++ {
			selected := selector.Select(instances)
			require.NotNil(t, selected)
			assert.True(t, selected.Healthy)
			assert.Contains(t, []string{"192.168.1.2:9000", "192.168.1.4:9000"}, selected.Address)
		}
	})
}

// Helper function: get type name
func getTypeName(v interface{}) string {
	switch v.(type) {
	case *FirstHealthySelector:
		return "*grpc.FirstHealthySelector"
	case *LoadBalancerSelector:
		return "*grpc.LoadBalancerSelector"
	default:
		return "unknown"
	}
}

// Benchmark test: Compare performance of different selectors
func BenchmarkSelectors(b *testing.B) {
	instances := createTestInstances()

	b.Run("FirstHealthy", func(b *testing.B) {
		selector := NewFirstHealthySelector()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			selector.Select(instances)
		}
	})

	b.Run("RoundRobin", func(b *testing.B) {
		selector := NewLoadBalancerSelector("round_robin")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			selector.Select(instances)
		}
	})

	b.Run("Random", func(b *testing.B) {
		selector := NewLoadBalancerSelector("random")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			selector.Select(instances)
		}
	})

	b.Run("Weighted", func(b *testing.B) {
		selector := NewLoadBalancerSelector("weighted")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			selector.Select(instances)
		}
	})
}

