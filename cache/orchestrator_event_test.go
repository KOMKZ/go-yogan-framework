package cache

import (
	"context"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/event"
)

func TestOrchestrator_WithEventDispatcher(t *testing.T) {
	dispatcher := event.NewDispatcher()
	defer dispatcher.Close()

	cfg := &Config{
		Enabled:      true,
		DefaultStore: "memory",
		Cacheables: []CacheableConfig{
			{Name: "user:getById", KeyPattern: "user:{0}", Store: "memory", Enabled: true},
		},
		InvalidationRules: []InvalidationRule{
			{
				Event:      "user.updated",
				Invalidate: []string{"user:getById"},
				Pattern:    "user:",
			},
		},
	}

	o := NewOrchestrator(cfg, dispatcher, nil)
	o.RegisterStore("memory", NewMemoryStore("memory", 100))

	var loadCount int
	o.RegisterLoader("user:getById", func(ctx context.Context, args ...any) (any, error) {
		loadCount++
		return map[string]any{"id": args[0]}, nil
	})

	ctx := context.Background()

	// First call - should load
	o.Call(ctx, "user:getById", 1)
	if loadCount != 1 {
		t.Errorf("loadCount = %d, want 1", loadCount)
	}

	// Second call - should hit cache
	o.Call(ctx, "user:getById", 1)
	if loadCount != 1 {
		t.Errorf("loadCount = %d, want 1 (cached)", loadCount)
	}

	// Dispatch event to invalidate
	dispatcher.Dispatch(ctx, &testEvent{name: "user.updated"})

	// Wait for async processing
	time.Sleep(50 * time.Millisecond)

	// Third call - should reload (cache invalidated)
	o.Call(ctx, "user:getById", 1)
	if loadCount != 2 {
		t.Errorf("loadCount = %d, want 2 (after invalidation)", loadCount)
	}
}

// testEvent test event
type testEvent struct {
	name string
}

func (e *testEvent) Name() string {
	return e.name
}

func (e *testEvent) Payload() any {
	return nil
}

// testEventWithCacheArgs implements the CacheInvalidator interface for testing
type testEventWithCacheArgs struct {
	name      string
	articleID uint
}

func (e *testEventWithCacheArgs) Name() string {
	return e.name
}

func (e *testEventWithCacheArgs) CacheArgs() []any {
	return []any{e.articleID}
}

func TestOrchestrator_CacheInvalidatorInterface(t *testing.T) {
	dispatcher := event.NewDispatcher()
	defer dispatcher.Close()

	cfg := &Config{
		Enabled:      true,
		DefaultStore: "memory",
		Cacheables: []CacheableConfig{
			{Name: "article:content", KeyPattern: "article:{0}", Store: "memory", Enabled: true},
		},
		InvalidationRules: []InvalidationRule{
			{Event: "article.content.updated", Invalidate: []string{"article:content"}},
		},
	}

	o := NewOrchestrator(cfg, dispatcher, nil)
	o.RegisterStore("memory", NewMemoryStore("memory", 100))

	var loadCount int
	o.RegisterLoader("article:content", func(ctx context.Context, args ...any) (any, error) {
		loadCount++
		return map[string]any{"content": "test", "id": args[0]}, nil
	})

	ctx := context.Background()

	// First call - should load
	o.Call(ctx, "article:content", uint(123))
	if loadCount != 1 {
		t.Errorf("loadCount = %d, want 1", loadCount)
	}

	// Second call - should hit cache
	o.Call(ctx, "article:content", uint(123))
	if loadCount != 1 {
		t.Errorf("loadCount = %d, want 1 (cached)", loadCount)
	}

	// Dispatch event with CacheInvalidator interface (precise invalidation)
	dispatcher.Dispatch(ctx, &testEventWithCacheArgs{
		name:      "article.content.updated",
		articleID: 123,
	})

	// Wait for async processing
	time.Sleep(50 * time.Millisecond)

	// Third call - should reload (cache invalidated by CacheInvalidator)
	o.Call(ctx, "article:content", uint(123))
	if loadCount != 2 {
		t.Errorf("loadCount = %d, want 2 (after CacheInvalidator invalidation)", loadCount)
	}

	// Different article should still be independent
	o.Call(ctx, "article:content", uint(456))
	if loadCount != 3 {
		t.Errorf("loadCount = %d, want 3 (different article)", loadCount)
	}
}

func TestOrchestrator_SubscribeInvalidationEvents(t *testing.T) {
	dispatcher := event.NewDispatcher()
	defer dispatcher.Close()

	cfg := &Config{
		Enabled: true,
		InvalidationRules: []InvalidationRule{
			{Event: "test.event", Invalidate: []string{"test"}},
		},
	}

	// Should not panic
	o := NewOrchestrator(cfg, dispatcher, nil)
	if o == nil {
		t.Error("NewOrchestrator returned nil")
	}
}

func TestOrchestrator_CreateInvalidationHandler(t *testing.T) {
	dispatcher := event.NewDispatcher()
	defer dispatcher.Close()

	cfg := &Config{
		Enabled:      true,
		DefaultStore: "memory",
		Cacheables: []CacheableConfig{
			{Name: "test", KeyPattern: "test:{0}", Store: "memory", Enabled: true},
		},
		InvalidationRules: []InvalidationRule{
			{Event: "test.event", Invalidate: []string{"test"}, Pattern: "test:"},
			{Event: "test.event2", Invalidate: []string{"test"}}, // Use the CacheInvalidator interface
		},
	}

	o := NewOrchestrator(cfg, dispatcher, nil)
	o.RegisterStore("memory", NewMemoryStore("memory", 100))
	o.RegisterLoader("test", func(ctx context.Context, args ...any) (any, error) {
		return "result", nil
	})

	ctx := context.Background()

	// Cache something
	o.Call(ctx, "test", 1)

	// Dispatch event with pattern
	dispatcher.Dispatch(ctx, &testEvent{name: "test.event"})
	time.Sleep(50 * time.Millisecond)

	// Dispatch event with key extract
	dispatcher.Dispatch(ctx, &testEvent{name: "test.event2"})
	time.Sleep(50 * time.Millisecond)
}
