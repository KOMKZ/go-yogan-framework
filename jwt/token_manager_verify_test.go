package jwt

import (
	"context"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTokenManager_VerifyToken_BlacklistCheckError Test blacklist check error
func TestTokenManager_VerifyToken_BlacklistCheckError(t *testing.T) {
	config := newTestConfig()
	config.Blacklist.Enabled = true

	log := logger.NewCtxZapLogger("yogan")

	// Create a TokenStore that will fail
	failingStore := &failingTokenStore{}
	manager, err := NewTokenManager(config, failingStore, log)
	require.NoError(t, err)

	ctx := context.Background()
	token, err := manager.GenerateAccessToken(ctx, "user123", nil)
	require.NoError(t, err)

	// Validate Token (blacklist check will fail)
	claims, err := manager.VerifyToken(ctx, token)
	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.Contains(t, err.Error(), "check blacklist failed")
}

// TestTokenManager_VerifyToken_UserBlacklistCheckError User blacklist check error during token verification test
func TestTokenManager_VerifyToken_UserBlacklistCheckError(t *testing.T) {
	config := newTestConfig()
	config.Blacklist.Enabled = true

	log := logger.NewCtxZapLogger("yogan")

	// Create a TokenStore that will fail during user blacklist check
	failingStore := &failingUserBlacklistStore{}
	manager, err := NewTokenManager(config, failingStore, log)
	require.NoError(t, err)

	ctx := context.Background()
	token, err := manager.GenerateAccessToken(ctx, "user123", nil)
	require.NoError(t, err)

	// Verify Token (blacklist check for user will fail)
	claims, err := manager.VerifyToken(ctx, token)
	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.Contains(t, err.Error(), "check user blacklist failed")
}

// failingTokenStore simulates a failed TokenStore
type failingTokenStore struct{}

func (f *failingTokenStore) IsBlacklisted(ctx context.Context, token string) (bool, error) {
	return false, assert.AnError
}

func (f *failingTokenStore) AddToBlacklist(ctx context.Context, token string, ttl time.Duration) error {
	return assert.AnError
}

func (f *failingTokenStore) RemoveFromBlacklist(ctx context.Context, token string) error {
	return assert.AnError
}

func (f *failingTokenStore) BlacklistUserTokens(ctx context.Context, subject string, ttl time.Duration) error {
	return assert.AnError
}

func (f *failingTokenStore) IsUserBlacklisted(ctx context.Context, subject string, issuedAt time.Time) (bool, error) {
	return false, nil
}

func (f *failingTokenStore) Close() error {
	return nil
}

// failingUserBlacklistStore simulates a failed token store for user blacklist checks
type failingUserBlacklistStore struct{}

func (f *failingUserBlacklistStore) IsBlacklisted(ctx context.Context, token string) (bool, error) {
	return false, nil
}

func (f *failingUserBlacklistStore) AddToBlacklist(ctx context.Context, token string, ttl time.Duration) error {
	return nil
}

func (f *failingUserBlacklistStore) RemoveFromBlacklist(ctx context.Context, token string) error {
	return nil
}

func (f *failingUserBlacklistStore) BlacklistUserTokens(ctx context.Context, subject string, ttl time.Duration) error {
	return nil
}

func (f *failingUserBlacklistStore) IsUserBlacklisted(ctx context.Context, subject string, issuedAt time.Time) (bool, error) {
	return false, assert.AnError
}

func (f *failingUserBlacklistStore) Close() error {
	return nil
}

