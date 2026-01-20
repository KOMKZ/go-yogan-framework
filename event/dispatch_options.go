package event

// dispatchOptions dispatch options
type dispatchOptions struct {
	driver         string // "memory" | "kafka"
	driverExplicit bool   // Is the drive explicitly specified by the code (highest priority)
	topic          string // Kafka topic (only for Kafka mode)
	key            string // Kafka message key
	async          bool   // Is asynchronous distribution
}

// DispatchOption function for distribution options
type DispatchOption func(*dispatchOptions)

// apply defaults
func (o *dispatchOptions) applyDefaults() {
	if o.driver == "" {
		o.driver = DriverMemory
	}
}

// Driver constants
const (
	DriverMemory = "memory"
	DriverKafka  = "kafka"
)

// Using Kafka driver to send events
// topic: Kafka topic name
// Note: The options specified in the code have the highest priority and will override route configuration
func WithKafka(topic string) DispatchOption {
	return func(o *dispatchOptions) {
		o.driver = DriverKafka
		o.driverExplicit = true
		o.topic = topic
	}
}

// Force the use of memory driver
// Note: The options specified in the code have the highest priority and will override route configuration
func WithMemory() DispatchOption {
	return func(o *dispatchOptions) {
		o.driver = DriverMemory
		o.driverExplicit = true
	}
}

// WithKafkaKey specifies the Kafka message key
// key: message key for partition routing
func WithKafkaKey(key string) DispatchOption {
	return func(o *dispatchOptions) {
		o.key = key
	}
}

// WithDispatchAsync Asynchronously dispatch event
// The event will be submitted to the coroutine pool for asynchronous processing and immediately returned
func WithDispatchAsync() DispatchOption {
	return func(o *dispatchOptions) {
		o.async = true
	}
}
