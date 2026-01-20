package kafka

import "context"

// ConsumerHandler consumer handler interface
// The application layer only needs to implement this interface; the kernel is responsible for runtime management
type ConsumerHandler interface {
	// Name Consumer name (for logs, metrics, configuration indexing)
	Name() string

	// List of Topics subscribed to for Topics
	Topics() []string

	// Handle message
	Handle(ctx context.Context, msg *ConsumedMessage) error
}

// ConsumerHandlerFunc functional Handler (simple scenario)
// For simple consumers that do not require dependency injection
type ConsumerHandlerFunc struct {
	name    string
	topics  []string
	handler MessageHandler
}

// NewConsumerHandlerFunc creates a functional Handler
func NewConsumerHandlerFunc(name string, topics []string, handler MessageHandler) *ConsumerHandlerFunc {
	return &ConsumerHandlerFunc{
		name:    name,
		topics:  topics,
		handler: handler,
	}
}

// Returns the consumer name
func (h *ConsumerHandlerFunc) Name() string {
	return h.name
}

// Topics return the list of subscribed Topics
func (h *ConsumerHandlerFunc) Topics() []string {
	return h.topics
}

// Handle message
func (h *ConsumerHandlerFunc) Handle(ctx context.Context, msg *ConsumedMessage) error {
	if h.handler == nil {
		return nil
	}
	return h.handler(ctx, msg)
}

// Ensure ConsumerHandlerFunc implements ConsumerHandler
var _ ConsumerHandler = (*ConsumerHandlerFunc)(nil)
