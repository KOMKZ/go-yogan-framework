package event

import "context"

// Listener interface
type Listener interface {
	// Handle event
	// When returning an error, synchronous dispatch will stop further listener execution
	// Return ErrStopPropagation stops propagation but is not considered an error
	Handle(ctx context.Context, event Event) error
}

// ListenerFunc functional listener adapter
type ListenerFunc func(ctx context.Context, event Event) error

// Handle implements Listener interface
func (f ListenerFunc) Handle(ctx context.Context, event Event) error {
	return f(ctx, event)
}

