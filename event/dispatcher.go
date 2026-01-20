package event

import (
	"context"
	"errors"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/panjf2000/ants/v2"
	"go.uber.org/zap"
)

// Unsubscribe function
type UnsubscribeFunc func()

// Dispatcher event dispatcher interface
type Dispatcher interface {
	// Subscribe to event, return unsubscribe function
	Subscribe(eventName string, listener Listener, opts ...SubscribeOption) UnsubscribeFunc

	// Dispatch event distribution
	// Supports DispatchOption to control distribution behavior:
	// - Default: memory synchronization distribution
	// - WithDispatchAsync(): Memory asynchronous distribution
	// - WithKafka(topic): send to Kafka
	// - WithKafka(topic) + WithDispatchAsync(): Asynchronously send to Kafka
	Dispatch(ctx context.Context, event Event, opts ...DispatchOption) error

	// DispatchAsync asynchronously dispatches events (compatible with old API)
	// Equivalent to Dispatch(ctx, event, WithDispatchAsync())
	DispatchAsync(ctx context.Context, event Event)

	// Use register global interceptors
	Use(interceptor Interceptor)
}

// dispatcher event dispatcher implementation
type dispatcher struct {
	mu             sync.RWMutex
	listeners      map[string][]listenerEntry
	interceptors   []Interceptor
	nextID         uint64
	pool           *ants.Pool
	poolSize       int
	logger         *logger.CtxZapLogger
	closed         int32
	kafkaPublisher KafkaPublisher // Kafka publisher (optional)
	router         *Router        // Event router (optional)
	setAllSync     bool
}

// Create event dispatcher
func NewDispatcher(opts ...DispatcherOption) *dispatcher {
	d := &dispatcher{
		listeners: make(map[string][]listenerEntry),
		poolSize:  100,
		logger:    logger.GetLogger("yogan"),
	}

	for _, opt := range opts {
		opt(d)
	}

	// Create coroutine pool
	var err error
	d.pool, err = ants.NewPool(d.poolSize)
	if err != nil {
		d.logger.Error("创建协程池失败，使用默认配置", zap.Error(err))
		d.pool, _ = ants.NewPool(100)
	}

	return d
}

// Subscribe to event
func (d *dispatcher) Subscribe(eventName string, listener Listener, opts ...SubscribeOption) UnsubscribeFunc {
	if eventName == "" || listener == nil {
		return func() {}
	}

	entry := listenerEntry{
		id:       atomic.AddUint64(&d.nextID, 1),
		listener: listener,
	}

	for _, opt := range opts {
		opt(&entry)
		if d.setAllSync {
			entry.async = false
		}
	}

	d.mu.Lock()
	d.listeners[eventName] = append(d.listeners[eventName], entry)
	// Sort by priority
	sort.SliceStable(d.listeners[eventName], func(i, j int) bool {
		return d.listeners[eventName][i].priority < d.listeners[eventName][j].priority
	})
	d.mu.Unlock()

	// Return the unsubscribe function
	return func() {
		d.unsubscribe(eventName, entry.id)
	}
}

// unsubscribe cancel subscription
func (d *dispatcher) unsubscribe(eventName string, id uint64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	entries := d.listeners[eventName]
	for i, e := range entries {
		if e.id == id {
			d.listeners[eventName] = append(entries[:i], entries[i+1:]...)
			return
		}
	}
}

// Use register global interceptors
func (d *dispatcher) Use(interceptor Interceptor) {
	d.mu.Lock()
	d.interceptors = append(d.interceptors, interceptor)
	d.mu.Unlock()
}

// Dispatch event distribution
// Priority: Code option > Configured route > Default (memory)
func (d *dispatcher) Dispatch(ctx context.Context, event Event, opts ...DispatchOption) error {
	if event == nil {
		return nil
	}

	// Parse options
	options := &dispatchOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// If the code does not explicitly specify the driver, try to obtain it from the route configuration
	if !options.driverExplicit && d.router != nil {
		if route := d.router.Match(event.Name()); route != nil {
			d.logger.DebugCtx(ctx, "🎯 事件路由匹配成功",
				zap.String("event", event.Name()),
				zap.String("driver", route.Driver),
				zap.String("topic", route.Topic))
			options.driver = route.Driver
			if route.Driver == DriverKafka && options.topic == "" {
				options.topic = route.Topic
			}
		}
	}

	// Apply default values
	options.applyDefaults()

	// Distribute based on drive selection
	switch options.driver {
	case DriverKafka:
		return d.dispatchToKafka(ctx, event, options)
	default:
		// setAllSync force synchronization distribution (ignore options.async)
		if options.async && !d.setAllSync {
			d.dispatchAsyncMemory(ctx, event)
			return nil
		}
		return d.dispatchMemory(ctx, event)
	}
}

