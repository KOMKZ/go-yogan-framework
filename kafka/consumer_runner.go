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

// ConsumerRunnerConfig 运行器配置
type ConsumerRunnerConfig struct {
	// GroupID 消费者组（可选，默认使用 handler.Name()）
	GroupID string

	// Workers 并发消费者数量（默认 1）
	Workers int

	// OffsetInitial 初始 Offset：-1=Newest, -2=Oldest（默认 -1）
	OffsetInitial int64

	// AutoCommit 是否自动提交（默认 true）
	AutoCommit bool

	// AutoCommitInterval 自动提交间隔（默认 1s）
	AutoCommitInterval time.Duration

	// MaxProcessingTime 单条消息最大处理时间（默认 30s）
	MaxProcessingTime time.Duration

	// SessionTimeout 会话超时（默认 10s）
	SessionTimeout time.Duration

	// HeartbeatInterval 心跳间隔（默认 3s）
	HeartbeatInterval time.Duration
}

// applyDefaults 应用默认值
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

// ConsumerRunner 消费者运行器
// 封装信号处理、Worker 管理、生命周期控制
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

// NewConsumerRunner 创建消费者运行器
func NewConsumerRunner(manager *Manager, handler ConsumerHandler, cfg ConsumerRunnerConfig) *ConsumerRunner {
	cfg.applyDefaults(handler.Name())

	return &ConsumerRunner{
		manager: manager,
		handler: handler,
		config:  cfg,
		logger:  manager.logger.With(zap.String("consumer", handler.Name())),
	}
}

// Run 阻塞运行（内部处理信号）
// 会阻塞直到收到 SIGINT/SIGTERM 信号
func (r *ConsumerRunner) Run(ctx context.Context) error {
	// 启动消费者
	if err := r.Start(ctx); err != nil {
		return err
	}

	// 设置信号处理
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	r.logger.Info("📡 消费者运行中，等待消息... (按 Ctrl+C 退出)",
		zap.String("group_id", r.config.GroupID),
		zap.Strings("topics", r.handler.Topics()),
		zap.Int("workers", r.config.Workers))

	// 等待信号或上下文取消
	select {
	case sig := <-sigCh:
		r.logger.Info("🛑 收到退出信号", zap.String("signal", sig.String()))
	case <-ctx.Done():
		r.logger.Info("🛑 上下文已取消")
	}

	// 停止消费者
	return r.Stop()
}

// Start 非阻塞启动
func (r *ConsumerRunner) Start(ctx context.Context) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return fmt.Errorf("consumer runner is already running")
	}
	r.running = true
	r.mu.Unlock()

	// 创建可取消的上下文
	runCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	// 构建消费者配置
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

	// 启动多个 Worker
	r.consumers = make([]*ConsumerGroup, r.config.Workers)
	for i := 0; i < r.config.Workers; i++ {
		workerID := i + 1
		consumerName := fmt.Sprintf("%s-worker-%d", r.handler.Name(), workerID)

		consumer, err := r.manager.CreateConsumer(consumerName, consumerCfg)
		if err != nil {
			// 清理已创建的消费者
			r.cleanupConsumers()
			return fmt.Errorf("create consumer %s failed: %w", consumerName, err)
		}

		r.consumers[i] = consumer
		r.wg.Add(1)

		go r.runWorker(runCtx, workerID, consumer)

		r.logger.Info("✅ Worker 启动成功",
			zap.Int("worker_id", workerID),
			zap.String("consumer", consumerName))
	}

	r.logger.Info("🚀 消费者运行器已启动",
		zap.String("name", r.handler.Name()),
		zap.String("group_id", r.config.GroupID),
		zap.Int("workers", r.config.Workers),
		zap.Strings("topics", r.handler.Topics()))

	return nil
}

// runWorker 运行单个 Worker
func (r *ConsumerRunner) runWorker(ctx context.Context, workerID int, consumer *ConsumerGroup) {
	defer r.wg.Done()

	// 包装 handler，添加 workerID 到日志
	wrappedHandler := func(ctx context.Context, msg *ConsumedMessage) error {
		return r.handler.Handle(ctx, msg)
	}

	err := consumer.Start(ctx, wrappedHandler)
	if err != nil && err != context.Canceled {
		r.logger.Error("worker 异常退出",
			zap.Int("worker_id", workerID),
			zap.Error(err))
	}
}

// Stop 优雅停止
func (r *ConsumerRunner) Stop() error {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return nil
	}
	r.running = false
	r.mu.Unlock()

	r.logger.Info("🛑 正在停止消费者...")

	// 取消上下文
	if r.cancel != nil {
		r.cancel()
	}

	// 停止所有消费者
	r.cleanupConsumers()

	// 等待所有 Worker 完成
	r.wg.Wait()

	r.logger.Info("✅ 消费者运行器已停止", zap.String("name", r.handler.Name()))
	return nil
}

// cleanupConsumers 清理所有消费者
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

// IsRunning 检查是否运行中
func (r *ConsumerRunner) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

// GetConfig 获取配置
func (r *ConsumerRunner) GetConfig() ConsumerRunnerConfig {
	return r.config
}
