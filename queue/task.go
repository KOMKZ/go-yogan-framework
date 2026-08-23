package queue

import (
	"context"
	"encoding/json"
	"time"
)

const (
	TraceIDKey    = "trace_id"
	TraceIDHeader = "x-trace-id"
	ModeQueue     = "queue"
	ModeSync      = "sync"
)

type Task struct {
	ID      string
	Type    string
	Payload []byte
	Queue   string
	Headers map[string]string
}

type EnqueueRequest struct {
	ID        string
	Type      string
	Payload   []byte
	Queue     string
	Headers   map[string]string
	Timeout   time.Duration
	Retention time.Duration
	UniqueTTL time.Duration
	ProcessAt time.Time
	ProcessIn time.Duration
}

type TaskInfo struct {
	ID      string
	Type    string
	Queue   string
	State   string
	Retried int
}

type Client interface {
	Enqueue(ctx context.Context, req EnqueueRequest) (*TaskInfo, error)
	Close() error
}

type CapacityChecker interface {
	CheckCapacity(ctx context.Context, queueName string) error
}

type Handler interface {
	HandleTask(ctx context.Context, task Task) error
}

type HandlerFunc func(ctx context.Context, task Task) error

func (f HandlerFunc) HandleTask(ctx context.Context, task Task) error {
	return f(ctx, task)
}

type HandlerDescriptor struct {
	Type        string
	Queue       string
	Description string
	Handler     Handler
}

// TaskDefinition declares the stable metadata for a queue task.
type TaskDefinition[C any] struct {
	Type           string
	Queue          string
	Description    string
	Mode           string
	Timeout        time.Duration
	Retention      time.Duration
	UniqueTTL      time.Duration
	IdempotencyKey func(C) string
	Encode         func(C) ([]byte, error)
	Decode         func([]byte) (C, error)
}

// MarshalCommand encodes a typed command for transport.
func (d TaskDefinition[C]) MarshalCommand(cmd C) ([]byte, error) {
	if d.Encode != nil {
		return d.Encode(cmd)
	}
	return json.Marshal(cmd)
}

// UnmarshalCommand decodes a transport payload into a typed command.
func (d TaskDefinition[C]) UnmarshalCommand(payload []byte) (C, error) {
	if d.Decode != nil {
		return d.Decode(payload)
	}
	var cmd C
	err := json.Unmarshal(payload, &cmd)
	return cmd, err
}

// ApplyTaskConfig merges runtime queue task configuration into the definition.
func (d TaskDefinition[C]) ApplyTaskConfig(cfg TaskConfig) TaskDefinition[C] {
	if cfg.Queue != "" {
		d.Queue = cfg.Queue
	}
	if cfg.Mode != "" {
		d.Mode = cfg.Mode
	}
	if cfg.Timeout != 0 {
		d.Timeout = cfg.Timeout
	}
	if cfg.Retention != 0 {
		d.Retention = cfg.Retention
	}
	if cfg.UniqueTTL != 0 {
		d.UniqueTTL = cfg.UniqueTTL
	}
	if d.Queue == "" {
		d.Queue = DefaultQueue
	}
	if d.Mode == "" {
		d.Mode = ModeQueue
	}
	return d
}

// Descriptor creates a registry descriptor for the task definition.
func (d TaskDefinition[C]) Descriptor(handler Handler) HandlerDescriptor {
	return HandlerDescriptor{
		Type:        d.Type,
		Queue:       d.Queue,
		Description: d.Description,
		Handler:     handler,
	}
}

// EnqueueRequest builds a transport request with task defaults.
func (d TaskDefinition[C]) EnqueueRequest(id string, payload []byte, headers map[string]string) EnqueueRequest {
	return EnqueueRequest{
		ID:        id,
		Type:      d.Type,
		Payload:   payload,
		Queue:     d.Queue,
		Headers:   headers,
		Timeout:   d.Timeout,
		Retention: d.Retention,
		UniqueTTL: d.UniqueTTL,
	}
}

func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if value := ctx.Value(TraceIDKey); value != nil {
		if traceID, ok := value.(string); ok {
			return traceID
		}
	}
	return ""
}

func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if traceID == "" {
		return ctx
	}
	return context.WithValue(ctx, TraceIDKey, traceID)
}
