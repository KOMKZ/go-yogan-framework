// 本文件定义 JWT 服务端会话的公共契约。
// 消费方只依赖 IssueTokenInput、TokenPair 和 Session 信息，不依赖 Redis 或 memory 的内部存储结构。
// 维护时新增会话字段要同步更新 Redis/memory store、TokenManager 签发校验和框架文档。
package jwt

import "time"

const (
	SessionStatusActive  = "active"
	SessionStatusRevoked = "revoked"
)

const (
	SessionRefreshReuseReject        = "reject"
	SessionRefreshReuseRevokeSession = "revoke_session"
)

// ClientInfo describes the device or client that owns a session.
type ClientInfo struct {
	IP         string `json:"ip"`
	UserAgent  string `json:"user_agent"`
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
}

// Session is the server-side state behind JWT sid/ati/rti claims.
type Session struct {
	SID                    string                 `json:"sid"`
	Subject                string                 `json:"subject"`
	Status                 string                 `json:"status"`
	CreatedAt              time.Time              `json:"created_at"`
	LastSeenAt             time.Time              `json:"last_seen_at"`
	ExpiresAt              time.Time              `json:"expires_at"`
	RefreshExpiresAt       time.Time              `json:"refresh_expires_at"`
	CurrentAccessID        string                 `json:"current_access_id"`
	CurrentRefreshID       string                 `json:"current_refresh_id"`
	RefreshRotatedAt       time.Time              `json:"refresh_rotated_at"`
	RefreshReuseDetectedAt time.Time              `json:"refresh_reuse_detected_at,omitempty"`
	Client                 ClientInfo             `json:"client"`
	Claims                 map[string]interface{} `json:"claims,omitempty"`
	ClaimsVersion          map[string]int64       `json:"claims_version,omitempty"`
	RevokedAt              time.Time              `json:"revoked_at,omitempty"`
	RevokedReason          string                 `json:"revoked_reason,omitempty"`
}

// IssueTokenInput carries application data for a new server-side JWT session.
type IssueTokenInput struct {
	Subject       string
	Claims        map[string]interface{}
	Client        ClientInfo
	ClaimsVersion map[string]int64
	MaxSessions   int
}

// RefreshTokenInput carries request metadata for refresh-token rotation.
type RefreshTokenInput struct {
	RefreshToken string
	Client       ClientInfo
}

// TokenPair is returned when a session issues or rotates JWT credentials.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	Session      Session
	ExpiresIn    int64
	RefreshAfter int64
}

// SessionFilter selects sessions for list operations.
type SessionFilter struct {
	Subject string
}
