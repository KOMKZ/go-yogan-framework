package cache

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// Get test Redis client
func getTestRedisClient(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	// Check connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	return client
}

func TestRedisStore_Basic(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	store := NewRedisStore("test-redis", client, "cache:test:")
	ctx := context.Background()

	// Clean up test data
	defer store.DeleteByPrefix(ctx, "")

	t.Run("Name", func(t *testing.T) {
		if store.Name() != "test-redis" {
			t.Errorf("Name() = %v, want test-redis", store.Name())
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
		_, err := store.Get(ctx, "non-existent-key-xyz")
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
		if store.Exists(ctx, "non-existent-key-abc") {
			t.Error("Exists() = true, want false")
		}
	})
}

func TestRedisStore_TTL(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	store := NewRedisStore("test-redis", client, "cache:ttl:")
	ctx := context.Background()

	defer store.DeleteByPrefix(ctx, "")

	t.Run("Expired key", func(t *testing.T) {
		store.Set(ctx, "expiring", []byte("value"), 100*time.Millisecond)

		// Should exist initially
		if !store.Exists(ctx, "expiring") {
			t.Error("Key should exist before expiration")
		}

		// Wait for expiration
		time.Sleep(200 * time.Millisecond)

		// Should be expired
		_, err := store.Get(ctx, "expiring")
		if err == nil {
			t.Error("Get() expected error for expired key")
		}
	})
}

func TestRedisStore_DeleteByPrefix(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	store := NewRedisStore("test-redis", client, "cache:prefix:")
	ctx := context.Background()

	defer store.DeleteByPrefix(ctx, "")

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

func TestRedisStore_DeleteByPattern(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	store := NewRedisStore("test-redis", client, "cache:pattern:")
	ctx := context.Background()

	defer store.DeleteByPrefix(ctx, "")

	t.Run("Delete exact key", func(t *testing.T) {
		store.Set(ctx, "exact", []byte("value"), time.Minute)
		err := store.DeleteByPattern(ctx, "exact")
		if err != nil {
			t.Errorf("DeleteByPattern() error = %v", err)
		}
		if store.Exists(ctx, "exact") {
			t.Error("Key should be deleted")
		}
	})

	t.Run("Delete with wildcard", func(t *testing.T) {
		store.Set(ctx, "wild:1", []byte("a"), time.Minute)
		store.Set(ctx, "wild:2", []byte("b"), time.Minute)

		err := store.DeleteByPattern(ctx, "wild:*")
		if err != nil {
			t.Errorf("DeleteByPattern() error = %v", err)
		}
		if store.Exists(ctx, "wild:1") || store.Exists(ctx, "wild:2") {
			t.Error("Wildcard keys should be deleted")
		}
	})
}

func TestRedisStore_Close(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	store := NewRedisStore("test-redis", client, "cache:close:")

	// Close should not return error
	err := store.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestRedisStore_BuildKey(t *testing.T) {
	client := getTestRedisClient(t)
	defer client.Close()

	t.Run("With prefix", func(t *testing.T) {
		store := NewRedisStore("test", client, "myprefix:")
		key := store.buildKey("mykey")
		if key != "myprefix:mykey" {
			t.Errorf("buildKey() = %v, want myprefix:mykey", key)
		}
	})

	t.Run("Without prefix", func(t *testing.T) {
		store := NewRedisStore("test", client, "")
		key := store.buildKey("mykey")
		if key != "mykey" {
			t.Errorf("buildKey() = %v, want mykey", key)
		}
	})
}
