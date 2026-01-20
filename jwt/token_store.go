package jwt

import (
	"context"
	"time"
)

// Token storage interface (blacklist)
type TokenStore interface {
	// Check if Token is in blacklist
	IsBlacklisted(ctx context.Context, token string) (bool, error)

	// AddToBlacklist Add to blacklist (TTL is remaining expiration time)
	AddToBlacklist(ctx context.Context, token string, ttl time.Duration) error

	// RemoveFromBlacklist Remove from blacklist (for testing only)
	RemoveFromBlacklist(ctx context.Context, token string) error

	// Add all user tokens to the blacklist (logout from all devices)
	BlacklistUserTokens(ctx context.Context, subject string, ttl time.Duration) error

	// Check if the user is globally blacklisted
	IsUserBlacklisted(ctx context.Context, subject string, issuedAt time.Time) (bool, error)

	// Close connection
	Close() error
}
