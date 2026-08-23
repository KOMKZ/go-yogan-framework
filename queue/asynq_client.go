package queue

import (
	"context"

	"github.com/hibiken/asynq"
)

type AsynqClient struct {
	cfg       Config
	brokers   *BrokerManager
	inspector Inspector
	guard     *CapacityGuard
}

func NewAsynqClient(cfg Config) (*AsynqClient, error) {
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return nil, nil
	}
	var inspector Inspector
	created, err := NewAsynqInspector(cfg)
	if err != nil {
		return nil, err
	}
	inspector = created
	brokers, err := NewBrokerManager(cfg)
	if err != nil {
		return nil, err
	}
	return newAsynqClient(cfg, brokers, inspector), nil
}

func newAsynqClient(cfg Config, brokers *BrokerManager, inspector Inspector) *AsynqClient {
	return &AsynqClient{
		cfg:       cfg,
		brokers:   brokers,
		inspector: inspector,
		guard:     NewCapacityGuard(cfg, inspector),
	}
}

func (c *AsynqClient) Enqueue(ctx context.Context, req EnqueueRequest) (*TaskInfo, error) {
	if c == nil || c.brokers == nil {
		return nil, ErrDisabled
	}
	taskCfg := c.cfg.TaskConfig(req.Type)
	req = applyTaskDefaults(req, taskCfg)
	if err := c.CheckCapacity(ctx, req.Queue); err != nil {
		return nil, err
	}
	route, err := c.cfg.QueueRoute(req.Queue)
	if err != nil {
		return nil, err
	}
	client, err := c.brokers.Client(route.Broker)
	if err != nil {
		return nil, err
	}
	req.Queue = route.PhysicalName

	task := asynq.NewTaskWithHeaders(req.Type, req.Payload, requestHeaders(ctx, req))
	info, err := client.EnqueueContext(ctx, task, enqueueOptions(req)...)
	if err != nil {
		return nil, err
	}
	converted := convertTaskInfo(info)
	if converted != nil {
		converted.Queue = route.LogicalQueue
	}
	return converted, nil
}

func (c *AsynqClient) Close() error {
	if c == nil {
		return nil
	}
	var brokerErr error
	if c.brokers != nil {
		brokerErr = c.brokers.Close()
	}
	if c.inspector != nil {
		if err := c.inspector.Close(); err != nil && brokerErr == nil {
			brokerErr = err
		}
	}
	return brokerErr
}

func (c *AsynqClient) CheckCapacity(ctx context.Context, queueName string) error {
	if c == nil || c.guard == nil {
		return nil
	}
	return c.guard.Check(ctx, queueName)
}

func applyTaskDefaults(req EnqueueRequest, cfg TaskConfig) EnqueueRequest {
	if req.Queue == "" {
		req.Queue = cfg.Queue
	}
	if req.Timeout == 0 {
		req.Timeout = cfg.Timeout
	}
	if req.Retention == 0 {
		req.Retention = cfg.Retention
	}
	if req.UniqueTTL == 0 {
		req.UniqueTTL = cfg.UniqueTTL
	}
	return req
}

var _ CapacityChecker = (*AsynqClient)(nil)

func requestHeaders(ctx context.Context, req EnqueueRequest) map[string]string {
	headers := make(map[string]string, len(req.Headers)+1)
	for key, value := range req.Headers {
		headers[key] = value
	}
	if _, ok := headers[TraceIDHeader]; !ok {
		if traceID := TraceIDFromContext(ctx); traceID != "" {
			headers[TraceIDHeader] = traceID
		}
	}
	return headers
}

func enqueueOptions(req EnqueueRequest) []asynq.Option {
	opts := []asynq.Option{asynq.Queue(req.Queue)}
	if req.ID != "" {
		opts = append(opts, asynq.TaskID(req.ID))
	}
	opts = append(opts, asynq.MaxRetry(0))
	if req.Timeout > 0 {
		opts = append(opts, asynq.Timeout(req.Timeout))
	}
	if req.Retention > 0 {
		opts = append(opts, asynq.Retention(req.Retention))
	}
	if req.UniqueTTL > 0 {
		opts = append(opts, asynq.Unique(req.UniqueTTL))
	}
	if !req.ProcessAt.IsZero() {
		opts = append(opts, asynq.ProcessAt(req.ProcessAt))
	}
	if req.ProcessIn > 0 {
		opts = append(opts, asynq.ProcessIn(req.ProcessIn))
	}
	return opts
}

func convertTaskInfo(info *asynq.TaskInfo) *TaskInfo {
	if info == nil {
		return nil
	}
	return &TaskInfo{
		ID:      info.ID,
		Type:    info.Type,
		Queue:   info.Queue,
		State:   info.State.String(),
		Retried: info.Retried,
	}
}
