package breaker

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// eventBus 事件总线实现
type eventBus struct {
	listeners map[SubscriptionID]*subscription
	buffer    chan Event
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	closed    int32 // 使用atomic标记是否已关闭
}

// subscription 订阅信息
type subscription struct {
	id       SubscriptionID
	listener EventListener
	filters  map[EventType]bool
}

// NewEventBus 创建事件总线
func NewEventBus(bufferSize int) EventBus {
	ctx, cancel := context.WithCancel(context.Background())
	
	bus := &eventBus{
		listeners: make(map[SubscriptionID]*subscription),
		buffer:    make(chan Event, bufferSize),
		ctx:       ctx,
		cancel:    cancel,
	}
	
	// 启动事件分发协程
	bus.wg.Add(1)
	go bus.dispatch()
	
	return bus
}

// Subscribe 订阅事件
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

// Unsubscribe 取消订阅
func (eb *eventBus) Unsubscribe(id SubscriptionID) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	
	delete(eb.listeners, id)
}

// Publish 发布事件
func (eb *eventBus) Publish(event Event) {
	// 检查是否已关闭
	if atomic.LoadInt32(&eb.closed) == 1 {
		return
	}
	
	select {
	case eb.buffer <- event:
		// 成功发布
	case <-eb.ctx.Done():
		// 总线已关闭，静默忽略
		return
	default:
		// 缓冲区满，丢弃事件（或者可以选择阻塞）
	}
}

// Close 关闭事件总线
func (eb *eventBus) Close() {
	// 标记为已关闭
	atomic.StoreInt32(&eb.closed, 1)
	// 先取消context，触发dispatch退出
	eb.cancel()
	// 等待dispatch处理完剩余事件
	eb.wg.Wait()
	// 最后关闭channel（此时dispatch已经退出，不会再读取）
	close(eb.buffer)
}

// dispatch 分发事件给订阅者
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
			// 处理剩余事件
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

// notifyListeners 通知所有匹配的监听者
func (eb *eventBus) notifyListeners(event Event) {
	eb.mu.RLock()
	// 复制监听者列表，避免持有锁太久
	listeners := make([]*subscription, 0, len(eb.listeners))
	for _, sub := range eb.listeners {
		listeners = append(listeners, sub)
	}
	eb.mu.RUnlock()
	
	eventType := event.Type()
	
	for _, sub := range listeners {
		// 检查过滤器
		if len(sub.filters) > 0 && !sub.filters[eventType] {
			continue
		}
		
		// 异步通知（避免阻塞）
		go func(l EventListener, e Event) {
			defer func() {
				// 捕获监听者panic
				if r := recover(); r != nil {
					// 可以记录日志
				}
			}()
			l.OnEvent(e)
		}(sub.listener, event)
	}
}

