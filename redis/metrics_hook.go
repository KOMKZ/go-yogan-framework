package redis

import (
	"context"
	"net"
	"time"

	"github.com/redis/go-redis/v9"
)

// MetricsHook implements redis.Hook to record command metrics
type MetricsHook struct {
	metrics  *RedisMetrics
	instance string
}

// NewMetricsHook creates a new MetricsHook
func NewMetricsHook(metrics *RedisMetrics, instance string) *MetricsHook {
	return &MetricsHook{
		metrics:  metrics,
		instance: instance,
	}
}

// DialHook is called when a new connection is established (pass through)
func (h *MetricsHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return next(ctx, network, addr)
	}
}

// ProcessHook intercepts single Redis commands and records metrics
func (h *MetricsHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmd)
		duration := time.Since(start)

		// Record command metrics
		h.metrics.RecordCommand(ctx, h.instance, cmd.Name(), duration, err)

		// Record cache hit/miss for GET commands
		if h.metrics.config.RecordHitMiss && cmd.Name() == "get" {
			if err == redis.Nil {
				h.metrics.RecordCacheMiss(ctx, h.instance)
			} else if err == nil {
				h.metrics.RecordCacheHit(ctx, h.instance)
			}
		}

		return err
	}
}

// ProcessPipelineHook intercepts pipeline commands and records metrics
func (h *MetricsHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmds)
		duration := time.Since(start)

		// Record each command in the pipeline
		cmdDuration := duration / time.Duration(len(cmds))
		for _, cmd := range cmds {
			h.metrics.RecordCommand(ctx, h.instance, cmd.Name(), cmdDuration, cmd.Err())
		}

		return err
	}
}
