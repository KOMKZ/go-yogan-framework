package cache

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStore_Basic(t *testing.T) {
	store := NewMemoryStore("test", 100)
	ctx := context.Background()

	t.Run("Name", func(t *testing.T) {
		if store.Name() != "test" {
			t.Errorf("Name() = %v, want test", store.Name())
		}
	})

	t.Run("Set and Get", func(t *testing.T) {
		err := store.Set(ctx, "key1", []byte("value1"), time.Minute)
		if err != nil {
			t.Errorf("Set() error = %v", err)
		}

		data, err := store.Get(ctx, "key1")
		if err != nil {
			t.Errorf("Get() error = %v", err)
		}
		if string(data) != "value1" {
			t.Errorf("Get() = %v, want value1", string(data))
		}
	})

	t.Run("Get non-existent key", func(t *testing.T) {
		_, err := store.Get(ctx, "non-existent")
		if err == nil {
			t.Error("Get() expected error for non-existent key")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		store.Set(ctx, "key2", []byte("value2"), time.Minute)
		err := store.Delete(ctx, "key2")
		if err != nil {
			t.Errorf("Delete() error = %v", err)
		}

		_, err = store.Get(ctx, "key2")
		if err == nil {
			t.Error("Get() expected error after delete")
		}
	})

	t.Run("Exists", func(t *testing.T) {
		store.Set(ctx, "key3", []byte("value3"), time.Minute)
		if !store.Exists(ctx, "key3") {
			t.Error("Exists() = false, want true")
		}
		if store.Exists(ctx, "non-existent") {
			t.Error("Exists() = true, want false")
		}
	})
}

func TestMemoryStore_TTL(t *testing.T) {
	store := NewMemoryStore("test", 100)
	ctx := context.Background()

	t.Run("Expired key", func(t *testing.T) {
		store.Set(ctx, "expiring", []byte("value"), 50*time.Millisecond)

		// Should exist initially
		if !store.Exists(ctx, "expiring") {
			t.Error("Key should exist before expiration")
		}

		// Wait for expiration
		time.Sleep(100 * time.Millisecond)

		// Should be expired
		_, err := store.Get(ctx, "expiring")
		if err == nil {
			t.Error("Get() expected error for expired key")
		}
	})

	t.Run("No TTL", func(t *testing.T) {
		store.Set(ctx, "no-ttl", []byte("value"), 0)
		if !store.Exists(ctx, "no-ttl") {
			t.Error("Key with no TTL should exist")
		}
	})
}

func TestMemoryStore_DeleteByPrefix(t *testing.T) {
	store := NewMemoryStore("test", 100)
	ctx := context.Background()

	// Set keys with prefix
	store.Set(ctx, "user:1", []byte("a"), time.Minute)
	store.Set(ctx, "user:2", []byte("b"), time.Minute)
	store.Set(ctx, "order:1", []byte("c"), time.Minute)

	// Delete by prefix
	err := store.DeleteByPrefix(ctx, "user:")
	if err != nil {
		t.Errorf("DeleteByPrefix() error = %v", err)
	}

	// user keys should be gone
	if store.Exists(ctx, "user:1") || store.Exists(ctx, "user:2") {
		t.Error("User keys should be deleted")
	}

	// order key should still exist
	if !store.Exists(ctx, "order:1") {
		t.Error("Order key should still exist")
	}
}

func TestMemoryStore_MaxSize(t *testing.T) {
	store := NewMemoryStore("test", 3)
	ctx := context.Background()

	// Fill the store
	store.Set(ctx, "key1", []byte("v1"), time.Minute)
	store.Set(ctx, "key2", []byte("v2"), time.Minute)
	store.Set(ctx, "key3", []byte("v3"), time.Minute)

	// Add one more, should evict one
	store.Set(ctx, "key4", []byte("v4"), time.Minute)

	// Should have max 3 keys
	if store.Size() > 3 {
		t.Errorf("Size() = %d, want <= 3", store.Size())
	}
}

func TestMemoryStore_Close(t *testing.T) {
	store := NewMemoryStore("test", 100)
	ctx := context.Background()

	store.Set(ctx, "key1", []byte("value"), time.Minute)
	store.Close()

	// After close, store should be empty
	if store.Size() != 0 {
		t.Errorf("Size() = %d after Close(), want 0", store.Size())
	}
}

func TestMemoryStore_DefaultMaxSize(t *testing.T) {
	// Test with 0 maxSize, should use default
	store := NewMemoryStore("test", 0)
	if store == nil {
		t.Fatal("NewMemoryStore returned nil")
	}
}

func TestMemoryStore_ExistsExpired(t *testing.T) {
	store := NewMemoryStore("test", 100)
	ctx := context.Background()

	store.Set(ctx, "expiring", []byte("value"), 50*time.Millisecond)
	time.Sleep(100 * time.Millisecond)

	// Should return false for expired key
	if store.Exists(ctx, "expiring") {
		t.Error("Exists() should return false for expired key")
	}
}
