package limiter

import (
	"sync"
)

// Event bus implementation
type eventBus struct {
	listeners []EventListener
	eventChan chan Event
	closed    bool
	mu        sync.RWMutex
	wg        sync.WaitGroup
}

// Create event bus
func NewEventBus(bufferSize int) EventBus {
	if bufferSize <= 0 {
		bufferSize = 100
	}

	bus := &eventBus{
		listeners: make([]EventListener, 0),
		eventChan: make(chan Event, bufferSize),
	}

	// Start event distribution coroutine
	bus.wg.Add(1)
	go bus.dispatch()

	return bus
}

// Subscribe to event
func (b *eventBus) Subscribe(listener EventListener) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}

	b.listeners = append(b.listeners, listener)
}

// Publish event
func (b *eventBus) Publish(event Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return
	}

	// non-blocking send
	select {
	case b.eventChan <- event:
	default:
		// Buffer full, discard event (prevent blocking)
	}
}

// Close event bus
func (b *eventBus) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	close(b.eventChan)
	b.mu.Unlock()

	// wait for distribution coroutine to finish
	b.wg.Wait()
}

// dispatch events to all listeners
func (b *eventBus) dispatch() {
	defer b.wg.Done()

	for event := range b.eventChan {
		b.mu.RLock()
		listeners := make([]EventListener, len(b.listeners))
		copy(listeners, b.listeners)
		b.mu.RUnlock()

		// Notify all listeners
		for _, listener := range listeners {
			// safe call to avoid panic affecting other listeners
			func() {
				defer func() {
					if r := recover(); r != nil {
						// Ignore panic
					}
				}()
				listener.OnEvent(event)
			}()
		}
	}
}

