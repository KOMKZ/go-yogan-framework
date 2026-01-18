package event

// dispatchOptions 分发选项
type dispatchOptions struct {
	driver string // "memory" | "kafka"
	topic  string // Kafka topic（仅 Kafka 模式）
	key    string // Kafka 消息键
	async  bool   // 是否异步分发
}

// DispatchOption 分发选项函数
type DispatchOption func(*dispatchOptions)

// applyDefaults 应用默认值
func (o *dispatchOptions) applyDefaults() {
	if o.driver == "" {
		o.driver = DriverMemory
	}
}

// 驱动器常量
const (
	DriverMemory = "memory"
	DriverKafka  = "kafka"
)

// WithKafka 使用 Kafka 驱动器发送事件
// topic: Kafka topic 名称
func WithKafka(topic string) DispatchOption {
	return func(o *dispatchOptions) {
		o.driver = DriverKafka
		o.topic = topic
	}
}

// WithKafkaKey 指定 Kafka 消息键
// key: 消息键，用于分区路由
func WithKafkaKey(key string) DispatchOption {
	return func(o *dispatchOptions) {
		o.key = key
	}
}

// WithDispatchAsync 异步分发事件
// 事件将提交到协程池异步处理，立即返回
func WithDispatchAsync() DispatchOption {
	return func(o *dispatchOptions) {
		o.async = true
	}
}
