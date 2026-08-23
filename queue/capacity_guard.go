package queue

import "context"

type CapacityGuard struct {
	cfg       Config
	inspector Inspector
}

func NewCapacityGuard(cfg Config, inspector Inspector) *CapacityGuard {
	cfg.ApplyDefaults()
	return &CapacityGuard{cfg: cfg, inspector: inspector}
}

func (g *CapacityGuard) Check(ctx context.Context, queueName string) error {
	if g == nil {
		return nil
	}
	route, err := g.cfg.QueueRoute(queueName)
	if err != nil {
		return err
	}
	if route.MaxPending <= 0 {
		return nil
	}
	if g.inspector == nil {
		return ErrQueueInspectorRequired
	}
	stats, err := g.inspector.QueueStats(ctx, queueName)
	if err != nil {
		return err
	}
	if stats.Pending >= route.MaxPending {
		return &BacklogExceededError{
			Queue:      queueName,
			Pending:    stats.Pending,
			MaxPending: route.MaxPending,
		}
	}
	return nil
}
