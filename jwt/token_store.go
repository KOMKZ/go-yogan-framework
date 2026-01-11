package jwt

import (
	"context"
	"time"
)

// TokenStore Token 存储接口（黑名单）
type TokenStore interface {
	// IsBlacklisted 检查 Token 是否在黑名单
	IsBlacklisted(ctx context.Context, token string) (bool, error)

	// AddToBlacklist 添加到黑名单（ttl 为剩余过期时间）
	AddToBlacklist(ctx context.Context, token string, ttl time.Duration) error

	// RemoveFromBlacklist 从黑名单移除（仅测试使用）
	RemoveFromBlacklist(ctx context.Context, token string) error

	// BlacklistUserTokens 添加用户所有 Token 到黑名单（登出所有设备）
	BlacklistUserTokens(ctx context.Context, subject string, ttl time.Duration) error

	// IsUserBlacklisted 检查用户是否被全局拉黑
	IsUserBlacklisted(ctx context.Context, subject string, issuedAt time.Time) (bool, error)

	// Close 关闭连接
	Close() error
}
