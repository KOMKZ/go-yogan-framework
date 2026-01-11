package redis

import (
	"fmt"
	"time"
)

// Config Redis 配置
type Config struct {
	// Mode 模式："standalone"（单机）或 "cluster"（集群）
	Mode string `mapstructure:"mode"`

	// Addrs 地址列表
	// 单机模式：使用第一个地址
	// 集群模式：使用所有地址
	Addrs []string `mapstructure:"addrs"`

	// Addr 单个地址（向后兼容，优先使用 Addrs）
	Addr string `mapstructure:"addr"`

	// Password 密码（可选）
	Password string `mapstructure:"password"`

	// DB 数据库编号（0-15，仅单机模式有效）
	DB int `mapstructure:"db"`

	// PoolSize 连接池大小（默认 10）
	PoolSize int `mapstructure:"pool_size"`

	// MinIdleConns 最小空闲连接数（默认 5）
	MinIdleConns int `mapstructure:"min_idle_conns"`

	// MaxRetries 最大重试次数（默认 3）
	MaxRetries int `mapstructure:"max_retries"`

	// DialTimeout 连接超时（默认 5s）
	DialTimeout time.Duration `mapstructure:"dial_timeout"`

	// ReadTimeout 读取超时（默认 3s）
	ReadTimeout time.Duration `mapstructure:"read_timeout"`

	// WriteTimeout 写入超时（默认 3s）
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// Validate 验证配置
func (c *Config) Validate() error {
	// 验证模式
	if c.Mode != "standalone" && c.Mode != "cluster" {
		return fmt.Errorf("invalid mode: %s (must be standalone or cluster)", c.Mode)
	}

	// 验证地址
	if len(c.Addrs) == 0 {
		return fmt.Errorf("addrs cannot be empty")
	}

	// 单机模式验证
	if c.Mode == "standalone" {
		if c.DB < 0 || c.DB > 15 {
			return fmt.Errorf("db must be between 0 and 15, got: %d", c.DB)
		}
	}

	// 连接池验证
	if c.PoolSize < 0 {
		return fmt.Errorf("pool_size must be >= 0, got: %d", c.PoolSize)
	}

	if c.MinIdleConns < 0 {
		return fmt.Errorf("min_idle_conns must be >= 0, got: %d", c.MinIdleConns)
	}

	return nil
}

// ApplyDefaults 应用默认值
func (c *Config) ApplyDefaults() {
	// 默认单机模式
	if c.Mode == "" {
		c.Mode = "standalone"
	}

	// 向后兼容：如果使用了 Addr（单数），转换为 Addrs（复数）
	if c.Addr != "" && len(c.Addrs) == 0 {
		c.Addrs = []string{c.Addr}
	}

	// 连接池默认值
	if c.PoolSize == 0 {
		c.PoolSize = 10
	}

	if c.MinIdleConns == 0 {
		c.MinIdleConns = 5
	}

	if c.MaxRetries == 0 {
		c.MaxRetries = 3
	}

	// 超时默认值
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
