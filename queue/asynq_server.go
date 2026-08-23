package queue

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

var ErrDisabled = errors.New("queue component is disabled")

type Server struct {
	cfg      Config
	registry *Registry
	log      *logger.CtxZapLogger

	mu      sync.Mutex
	started bool
	servers map[string]*asynq.Server
}

func NewServer(cfg Config, registry *Registry, log *logger.CtxZapLogger) (*Server, error) {
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if registry == nil {
		registry = NewRegistry()
	}
	if log == nil {
		log = logger.GetLogger("queue_worker")
	}
	return &Server{
		cfg:      cfg,
		registry: registry,
		log:      log,
		servers:  map[string]*asynq.Server{},
	}, nil
}

func (s *Server) Start(ctx context.Context, profileName string) error {
	if s == nil {
		return ErrDisabled
	}
	if !s.cfg.Enabled {
		return ErrDisabled
	}

	workers, err := s.cfg.ProfileWorkers(profileName)
	if err != nil {
		return err
	}
	if err := s.registry.ValidateConfig(s.cfg); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return nil
	}

	for _, worker := range workers {
		broker := s.cfg.Brokers[worker.Broker]
		server := asynq.NewServer(redisClientOpt(broker.Redis), s.asynqConfig(ctx, worker))
		if err := server.Start(s.buildMux()); err != nil {
			s.shutdownStartedServers()
			return fmt.Errorf("start asynq server for broker %q: %w", worker.Broker, err)
		}
		s.servers[worker.Broker] = server
		s.log.InfoCtx(ctx, "queue worker broker server started",
			zap.String("driver", s.cfg.Driver),
			zap.String("profile", profileName),
			zap.String("broker", worker.Broker),
			zap.Int("concurrency", worker.Concurrency),
			zap.Any("queues", worker.Queues))
	}

	s.started = true
	s.log.InfoCtx(ctx, "queue worker server started",
		zap.String("driver", s.cfg.Driver),
		zap.String("profile", profileName),
		zap.Int("brokers", len(workers)))
	return nil
}

func (s *Server) Shutdown() error {
	if s == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return nil
	}
	s.shutdownStartedServers()
	s.started = false
	return nil
}

func (s *Server) IsStarted() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.started
}

func (s *Server) shutdownStartedServers() {
	for broker, server := range s.servers {
		if server != nil {
			server.Shutdown()
		}
		delete(s.servers, broker)
	}
}

func (s *Server) asynqConfig(ctx context.Context, worker BrokerWorker) asynq.Config {
	return asynq.Config{
		Concurrency:     worker.Concurrency,
		Queues:          s.physicalQueues(worker.Queues),
		StrictPriority:  worker.StrictPriority,
		ShutdownTimeout: s.cfg.ShutdownTimeout,
		BaseContext: func() context.Context {
			return ctx
		},
		Logger: asynqLogger{log: s.log},
		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
			taskID, _ := asynq.GetTaskID(ctx)
			queueName, _ := asynq.GetQueueName(ctx)
			s.log.ErrorCtx(traceContext(ctx, task), "queue task failed",
				zap.String("task_id", taskID),
				zap.String("task_type", task.Type()),
				zap.String("queue", queueName),
				zap.Error(err))
		}),
	}
}

func (s *Server) physicalQueues(logicalQueues map[string]int) map[string]int {
	queues := make(map[string]int, len(logicalQueues))
	for logicalQueue, weight := range logicalQueues {
		route, err := s.cfg.QueueRoute(logicalQueue)
		if err != nil {
			continue
		}
		queues[route.PhysicalName] = weight
	}
	return queues
}

func (s *Server) buildMux() *asynq.ServeMux {
	mux := asynq.NewServeMux()
	for _, desc := range s.registry.List() {
		desc := desc
		mux.HandleFunc(desc.Type, func(ctx context.Context, task *asynq.Task) error {
			return s.handleTask(ctx, task, desc)
		})
	}
	return mux
}

func (s *Server) handleTask(ctx context.Context, task *asynq.Task, desc HandlerDescriptor) error {
	ctx = traceContext(ctx, task)
	taskID, _ := asynq.GetTaskID(ctx)
	queueName, _ := asynq.GetQueueName(ctx)

	s.log.InfoCtx(ctx, "queue task received",
		zap.String("task_id", taskID),
		zap.String("task_type", task.Type()),
		zap.String("queue", queueName))

	err := s.registry.Dispatch(ctx, Task{
		ID:      taskID,
		Type:    task.Type(),
		Payload: task.Payload(),
		Queue:   desc.Queue,
		Headers: task.Headers(),
	})
	if err != nil {
		return err
	}

	s.log.InfoCtx(ctx, "queue task completed",
		zap.String("task_id", taskID),
		zap.String("task_type", task.Type()),
		zap.String("queue", queueName))
	return nil
}

func traceContext(ctx context.Context, task *asynq.Task) context.Context {
	if task == nil {
		return ctx
	}
	return ContextWithTraceID(ctx, task.Headers()[TraceIDHeader])
}

type asynqLogger struct {
	log *logger.CtxZapLogger
}

func (l asynqLogger) Debug(args ...interface{}) { l.write("debug", args...) }
func (l asynqLogger) Info(args ...interface{})  { l.write("info", args...) }
func (l asynqLogger) Warn(args ...interface{})  { l.write("warn", args...) }
func (l asynqLogger) Error(args ...interface{}) { l.write("error", args...) }
func (l asynqLogger) Fatal(args ...interface{}) { l.write("error", args...) }

func (l asynqLogger) write(level string, args ...interface{}) {
	if l.log == nil {
		return
	}
	msg := strings.TrimSpace(fmt.Sprint(args...))
	switch level {
	case "debug":
		l.log.Debug(msg)
	case "warn":
		l.log.Warn(msg)
	case "error":
		l.log.Error(msg)
	default:
		l.log.Info(msg)
	}
}
