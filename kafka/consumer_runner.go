package kafka

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// ConsumerRunnerConfig runner configuration
type ConsumerRunnerConfig struct {
	// GroupID consumer group (optional, defaults to handler.Name())
	GroupID string

	// Number of concurrent consumer workers (default 1)
	Workers int

	// OffsetInitial Initial Offset: -1=Newest, -2=Oldest (default -1)
	OffsetInitial int64

	// Whether to auto-commit (default true)
	AutoCommit bool

	// AutoCommitInterval Automatic commit interval (default 1s)
	AutoCommitInterval time.Duration

	// MaxProcessingTime Maximum processing time for a single message (default 30s)
	MaxProcessingTime time.Duration

	// Session timeout (default 10 seconds)
	SessionTimeout time.Duration

	// HeartbeatInterval heartbeat interval (default 3s)
	HeartbeatInterval time.Duration
}

// Apply default values
func (c *ConsumerRunnerConfig) applyDefaults(handlerName string) {
	if c.GroupID == "" {
		c.GroupID = handlerName + "-group"
	}
	if c.Workers <= 0 {
		c.Workers = 1
	}
	if c.OffsetInitial == 0 {
		c.OffsetInitial = -1 // Newest
	}
	if c.AutoCommitInterval == 0 {
		c.AutoCommitInterval = time.Second
	}
	if c.MaxProcessingTime == 0 {
		c.MaxProcessingTime = 30 * time.Second
	}
	if c.SessionTimeout == 0 {
		c.SessionTimeout = 10 * time.Second
	}
	if c.HeartbeatInterval == 0 {
		c.HeartbeatInterval = 3 * time.Second
	}
}

// ConsumerRunner consumer runner
// Encapsulate signal handling, worker management, lifecycle control
type ConsumerRunner struct {
	manager *Manager
	handler ConsumerHandler
	config  ConsumerRunnerConfig
	logger  *zap.Logger

	consumers []*ConsumerGroup
	wg        sync.WaitGroup
	cancel    context.CancelFunc
	mu        sync.RWMutex
	running   bool
}

// Create consumer runner
func NewConsumerRunner(manager *Manager, handler ConsumerHandler, cfg ConsumerRunnerConfig) *ConsumerRunner {
	cfg.applyDefaults(handler.Name())

	return &ConsumerRunner{
		manager: manager,
		handler: handler,
		config:  cfg,
		logger:  manager.logger.With(zap.String("consumer", handler.Name())),
	}
}

// Run blocking execution (internal signal handling)
// will block until a SIGINT/SIGTERM signal is received
func (r *ConsumerRunner) Run(ctx context.Context) error {
	// Start consumer
	if err := r.Start(ctx); err != nil {
		return err
	}

	// Set signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	r.logger.Info("📡 English: .Consumer running, waiting for messages... (Press Ctrl+C to exit), English: .Consumer running, waiting for messages... (Press Ctrl+C to exit)... (English: .Consumer running, waiting for messages... (Press Ctrl+C to exit) Ctrl+C English: .Consumer running, waiting for messages... (Press Ctrl+C to exit))：.Consumer running, waiting for messages... (Press Ctrl+C to exit)，English: .Consumer running, waiting for messages... (Press Ctrl+C to exit), English: .Consumer running, waiting for messages... (Press Ctrl+C to exit)... (English: .Consumer running, waiting for messages... (Press Ctrl+C to exit) Ctrl+C English: .Consumer running, waiting for messages... (Press Ctrl+C to exit))：.Consumer running, waiting for messages... (Press Ctrl+C to exit)... (English: .Consumer running, waiting for messages... (Press Ctrl+C to exit), English: .Consumer running, waiting for messages... (Press Ctrl+C to exit)... (English: .Consumer running, waiting for messages... (Press Ctrl+C to exit) Ctrl+C English: .Consumer running, waiting for messages... (Press Ctrl+C to exit))：.Consumer running, waiting for messages... (Press Ctrl+C to exit) Ctrl+C English: .Consumer running, waiting for messages... (Press Ctrl+C to exit), English: .Consumer running, waiting for messages... (Press Ctrl+C to exit)... (English: .Consumer running, waiting for messages... (Press Ctrl+C to exit) Ctrl+C English: .Consumer running, waiting for messages... (Press Ctrl+C to exit))：.Consumer running, waiting for messages... (Press Ctrl+C to exit))",
		zap.String("group_id", r.config.GroupID),
		zap.Strings("topics", r.handler.Topics()),
		zap.Int("workers", r.config.Workers))

	// wait for signal or context cancellation
	select {
	case sig := <-sigCh:
		r.logger.Info("🛑 English: STOP Received exit signal", zap.String("signal", sig.String()))
	case <-ctx.Done():
		r.logger.Info("🛑 English: ⛔ Context has been cancelled")
	}

	// Stop consumer
	return r.Stop()
}

