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

// LoginAttemptStore 登录尝试存储接口
type LoginAttemptStore interface {
	// GetAttempts 获取登录尝试次数
	GetAttempts(ctx context.Context, username string) (int, error)
	
	// IncrementAttempts 增加登录尝试次数
	IncrementAttempts(ctx context.Context, username string, ttl time.Duration) error
	
	// ResetAttempts 重置登录尝试次数
	ResetAttempts(ctx context.Context, username string) error
	
	// IsLocked 检查账户是否被锁定
	IsLocked(ctx context.Context, username string, maxAttempts int) (bool, error)
	
	// Close 关闭存储
	Close() error
}

// RedisLoginAttemptStore Redis 登录尝试存储
type RedisLoginAttemptStore struct {
	client *redis.Client
	prefix string
	logger *logger.CtxZapLogger
}

// NewRedisLoginAttemptStore 创建 Redis 登录尝试存储
func NewRedisLoginAttemptStore(client *redis.Client, prefix string, logger *logger.CtxZapLogger) *RedisLoginAttemptStore {
	return &RedisLoginAttemptStore{
		client: client,
		prefix: prefix,
		logger: logger,
	}
}

// GetAttempts 获取登录尝试次数
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

// IncrementAttempts 增加登录尝试次数
func (s *RedisLoginAttemptStore) IncrementAttempts(ctx context.Context, username string, ttl time.Duration) error {
	key := s.prefix + username
	pipe := s.client.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// ResetAttempts 重置登录尝试次数
func (s *RedisLoginAttemptStore) ResetAttempts(ctx context.Context, username string) error {
	key := s.prefix + username
	return s.client.Del(ctx, key).Err()
}

// IsLocked 检查账户是否被锁定
func (s *RedisLoginAttemptStore) IsLocked(ctx context.Context, username string, maxAttempts int) (bool, error) {
	attempts, err := s.GetAttempts(ctx, username)
	if err != nil {
		return false, err
	}
	return attempts >= maxAttempts, nil
}

// Close 关闭存储
func (s *RedisLoginAttemptStore) Close() error {
	// Redis 连接由 Redis 组件管理，这里不需要关闭
	return nil
}

// MemoryLoginAttemptStore 内存登录尝试存储（用于测试）
type MemoryLoginAttemptStore struct {
	mu       sync.RWMutex
	attempts map[string]*attemptRecord
	logger   *logger.CtxZapLogger
}

type attemptRecord struct {
	count     int
	expiresAt time.Time
}

// NewMemoryLoginAttemptStore 创建内存登录尝试存储
func NewMemoryLoginAttemptStore(logger *logger.CtxZapLogger) *MemoryLoginAttemptStore {
	store := &MemoryLoginAttemptStore{
		attempts: make(map[string]*attemptRecord),
		logger:   logger,
	}
	
	// 启动清理 goroutine
	go store.cleanup()
	
	return store
}

// GetAttempts 获取登录尝试次数
func (s *MemoryLoginAttemptStore) GetAttempts(ctx context.Context, username string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.attempts[username]
	if !exists {
		return 0, nil
	}

	// 检查是否过期
	if time.Now().After(record.expiresAt) {
		return 0, nil
	}

	return record.count, nil
}

// IncrementAttempts 增加登录尝试次数
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

// ResetAttempts 重置登录尝试次数
func (s *MemoryLoginAttemptStore) ResetAttempts(ctx context.Context, username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.attempts, username)
	return nil
}

// IsLocked 检查账户是否被锁定
func (s *MemoryLoginAttemptStore) IsLocked(ctx context.Context, username string, maxAttempts int) (bool, error) {
	attempts, err := s.GetAttempts(ctx, username)
	if err != nil {
		return false, err
	}
	return attempts >= maxAttempts, nil
}

// Close 关闭存储
func (s *MemoryLoginAttemptStore) Close() error {
	return nil
}

// cleanup 清理过期记录
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

// createLoginAttemptStore 创建登录尝试存储（工厂函数）
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

