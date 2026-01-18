package kafka

import "context"

// ConsumerHandler 消费者处理器接口
// 应用层只需实现此接口，内核负责运行时管理
type ConsumerHandler interface {
	// Name 消费者名称（用于日志、指标、配置索引）
	Name() string

	// Topics 订阅的 Topic 列表
	Topics() []string

	// Handle 处理消息
	Handle(ctx context.Context, msg *ConsumedMessage) error
}

// ConsumerHandlerFunc 函数式 Handler（简单场景）
// 适用于不需要依赖注入的简单消费者
type ConsumerHandlerFunc struct {
	name    string
	topics  []string
	handler MessageHandler
}

// NewConsumerHandlerFunc 创建函数式 Handler
func NewConsumerHandlerFunc(name string, topics []string, handler MessageHandler) *ConsumerHandlerFunc {
	return &ConsumerHandlerFunc{
		name:    name,
		topics:  topics,
		handler: handler,
	}
}

// Name 返回消费者名称
func (h *ConsumerHandlerFunc) Name() string {
	return h.name
}

// Topics 返回订阅的 Topic 列表
func (h *ConsumerHandlerFunc) Topics() []string {
	return h.topics
}

// Handle 处理消息
func (h *ConsumerHandlerFunc) Handle(ctx context.Context, msg *ConsumedMessage) error {
	if h.handler == nil {
		return nil
	}
	return h.handler(ctx, msg)
}

// Ensure ConsumerHandlerFunc implements ConsumerHandler
var _ ConsumerHandler = (*ConsumerHandlerFunc)(nil)
