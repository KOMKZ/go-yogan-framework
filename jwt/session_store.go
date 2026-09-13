// 本文件定义 JWT session store 的持久化契约。
// TokenManager 和 SessionManager 只依赖这些语义化方法；Redis、memory 等实现必须保持相同行为。
// Store 不负责 JWT 签名和业务权限判断，只负责会话状态、索引、过期和原子轮换。
package jwt

import (
	"context"
	"time"
)

// SessionStore persists server-side JWT sessions.
type SessionStore interface {
	CreateSession(ctx context.Context, session Session) error
	GetSession(ctx context.Context, sid string) (*Session, error)
	ListSubjectSessions(ctx context.Context, subject string) ([]Session, error)
	RotateSessionTokens(ctx context.Context, input RotateSessionTokensInput) (*Session, error)
	TouchSession(ctx context.Context, sid string, now time.Time, interval time.Duration) error
	RevokeSession(ctx context.Context, sid string, reason string, now time.Time) error
	RevokeSubjectSessions(ctx context.Context, subject string, reason string, now time.Time) error
	DeleteExpired(ctx context.Context, now time.Time) error
	Close() error
}

// RotateSessionTokensInput updates the current access/refresh ids for a session.
type RotateSessionTokensInput struct {
	SID              string
	ExpectedRefresh  string
	NextAccess       string
	NextRefresh      string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
	Now              time.Time
}
