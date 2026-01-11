package limiter

import (
	"sync"
)

// eventBus 事件总线实现
type eventBus struct {
	listeners []EventListener
	eventChan chan Event
	closed    bool
	mu        sync.RWMutex
	wg        sync.WaitGroup
}

// NewEventBus 创建事件总线
func NewEventBus(bufferSize int) EventBus {
	if bufferSize <= 0 {
		bufferSize = 100
	}

	bus := &eventBus{
		listeners: make([]EventListener, 0),
		eventChan: make(chan Event, bufferSize),
	}

	// 启动事件分发协程
	bus.wg.Add(1)
	go bus.dispatch()

	return bus
}

// Subscribe 订阅事件
func (b *eventBus) Subscribe(listener EventListener) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}

	b.listeners = append(b.listeners, listener)
}

// Publish 发布事件
func (b *eventBus) Publish(event Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return
	}

	// 非阻塞发送
	select {
	case b.eventChan <- event:
	default:
		// 缓冲区满，丢弃事件（防止阻塞）
	}
}

// Close 关闭事件总线
func (b *eventBus) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	close(b.eventChan)
	b.mu.Unlock()

	// 等待分发协程结束
	b.wg.Wait()
}

// dispatch 分发事件到所有监听器
func (b *eventBus) dispatch() {
	defer b.wg.Done()

	for event := range b.eventChan {
		b.mu.RLock()
		listeners := make([]EventListener, len(b.listeners))
		copy(listeners, b.listeners)
		b.mu.RUnlock()

		// 通知所有监听器
		for _, listener := range listeners {
			// 安全调用，避免panic影响其他监听器
			func() {
				defer func() {
					if r := recover(); r != nil {
						// 忽略panic
					}
				}()
				listener.OnEvent(event)
			}()
		}
	}
}

