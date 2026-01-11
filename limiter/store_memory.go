package limiter

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"
)

// memoryStore 内存存储实现
type memoryStore struct {
	mu     sync.RWMutex
	data   map[string]*memoryValue
	zsets  map[string]*memoryZSet
	closed bool
}

// memoryValue 内存值
type memoryValue struct {
	data     string
	expireAt time.Time
}

// memoryZSet 内存有序集合
type memoryZSet struct {
	members  map[string]float64 // member -> score
	expireAt time.Time
}

// NewMemoryStore 创建内存存储
func NewMemoryStore() Store {
	store := &memoryStore{
		data:  make(map[string]*memoryValue),
		zsets: make(map[string]*memoryZSet),
	}

	// 启动清理协程
	go store.cleanupLoop(1 * time.Minute)

	return store
}

// Get 获取值
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

	// 检查是否过期
	if !val.expireAt.IsZero() && time.Now().After(val.expireAt) {
		return "", ErrKeyNotFound
	}

	return val.data, nil
}

// Set 设置值
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

// GetInt64 获取整数值
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

// SetInt64 设置整数值
func (s *memoryStore) SetInt64(ctx context.Context, key string, value int64, ttl time.Duration) error {
	return s.Set(ctx, key, strconv.FormatInt(value, 10), ttl)
}

// Incr 原子递增
func (s *memoryStore) Incr(ctx context.Context, key string) (int64, error) {
	return s.IncrBy(ctx, key, 1)
}

// IncrBy 原子递增指定值
func (s *memoryStore) IncrBy(ctx context.Context, key string, delta int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return 0, errors.New("store is closed")
	}

	var currentVal int64
	if val, exists := s.data[key]; exists {
		// 检查是否过期
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

	// 保留原来的过期时间
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

// Decr 原子递减
func (s *memoryStore) Decr(ctx context.Context, key string) (int64, error) {
	return s.DecrBy(ctx, key, 1)
}

// DecrBy 原子递减指定值
func (s *memoryStore) DecrBy(ctx context.Context, key string, delta int64) (int64, error) {
	return s.IncrBy(ctx, key, -delta)
}

// Expire 设置过期时间
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

// TTL 获取剩余过期时间
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
		return -1, nil // 永不过期
	}

	ttl := time.Until(val.expireAt)
	if ttl < 0 {
		return 0, ErrKeyNotFound // 已过期
	}

	return ttl, nil
}

// Del 删除键
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

// Exists 检查键是否存在
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

	// 检查是否过期
	if !val.expireAt.IsZero() && time.Now().After(val.expireAt) {
		return false, nil
	}

	return true, nil
}

// ZAdd 添加到有序集合
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

// ZRemRangeByScore 按分数范围删除
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

// ZCount 统计分数范围内的元素数量
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

	// 检查是否过期
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

// Eval 执行Lua脚本（内存存储不支持）
func (s *memoryStore) Eval(ctx context.Context, script string, keys []string, args []interface{}) (interface{}, error) {
	return nil, ErrStoreNotSupported
}

// Close 关闭连接
func (s *memoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	s.data = nil
	s.zsets = nil

	return nil
}

// cleanupLoop 定期清理过期数据
func (s *memoryStore) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanup()

		// 检查是否已关闭
		s.mu.RLock()
		closed := s.closed
		s.mu.RUnlock()

		if closed {
			return
		}
	}
}

// cleanup 清理过期数据
func (s *memoryStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	now := time.Now()

	// 清理普通键
	for key, val := range s.data {
		if !val.expireAt.IsZero() && now.After(val.expireAt) {
			delete(s.data, key)
		}
	}

	// 清理有序集合
	for key, zset := range s.zsets {
		if !zset.expireAt.IsZero() && now.After(zset.expireAt) {
			delete(s.zsets, key)
		}
	}
}

