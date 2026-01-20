package breaker

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Event bus implementation
type eventBus struct {
	listeners map[SubscriptionID]*subscription
	buffer    chan Event
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	closed    int32 // Use atomic flag to indicate if already closed
}

// subscription information
type subscription struct {
	id       SubscriptionID
	listener EventListener
	filters  map[EventType]bool
}

// Create event bus
func NewEventBus(bufferSize int) EventBus {
	ctx, cancel := context.WithCancel(context.Background())
	
	bus := &eventBus{
		listeners: make(map[SubscriptionID]*subscription),
		buffer:    make(chan Event, bufferSize),
		ctx:       ctx,
		cancel:    cancel,
	}
	
	// Start event distribution coroutine
	bus.wg.Add(1)
	go bus.dispatch()
	
	return bus
}

// Subscribe to event
func (eb *eventBus) Subscribe(listener EventListener, filters ...EventType) SubscriptionID {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	
	id := SubscriptionID(time.Now().Format("20060102150405.000000"))
	
	filterMap := make(map[EventType]bool)
	if len(filters) > 0 {
		for _, f := range filters {
			filterMap[f] = true
		}
	}
	
	eb.listeners[id] = &subscription{
		id:       id,
		listener: listener,
		filters:  filterMap,
	}
	
	return id
}

// Unsubscribe from subscription
func (eb *eventBus) Unsubscribe(id SubscriptionID) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	
	delete(eb.listeners, id)
}

// Publish event
func (eb *eventBus) Publish(event Event) {
	// Check if closed
	if atomic.LoadInt32(&eb.closed) == 1 {
		return
	}
	
	select {
	case eb.buffer <- event:
		// Successful publication
	case <-eb.ctx.Done():
		// Bus is closed, silently ignore
		return
	default:
		// Buffer full, discard event (or can choose to block)
	}
}

// Close event bus
func (eb *eventBus) Close() {
	// marked as closed
	atomic.StoreInt32(&eb.closed, 1)
	// First cancel the context, then trigger dispatch to exit
	eb.cancel()
	// wait for dispatch to finish processing remaining events
	eb.wg.Wait()
	// Finally close the channel (at this point, dispatch has exited and will not read anymore)
	close(eb.buffer)
}

// dispatch events to subscribers
func (eb *eventBus) dispatch() {
	defer eb.wg.Done()
	
	for {
		select {
		case event, ok := <-eb.buffer:
			if !ok {
				return
			}
			eb.notifyListeners(event)
			
		case <-eb.ctx.Done():
			// Handle remaining events
			for {
				select {
				case event, ok := <-eb.buffer:
					if !ok {
						return
					}
					eb.notifyListeners(event)
				default:
					return
				}
			}
		}
	}
}

// notifyListeners notifies all matching listeners
func (eb *eventBus) notifyListeners(event Event) {
	eb.mu.RLock()
	// Copy the list of listeners to avoid holding the lock for too long
	listeners := make([]*subscription, 0, len(eb.listeners))
	for _, sub := range eb.listeners {
		listeners = append(listeners, sub)
	}
	eb.mu.RUnlock()
	
	eventType := event.Type()
	
	for _, sub := range listeners {
		// Check filter
		if len(sub.filters) > 0 && !sub.filters[eventType] {
			continue
		}
		
		// Asynchronous notification (avoid blocking)
		go func(l EventListener, e Event) {
			defer func() {
				// Capture listener panic
				if r := recover(); r != nil {
					// can log messages
				}
			}()
			l.OnEvent(e)
		}(sub.listener, event)
	}
}

