package event

// listenerEntry 监听器条目
type listenerEntry struct {
	id       uint64   // 唯一 ID（用于取消订阅）
	listener Listener // 监听器
	priority int      // 优先级（数字越小越先执行）
	async    bool     // 是否异步执行
	once     bool     // 是否只执行一次
}

// SubscribeOption 订阅选项
type SubscribeOption func(*listenerEntry)

// WithPriority 设置优先级
// 数字越小优先级越高，越先执行
// 默认优先级为 0
func WithPriority(priority int) SubscribeOption {
	return func(e *listenerEntry) {
		e.priority = priority
	}
}

// WithAsync 标记为异步监听器
// 即使使用 Dispatch 同步分发，该监听器也会异步执行
// 异步监听器的错误不会影响事件传播
func WithAsync() SubscribeOption {
	return func(e *listenerEntry) {
		e.async = true
	}
}

// WithOnce 只执行一次后自动取消订阅
func WithOnce() SubscribeOption {
	return func(e *listenerEntry) {
		e.once = true
	}
}

// DispatcherOption 分发器配置选项
type DispatcherOption func(*dispatcher)

// WithPoolSize 设置异步协程池大小
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

// WithKafkaPublisher 设置 Kafka 发布者
// 设置后可使用 WithKafka() 选项发送事件到 Kafka
func WithKafkaPublisher(publisher KafkaPublisher) DispatcherOption {
	return func(d *dispatcher) {
		d.kafkaPublisher = publisher
	}
}

// WithRouter 设置事件路由器
// 路由器根据配置决定事件发送到 Kafka 还是内存
// 优先级：代码选项 > 配置路由 > 默认(内存)
func WithRouter(router *Router) DispatcherOption {
	return func(d *dispatcher) {
		d.router = router
	}
}
