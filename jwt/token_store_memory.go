package jwt

import (
	"context"
	"sync"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.uber.org/zap"
)

// MemoryTokenStore 内存 Token 存储（开发/测试环境）
type MemoryTokenStore struct {
	blacklist     sync.Map // map[string]time.Time (token -> expiry)
	userBlacklist sync.Map // map[string]time.Time (subject -> timestamp)
	logger        *logger.CtxZapLogger
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

// NewMemoryTokenStore 创建内存 Token 存储
func NewMemoryTokenStore(cleanupInterval time.Duration, log *logger.CtxZapLogger) *MemoryTokenStore {
	store := &MemoryTokenStore{
		logger: log,
		stopCh: make(chan struct{}),
	}

	// 启动定期清理
	if cleanupInterval > 0 {
		store.wg.Add(1)
		go store.cleanup(cleanupInterval)
	}

	return store
}

// IsBlacklisted 检查 Token 是否在黑名单
func (s *MemoryTokenStore) IsBlacklisted(ctx context.Context, token string) (bool, error) {
	value, exists := s.blacklist.Load(token)
	if !exists {
		return false, nil
	}

	expiry := value.(time.Time)
	if time.Now().After(expiry) {
		// 已过期，删除
		s.blacklist.Delete(token)
		return false, nil
	}

	return true, nil
}

// AddToBlacklist 添加到黑名单
func (s *MemoryTokenStore) AddToBlacklist(ctx context.Context, token string, ttl time.Duration) error {
	expiry := time.Now().Add(ttl)
	s.blacklist.Store(token, expiry)

	s.logger.DebugCtx(ctx, "token added to blacklist",
		zap.String("token_prefix", truncateToken(token)),
		zap.Duration("ttl", ttl),
	)

	return nil
}

// RemoveFromBlacklist 从黑名单移除
func (s *MemoryTokenStore) RemoveFromBlacklist(ctx context.Context, token string) error {
	s.blacklist.Delete(token)

	s.logger.DebugCtx(ctx, "token removed from blacklist",
		zap.String("token_prefix", truncateToken(token)),
	)

	return nil
}

// BlacklistUserTokens 添加用户所有 Token 到黑名单
func (s *MemoryTokenStore) BlacklistUserTokens(ctx context.Context, subject string, ttl time.Duration) error {
	timestamp := time.Now()
	s.userBlacklist.Store(subject, timestamp)

	s.logger.InfoCtx(ctx, "user tokens blacklisted",
		zap.String("subject", subject),
		zap.Time("timestamp", timestamp),
	)

	return nil
}

// IsUserBlacklisted 检查用户是否被全局拉黑
func (s *MemoryTokenStore) IsUserBlacklisted(ctx context.Context, subject string, issuedAt time.Time) (bool, error) {
	value, exists := s.userBlacklist.Load(subject)
	if !exists {
		return false, nil
	}

	blacklistTime := value.(time.Time)
	// 如果 Token 签发时间早于拉黑时间，则认为被拉黑
	return issuedAt.Before(blacklistTime), nil
}

// Close 关闭连接
func (s *MemoryTokenStore) Close() error {
	close(s.stopCh)
	s.wg.Wait()
	return nil
}

// cleanup 定期清理过期条目
func (s *MemoryTokenStore) cleanup(interval time.Duration) {
	defer s.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.doCleanup()
		case <-s.stopCh:
			return
		}
	}
}

// doCleanup 执行清理
func (s *MemoryTokenStore) doCleanup() {
	now := time.Now()
	count := 0

	s.blacklist.Range(func(key, value interface{}) bool {
		expiry := value.(time.Time)
		if now.After(expiry) {
			s.blacklist.Delete(key)
			count++
		}
		return true
	})

	if count > 0 {
		s.logger.DebugCtx(context.Background(), "cleaned up expired blacklist entries",
			zap.Int("count", count),
		)
	}
}

// truncateToken 截取 Token 前缀（日志用）
func truncateToken(token string) string {
	if len(token) <= 10 {
		return token
	}
	return token[:10] + "..."
}
