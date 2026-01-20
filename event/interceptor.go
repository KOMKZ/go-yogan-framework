package event

import "context"

// Proceed to the next interceptor/listener
type Next func(ctx context.Context, event Event) error

// Interceptor event interceptor
// can be used for logging, error handling, event filtering etc.
type Interceptor func(ctx context.Context, event Event, next Next) error

