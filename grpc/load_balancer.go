package grpc

import (
	"errors"
	"math/rand"
	"sync"
)

// LoadBalancer 负载均衡器接口
type LoadBalancer interface {
	// Select 从地址列表中选择一个地址
	Select(addresses []string) (string, error)
	// Update 更新地址列表（当服务实例变化时）
	Update(addresses []string)
}

// RoundRobinBalancer 轮询负载均衡器
type RoundRobinBalancer struct {
	mu        sync.Mutex
	current   int
	addresses []string
}

// NewRoundRobinBalancer 创建轮询负载均衡器
func NewRoundRobinBalancer() *RoundRobinBalancer {
	return &RoundRobinBalancer{
		addresses: make([]string, 0),
	}
}

// Update 更新地址列表
func (b *RoundRobinBalancer) Update(addresses []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	b.addresses = make([]string, len(addresses))
	copy(b.addresses, addresses)
	
	// 重置索引（避免越界）
	if b.current >= len(b.addresses) {
		b.current = 0
	}
}

// Select 选择下一个地址
func (b *RoundRobinBalancer) Select(addresses []string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// 优先使用参数传入的地址列表
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

// RandomBalancer 随机负载均衡器
type RandomBalancer struct {
	mu        sync.Mutex
	addresses []string
}

// NewRandomBalancer 创建随机负载均衡器
func NewRandomBalancer() *RandomBalancer {
	return &RandomBalancer{
		addresses: make([]string, 0),
	}
}

// Update 更新地址列表
func (b *RandomBalancer) Update(addresses []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	b.addresses = make([]string, len(addresses))
	copy(b.addresses, addresses)
}

// Select 随机选择一个地址
func (b *RandomBalancer) Select(addresses []string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	// 优先使用参数传入的地址列表
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

// NewLoadBalancer 根据策略创建负载均衡器
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

