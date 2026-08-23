package queue

import (
	"context"
)

type AsynqInspector struct {
	cfg     Config
	brokers *BrokerManager
}

func NewAsynqInspector(cfg Config) (*AsynqInspector, error) {
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return nil, nil
	}
	brokers, err := NewBrokerManager(cfg)
	if err != nil {
		return nil, err
	}
	return &AsynqInspector{cfg: cfg, brokers: brokers}, nil
}

func (i *AsynqInspector) QueueStats(ctx context.Context, queueName string) (QueueStats, error) {
	if i == nil || i.brokers == nil {
		return QueueStats{}, ErrDisabled
	}
	route, err := i.cfg.QueueRoute(queueName)
	if err != nil {
		return QueueStats{}, err
	}
	inspector, err := i.brokers.Inspector(route.Broker)
	if err != nil {
		return QueueStats{}, err
	}
	info, err := inspector.GetQueueInfo(route.PhysicalName)
	if err != nil {
		return QueueStats{}, err
	}
	return QueueStats{
		Queue:     route.LogicalQueue,
		Pending:   int64(info.Pending),
		Active:    int64(info.Active),
		Scheduled: int64(info.Scheduled),
		CheckedAt: info.Timestamp,
	}, nil
}

func (i *AsynqInspector) Close() error {
	if i == nil || i.brokers == nil {
		return nil
	}
	return i.brokers.Close()
}

var _ Inspector = (*AsynqInspector)(nil)
