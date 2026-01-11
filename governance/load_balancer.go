package governance

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// LoadBalancer 负载均衡器接口
type LoadBalancer interface {
	// Select 选择一个服务实例
	Select(instances []*ServiceInstance) *ServiceInstance

	// Name 负载均衡器名称
	Name() string
}

// RoundRobinBalancer 轮询负载均衡器
type RoundRobinBalancer struct {
	counter uint64
}

// NewRoundRobinBalancer 创建轮询负载均衡器
func NewRoundRobinBalancer() *RoundRobinBalancer {
	return &RoundRobinBalancer{}
}

// Select 轮询选择实例
func (b *RoundRobinBalancer) Select(instances []*ServiceInstance) *ServiceInstance {
	if len(instances) == 0 {
		return nil
	}

	// 原子递增计数器
	idx := atomic.AddUint64(&b.counter, 1) - 1
	return instances[int(idx)%len(instances)]
}

// Name 负载均衡器名称
func (b *RoundRobinBalancer) Name() string {
	return "round_robin"
}

// RandomBalancer 随机负载均衡器
type RandomBalancer struct {
	rand *rand.Rand
	mu   sync.Mutex
}

// NewRandomBalancer 创建随机负载均衡器
func NewRandomBalancer() *RandomBalancer {
	return &RandomBalancer{
		rand: rand.New(rand.NewSource(int64(time.Now().UnixNano()))),
	}
}

// Select 随机选择实例
func (b *RandomBalancer) Select(instances []*ServiceInstance) *ServiceInstance {
	if len(instances) == 0 {
		return nil
	}

	b.mu.Lock()
	idx := b.rand.Intn(len(instances))
	b.mu.Unlock()

	return instances[idx]
}

// Name 负载均衡器名称
func (b *RandomBalancer) Name() string {
	return "random"
}

// WeightedBalancer 加权负载均衡器
type WeightedBalancer struct {
	counter uint64
}

// NewWeightedBalancer 创建加权负载均衡器
func NewWeightedBalancer() *WeightedBalancer {
	return &WeightedBalancer{}
}

// Select 根据权重选择实例
func (b *WeightedBalancer) Select(instances []*ServiceInstance) *ServiceInstance {
	if len(instances) == 0 {
		return nil
	}

	// 计算总权重
	totalWeight := 0
	for _, inst := range instances {
		if inst.Healthy && inst.Weight > 0 {
			totalWeight += inst.Weight
		}
	}

	if totalWeight == 0 {
		// 降级到轮询
		idx := atomic.AddUint64(&b.counter, 1) - 1
		return instances[int(idx)%len(instances)]
	}

	// 基于权重选择
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

	// 兜底：返回第一个
	return instances[0]
}

// Name 负载均衡器名称
func (b *WeightedBalancer) Name() string {
	return "weighted"
}

// NewLoadBalancer 根据名称创建负载均衡器
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

