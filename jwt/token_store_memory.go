package jwt

import (
	"context"
	"sync"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.uber.org/zap"
)

// MemoryTokenStore (in-memory token storage for development/test environment)
type MemoryTokenStore struct {
	blacklist     sync.Map // map[string]time.Time (token -> expiry)
	userBlacklist sync.Map // map[string]time.Time (subject -> timestamp)
	logger        *logger.CtxZapLogger
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

// Create memory token storage
func NewMemoryTokenStore(cleanupInterval time.Duration, log *logger.CtxZapLogger) *MemoryTokenStore {
	store := &MemoryTokenStore{
		logger: log,
		stopCh: make(chan struct{}),
	}

	// Start regular cleanup
	if cleanupInterval > 0 {
		store.wg.Add(1)
		go store.cleanup(cleanupInterval)
	}

	return store
}

// Check if Token is in blacklist
func (s *MemoryTokenStore) IsBlacklisted(ctx context.Context, token string) (bool, error) {
	value, exists := s.blacklist.Load(token)
	if !exists {
		return false, nil
	}

	expiry := value.(time.Time)
	if time.Now().After(expiry) {
		// Expired, delete
		s.blacklist.Delete(token)
		return false, nil
	}

	return true, nil
}

// AddToBlacklist Add to blacklist
func (s *MemoryTokenStore) AddToBlacklist(ctx context.Context, token string, ttl time.Duration) error {
	expiry := time.Now().Add(ttl)
	s.blacklist.Store(token, expiry)

	s.logger.DebugCtx(ctx, "token added to blacklist",
		zap.String("token_prefix", truncateToken(token)),
		zap.Duration("ttl", ttl),
	)

	return nil
}

// RemoveFromBlacklist Remove from blacklist
func (s *MemoryTokenStore) RemoveFromBlacklist(ctx context.Context, token string) error {
	s.blacklist.Delete(token)

	s.logger.DebugCtx(ctx, "token removed from blacklist",
		zap.String("token_prefix", truncateToken(token)),
	)

	return nil
}

// Add all user tokens to the blacklist
func (s *MemoryTokenStore) BlacklistUserTokens(ctx context.Context, subject string, ttl time.Duration) error {
	timestamp := time.Now()
	s.userBlacklist.Store(subject, timestamp)

	s.logger.InfoCtx(ctx, "user tokens blacklisted",
		zap.String("subject", subject),
		zap.Time("timestamp", timestamp),
	)

	return nil
}

// Check if user is globally blacklisted
func (s *MemoryTokenStore) IsUserBlacklisted(ctx context.Context, subject string, issuedAt time.Time) (bool, error) {
	value, exists := s.userBlacklist.Load(subject)
	if !exists {
		return false, nil
	}

	blacklistTime := value.(time.Time)
	// If the token issuance time is earlier than the blacklist time, it is considered blacklisted
	return issuedAt.Before(blacklistTime), nil
}

// Close the connection
func (s *MemoryTokenStore) Close() error {
	close(s.stopCh)
	s.wg.Wait()
	return nil
}

// clean up expired entries regularly
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

// doCleanup performs cleanup
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

// truncateToken truncates the token prefix (for logging)
func truncateToken(token string) string {
	if len(token) <= 10 {
		return token
	}
	return token[:10] + "..."
}
