package kafka

import (
	"fmt"
	"sync"
)

// ConsumerRegistryKey registry key in the Registry
const ConsumerRegistryKey = "kafka.consumer.registry"

// ConsumerRegistry consumer registry table
// For centralized management of all consumer Handlers
type ConsumerRegistry struct {
	handlers map[string]ConsumerHandler
	mu       sync.RWMutex
}

// Create consumer registry
func NewConsumerRegistry() *ConsumerRegistry {
	return &ConsumerRegistry{
		handlers: make(map[string]ConsumerHandler),
	}
}

// Register consumer
func (r *ConsumerRegistry) Register(handler ConsumerHandler) error {
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}
	if handler.Name() == "" {
		return fmt.Errorf("handler name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[handler.Name()]; exists {
		return fmt.Errorf("consumer handler %s already registered", handler.Name())
	}

	r.handlers[handler.Name()] = handler
	return nil
}

// MustRegister register consumer (panic on failure)
func (r *ConsumerRegistry) MustRegister(handler ConsumerHandler) {
	if err := r.Register(handler); err != nil {
		panic(err)
	}
}

// Get consumer
func (r *ConsumerRegistry) Get(name string) (ConsumerHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, ok := r.handlers[name]
	return handler, ok
}

// Get all consumers
func (r *ConsumerRegistry) All() []ConsumerHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handlers := make([]ConsumerHandler, 0, len(r.handlers))
	for _, h := range r.handlers {
		handlers = append(handlers, h)
	}
	return handlers
}

// Get all consumer names
func (r *ConsumerRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.handlers))
	for name := range r.handlers {
		names = append(names, name)
	}
	return names
}

// Count Get consumer quantity
func (r *ConsumerRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.handlers)
}

// Unregister deregister consumer
func (r *ConsumerRegistry) Unregister(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[name]; exists {
		delete(r.handlers, name)
		return true
	}
	return false
}