// Start Non-blocking startup
func (r *ConsumerRunner) Start(ctx context.Context) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return fmt.Errorf("consumer runner is already running")
	}
	r.running = true
	r.mu.Unlock()

	// Create cancellable context
	runCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	// Build consumer configuration
	consumerCfg := ConsumerConfig{
		GroupID:            r.config.GroupID,
		Topics:             r.handler.Topics(),
		OffsetInitial:      r.config.OffsetInitial,
		AutoCommit:         r.config.AutoCommit,
		AutoCommitInterval: r.config.AutoCommitInterval,
		MaxProcessingTime:  r.config.MaxProcessingTime,
		SessionTimeout:     r.config.SessionTimeout,
		HeartbeatInterval:  r.config.HeartbeatInterval,
	}

	// Start multiple Workers
	r.consumers = make([]*ConsumerGroup, r.config.Workers)
	for i := 0; i < r.config.Workers; i++ {
		workerID := i + 1
		consumerName := fmt.Sprintf("%s-worker-%d", r.handler.Name(), workerID)

		consumer, err := r.manager.CreateConsumer(consumerName, consumerCfg)
		if err != nil {
			// Clean up created consumers
			r.cleanupConsumers()
			return fmt.Errorf("create consumer %s failed: %w", consumerName, err)
		}

		r.consumers[i] = consumer
		r.wg.Add(1)

		go r.runWorker(runCtx, workerID, consumer)

		r.logger.Info("✅ Worker English: Worker started successfully",
			zap.Int("worker_id", workerID),
			zap.String("consumer", consumerName))
	}

	r.logger.Info("🚀 English: Rocket Consumer runner has started",
		zap.String("name", r.handler.Name()),
		zap.String("group_id", r.config.GroupID),
		zap.Int("workers", r.config.Workers),
		zap.Strings("topics", r.handler.Topics()))

	return nil
}

// runWorker runs a single Worker
func (r *ConsumerRunner) runWorker(ctx context.Context, workerID int, consumer *ConsumerGroup) {
	defer r.wg.Done()

	// Wrap handler, add workerID to log
	wrappedHandler := func(ctx context.Context, msg *ConsumedMessage) error {
		return r.handler.Handle(ctx, msg)
	}

	err := consumer.Start(ctx, wrappedHandler)
	if err != nil && err != context.Canceled {
		r.logger.Error("worker English: worker abnormally exited",
			zap.Int("worker_id", workerID),
			zap.Error(err))
	}
}

// Graceful shutdown
func (r *ConsumerRunner) Stop() error {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return nil
	}
	r.running = false
	r.mu.Unlock()

	r.logger.Info("🛑 Stopping consumer......")

	// Cancel context
	if r.cancel != nil {
		r.cancel()
	}

	// Stop all consumers
	r.cleanupConsumers()

	// wait for all workers to complete
	r.wg.Wait()

	r.logger.Info("✅ English: The consumer runner has stopped", zap.String("name", r.handler.Name()))
	return nil
}

// cleanupConsumers Clean up all consumers
func (r *ConsumerRunner) cleanupConsumers() {
	for i, consumer := range r.consumers {
		if consumer != nil {
			if err := consumer.Stop(); err != nil {
				r.logger.Error("stop consumer failed",
					zap.Int("worker_id", i+1),
					zap.Error(err))
			}
		}
	}
}

// Check if running
func (r *ConsumerRunner) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

// GetConfig Retrieve configuration
func (r *ConsumerRunner) GetConfig() ConsumerRunnerConfig {
	return r.config
}
