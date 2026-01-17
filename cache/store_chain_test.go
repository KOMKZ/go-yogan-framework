package cache

import (
	"context"
	"testing"
	"time"
)

func TestChainStore_Basic(t *testing.T) {
	l1 := NewMemoryStore("l1", 100)
	l2 := NewMemoryStore("l2", 100)
	chain := NewChainStore("chain", l1, l2)
	ctx := context.Background()

	t.Run("Name", func(t *testing.T) {
		if chain.Name() != "chain" {
			t.Errorf("Name() = %v, want chain", chain.Name())
		}
	})

	t.Run("Set writes to all layers", func(t *testing.T) {
		err := chain.Set(ctx, "key1", []byte("value1"), time.Minute)
		if err != nil {
			t.Errorf("Set() error = %v", err)
		}

		// Both layers should have the value
		if !l1.Exists(ctx, "key1") {
			t.Error("L1 should have the key")
		}
		if !l2.Exists(ctx, "key1") {
			t.Error("L2 should have the key")
		}
	})

	t.Run("Get from L1", func(t *testing.T) {
		chain.Set(ctx, "key2", []byte("value2"), time.Minute)

		data, err := chain.Get(ctx, "key2")
		if err != nil {
			t.Errorf("Get() error = %v", err)
		}
		if string(data) != "value2" {
			t.Errorf("Get() = %v, want value2", string(data))
		}
	})

	t.Run("Get from L2 and backfill L1", func(t *testing.T) {
		// Set only in L2
		l2.Set(ctx, "key3", []byte("value3"), time.Minute)

		// L1 should not have it
		if l1.Exists(ctx, "key3") {
			t.Error("L1 should not have key3 initially")
		}

		// Get from chain
		data, err := chain.Get(ctx, "key3")
		if err != nil {
			t.Errorf("Get() error = %v", err)
		}
		if string(data) != "value3" {
			t.Errorf("Get() = %v, want value3", string(data))
		}

		// L1 should now be backfilled
		if !l1.Exists(ctx, "key3") {
			t.Error("L1 should be backfilled with key3")
		}
	})

	t.Run("Get non-existent", func(t *testing.T) {
		_, err := chain.Get(ctx, "non-existent")
		if err == nil {
			t.Error("Get() expected error for non-existent key")
		}
	})
}

func TestChainStore_Delete(t *testing.T) {
	l1 := NewMemoryStore("l1", 100)
	l2 := NewMemoryStore("l2", 100)
	chain := NewChainStore("chain", l1, l2)
	ctx := context.Background()

	chain.Set(ctx, "key1", []byte("value1"), time.Minute)

	err := chain.Delete(ctx, "key1")
	if err != nil {
		t.Errorf("Delete() error = %v", err)
	}

	// Both layers should not have the key
	if l1.Exists(ctx, "key1") {
		t.Error("L1 should not have the key after delete")
	}
	if l2.Exists(ctx, "key1") {
		t.Error("L2 should not have the key after delete")
	}
}

func TestChainStore_DeleteByPrefix(t *testing.T) {
	l1 := NewMemoryStore("l1", 100)
	l2 := NewMemoryStore("l2", 100)
	chain := NewChainStore("chain", l1, l2)
	ctx := context.Background()

	chain.Set(ctx, "user:1", []byte("a"), time.Minute)
	chain.Set(ctx, "user:2", []byte("b"), time.Minute)

	err := chain.DeleteByPrefix(ctx, "user:")
	if err != nil {
		t.Errorf("DeleteByPrefix() error = %v", err)
	}

	if l1.Exists(ctx, "user:1") || l1.Exists(ctx, "user:2") {
		t.Error("L1 should not have user keys after delete")
	}
	if l2.Exists(ctx, "user:1") || l2.Exists(ctx, "user:2") {
		t.Error("L2 should not have user keys after delete")
	}
}

func TestChainStore_Exists(t *testing.T) {
	l1 := NewMemoryStore("l1", 100)
	l2 := NewMemoryStore("l2", 100)
	chain := NewChainStore("chain", l1, l2)
	ctx := context.Background()

	// Set only in L2
	l2.Set(ctx, "key1", []byte("value1"), time.Minute)

	if !chain.Exists(ctx, "key1") {
		t.Error("Exists() = false, want true")
	}

	if chain.Exists(ctx, "non-existent") {
		t.Error("Exists() = true, want false")
	}
}

func TestChainStore_Close(t *testing.T) {
	l1 := NewMemoryStore("l1", 100)
	l2 := NewMemoryStore("l2", 100)
	chain := NewChainStore("chain", l1, l2)
	ctx := context.Background()

	chain.Set(ctx, "key1", []byte("value1"), time.Minute)

	err := chain.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}
