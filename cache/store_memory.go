package cache

import (
	"context"
	"strings"
	"sync"
	"time"
)

// MemoryStore 内存缓存存储
type MemoryStore struct {
	name    string
	data    map[string]*memoryItem
	mu      sync.RWMutex
	maxSize int
}

// memoryItem 缓存项
type memoryItem struct {
	value     []byte
	expiresAt time.Time
}

// NewMemoryStore 创建内存存储
func NewMemoryStore(name string, maxSize int) *MemoryStore {
	if maxSize <= 0 {
		maxSize = 10000
	}
	store := &MemoryStore{
		name:    name,
		data:    make(map[string]*memoryItem),
		maxSize: maxSize,
	}
	// 启动过期清理协程
	go store.cleanupLoop()
	return store
}

// Name 返回存储名称
func (s *MemoryStore) Name() string {
	return s.name
}

// Get 获取缓存值
func (s *MemoryStore) Get(ctx context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	item, ok := s.data[key]
	s.mu.RUnlock()

	if !ok {
		return nil, ErrCacheMiss
	}

	// 检查过期
	if !item.expiresAt.IsZero() && time.Now().After(item.expiresAt) {
		s.Delete(ctx, key)
		return nil, ErrCacheMiss
	}

	return item.value, nil
}

// Set 设置缓存值
func (s *MemoryStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查容量，超出时删除最旧的
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

// Delete 删除缓存
func (s *MemoryStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

// DeleteByPrefix 按前缀删除
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

// Exists 检查 Key 是否存在
func (s *MemoryStore) Exists(ctx context.Context, key string) bool {
	s.mu.RLock()
	item, ok := s.data[key]
	s.mu.RUnlock()

	if !ok {
		return false
	}

	// 检查过期
	if !item.expiresAt.IsZero() && time.Now().After(item.expiresAt) {
		return false
	}

	return true
}

// Close 关闭存储
func (s *MemoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make(map[string]*memoryItem)
	return nil
}

// Size 返回当前缓存大小
func (s *MemoryStore) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

// evictOne 淘汰一个条目（简单 FIFO）
func (s *MemoryStore) evictOne() {
	// 找到最早过期的，或者第一个
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

// cleanupLoop 定期清理过期条目
func (s *MemoryStore) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanup()
	}
}

// cleanup 清理过期条目
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
