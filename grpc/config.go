package grpc

import "fmt"

// Config gRPC 组件配置（第一阶段：基础功能）
type Config struct {
	Server  ServerConfig            `mapstructure:"server"`
	Clients map[string]ClientConfig `mapstructure:"clients"`
}

// ServerConfig gRPC 服务端配置
type ServerConfig struct {
	Enabled       bool            `mapstructure:"enabled"`        // 是否启用 gRPC Server
	Port          int             `mapstructure:"port"`           // 监听端口
	MaxRecvSize   int             `mapstructure:"max_recv_size"`  // 最大接收大小（MB）
	MaxSendSize   int             `mapstructure:"max_send_size"`  // 最大发送大小（MB）
	EnableReflect bool            `mapstructure:"enable_reflect"` // 启用反射（方便调试）
	EnableLog     *bool           `mapstructure:"enable_log"`     // 启用拦截器日志（nil=默认true，false=禁用）
	Registry      RegistryConfig  `mapstructure:"registry"`       // 服务注册配置
}

// IsLogEnabled 返回是否启用日志（默认 true）
func (c *ServerConfig) IsLogEnabled() bool {
	if c.EnableLog == nil {
		return true // 默认启用
	}
	return *c.EnableLog
}

// RegistryConfig 服务注册配置
type RegistryConfig struct {
	Enabled     bool     `mapstructure:"enabled"`      // 是否启用服务注册
	ServiceName string   `mapstructure:"service_name"` // 服务名称
	TTL         int64    `mapstructure:"ttl"`          // 租约TTL（秒）
	Endpoints   []string `mapstructure:"endpoints"`    // etcd 节点地址（如 ["127.0.0.1:2379"]）
	Address     string   `mapstructure:"address"`      // 服务注册地址（可选，默认自动获取本机IP）
}

// ClientConfig gRPC 客户端配置
type ClientConfig struct {
	// 模式1：直连模式（向下兼容）
	Target  string `mapstructure:"target"`  // 直连地址（如 127.0.0.1:9000）
	Timeout int    `mapstructure:"timeout"` // 超时时间（秒，默认5秒）
	
	// 模式2：服务发现模式
	DiscoveryMode string `mapstructure:"discovery_mode"` // 发现模式: "direct" | "etcd"
	ServiceName   string `mapstructure:"service_name"`   // 服务名（用于etcd服务发现）
	LoadBalance   string `mapstructure:"load_balance"`   // 负载均衡: "round_robin" | "random"
	
	// 日志配置
	EnableLog *bool `mapstructure:"enable_log"` // 启用拦截器日志（nil=默认true，false=禁用）
}

// GetTimeout 返回超时时间（秒，默认5秒）
func (c *ClientConfig) GetTimeout() int {
	if c.Timeout <= 0 {
		return 5 // 默认5秒
	}
	return c.Timeout
}

// IsLogEnabled 返回是否启用日志（默认 true）
func (c *ClientConfig) IsLogEnabled() bool {
	if c.EnableLog == nil {
		return true // 默认启用
	}
	return *c.EnableLog
}

// GetMode 获取连接模式（兼容旧配置）
func (c *ClientConfig) GetMode() string {
	if c.DiscoveryMode != "" {
		return c.DiscoveryMode
	}
	// 如果配置了 Target，默认为直连模式
	if c.Target != "" {
		return "direct"
	}
	// 如果配置了 ServiceName，默认为 etcd 模式
	if c.ServiceName != "" {
		return "etcd"
	}
	return "direct"
}

// Validate 验证配置
func (c *ClientConfig) Validate() error {
	mode := c.GetMode()
	
	if mode == "direct" {
		if c.Target == "" {
			return fmt.Errorf("direct 模式下 target 不能为空")
		}
	} else if mode == "etcd" {
		if c.ServiceName == "" {
			return fmt.Errorf("etcd 模式下 service_name 不能为空")
		}
	}
	
	return nil
}
