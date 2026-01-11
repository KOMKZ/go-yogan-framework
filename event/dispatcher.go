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

// UnsubscribeFunc 取消订阅函数
type UnsubscribeFunc func()

// Dispatcher 事件分发器接口
type Dispatcher interface {
	// Subscribe 订阅事件，返回取消订阅函数
	Subscribe(eventName string, listener Listener, opts ...SubscribeOption) UnsubscribeFunc

	// Dispatch 同步分发事件
	// - 按优先级顺序执行监听器
	// - 任一监听器返回错误则停止后续执行并返回该错误
	// - 监听器返回 ErrStopPropagation 则停止后续执行但不返回错误
	Dispatch(ctx context.Context, event Event) error

	// DispatchAsync 异步分发事件
	// - 立即返回，不阻塞调用方
	// - 监听器错误只记录日志
	DispatchAsync(ctx context.Context, event Event)

	// Use 注册全局拦截器
	Use(interceptor Interceptor)
}

// dispatcher 事件分发器实现
type dispatcher struct {
	mu           sync.RWMutex
	listeners    map[string][]listenerEntry
	interceptors []Interceptor
	nextID       uint64
	pool         *ants.Pool
	poolSize     int
	logger       *logger.CtxZapLogger
	closed       int32
}

// NewDispatcher 创建事件分发器
func NewDispatcher(opts ...DispatcherOption) *dispatcher {
	d := &dispatcher{
		listeners: make(map[string][]listenerEntry),
		poolSize:  100,
		logger:    logger.GetLogger("event"),
	}

	for _, opt := range opts {
		opt(d)
	}

	// 创建协程池
	var err error
	d.pool, err = ants.NewPool(d.poolSize)
	if err != nil {
		d.logger.Error("创建协程池失败，使用默认配置", zap.Error(err))
		d.pool, _ = ants.NewPool(100)
	}

	return d
}

// Subscribe 订阅事件
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
	}

	d.mu.Lock()
	d.listeners[eventName] = append(d.listeners[eventName], entry)
	// 按优先级排序
	sort.SliceStable(d.listeners[eventName], func(i, j int) bool {
		return d.listeners[eventName][i].priority < d.listeners[eventName][j].priority
	})
	d.mu.Unlock()

	// 返回取消订阅函数
	return func() {
		d.unsubscribe(eventName, entry.id)
	}
}

// unsubscribe 取消订阅
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

// Use 注册全局拦截器
func (d *dispatcher) Use(interceptor Interceptor) {
	d.mu.Lock()
	d.interceptors = append(d.interceptors, interceptor)
	d.mu.Unlock()
}

// Dispatch 同步分发事件
func (d *dispatcher) Dispatch(ctx context.Context, event Event) error {
	if event == nil {
		return nil
	}

	// 获取拦截器和监听器的副本
	d.mu.RLock()
	interceptors := make([]Interceptor, len(d.interceptors))
	copy(interceptors, d.interceptors)
	entries := make([]listenerEntry, len(d.listeners[event.Name()]))
	copy(entries, d.listeners[event.Name()])
	d.mu.RUnlock()

	// 构建执行链：拦截器 -> 监听器
	handler := d.buildHandlerChain(ctx, entries, interceptors)

	err := handler(ctx, event)

	// 清理一次性监听器
	d.cleanupOnceListeners(event.Name(), entries)

	// ErrStopPropagation 不视为错误
	if errors.Is(err, ErrStopPropagation) {
		return nil
	}

	return err
}

// DispatchAsync 异步分发事件
func (d *dispatcher) DispatchAsync(ctx context.Context, event Event) {
	if event == nil || atomic.LoadInt32(&d.closed) == 1 {
		return
	}

	// 复制 context 中的关键信息（避免 context 被取消）
	asyncCtx := context.Background()
	if traceID := ctx.Value("trace_id"); traceID != nil {
		asyncCtx = context.WithValue(asyncCtx, "trace_id", traceID)
	}

	eventName := event.Name()

	err := d.pool.Submit(func() {
		if err := d.Dispatch(asyncCtx, event); err != nil {
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

// buildHandlerChain 构建执行链
func (d *dispatcher) buildHandlerChain(ctx context.Context, entries []listenerEntry, interceptors []Interceptor) Next {
	// 最内层：执行监听器
	handler := func(ctx context.Context, event Event) error {
		return d.executeListeners(ctx, event, entries)
	}

	// 从后向前包装拦截器
	for i := len(interceptors) - 1; i >= 0; i-- {
		interceptor := interceptors[i]
		next := handler
		handler = func(ctx context.Context, event Event) error {
			return interceptor(ctx, event, next)
		}
	}

	return handler
}

// executeListeners 执行监听器
func (d *dispatcher) executeListeners(ctx context.Context, event Event, entries []listenerEntry) error {
	for _, entry := range entries {
		if entry.async {
			// 异步监听器提交到协程池
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

		// 同步执行
		if err := entry.listener.Handle(ctx, event); err != nil {
			return err
		}
	}

	return nil
}

// cleanupOnceListeners 清理一次性监听器
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

// Close 关闭分发器
func (d *dispatcher) Close() {
	atomic.StoreInt32(&d.closed, 1)
	if d.pool != nil {
		d.pool.Release()
	}
}

// ListenerCount 获取指定事件的监听器数量（用于测试）
func (d *dispatcher) ListenerCount(eventName string) int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.listeners[eventName])
}

