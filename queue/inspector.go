package queue

import (
	"context"
	"time"
)

type Inspector interface {
	QueueStats(ctx context.Context, queueName string) (QueueStats, error)
	Close() error
}

type QueueStats struct {
	Queue     string
	Pending   int64
	Active    int64
	Scheduled int64
	CheckedAt time.Time
}
