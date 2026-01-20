package grpc

import "fmt"

// Configure gRPC component configuration (Phase One: Basic Functionality)
type Config struct {
	Server  ServerConfig            `mapstructure:"server"`
	Clients map[string]ClientConfig `mapstructure:"clients"`
}

// ServerConfig gRPC server configuration
type ServerConfig struct {
	Enabled       bool            `mapstructure:"enabled"`        // Whether to enable gRPC Server
	Port          int             `mapstructure:"port"`           // Listen for port
	MaxRecvSize   int             `mapstructure:"max_recv_size"`  // Maximum receive size (MB)
	MaxSendSize   int             `mapstructure:"max_send_size"`  // Maximum send size (MB)
	EnableReflect bool            `mapstructure:"enable_reflect"` // Enable reflection (for convenient debugging)
	EnableLog     *bool           `mapstructure:"enable_log"`     // Enable interceptor logging (nil=default true, false=disable)
	Registry      RegistryConfig  `mapstructure:"registry"`       // Service registration configuration
}

// Returns whether logging is enabled (default true)
func (c *ServerConfig) IsLogEnabled() bool {
	if c.EnableLog == nil {
		return true // Default enabled
	}
	return *c.EnableLog
}

// RegistryConfig service registration configuration
type RegistryConfig struct {
	Enabled     bool     `mapstructure:"enabled"`      // Whether service registration is enabled
	ServiceName string   `mapstructure:"service_name"` // service name
	TTL         int64    `mapstructure:"ttl"`          // lease TTL (seconds)
	Endpoints   []string `mapstructure:"endpoints"`    // etcd node address (e.g., ["127.0.0.1:2379"])
	Address     string   `mapstructure:"address"`      // Service registration address (optional, defaults to automatically obtaining the local IP)
}

// ClientConfig gRPC client configuration
type ClientConfig struct {
	// Mode 1: Direct Connection Mode (Backward Compatible)
	Target  string `mapstructure:"target"`  // Direct connection address (e.g., 127.0.0.1:9000)
	Timeout int    `mapstructure:"timeout"` // Timeout duration in seconds (default 5 seconds)
	
	// Mode 2: Service Discovery Mode
	DiscoveryMode string `mapstructure:"discovery_mode"` // Discover pattern: "direct" | "etcd"
	ServiceName   string `mapstructure:"service_name"`   // Service name (for etcd service discovery)
	LoadBalance   string `mapstructure:"load_balance"`   // Load balancing: "round_robin" | "random"
	
	// log configuration
	EnableLog *bool `mapstructure:"enable_log"` // Enable interceptor logs (nil=default true, false=disable)
}

// GetTimeout returns the timeout duration in seconds (default 5 seconds)
func (c *ClientConfig) GetTimeout() int {
	if c.Timeout <= 0 {
		return 5 // Default 5 seconds
	}
	return c.Timeout
}

// Returns whether logging is enabled (default true)
func (c *ClientConfig) IsLogEnabled() bool {
	if c.EnableLog == nil {
		return true // Enable by default
	}
	return *c.EnableLog
}

// Get connection mode (compatible with old configuration)
func (c *ClientConfig) GetMode() string {
	if c.DiscoveryMode != "" {
		return c.DiscoveryMode
	}
	// If Target is configured, default to direct connection mode
	if c.Target != "" {
		return "direct"
	}
	// If ServiceName is configured, default to etcd mode
	if c.ServiceName != "" {
		return "etcd"
	}
	return "direct"
}

// Validate configuration
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
