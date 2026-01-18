package event

import "context"

// KafkaPublisher Kafka 发布者接口
// 用于解耦 event 包与 kafka 包的直接依赖
type KafkaPublisher interface {
	// PublishJSON 发布 JSON 消息到指定 Topic
	PublishJSON(ctx context.Context, topic string, key string, payload any) error
}
