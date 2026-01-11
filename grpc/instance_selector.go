package grpc

import (
	"github.com/KOMKZ/go-yogan-framework/governance"
)

// InstanceSelector 实例选择器接口
// 职责：从服务实例列表中选择一个用于连接
type InstanceSelector interface {
	// Select 从实例列表中选择一个实例
	// 返回 nil 表示没有可用实例
	Select(instances []*governance.ServiceInstance) *governance.ServiceInstance
}

// FirstHealthySelector 选择第一个健康实例（默认策略）
// 适用场景：简单场景，快速返回，无状态
type FirstHealthySelector struct{}

// NewFirstHealthySelector 创建第一个健康实例选择器
func NewFirstHealthySelector() *FirstHealthySelector {
	return &FirstHealthySelector{}
}

// Select 选择第一个健康实例
func (s *FirstHealthySelector) Select(instances []*governance.ServiceInstance) *governance.ServiceInstance {
	for _, inst := range instances {
		if inst.Healthy {
			return inst
		}
	}
	return nil
}

// LoadBalancerSelector 负载均衡选择器（适配器模式）
// 职责：复用 governance.LoadBalancer 的算法实现
type LoadBalancerSelector struct {
	balancer governance.LoadBalancer
}

// NewLoadBalancerSelector 创建负载均衡选择器
// strategy: "round_robin" | "random" | "weighted"
func NewLoadBalancerSelector(strategy string) *LoadBalancerSelector {
	return &LoadBalancerSelector{
		balancer: governance.NewLoadBalancer(strategy),
	}
}

// Select 使用负载均衡算法选择实例
// 自动过滤不健康的实例
func (s *LoadBalancerSelector) Select(instances []*governance.ServiceInstance) *governance.ServiceInstance {
	// 过滤健康实例
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

// NewInstanceSelector 根据策略名称创建选择器（工厂方法）
func NewInstanceSelector(strategy string) InstanceSelector {
	switch strategy {
	case "first", "":
		return NewFirstHealthySelector()
	case "round_robin", "random", "weighted":
		return NewLoadBalancerSelector(strategy)
	default:
		// 未知策略，降级到第一个健康实例
		return NewFirstHealthySelector()
	}
}

