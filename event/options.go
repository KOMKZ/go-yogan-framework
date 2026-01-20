package event

// listener entry
type listenerEntry struct {
	id       uint64   // Unique ID (for unsubscribing)
	listener Listener // listener
	priority int      // Priority (the smaller the number, the higher the priority)
	async    bool     // Is asynchronous execution
	once     bool     // Should it be executed only once?
}

// SubscribeOption subscription options
type SubscribeOption func(*listenerEntry)

// WithPriority sets the priority
// The smaller the number, the higher the priority, and it is executed first
// Default priority is 0
func WithPriority(priority int) SubscribeOption {
	return func(e *listenerEntry) {
		e.priority = priority
	}
}

// WithAsync marked as asynchronous listener
// Even with Dispatch synchronous distribution, this listener will execute asynchronously
// Asynchronous listener errors will not affect event propagation
func WithAsync() SubscribeOption {
	return func(e *listenerEntry) {
		e.async = true
	}
}

// Executes only once and then automatically unsubscribes
func WithOnce() SubscribeOption {
	return func(e *listenerEntry) {
		e.once = true
	}
}

// DispatcherOption Dispatcher configuration options
type DispatcherOption func(*dispatcher)

// WithPoolSize sets the size of the asynchronous coroutine pool
func WithPoolSize(size int) DispatcherOption {
	return func(d *dispatcher) {
		d.poolSize = size
	}
}

func WithSetAllSync(v bool) DispatcherOption {
	return func(d *dispatcher) {
		d.setAllSync = v
	}
}

// WithKafkaPublisher set Kafka publisher
// After setting up, use the WithKafka() option to send events to Kafka
func WithKafkaPublisher(publisher KafkaPublisher) DispatcherOption {
	return func(d *dispatcher) {
		d.kafkaPublisher = publisher
	}
}

// WithRouter sets the event router
// The router decides based on the configuration whether an event is sent to Kafka or to memory
// Priority: Code option > Configured route > Default (memory)
func WithRouter(router *Router) DispatcherOption {
	return func(d *dispatcher) {
		d.router = router
	}
}
