package event

import "errors"

// ErrStopPropagation stops event propagation (not considered an error)
// When the listener returns this error, subsequent listeners do not execute, but Dispatch does not return an error
var ErrStopPropagation = errors.New("stop propagation")

// ErrKafkaNotAvailable Kafka publisher not configured
var ErrKafkaNotAvailable = errors.New("kafka publisher not available")

// ErrKafkaTopicRequired Kafka topic not specified
var ErrKafkaTopicRequired = errors.New("kafka topic is required")
