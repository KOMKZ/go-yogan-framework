package queue

import (
	"errors"
	"fmt"
)

var (
	ErrQueueBacklogExceeded   = errors.New("queue backlog exceeded")
	ErrQueueInspectorRequired = errors.New("queue inspector is required when queue max_pending is enabled")
)

type BacklogExceededError struct {
	Queue      string
	Pending    int64
	MaxPending int64
}

func (e *BacklogExceededError) Error() string {
	return fmt.Sprintf("queue backlog exceeded: queue=%s pending=%d max_pending=%d", e.Queue, e.Pending, e.MaxPending)
}

func (e *BacklogExceededError) Unwrap() error {
	return ErrQueueBacklogExceeded
}
