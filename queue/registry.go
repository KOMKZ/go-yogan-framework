package queue

import (
	"context"
	"fmt"
	"sync"
)

type Registry struct {
	mu       sync.RWMutex
	handlers map[string]HandlerDescriptor
}

func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]HandlerDescriptor),
	}
}

func (r *Registry) Register(desc HandlerDescriptor) error {
	if desc.Type == "" {
		return fmt.Errorf("queue handler type is required")
	}
	if desc.Handler == nil {
		return fmt.Errorf("queue handler %q is nil", desc.Type)
	}
	if desc.Queue == "" {
		desc.Queue = DefaultQueue
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[desc.Type]; exists {
		return fmt.Errorf("queue handler %q already registered", desc.Type)
	}
	r.handlers[desc.Type] = desc
	return nil
}

// RegisterMany registers a batch of descriptors in order.
func (r *Registry) RegisterMany(descs ...HandlerDescriptor) error {
	for _, desc := range descs {
		if err := r.Register(desc); err != nil {
			return err
		}
	}
	return nil
}

func (r *Registry) MustRegister(desc HandlerDescriptor) {
	if err := r.Register(desc); err != nil {
		panic(err)
	}
}

// Dispatch executes the registered handler for a task type in-process.
func (r *Registry) Dispatch(ctx context.Context, task Task) error {
	desc, ok := r.Get(task.Type)
	if !ok {
		return fmt.Errorf("queue handler %q is not registered", task.Type)
	}
	if desc.Handler == nil {
		return fmt.Errorf("queue handler %q is nil", task.Type)
	}
	if task.Queue == "" {
		task.Queue = desc.Queue
	}
	return desc.Handler.HandleTask(ctx, task)
}

func (r *Registry) Get(taskType string) (HandlerDescriptor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	desc, ok := r.handlers[taskType]
	return desc, ok
}

func (r *Registry) List() []HandlerDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]HandlerDescriptor, 0, len(r.handlers))
	for _, desc := range r.handlers {
		items = append(items, desc)
	}
	return items
}

func (r *Registry) ValidateConfig(cfg Config) error {
	for _, desc := range r.List() {
		taskCfg := cfg.TaskConfig(desc.Type)
		queueName := desc.Queue
		if queueName == "" {
			queueName = taskCfg.Queue
		}
		if _, err := cfg.QueueRoute(queueName); err != nil {
			return fmt.Errorf("queue handler %q uses invalid queue %q: %w", desc.Type, queueName, err)
		}
		if !queueInAnyProfile(cfg, queueName) {
			return fmt.Errorf("queue handler %q uses queue %q not covered by any profile", desc.Type, queueName)
		}
	}
	return nil
}

func queueInAnyProfile(cfg Config, queueName string) bool {
	for _, profile := range cfg.Profiles {
		for _, worker := range profile.Workers {
			if _, ok := worker.Queues[queueName]; ok {
				return true
			}
		}
	}
	return false
}
