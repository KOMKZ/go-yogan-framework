package queue

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRequestHeadersInjectsTraceID(t *testing.T) {
	ctx := ContextWithTraceID(context.Background(), "trace-123")

	headers := requestHeaders(ctx, EnqueueRequest{})

	if headers[TraceIDHeader] != "trace-123" {
		t.Fatalf("trace header = %q, want trace-123", headers[TraceIDHeader])
	}
}

func TestApplyTaskDefaultsKeepsExplicitValues(t *testing.T) {
	req := EnqueueRequest{
		Queue:   "custom",
		Timeout: 30 * time.Second,
	}
	cfg := TaskConfig{
		Queue:   "default",
		Timeout: time.Minute,
	}

	got := applyTaskDefaults(req, cfg)

	if got.Queue != "custom" || got.Timeout != 30*time.Second {
		t.Fatalf("explicit request values were not preserved: %+v", got)
	}
}

func TestApplyTaskDefaultsFillsMissingValues(t *testing.T) {
	got := applyTaskDefaults(EnqueueRequest{}, TaskConfig{
		Queue:     "export",
		Timeout:   time.Minute,
		Retention: time.Hour,
		UniqueTTL: 10 * time.Second,
	})

	if got.Queue != "export" || got.Timeout != time.Minute {
		t.Fatalf("defaults not applied: %+v", got)
	}
	if got.Retention != time.Hour || got.UniqueTTL != 10*time.Second {
		t.Fatalf("advanced defaults not applied: %+v", got)
	}
}

func TestCapacityGuardRejectsWhenPendingExceedsLimit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Queues[DefaultQueue] = QueueConfig{Broker: DefaultBroker, Name: DefaultQueue, MaxPending: 10}
	guard := NewCapacityGuard(cfg, &fakeInspector{pending: 10})

	err := guard.Check(context.Background(), "default")

	if !errors.Is(err, ErrQueueBacklogExceeded) {
		t.Fatalf("error = %v, want ErrQueueBacklogExceeded", err)
	}
	var backlogErr *BacklogExceededError
	if !errors.As(err, &backlogErr) {
		t.Fatalf("error type = %T, want *BacklogExceededError", err)
	}
	if backlogErr.Queue != "default" || backlogErr.Pending != 10 || backlogErr.MaxPending != 10 {
		t.Fatalf("backlog error fields = %+v", backlogErr)
	}
}

func TestCapacityGuardAllowsWhenBelowLimit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Queues[DefaultQueue] = QueueConfig{Broker: DefaultBroker, Name: DefaultQueue, MaxPending: 10}
	guard := NewCapacityGuard(cfg, &fakeInspector{pending: 9})

	if err := guard.Check(context.Background(), "default"); err != nil {
		t.Fatalf("check failed: %v", err)
	}
}

func TestCapacityGuardRequiresInspector(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Queues[DefaultQueue] = QueueConfig{Broker: DefaultBroker, Name: DefaultQueue, MaxPending: 10}
	guard := NewCapacityGuard(cfg, nil)

	err := guard.Check(context.Background(), "default")

	if !errors.Is(err, ErrQueueInspectorRequired) {
		t.Fatalf("error = %v, want ErrQueueInspectorRequired", err)
	}
}

type fakeInspector struct {
	pending int64
	err     error
}

func (i *fakeInspector) QueueStats(ctx context.Context, queueName string) (QueueStats, error) {
	if i.err != nil {
		return QueueStats{}, i.err
	}
	return QueueStats{
		Queue:     queueName,
		Pending:   i.pending,
		CheckedAt: time.Now(),
	}, nil
}

func (i *fakeInspector) Close() error {
	return nil
}
