package governance

import (
	"context"
)

// ServiceDiscovery 服务发现接口
type ServiceDiscovery interface {
	// Discover 发现服务实例列表
	Discover(ctx context.Context, serviceName string) ([]*ServiceInstance, error)

	// Watch 监听服务变更
	Watch(ctx context.Context, serviceName string) (<-chan []*ServiceInstance, error)

	// Stop 停止监听
	Stop()
}

// ServiceInstance 服务实例信息
type ServiceInstance struct {
	ID       string            `json:"id"`       // 实例ID
	Service  string            `json:"service"`  // 服务名称
	Address  string            `json:"address"`  // IP地址
	Port     int               `json:"port"`     // 端口
	Metadata map[string]string `json:"metadata"` // 元数据
	Weight   int               `json:"weight"`   // 权重（用于负载均衡）
	Healthy  bool              `json:"healthy"`  // 健康状态
}

// GetAddress 获取完整地址
func (s *ServiceInstance) GetAddress() string {
	return FormatServiceAddress(s.Address, s.Port)
}

