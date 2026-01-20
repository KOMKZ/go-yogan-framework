package limiter

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"
)

// memoryStore memory storage implementation
type memoryStore struct {
	mu     sync.RWMutex
	data   map[string]*memoryValue
	zsets  map[string]*memoryZSet
	closed bool
}

// memory value
type memoryValue struct {
	data     string
	expireAt time.Time
}

// memoryZSet ordered memory set
type memoryZSet struct {
	members  map[string]float64 // member -> score
	expireAt time.Time
}

// Create memory store
func NewMemoryStore() Store {
	store := &memoryStore{
		data:  make(map[string]*memoryValue),
		zsets: make(map[string]*memoryZSet),
	}

	// Start cleanup coroutine
	go store.cleanupLoop(1 * time.Minute)

	return store
}

// Get Retrieve value
func (s *memoryStore) Get(ctx context.Context, key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return "", errors.New("store is closed")
	}

	val, exists := s.data[key]
	if !exists {
		return "", ErrKeyNotFound
	}

	// Check if expired
	if !val.expireAt.IsZero() && time.Now().After(val.expireAt) {
		return "", ErrKeyNotFound
	}

	return val.data, nil
}

// Set configuration value
func (s *memoryStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("store is closed")
	}

	var expireAt time.Time
	if ttl > 0 {
		expireAt = time.Now().Add(ttl)
	}

	s.data[key] = &memoryValue{
		data:     value,
		expireAt: expireAt,
	}

	return nil
}

// GetInt64 get integer value
func (s *memoryStore) GetInt64(ctx context.Context, key string) (int64, error) {
	str, err := s.Get(ctx, key)
	if err != nil {
		return 0, err
	}

	val, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse int64 failed: %w", err)
	}

	return val, nil
}

// SetInt64 sets an integer value
func (s *memoryStore) SetInt64(ctx context.Context, key string, value int64, ttl time.Duration) error {
	return s.Set(ctx, key, strconv.FormatInt(value, 10), ttl)
}

// Increment atomic increment
func (s *memoryStore) Incr(ctx context.Context, key string) (int64, error) {
	return s.IncrBy(ctx, key, 1)
}

// IncrBy atomic increment by specified value
func (s *memoryStore) IncrBy(ctx context.Context, key string, delta int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return 0, errors.New("store is closed")
	}

	var currentVal int64
	if val, exists := s.data[key]; exists {
		// Check if expired
		if !val.expireAt.IsZero() && time.Now().After(val.expireAt) {
			currentVal = 0
		} else {
			parsed, err := strconv.ParseInt(val.data, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("parse current value failed: %w", err)
			}
			currentVal = parsed
		}
	}

	newVal := currentVal + delta

	// Keep the original expiration time
	var expireAt time.Time
	if val, exists := s.data[key]; exists && !val.expireAt.IsZero() {
		expireAt = val.expireAt
	}

	s.data[key] = &memoryValue{
		data:     strconv.FormatInt(newVal, 10),
		expireAt: expireAt,
	}

	return newVal, nil
}

// Decrement atomic decrement
func (s *memoryStore) Decr(ctx context.Context, key string) (int64, error) {
	return s.DecrBy(ctx, key, 1)
}

// Atomically decrement the specified value
func (s *memoryStore) DecrBy(ctx context.Context, key string, delta int64) (int64, error) {
	return s.IncrBy(ctx, key, -delta)
}

// Set expiration time
func (s *memoryStore) Expire(ctx context.Context, key string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("store is closed")
	}

	val, exists := s.data[key]
	if !exists {
		return ErrKeyNotFound
	}

	if ttl > 0 {
		val.expireAt = time.Now().Add(ttl)
	} else {
		val.expireAt = time.Time{}
	}

	return nil
}

// Get remaining TTL time
func (s *memoryStore) TTL(ctx context.Context, key string) (time.Duration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return 0, errors.New("store is closed")
	}

	val, exists := s.data[key]
	if !exists {
		return 0, ErrKeyNotFound
	}

	if val.expireAt.IsZero() {
		return -1, nil // never expires
	}

	ttl := time.Until(val.expireAt)
	if ttl < 0 {
		return 0, ErrKeyNotFound // Expired
	}

	return ttl, nil
}

// Delete key
func (s *memoryStore) Del(ctx context.Context, keys ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("store is closed")
	}

	for _, key := range keys {
		delete(s.data, key)
		delete(s.zsets, key)
	}

	return nil
}

// Exists Check if key exists
func (s *memoryStore) Exists(ctx context.Context, key string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return false, errors.New("store is closed")
	}

	val, exists := s.data[key]
	if !exists {
		return false, nil
	}

	// Check if expired
	if !val.expireAt.IsZero() && time.Now().After(val.expireAt) {
		return false, nil
	}

	return true, nil
}

// Add to sorted set
func (s *memoryStore) ZAdd(ctx context.Context, key string, score float64, member string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("store is closed")
	}

	zset, exists := s.zsets[key]
	if !exists {
		zset = &memoryZSet{
			members: make(map[string]float64),
		}
		s.zsets[key] = zset
	}

	zset.members[member] = score

	return nil
}

// Remove by score range
func (s *memoryStore) ZRemRangeByScore(ctx context.Context, key string, min, max float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("store is closed")
	}

	zset, exists := s.zsets[key]
	if !exists {
		return nil
	}

	for member, score := range zset.members {
		if score >= min && score <= max {
			delete(zset.members, member)
		}
	}

	return nil
}

// ZCount statistics the number of elements within a score range
func (s *memoryStore) ZCount(ctx context.Context, key string, min, max float64) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return 0, errors.New("store is closed")
	}

	zset, exists := s.zsets[key]
	if !exists {
		return 0, nil
	}

	// Check if expired
	if !zset.expireAt.IsZero() && time.Now().After(zset.expireAt) {
		return 0, nil
	}

	var count int64
	for _, score := range zset.members {
		if score >= min && score <= max {
			count++
		}
	}

	return count, nil
}

// Eval executes Lua scripts (in-memory storage does not support)
func (s *memoryStore) Eval(ctx context.Context, script string, keys []string, args []interface{}) (interface{}, error) {
	return nil, ErrStoreNotSupported
}

// Close connection
func (s *memoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	s.data = nil
	s.zsets = nil

	return nil
}

// cleanupLoop periodically cleans up expired data
func (s *memoryStore) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanup()

		// Check if closed
		s.mu.RLock()
		closed := s.closed
		s.mu.RUnlock()

		if closed {
			return
		}
	}
}

// cleanup Remove expired data
func (s *memoryStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	now := time.Now()

	// Clear regular keys
	for key, val := range s.data {
		if !val.expireAt.IsZero() && now.After(val.expireAt) {
			delete(s.data, key)
		}
	}

	// Clear ordered set
	for key, zset := range s.zsets {
		if !zset.expireAt.IsZero() && now.After(zset.expireAt) {
			delete(s.zsets, key)
		}
	}
}

