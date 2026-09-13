// 本文件实现 JWT session 的进程内存储。
// 它面向本地开发、测试和单实例运行；不会跨进程共享状态，重启后会话全部丢失。
// 维护时必须保持与 RedisSessionStore 相同的 active-only 语义，避免测试路径和生产路径行为分叉。
package jwt

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.uber.org/zap"
)

// MemorySessionStore stores JWT sessions in process memory.
type MemorySessionStore struct {
	mu       sync.Mutex
	cfg      SessionConfig
	logger   *logger.CtxZapLogger
	sessions map[string]Session
	byUser   map[string]map[string]struct{}
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewMemorySessionStore creates an in-memory JWT session store.
func NewMemorySessionStore(cfg SessionConfig, log *logger.CtxZapLogger) *MemorySessionStore {
	store := &MemorySessionStore{
		cfg:      cfg,
		logger:   log,
		sessions: make(map[string]Session),
		byUser:   make(map[string]map[string]struct{}),
		stopCh:   make(chan struct{}),
	}
	if cfg.CleanupInterval > 0 {
		store.wg.Add(1)
		go store.cleanup(cfg.CleanupInterval)
	}
	return store
}

func (s *MemorySessionStore) CreateSession(ctx context.Context, session Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.SID] = session
	if s.byUser[session.Subject] == nil {
		s.byUser[session.Subject] = make(map[string]struct{})
	}
	s.byUser[session.Subject][session.SID] = struct{}{}
	s.logger.DebugCtx(ctx, "jwt session created", zap.String("sid", session.SID), zap.String("subject", session.Subject))
	return nil
}

func (s *MemorySessionStore) GetSession(ctx context.Context, sid string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sid]
	if !ok || isSessionExpired(session, time.Now()) {
		if ok {
			s.deleteLocked(session)
		}
		return nil, ErrSessionNotFound
	}
	return cloneSession(session), nil
}

func (s *MemorySessionStore) ListSubjectSessions(ctx context.Context, subject string) ([]Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	var sessions []Session
	for sid := range s.byUser[subject] {
		session, ok := s.sessions[sid]
		if !ok || isSessionExpired(session, now) {
			s.deleteSIDLocked(subject, sid)
			continue
		}
		if session.Status != SessionStatusActive {
			s.deleteLocked(session)
			continue
		}
		sessions = append(sessions, *cloneSession(session))
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastSeenAt.After(sessions[j].LastSeenAt)
	})
	return sessions, nil
}

func (s *MemorySessionStore) RotateSessionTokens(ctx context.Context, input RotateSessionTokensInput) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[input.SID]
	if !ok || isSessionExpired(session, input.Now) {
		if ok {
			s.deleteLocked(session)
		}
		return nil, ErrSessionNotFound
	}
	if session.Status != SessionStatusActive {
		return nil, ErrSessionRevoked
	}
	if session.CurrentRefreshID != input.ExpectedRefresh {
		return nil, ErrRefreshTokenReused
	}
	session.CurrentAccessID = input.NextAccess
	session.CurrentRefreshID = input.NextRefresh
	session.ExpiresAt = input.AccessExpiresAt
	session.RefreshExpiresAt = input.RefreshExpiresAt
	session.RefreshRotatedAt = input.Now
	session.LastSeenAt = input.Now
	s.sessions[session.SID] = session
	return cloneSession(session), nil
}

func (s *MemorySessionStore) TouchSession(ctx context.Context, sid string, now time.Time, interval time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sid]
	if !ok || session.Status != SessionStatusActive {
		return nil
	}
	if interval > 0 && now.Sub(session.LastSeenAt) < interval {
		return nil
	}
	session.LastSeenAt = now
	s.sessions[sid] = session
	return nil
}

func (s *MemorySessionStore) RevokeSession(ctx context.Context, sid string, reason string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sid]
	if !ok {
		return ErrSessionNotFound
	}
	s.deleteLocked(session)
	s.logger.InfoCtx(ctx, "jwt session deleted", zap.String("sid", sid), zap.String("reason", reason))
	return nil
}

func (s *MemorySessionStore) RevokeSubjectSessions(ctx context.Context, subject string, reason string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for sid := range s.byUser[subject] {
		session, ok := s.sessions[sid]
		if !ok {
			continue
		}
		s.deleteLocked(session)
	}
	delete(s.byUser, subject)
	s.logger.InfoCtx(ctx, "jwt subject sessions deleted", zap.String("subject", subject), zap.String("reason", reason))
	return nil
}

func (s *MemorySessionStore) DeleteExpired(ctx context.Context, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, session := range s.sessions {
		if isSessionExpired(session, now) {
			s.deleteLocked(session)
		}
	}
	return nil
}

func (s *MemorySessionStore) Close() error {
	close(s.stopCh)
	s.wg.Wait()
	return nil
}

func (s *MemorySessionStore) cleanup(interval time.Duration) {
	defer s.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			_ = s.DeleteExpired(context.Background(), time.Now())
		case <-s.stopCh:
			return
		}
	}
}

func (s *MemorySessionStore) deleteLocked(session Session) {
	delete(s.sessions, session.SID)
	s.deleteSIDLocked(session.Subject, session.SID)
}

func (s *MemorySessionStore) deleteSIDLocked(subject string, sid string) {
	if sessions := s.byUser[subject]; sessions != nil {
		delete(sessions, sid)
		if len(sessions) == 0 {
			delete(s.byUser, subject)
		}
	}
}

func isSessionExpired(session Session, now time.Time) bool {
	return !session.RefreshExpiresAt.IsZero() && now.After(session.RefreshExpiresAt)
}

func cloneSession(session Session) *Session {
	copied := session
	if session.ClaimsVersion != nil {
		copied.ClaimsVersion = make(map[string]int64, len(session.ClaimsVersion))
		for key, value := range session.ClaimsVersion {
			copied.ClaimsVersion[key] = value
		}
	}
	if session.Claims != nil {
		copied.Claims = cloneClaimsMap(session.Claims)
	}
	return &copied
}

func validateSession(session Session) error {
	if session.SID == "" {
		return fmt.Errorf("jwt: session sid is required")
	}
	if session.Subject == "" {
		return fmt.Errorf("jwt: session subject is required")
	}
	return nil
}
