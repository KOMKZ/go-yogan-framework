// 本文件实现按 key 分桶的并发许可限制（channel semaphore）。
// 与 HTTP 请求限速（token bucket）不同：并发限制用于保护外部 provider（LLM/TTS/文生图）的全局 QPS/并发上限，
// 由调用方（如 media-jobs executor）在调用 provider 前 Acquire、完成后 release。
package limiter

import (
	"context"
	"sync"
)

// ConcurrencyLimiter 按 key 限制并发执行数。
// key 的并发上限在首次 Acquire 时固定（后续 Acquire 传不同 limit 不生效，避免同 key 限流漂移）。
type ConcurrencyLimiter struct {
	mu     sync.Mutex
	limits map[string]chan struct{}
}

// NewConcurrencyLimiter 构造空的并发限制器。
func NewConcurrencyLimiter() *ConcurrencyLimiter {
	return &ConcurrencyLimiter{limits: map[string]chan struct{}{}}
}

// Acquire 获取 key 的并发许可；等待期间受 ctx 取消控制。
// limit <= 0 时视为不限流，直接返回空 release。
// 返回的 release 幂等可重复调用。
func (l *ConcurrencyLimiter) Acquire(ctx context.Context, key string, limit int) (release func(), err error) {
	if l == nil || limit <= 0 {
		return func() {}, nil
	}
	slot := l.slot(key, limit)
	select {
	case slot <- struct{}{}:
		var once sync.Once
		return func() { once.Do(func() { <-slot }) }, nil
	case <-ctx.Done():
		return func() {}, ctx.Err()
	}
}

func (l *ConcurrencyLimiter) slot(key string, limit int) chan struct{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	if slot, ok := l.limits[key]; ok {
		return slot
	}
	slot := make(chan struct{}, limit)
	l.limits[key] = slot
	return slot
}
