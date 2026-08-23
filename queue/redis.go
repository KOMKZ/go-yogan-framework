package queue

import (
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

func (c *RedisConfig) ApplyDefaults() {
	if c.Mode == "" {
		c.Mode = "dedicated"
	}
	if c.DialTimeout == 0 {
		c.DialTimeout = 5 * time.Second
	}
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 3 * time.Second
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 3 * time.Second
	}
	if c.PoolSize == 0 {
		c.PoolSize = 10
	}
}

func (c RedisConfig) Validate() error {
	if c.Mode != "dedicated" {
		return fmt.Errorf("unsupported mode %q", c.Mode)
	}
	if c.Addr == "" {
		return fmt.Errorf("addr is required")
	}
	if c.DB < 0 || c.DB > 15 {
		return fmt.Errorf("db must be between 0 and 15")
	}
	if c.PoolSize < 0 {
		return fmt.Errorf("pool_size must be greater than or equal to zero")
	}
	return nil
}

func redisClientOpt(cfg RedisConfig) asynq.RedisClientOpt {
	return asynq.RedisClientOpt{
		Addr:         cfg.Addr,
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolSize:     cfg.PoolSize,
	}
}
