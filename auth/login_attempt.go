package auth

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/redis/go-redis/v9"
)

// LoginAttemptStore login attempt storage interface
type LoginAttemptStore interface {
	// GetAttempts Get login attempt count
	GetAttempts(ctx context.Context, username string) (int, error)
	
	// Increment login attempt count
	IncrementAttempts(ctx context.Context, username string, ttl time.Duration) error
	
	// Reset login attempt count
	ResetAttempts(ctx context.Context, username string) error
	
	// Check if the account is locked
	IsLocked(ctx context.Context, username string, maxAttempts int) (bool, error)
	
	// Close storage
	Close() error
}

// RedisLoginAttemptStore Redis login attempt storage
type RedisLoginAttemptStore struct {
	client *redis.Client
	prefix string
	logger *logger.CtxZapLogger
}

// Create new Redis login attempt store
func NewRedisLoginAttemptStore(client *redis.Client, prefix string, logger *logger.CtxZapLogger) *RedisLoginAttemptStore {
	return &RedisLoginAttemptStore{
		client: client,
		prefix: prefix,
		logger: logger,
	}
}

// GetAttempts Get login attempt count
func (s *RedisLoginAttemptStore) GetAttempts(ctx context.Context, username string) (int, error) {
	key := s.prefix + username
	val, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	attempts, err := strconv.Atoi(val)
	if err != nil {
		return 0, err
	}

	return attempts, nil
}

// Increment login attempt count
func (s *RedisLoginAttemptStore) IncrementAttempts(ctx context.Context, username string, ttl time.Duration) error {
	key := s.prefix + username
	pipe := s.client.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// Reset login attempt count
func (s *RedisLoginAttemptStore) ResetAttempts(ctx context.Context, username string) error {
	key := s.prefix + username
	return s.client.Del(ctx, key).Err()
}

// Check if the account is locked
func (s *RedisLoginAttemptStore) IsLocked(ctx context.Context, username string, maxAttempts int) (bool, error) {
	attempts, err := s.GetAttempts(ctx, username)
	if err != nil {
		return false, err
	}
	return attempts >= maxAttempts, nil
}

// Close storage
func (s *RedisLoginAttemptStore) Close() error {
	// Redis connection is managed by the Redis component, so there is no need to close it here
	return nil
}

// MemoryLoginAttemptStore In-memory login attempt storage (for testing)
type MemoryLoginAttemptStore struct {
	mu       sync.RWMutex
	attempts map[string]*attemptRecord
	logger   *logger.CtxZapLogger
}

type attemptRecord struct {
	count     int
	expiresAt time.Time
}

// Create memory login attempt store
func NewMemoryLoginAttemptStore(logger *logger.CtxZapLogger) *MemoryLoginAttemptStore {
	store := &MemoryLoginAttemptStore{
		attempts: make(map[string]*attemptRecord),
		logger:   logger,
	}
	
	// Start cleanup goroutine
	go store.cleanup()
	
	return store
}

// GetAttempts Get login attempt count
func (s *MemoryLoginAttemptStore) GetAttempts(ctx context.Context, username string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.attempts[username]
	if !exists {
		return 0, nil
	}

	// Check if expired
	if time.Now().After(record.expiresAt) {
		return 0, nil
	}

	return record.count, nil
}

// Increment login attempt count
func (s *MemoryLoginAttemptStore) IncrementAttempts(ctx context.Context, username string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, exists := s.attempts[username]
	if !exists || time.Now().After(record.expiresAt) {
		s.attempts[username] = &attemptRecord{
			count:     1,
			expiresAt: time.Now().Add(ttl),
		}
	} else {
		record.count++
		record.expiresAt = time.Now().Add(ttl)
	}

	return nil
}

// Reset login attempt count
func (s *MemoryLoginAttemptStore) ResetAttempts(ctx context.Context, username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.attempts, username)
	return nil
}

// Check if the account is locked
func (s *MemoryLoginAttemptStore) IsLocked(ctx context.Context, username string, maxAttempts int) (bool, error) {
	attempts, err := s.GetAttempts(ctx, username)
	if err != nil {
		return false, err
	}
	return attempts >= maxAttempts, nil
}

// Close storage
func (s *MemoryLoginAttemptStore) Close() error {
	return nil
}

// cleanup Remove expired records
func (s *MemoryLoginAttemptStore) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for username, record := range s.attempts {
			if now.After(record.expiresAt) {
				delete(s.attempts, username)
			}
		}
		s.mu.Unlock()
	}
}

// createLoginAttemptStore Create login attempt store (factory function)
func createLoginAttemptStore(
	config LoginAttemptConfig,
	redisClient *redis.Client,
	logger *logger.CtxZapLogger,
) (LoginAttemptStore, error) {
	if !config.Enabled {
		return nil, nil
	}

	switch config.Storage {
	case "redis":
		if redisClient == nil {
			return nil, fmt.Errorf("redis client is required for redis storage")
		}
		return NewRedisLoginAttemptStore(redisClient, config.RedisKeyPrefix, logger), nil
	case "memory":
		return NewMemoryLoginAttemptStore(logger), nil
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", config.Storage)
	}
}

