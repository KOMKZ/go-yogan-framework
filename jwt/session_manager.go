// 本文件实现 JWT session 的框架级业务编排。
// 它负责创建会话、限制同一主体并发登录、刷新 token 轮换、单会话下线和全端下线。
// 消费方不应复制这些策略；业务层只在禁用账号、重置凭证或权限变更时调用公开撤销方法。
package jwt

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// SessionManager coordinates SessionStore operations for JWT authentication.
type SessionManager struct {
	cfg   SessionConfig
	store SessionStore
}

// NewSessionManager creates a session manager.
func NewSessionManager(cfg SessionConfig, store SessionStore) *SessionManager {
	return &SessionManager{cfg: cfg, store: store}
}

func (m *SessionManager) Create(ctx context.Context, input IssueTokenInput, accessTTL time.Duration, refreshTTL time.Duration, now time.Time) (Session, error) {
	maxSessions := input.MaxSessions
	if maxSessions <= 0 {
		maxSessions = m.cfg.MaxSessionsPerSubject
	}
	session := Session{
		SID:              newSessionID("s"),
		Subject:          input.Subject,
		Status:           SessionStatusActive,
		CreatedAt:        now,
		LastSeenAt:       now,
		ExpiresAt:        now.Add(accessTTL),
		RefreshExpiresAt: now.Add(refreshTTL),
		CurrentAccessID:  newSessionID("a"),
		CurrentRefreshID: newSessionID("r"),
		RefreshRotatedAt: now,
		Client:           input.Client,
		Claims:           compactSessionClaims(input.Claims),
		ClaimsVersion:    cloneVersionMap(input.ClaimsVersion),
	}
	if err := m.store.CreateSession(ctx, session); err != nil {
		return Session{}, err
	}
	if maxSessions > 0 {
		if err := m.enforceMaxSessions(ctx, input.Subject, maxSessions, session.SID, now); err != nil {
			return Session{}, err
		}
	}
	return session, nil
}

func (m *SessionManager) Verify(ctx context.Context, claims *Claims, now time.Time) error {
	if claims == nil || claims.SID == "" {
		return ErrSessionRequired
	}
	session, err := m.store.GetSession(ctx, claims.SID)
	if err != nil {
		return err
	}
	if session.Status != SessionStatusActive {
		return ErrSessionRevoked
	}
	if claims.TokenType == "access" && session.CurrentAccessID != "" && claims.ATI != session.CurrentAccessID {
		return ErrTokenInvalid
	}
	if claims.TokenType == "refresh" && claims.RTI != session.CurrentRefreshID {
		return ErrRefreshTokenReused
	}
	return m.store.TouchSession(ctx, claims.SID, now, m.cfg.LastSeenUpdateInterval)
}

func (m *SessionManager) Rotate(ctx context.Context, claims *Claims, now time.Time, accessTTL time.Duration, refreshTTL time.Duration) (Session, error) {
	nextAccessID := newSessionID("a")
	nextRefreshID := newSessionID("r")
	session, err := m.store.RotateSessionTokens(ctx, RotateSessionTokensInput{
		SID:              claims.SID,
		ExpectedRefresh:  claims.RTI,
		NextAccess:       nextAccessID,
		NextRefresh:      nextRefreshID,
		AccessExpiresAt:  now.Add(accessTTL),
		RefreshExpiresAt: now.Add(refreshTTL),
		Now:              now,
	})
	if err == ErrRefreshTokenReused && m.cfg.RefreshReusePolicy == SessionRefreshReuseRevokeSession {
		_ = m.store.RevokeSession(ctx, claims.SID, "refresh_reuse", now)
	}
	if err != nil {
		return Session{}, err
	}
	return *session, nil
}

func (m *SessionManager) RevokeSession(ctx context.Context, sid string, reason string) error {
	return m.store.RevokeSession(ctx, sid, reason, time.Now())
}

func (m *SessionManager) RevokeSubjectSessions(ctx context.Context, subject string, reason string) error {
	return m.store.RevokeSubjectSessions(ctx, subject, reason, time.Now())
}

func (m *SessionManager) ListSubjectSessions(ctx context.Context, subject string) ([]Session, error) {
	return m.store.ListSubjectSessions(ctx, subject)
}

func (m *SessionManager) enforceMaxSessions(ctx context.Context, subject string, max int, keepSID string, now time.Time) error {
	sessions, err := m.store.ListSubjectSessions(ctx, subject)
	if err != nil {
		return err
	}
	active := make([]Session, 0, len(sessions))
	for _, session := range sessions {
		if session.Status == SessionStatusActive {
			active = append(active, session)
		}
	}
	for i := max; i < len(active); i++ {
		if active[i].SID == keepSID {
			continue
		}
		if err := m.store.RevokeSession(ctx, active[i].SID, "max_sessions_exceeded", now); err != nil && err != ErrSessionNotFound {
			return err
		}
	}
	return nil
}

func newSessionID(prefix string) string {
	return prefix + "_" + uuid.NewString()
}

func cloneVersionMap(values map[string]int64) map[string]int64 {
	if len(values) == 0 {
		return nil
	}
	copied := make(map[string]int64, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return copied
}

func cloneClaimsMap(values map[string]interface{}) map[string]interface{} {
	if len(values) == 0 {
		return nil
	}
	copied := make(map[string]interface{}, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return copied
}

func compactSessionClaims(values map[string]interface{}) map[string]interface{} {
	if len(values) == 0 {
		return nil
	}
	allowed := map[string]struct{}{
		"user_id":  {},
		"username": {},
		"email":    {},
	}
	copied := make(map[string]interface{}, len(allowed))
	for key, value := range values {
		if _, ok := allowed[key]; ok {
			copied[key] = value
		}
	}
	if len(copied) == 0 {
		return nil
	}
	return copied
}
