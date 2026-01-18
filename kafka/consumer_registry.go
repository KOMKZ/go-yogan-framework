package kafka

import (
	"fmt"
	"sync"
)

// ConsumerRegistryKey 注册表在 Registry 中的 Key
const ConsumerRegistryKey = "kafka.consumer.registry"

// ConsumerRegistry 消费者注册表
// 用于集中管理所有消费者 Handler
type ConsumerRegistry struct {
	handlers map[string]ConsumerHandler
	mu       sync.RWMutex
}

// NewConsumerRegistry 创建消费者注册表
func NewConsumerRegistry() *ConsumerRegistry {
	return &ConsumerRegistry{
		handlers: make(map[string]ConsumerHandler),
	}
}

// Register 注册消费者
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

// MustRegister 注册消费者（失败时 panic）
func (r *ConsumerRegistry) MustRegister(handler ConsumerHandler) {
	if err := r.Register(handler); err != nil {
		panic(err)
	}
}

// Get 获取消费者
func (r *ConsumerRegistry) Get(name string) (ConsumerHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, ok := r.handlers[name]
	return handler, ok
}

// All 获取所有消费者
func (r *ConsumerRegistry) All() []ConsumerHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handlers := make([]ConsumerHandler, 0, len(r.handlers))
	for _, h := range r.handlers {
		handlers = append(handlers, h)
	}
	return handlers
}

// Names 获取所有消费者名称
func (r *ConsumerRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.handlers))
	for name := range r.handlers {
		names = append(names, name)
	}
	return names
}

// Count 获取消费者数量
func (r *ConsumerRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.handlers)
}

// Unregister 取消注册消费者
func (r *ConsumerRegistry) Unregister(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[name]; exists {
		delete(r.handlers, name)
		return true
	}
	return false
}
