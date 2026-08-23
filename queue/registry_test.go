package queue

import (
	"context"
	"testing"
)

func TestRegistryRegisterAndGet(t *testing.T) {
	registry := NewRegistry()
	handler := HandlerFunc(func(ctx context.Context, task Task) error {
		return nil
	})

	if err := registry.Register(HandlerDescriptor{
		Type:    "export.demo",
		Handler: handler,
	}); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	desc, ok := registry.Get("export.demo")
	if !ok {
		t.Fatal("handler not found")
	}
	if desc.Queue != DefaultQueue {
		t.Fatalf("queue = %q, want %q", desc.Queue, DefaultQueue)
	}
}

func TestRegistryRejectsDuplicateTaskType(t *testing.T) {
	registry := NewRegistry()
	desc := HandlerDescriptor{
		Type: "export.demo",
		Handler: HandlerFunc(func(ctx context.Context, task Task) error {
			return nil
		}),
	}

	if err := registry.Register(desc); err != nil {
		t.Fatalf("first register failed: %v", err)
	}
	if err := registry.Register(desc); err == nil {
		t.Fatal("expected duplicate register error")
	}
}

func TestRegistryValidateConfigRejectsUncoveredQueue(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(HandlerDescriptor{
		Type:  "export.demo",
		Queue: "export",
		Handler: HandlerFunc(func(ctx context.Context, task Task) error {
			return nil
		}),
	}); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	cfg := DefaultConfig()
	if err := registry.ValidateConfig(cfg); err == nil {
		t.Fatal("expected uncovered queue error")
	}
}

func TestRegistryDispatchUsesRegisteredHandler(t *testing.T) {
	registry := NewRegistry()
	called := false
	if err := registry.Register(HandlerDescriptor{
		Type: "export.demo",
		Handler: HandlerFunc(func(ctx context.Context, task Task) error {
			called = true
			if task.Queue != DefaultQueue {
				t.Fatalf("queue = %q, want %q", task.Queue, DefaultQueue)
			}
			return nil
		}),
	}); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	if err := registry.Dispatch(context.Background(), Task{Type: "export.demo"}); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}
	if !called {
		t.Fatal("handler was not called")
	}
}
