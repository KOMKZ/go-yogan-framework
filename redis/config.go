package redis

import (
	"fmt"
	"time"
)

// Configure Redis settings
type Config struct {
	// Mode: "standalone" (single machine) or "cluster" (cluster)
	Mode string `mapstructure:"mode"`

	// Address list
	// Single-machine mode: use the first address
	// Cluster mode: use all addresses
	Addrs []string `mapstructure:"addrs"`

	// Addr single address (backward compatibility, prefer using Addrs)
	Addr string `mapstructure:"addr"`

	// Password (optional)
	Password string `mapstructure:"password"`

	// Database number (0-15, valid only in single-machine mode)
	DB int `mapstructure:"db"`

	// PoolSize connection pool size (default 10)
	PoolSize int `mapstructure:"pool_size"`

	// Minimum idle connections (default 5)
	MinIdleConns int `mapstructure:"min_idle_conns"`

	// Maximum number of retries (default 3)
	MaxRetries int `mapstructure:"max_retries"`

	// DialTimeout connection timeout (default 5s)
	DialTimeout time.Duration `mapstructure:"dial_timeout"`

	// Read timeout (default 3s)
	ReadTimeout time.Duration `mapstructure:"read_timeout"`

	// WriteTimeout Write timeout (default 3s)
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// Validate configuration
func (c *Config) Validate() error {
	// Validate mode
	if c.Mode != "standalone" && c.Mode != "cluster" {
		return fmt.Errorf("invalid mode: %s (must be standalone or cluster)", c.Mode)
	}

	// Validate address
	if len(c.Addrs) == 0 {
		return fmt.Errorf("addrs cannot be empty")
	}

	// Single-machine mode verification
	if c.Mode == "standalone" {
		if c.DB < 0 || c.DB > 15 {
			return fmt.Errorf("db must be between 0 and 15, got: %d", c.DB)
		}
	}

	// Connection pool validation
	if c.PoolSize < 0 {
		return fmt.Errorf("pool_size must be >= 0, got: %d", c.PoolSize)
	}

	if c.MinIdleConns < 0 {
		return fmt.Errorf("min_idle_conns must be >= 0, got: %d", c.MinIdleConns)
	}

	return nil
}

// Apply defaults
func (c *Config) ApplyDefaults() {
	// Default single-machine mode
	if c.Mode == "" {
		c.Mode = "standalone"
	}

	// Backward compatibility: if Addr (singular) is used, convert to Addrs (plural)
	if c.Addr != "" && len(c.Addrs) == 0 {
		c.Addrs = []string{c.Addr}
	}

	// default pool values
	if c.PoolSize == 0 {
		c.PoolSize = 10
	}

	if c.MinIdleConns == 0 {
		c.MinIdleConns = 5
	}

	if c.MaxRetries == 0 {
		c.MaxRetries = 3
	}

	// default timeout value
	if c.DialTimeout == 0 {
		c.DialTimeout = 5 * time.Second
	}

	if c.ReadTimeout == 0 {
		c.ReadTimeout = 3 * time.Second
	}

	if c.WriteTimeout == 0 {
		c.WriteTimeout = 3 * time.Second
	}
}
