package event

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: testEvent and mockKafkaPublisher are defined in dispatch_options_test.go

func TestDispatcher_RouteToKafka(t *testing.T) {
	// Create router
	router := NewRouter()
	router.LoadRoutes(map[string]RouteConfig{
		"order.*": {Driver: DriverKafka, Topic: "events.order"},
	})

	// Create simulated publisher
	publisher := &mockKafkaPublisher{}

	// Create dispatcher
	d := NewDispatcher(
		WithRouter(router),
		WithKafkaPublisher(publisher),
	)
	defer d.Close()

	// Send event (without specifying options, let routing decide)
	err := d.Dispatch(context.Background(), &testEvent{name: "order.created"})
	require.NoError(t, err)

	// Verify send to Kafka
	messages := publisher.getMessages()
	assert.Len(t, messages, 1)
	assert.Equal(t, "events.order", messages[0].Topic)
}

func TestDispatcher_RouteToMemory(t *testing.T) {
	// Create router (no matching rules)
	router := NewRouter()
	router.LoadRoutes(map[string]RouteConfig{
		"order.*": {Driver: DriverKafka, Topic: "events.order"},
	})

	// Create dispatcher
	d := NewDispatcher(WithRouter(router))
	defer d.Close()

	// Subscribe to memory events
	var received int32
	d.Subscribe("user.login", ListenerFunc(func(ctx context.Context, e Event) error {
		atomic.AddInt32(&received, 1)
		return nil
	}))

	// Send event (no route match, go to memory)
	err := d.Dispatch(context.Background(), &testEvent{name: "user.login"})
	require.NoError(t, err)

	// Verify that it used memory
	assert.Equal(t, int32(1), atomic.LoadInt32(&received))
}

func TestDispatcher_CodeOptionOverridesRoute(t *testing.T) {
	// Create router (configuration to use Kafka)
	router := NewRouter()
	router.LoadRoutes(map[string]RouteConfig{
		"order.*": {Driver: DriverKafka, Topic: "events.order"},
	})

	// Create simulation publisher
	publisher := &mockKafkaPublisher{}

	// Create dispatcher
	d := NewDispatcher(
		WithRouter(router),
		WithKafkaPublisher(publisher),
	)
	defer d.Close()

	// Subscribe to memory events
	var received int32
	d.Subscribe("order.created", ListenerFunc(func(ctx context.Context, e Event) error {
		atomic.AddInt32(&received, 1)
		return nil
	}))

	// Use WithMemory() to force in-memory processing, overriding routing configuration
	err := d.Dispatch(context.Background(), &testEvent{name: "order.created"}, WithMemory())
	require.NoError(t, err)

	// Verify that it uses memory (not Kafka)
	assert.Equal(t, int32(1), atomic.LoadInt32(&received))
	assert.Len(t, publisher.getMessages(), 0) // Kafka not received
}

func TestDispatcher_CodeKafkaOverridesRoute(t *testing.T) {
	// Create router (configuration in memory)
	router := NewRouter()
	router.LoadRoutes(map[string]RouteConfig{
		"order.*": {Driver: DriverMemory},
	})

	// Create simulation publisher
	publisher := &mockKafkaPublisher{}

	// Create dispatcher
	d := NewDispatcher(
		WithRouter(router),
		WithKafkaPublisher(publisher),
	)
	defer d.Close()

	// Subscribe to memory events
	var received int32
	d.Subscribe("order.created", ListenerFunc(func(ctx context.Context, e Event) error {
		atomic.AddInt32(&received, 1)
		return nil
	}))

	// Use WithKafka() to force the use of Kafka, overriding routing configuration
	err := d.Dispatch(context.Background(), &testEvent{name: "order.created"}, WithKafka("custom.topic"))
	require.NoError(t, err)

	// Verify that Kafka is used (not memory)
	assert.Equal(t, int32(0), atomic.LoadInt32(&received)) // memory not received
	messages := publisher.getMessages()
	assert.Len(t, messages, 1)
	assert.Equal(t, "custom.topic", messages[0].Topic)
}

func TestDispatcher_NoRouterDefaultsToMemory(t *testing.T) {
	// Create dispatcher (routerless)
	d := NewDispatcher()
	defer d.Close()

	// Subscribe to memory events
	var received int32
	d.Subscribe("order.created", ListenerFunc(func(ctx context.Context, e Event) error {
		atomic.AddInt32(&received, 1)
		return nil
	}))

	// Send event (no routing, use default memory)
	err := d.Dispatch(context.Background(), &testEvent{name: "order.created"})
	require.NoError(t, err)

	// verify that it uses memory
	assert.Equal(t, int32(1), atomic.LoadInt32(&received))
}

func TestDispatcher_RouteWithUniversalWildcard(t *testing.T) {
	// Create router (generic wildcard)
	router := NewRouter()
	router.LoadRoutes(map[string]RouteConfig{
		"*": {Driver: DriverKafka, Topic: "events.all"},
	})

	// Create simulation publisher
	publisher := &mockKafkaPublisher{}

	// Create distributor
	d := NewDispatcher(
		WithRouter(router),
		WithKafkaPublisher(publisher),
	)
	defer d.Close()

	// Send multiple events
	_ = d.Dispatch(context.Background(), &testEvent{name: "order.created"})
	_ = d.Dispatch(context.Background(), &testEvent{name: "user.login"})
	_ = d.Dispatch(context.Background(), &testEvent{name: "anything.else"})

	// All events should be sent to Kafka
	messages := publisher.getMessages()
	assert.Len(t, messages, 3)
	for _, msg := range messages {
		assert.Equal(t, "events.all", msg.Topic)
	}
}

func TestDispatcher_RoutePriority(t *testing.T) {
	// Create router (multiple rules)
	router := NewRouter()
	router.LoadRoutes(map[string]RouteConfig{
		"*":             {Driver: DriverKafka, Topic: "events.all"},
		"order.*":       {Driver: DriverKafka, Topic: "events.order"},
		"order.created": {Driver: DriverKafka, Topic: "events.order.created"},
	})

	// Create simulated publisher
	publisher := &mockKafkaPublisher{}

	// Create dispatcher
	d := NewDispatcher(
		WithRouter(router),
		WithKafkaPublisher(publisher),
	)
	defer d.Close()

	// Exact match
	_ = d.Dispatch(context.Background(), &testEvent{name: "order.created"})
	messages := publisher.getMessages()
	assert.Equal(t, "events.order.created", messages[0].Topic)

	// wildcard matching
	_ = d.Dispatch(context.Background(), &testEvent{name: "order.updated"})
	messages = publisher.getMessages()
	assert.Equal(t, "events.order", messages[1].Topic)

	// General wildcard
	_ = d.Dispatch(context.Background(), &testEvent{name: "user.login"})
	messages = publisher.getMessages()
	assert.Equal(t, "events.all", messages[2].Topic)
}
