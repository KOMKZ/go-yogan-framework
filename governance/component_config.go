package governance

import "time"

import "github.com/KOMKZ/go-yogan-framework/breaker"

// Config 治理组件配置
type Config struct {
	Enabled      bool              `mapstructure:"enabled"`       // 是否启用
	RegistryType string            `mapstructure:"registry_type"` // 注册中心类型: etcd | consul | nacos
	ServiceName  string            `mapstructure:"service_name"`  // 服务名称
	Protocol     string            `mapstructure:"protocol"`      // 协议类型（grpc/http）
	Version      string            `mapstructure:"version"`       // 服务版本
	TTL          int64             `mapstructure:"ttl"`           // 心跳间隔（秒）
	Address      string            `mapstructure:"address"`       // 服务地址（可选，为空则自动获取本机IP）
	Metadata     map[string]string `mapstructure:"metadata"`      // 元数据

	// Etcd 配置
	Etcd EtcdRegistryConfig `mapstructure:"etcd"`

	// Consul 配置（待实现）
	Consul ConsulRegistryConfig `mapstructure:"consul"`

	// Nacos 配置（待实现）
	Nacos NacosRegistryConfig `mapstructure:"nacos"`
	
	// Breaker 熔断器配置
	Breaker breaker.Config `mapstructure:"breaker"`
}

// EtcdRegistryConfig Etcd 注册中心配置
type EtcdRegistryConfig struct {
	Endpoints   []string      `mapstructure:"endpoints"`    // etcd 节点地址
	DialTimeout time.Duration `mapstructure:"dial_timeout"` // 连接超时
	Username    string        `mapstructure:"username"`     // 用户名（可选）
	Password    string        `mapstructure:"password"`     // 密码（可选）

	// 重试策略
	EnableRetry       bool          `mapstructure:"enable_retry"`        // 是否启用自动重试（默认 true）
	MaxRetries        int           `mapstructure:"max_retries"`         // 最大重试次数（0=无限重试）
	InitialRetryDelay time.Duration `mapstructure:"initial_retry_delay"` // 初始重试延迟（默认 1s）
	MaxRetryDelay     time.Duration `mapstructure:"max_retry_delay"`     // 最大重试延迟（默认 30s）
	RetryBackoff      float64       `mapstructure:"retry_backoff"`       // 退避系数（默认 2.0）

	// 回调（运行时设置，不从配置文件读取）
	OnRegisterFailed func(error) `mapstructure:"-"` // 注册最终失败回调
}

// ConsulRegistryConfig Consul 注册中心配置（占位）
type ConsulRegistryConfig struct {
	Address string `mapstructure:"address"` // Consul 地址
	Token   string `mapstructure:"token"`   // ACL Token
}

// NacosRegistryConfig Nacos 注册中心配置（占位）
type NacosRegistryConfig struct {
	ServerAddr string `mapstructure:"server_addr"` // Nacos 服务地址
	Namespace  string `mapstructure:"namespace"`   // 命名空间
	Group      string `mapstructure:"group"`       // 分组
}

