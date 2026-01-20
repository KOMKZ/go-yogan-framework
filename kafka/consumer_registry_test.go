package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockHandler test handler
type mockHandler struct {
	name   string
	topics []string
}

func (h *mockHandler) Name() string                                           { return h.name }
func (h *mockHandler) Topics() []string                                        { return h.topics }
func (h *mockHandler) Handle(ctx context.Context, msg *ConsumedMessage) error { return nil }

func TestConsumerRegistry_Register(t *testing.T) {
	registry := NewConsumerRegistry()

	handler := &mockHandler{name: "test-handler", topics: []string{"topic1"}}
	err := registry.Register(handler)
	require.NoError(t, err)

	// Verify registration success
	h, ok := registry.Get("test-handler")
	assert.True(t, ok)
	assert.Equal(t, handler, h)
}

func TestConsumerRegistry_Register_Nil(t *testing.T) {
	registry := NewConsumerRegistry()
	err := registry.Register(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")
}

func TestConsumerRegistry_Register_EmptyName(t *testing.T) {
	registry := NewConsumerRegistry()
	handler := &mockHandler{name: "", topics: []string{"topic1"}}
	err := registry.Register(handler)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name cannot be empty")
}

func TestConsumerRegistry_Register_Duplicate(t *testing.T) {
	registry := NewConsumerRegistry()

	handler1 := &mockHandler{name: "test", topics: []string{"topic1"}}
	handler2 := &mockHandler{name: "test", topics: []string{"topic2"}}

	err := registry.Register(handler1)
	require.NoError(t, err)

	err = registry.Register(handler2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestConsumerRegistry_MustRegister_Panic(t *testing.T) {
	registry := NewConsumerRegistry()

	assert.Panics(t, func() {
		registry.MustRegister(nil)
	})
}

func TestConsumerRegistry_Get_NotFound(t *testing.T) {
	registry := NewConsumerRegistry()

	h, ok := registry.Get("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, h)
}

func TestConsumerRegistry_All(t *testing.T) {
	registry := NewConsumerRegistry()

	handler1 := &mockHandler{name: "handler1", topics: []string{"topic1"}}
	handler2 := &mockHandler{name: "handler2", topics: []string{"topic2"}}

	registry.MustRegister(handler1)
	registry.MustRegister(handler2)

	all := registry.All()
	assert.Len(t, all, 2)
}

func TestConsumerRegistry_Names(t *testing.T) {
	registry := NewConsumerRegistry()

	registry.MustRegister(&mockHandler{name: "alpha", topics: []string{"t1"}})
	registry.MustRegister(&mockHandler{name: "beta", topics: []string{"t2"}})

	names := registry.Names()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "alpha")
	assert.Contains(t, names, "beta")
}

func TestConsumerRegistry_Count(t *testing.T) {
	registry := NewConsumerRegistry()
	assert.Equal(t, 0, registry.Count())

	registry.MustRegister(&mockHandler{name: "h1", topics: []string{"t1"}})
	assert.Equal(t, 1, registry.Count())

	registry.MustRegister(&mockHandler{name: "h2", topics: []string{"t2"}})
	assert.Equal(t, 2, registry.Count())
}

func TestConsumerRegistry_Unregister(t *testing.T) {
	registry := NewConsumerRegistry()

	registry.MustRegister(&mockHandler{name: "test", topics: []string{"t1"}})
	assert.Equal(t, 1, registry.Count())

	removed := registry.Unregister("test")
	assert.True(t, removed)
	assert.Equal(t, 0, registry.Count())

	// Remove non-existent again
	removed = registry.Unregister("test")
	assert.False(t, removed)
}
