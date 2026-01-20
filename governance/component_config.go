package governance

import "time"

import "github.com/KOMKZ/go-yogan-framework/breaker"

// Configuration governance component configuration
type Config struct {
	Enabled      bool              `mapstructure:"enabled"`       // Whether to enable
	RegistryType string            `mapstructure:"registry_type"` // Registry center type: etcd | consul | nacos
	ServiceName  string            `mapstructure:"service_name"`  // service name
	Protocol     string            `mapstructure:"protocol"`      // protocol type (grpc/http)
	Version      string            `mapstructure:"version"`       // service version
	TTL          int64             `mapstructure:"ttl"`           // Heartbeat interval (seconds)
	Address      string            `mapstructure:"address"`       // Service address (optional, if left empty the local machine IP will be automatically obtained)
	Metadata     map[string]string `mapstructure:"metadata"`      // metadata

	// Etcd configuration
	Etcd EtcdRegistryConfig `mapstructure:"etcd"`

	// Consul configuration (to be implemented)
	Consul ConsulRegistryConfig `mapstructure:"consul"`

	// Nacos configuration (to be implemented)
	Nacos NacosRegistryConfig `mapstructure:"nacos"`
	
	// Circuit breaker configuration
	Breaker breaker.Config `mapstructure:"breaker"`
}

// EtcdRegistryConfig Etcd registry center configuration
type EtcdRegistryConfig struct {
	Endpoints   []string      `mapstructure:"endpoints"`    // etcd node address
	DialTimeout time.Duration `mapstructure:"dial_timeout"` // connection timeout
	Username    string        `mapstructure:"username"`     // Username (optional)
	Password    string        `mapstructure:"password"`     // password (optional)

	// Retry strategy
	EnableRetry       bool          `mapstructure:"enable_retry"`        // Whether to enable auto retry (default true)
	MaxRetries        int           `mapstructure:"max_retries"`         // Maximum number of retry attempts (0=infinite retries)
	InitialRetryDelay time.Duration `mapstructure:"initial_retry_delay"` // Initial retry delay (default 1s)
	MaxRetryDelay     time.Duration `mapstructure:"max_retry_delay"`     // Maximum retry delay (default 30s)
	RetryBackoff      float64       `mapstructure:"retry_backoff"`       // backoff factor (default 2.0)

	// Callback (runtime setting, not read from configuration file)
	OnRegisterFailed func(error) `mapstructure:"-"` // Register final failure callback
}

// ConsulRegistryConfig Consul registry center configuration (placeholder)
type ConsulRegistryConfig struct {
	Address string `mapstructure:"address"` // Consul address
	Token   string `mapstructure:"token"`   // ACL Token
}

// NacosRegistryConfig Nacos registry center configuration (placeholder)
type NacosRegistryConfig struct {
	ServerAddr string `mapstructure:"server_addr"` // Nacos service address
	Namespace  string `mapstructure:"namespace"`   // namespace
	Group      string `mapstructure:"group"`       // Grouping
}

