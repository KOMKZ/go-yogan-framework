package queue

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"
)

type HealthChecker struct {
	cfg Config
}

func NewHealthChecker(cfg Config) *HealthChecker {
	return &HealthChecker{cfg: cfg}
}

func (h *HealthChecker) Name() string {
	return "queue"
}

func (h *HealthChecker) Check(ctx context.Context) error {
	if h == nil {
		return fmt.Errorf("queue health checker is nil")
	}
	if !h.cfg.Enabled {
		return nil
	}

	h.cfg.ApplyDefaults()
	if err := h.cfg.Validate(); err != nil {
		return err
	}
	for brokerName, broker := range h.cfg.Brokers {
		if err := pingRedis(ctx, broker.Redis); err != nil {
			return fmt.Errorf("queue broker %q redis ping failed: %w", brokerName, err)
		}
	}
	return nil
}

func pingRedis(ctx context.Context, cfg RedisConfig) error {
	client := goredis.NewClient(&goredis.Options{
		Addr:         cfg.Addr,
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolSize:     cfg.PoolSize,
	})
	defer client.Close()
	return client.Ping(ctx).Err()
}
