package cache

import (
	"context"
	"strings"
	"sync"
	"time"
)

// MemoryStore memory cache storage
type MemoryStore struct {
	name    string
	data    map[string]*memoryItem
	mu      sync.RWMutex
	maxSize int
}

// memoryItem cache item
type memoryItem struct {
	value     []byte
	expiresAt time.Time
}

// Create memory store
func NewMemoryStore(name string, maxSize int) *MemoryStore {
	if maxSize <= 0 {
		maxSize = 10000
	}
	store := &MemoryStore{
		name:    name,
		data:    make(map[string]*memoryItem),
		maxSize: maxSize,
	}
	// Start expired item cleanup coroutine
	go store.cleanupLoop()
	return store
}

// Name Returns the storage name
func (s *MemoryStore) Name() string {
	return s.name
}

// Get cached value
func (s *MemoryStore) Get(ctx context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	item, ok := s.data[key]
	s.mu.RUnlock()

	if !ok {
		return nil, ErrCacheMiss
	}

	// Check expiration
	if !item.expiresAt.IsZero() && time.Now().After(item.expiresAt) {
		s.Delete(ctx, key)
		return nil, ErrCacheMiss
	}

	return item.value, nil
}

// Set cache value
func (s *MemoryStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check capacity, delete oldest when exceeded
	if len(s.data) >= s.maxSize {
		s.evictOne()
	}

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	s.data[key] = &memoryItem{
		value:     value,
		expiresAt: expiresAt,
	}

	return nil
}

// Delete cache
func (s *MemoryStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

// DeleteByPrefix delete by prefix
func (s *MemoryStore) DeleteByPrefix(ctx context.Context, prefix string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key := range s.data {
		if strings.HasPrefix(key, prefix) {
			delete(s.data, key)
		}
	}
	return nil
}

// Exists check if Key exists
func (s *MemoryStore) Exists(ctx context.Context, key string) bool {
	s.mu.RLock()
	item, ok := s.data[key]
	s.mu.RUnlock()

	if !ok {
		return false
	}

	// Check expiration
	if !item.expiresAt.IsZero() && time.Now().After(item.expiresAt) {
		return false
	}

	return true
}

// Close storage connection
func (s *MemoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make(map[string]*memoryItem)
	return nil
}

// Returns the current cache size
func (s *MemoryStore) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

// evictOne Evict one entry (simple FIFO)
func (s *MemoryStore) evictOne() {
	// find the earliest expiration or the first one
	var oldest string
	var oldestTime time.Time

	for key, item := range s.data {
		if oldest == "" || (!item.expiresAt.IsZero() && item.expiresAt.Before(oldestTime)) {
			oldest = key
			oldestTime = item.expiresAt
		}
	}

	if oldest != "" {
		delete(s.data, oldest)
	}
}

// cleanupLoop periodically cleans up expired entries
func (s *MemoryStore) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanup()
	}
}

// cleanup Remove expired entries
func (s *MemoryStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for key, item := range s.data {
		if !item.expiresAt.IsZero() && now.After(item.expiresAt) {
			delete(s.data, key)
		}
	}
}
