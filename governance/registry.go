package governance

import (
	"context"
)

// ServiceRegistry 服务注册接口
// 定义服务注册、注销、更新等核心能力
type ServiceRegistry interface {
	// Register 注册服务（阻塞直到成功或超时）
	// 成功后会自动启动心跳保活
	Register(ctx context.Context, info *ServiceInfo) error

	// Deregister 注销服务
	// 停止心跳并从注册中心移除服务信息
	Deregister(ctx context.Context) error

	// UpdateMetadata 更新服务元数据
	// 用于动态更新服务的附加信息（如权重、版本等）
	UpdateMetadata(ctx context.Context, metadata map[string]string) error

	// IsRegistered 检查服务是否已注册
	IsRegistered() bool
}

// ServiceInfo 服务注册信息
type ServiceInfo struct {
	// 基础信息
	ServiceName string `json:"service_name"` // 服务名称（如 "auth-app"）
	InstanceID  string `json:"instance_id"`  // 实例ID（唯一标识，如 "auth-app-192.168.1.100-9002"）
	Address     string `json:"address"`      // 服务地址（如 "192.168.1.100"）
	Port        int    `json:"port"`         // 服务端口（如 9002）

	// 协议信息
	Protocol string `json:"protocol"` // 协议类型（grpc/http/https）
	Version  string `json:"version"`  // 服务版本（如 "v1.0.0"）

	// 元数据
	Metadata map[string]string `json:"metadata"` // 自定义元数据（如权重、区域等）

	// 健康检查
	HealthCheck *HealthCheckConfig `json:"health_check,omitempty"` // 健康检查配置

	// 注册中心配置
	TTL int64 `json:"ttl"` // 心跳间隔（秒）
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	Enabled  bool   `json:"enabled"`  // 是否启用健康检查
	Interval int    `json:"interval"` // 检查间隔（秒）
	Timeout  int    `json:"timeout"`  // 超时时间（秒）
	Path     string `json:"path"`     // HTTP健康检查路径（如 "/health"）
}

// GetFullAddress 获取完整地址（address:port）
func (s *ServiceInfo) GetFullAddress() string {
	return FormatServiceAddress(s.Address, s.Port)
}

// Validate 验证服务信息
func (s *ServiceInfo) Validate() error {
	if s.ServiceName == "" {
		return ErrInvalidServiceName
	}
	if s.Address == "" {
		return ErrInvalidAddress
	}
	if s.Port <= 0 || s.Port > 65535 {
		return ErrInvalidPort
	}
	if s.TTL <= 0 {
		s.TTL = 10 // 默认 10 秒
	}
	if s.Protocol == "" {
		s.Protocol = "grpc" // 默认 grpc
	}
	if s.InstanceID == "" {
		// 自动生成实例ID
		s.InstanceID = GenerateInstanceID(s.ServiceName, s.Address, s.Port)
	}
	return nil
}

