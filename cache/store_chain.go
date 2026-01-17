package cache

import (
	"context"
	"time"
)

// ChainStore 链式缓存存储（多级缓存）
type ChainStore struct {
	name   string
	stores []Store
}

// NewChainStore 创建链式存储
func NewChainStore(name string, stores ...Store) *ChainStore {
	return &ChainStore{
		name:   name,
		stores: stores,
	}
}

// Name 返回存储名称
func (s *ChainStore) Name() string {
	return s.name
}

// Get 从链式存储获取
// 从前往后查询，命中后回填前面的层
func (s *ChainStore) Get(ctx context.Context, key string) ([]byte, error) {
	var hitIndex = -1
	var value []byte

	// 从前往后查询
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

	// 回填前面的层（L1 命中不需要回填）
	if hitIndex > 0 {
		for i := 0; i < hitIndex; i++ {
			// 使用较短的 TTL 回填上层
			s.stores[i].Set(ctx, key, value, time.Minute)
		}
	}

	return value, nil
}

// Set 设置到所有层
func (s *ChainStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	var lastErr error
	for _, store := range s.stores {
		if err := store.Set(ctx, key, value, ttl); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// Delete 从所有层删除
func (s *ChainStore) Delete(ctx context.Context, key string) error {
	var lastErr error
	for _, store := range s.stores {
		if err := store.Delete(ctx, key); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// DeleteByPrefix 从所有层按前缀删除
func (s *ChainStore) DeleteByPrefix(ctx context.Context, prefix string) error {
	var lastErr error
	for _, store := range s.stores {
		if err := store.DeleteByPrefix(ctx, prefix); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// Exists 检查是否存在（任意一层存在即可）
func (s *ChainStore) Exists(ctx context.Context, key string) bool {
	for _, store := range s.stores {
		if store.Exists(ctx, key) {
			return true
		}
	}
	return false
}

// Close 关闭所有层
func (s *ChainStore) Close() error {
	var lastErr error
	for _, store := range s.stores {
		if err := store.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