// dispatch memory synchronization distribution
func (d *dispatcher) dispatchMemory(ctx context.Context, event Event) error {
	// Get a copy of the interceptors and listeners
	d.mu.RLock()
	interceptors := make([]Interceptor, len(d.interceptors))
	copy(interceptors, d.interceptors)
	entries := make([]listenerEntry, len(d.listeners[event.Name()]))
	copy(entries, d.listeners[event.Name()])
	d.mu.RUnlock()

	// Build execution chain: interceptor -> listener
	handler := d.buildHandlerChain(ctx, entries, interceptors)

	err := handler(ctx, event)

	// Clean up one-time listeners
	d.cleanupOnceListeners(event.Name(), entries)

	// ErrStopPropagation is not considered an error
	if errors.Is(err, ErrStopPropagation) {
		return nil
	}

	return err
}

// dispatchAsyncMemory asynchronous memory distribution
func (d *dispatcher) dispatchAsyncMemory(ctx context.Context, event Event) {
	if atomic.LoadInt32(&d.closed) == 1 {
		return
	}

	// Copy key information from the context (to avoid losing the context)
	asyncCtx := context.Background()
	if traceID := ctx.Value("trace_id"); traceID != nil {
		asyncCtx = context.WithValue(asyncCtx, "trace_id", traceID)
	}

	eventName := event.Name()

	err := d.pool.Submit(func() {
		if err := d.dispatchMemory(asyncCtx, event); err != nil {
			d.logger.ErrorCtx(asyncCtx, "异步事件处理失败",
				zap.String("event", eventName),
				zap.Error(err))
		}
	})

	if err != nil {
		d.logger.ErrorCtx(ctx, "提交异步任务失败",
			zap.String("event", eventName),
			zap.Error(err))
	}
}

// dispatchToKafka Dispatch to Kafka
func (d *dispatcher) dispatchToKafka(ctx context.Context, event Event, opts *dispatchOptions) error {
	if d.kafkaPublisher == nil {
		return ErrKafkaNotAvailable
	}

	if opts.topic == "" {
		return ErrKafkaTopicRequired
	}

	// Get traceID
	traceID := ""
	if v := ctx.Value("trace_id"); v != nil {
		if s, ok := v.(string); ok {
			traceID = s
		}
	}

	// serialize event
	payload, err := SerializeEvent(event, traceID)
	if err != nil {
		return err
	}

	// Determine message key
	key := opts.key
	if key == "" {
		key = event.Name()
	}

	// Asynchronous send
	if opts.async {
		go func() {
			if err := d.kafkaPublisher.PublishJSON(ctx, opts.topic, key, payload); err != nil {
				d.logger.ErrorCtx(ctx, "Kafka 异步发送失败",
					zap.String("event", event.Name()),
					zap.String("topic", opts.topic),
					zap.Error(err))
			}
		}()
		return nil
	}

	// Synchronous send
	return d.kafkaPublisher.PublishJSON(ctx, opts.topic, key, payload)
}

// DispatchAsync asynchronously dispatches events (compatible with old API)
// Equivalent to Dispatch(ctx, event, WithDispatchAsync())
func (d *dispatcher) DispatchAsync(ctx context.Context, event Event) {
	_ = d.Dispatch(ctx, event, WithDispatchAsync())
}

// buildHandlerChain Build the execution chain
func (d *dispatcher) buildHandlerChain(ctx context.Context, entries []listenerEntry, interceptors []Interceptor) Next {
	// Innermost layer: execute listener
	handler := func(ctx context.Context, event Event) error {
		return d.executeListeners(ctx, event, entries)
	}

	// Wrap interceptors backward
	for i := len(interceptors) - 1; i >= 0; i-- {
		interceptor := interceptors[i]
		next := handler
		handler = func(ctx context.Context, event Event) error {
			return interceptor(ctx, event, next)
		}
	}

	return handler
}

// execute listeners
func (d *dispatcher) executeListeners(ctx context.Context, event Event, entries []listenerEntry) error {
	for _, entry := range entries {
		if entry.async {
			// Asynchronous listener submitted to coroutine pool
			listener := entry.listener
			eventName := event.Name()
			_ = d.pool.Submit(func() {
				if err := listener.Handle(ctx, event); err != nil && !errors.Is(err, ErrStopPropagation) {
					d.logger.ErrorCtx(ctx, "异步监听器执行失败",
						zap.String("event", eventName),
						zap.Error(err))
				}
			})
			continue
		}

		// Synchronous execution
		if err := entry.listener.Handle(ctx, event); err != nil {
			return err
		}
	}

	return nil
}

// cleanupOneTimeListeners
func (d *dispatcher) cleanupOnceListeners(eventName string, executed []listenerEntry) {
	var onceIDs []uint64
	for _, e := range executed {
		if e.once {
			onceIDs = append(onceIDs, e.id)
		}
	}

	if len(onceIDs) == 0 {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	entries := d.listeners[eventName]
	filtered := entries[:0]
	for _, e := range entries {
		remove := false
		for _, id := range onceIDs {
			if e.id == id {
				remove = true
				break
			}
		}
		if !remove {
			filtered = append(filtered, e)
		}
	}
	d.listeners[eventName] = filtered
}

// Close Disconnector
func (d *dispatcher) Close() {
	atomic.StoreInt32(&d.closed, 1)
	if d.pool != nil {
		d.pool.Release()
	}
}

// Get the number of listeners for a specified event (for testing)
func (d *dispatcher) ListenerCount(eventName string) int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.listeners[eventName])
}
