package event

import "context"

// Kafka Publisher interface
// For decoupling direct dependencies between the event package and the Kafka package
type KafkaPublisher interface {
	// PublishJSON publishes JSON messages to the specified topic
	PublishJSON(ctx context.Context, topic string, key string, payload any) error
}
