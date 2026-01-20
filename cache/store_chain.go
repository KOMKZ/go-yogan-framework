package cache

import (
	"context"
	"time"
)

// ChainStore chained cache storage (multi-level caching)
type ChainStore struct {
	name   string
	stores []Store
}

// NewChainStore creates chain storage
func NewChainStore(name string, stores ...Store) *ChainStore {
	return &ChainStore{
		name:   name,
		stores: stores,
	}
}

// Return storage name
func (s *ChainStore) Name() string {
	return s.name
}

// Get from chained storage
// Search from front to back, refill preceding layers upon hit
func (s *ChainStore) Get(ctx context.Context, key string) ([]byte, error) {
	var hitIndex = -1
	var value []byte

	// search from front to back
	for i, store := range s.stores {
		val, err := store.Get(ctx, key)
		if err == nil {
			value = val
			hitIndex = i
			break
		}
	}

	if hitIndex == -1 {
		return nil, ErrCacheMiss
	}

	// Refill previous levels (no refill needed for L1 hit)
	if hitIndex > 0 {
		for i := 0; i < hitIndex; i++ {
			// Use a shorter TTL for top-up repopulation
			s.stores[i].Set(ctx, key, value, time.Minute)
		}
	}

	return value, nil
}

// Set for all layers
func (s *ChainStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	var lastErr error
	for _, store := range s.stores {
		if err := store.Set(ctx, key, value, ttl); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// Delete from all layers
func (s *ChainStore) Delete(ctx context.Context, key string) error {
	var lastErr error
	for _, store := range s.stores {
		if err := store.Delete(ctx, key); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// DeleteByPrefix delete by prefix from all layers
func (s *ChainStore) DeleteByPrefix(ctx context.Context, prefix string) error {
	var lastErr error
	for _, store := range s.stores {
		if err := store.DeleteByPrefix(ctx, prefix); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// Exists Check if there is any existence (existence in any layer is sufficient)
func (s *ChainStore) Exists(ctx context.Context, key string) bool {
	for _, store := range s.stores {
		if store.Exists(ctx, key) {
			return true
		}
	}
	return false
}

// Close all layers
func (s *ChainStore) Close() error {
	var lastErr error
	for _, store := range s.stores {
		if err := store.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
