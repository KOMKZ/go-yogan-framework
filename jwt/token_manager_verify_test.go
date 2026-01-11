package jwt

import (
	"context"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTokenManager_VerifyToken_BlacklistCheckError 测试黑名单检查错误
func TestTokenManager_VerifyToken_BlacklistCheckError(t *testing.T) {
	config := newTestConfig()
	config.Blacklist.Enabled = true

	log := logger.NewCtxZapLogger("yogan")

	// 创建一个会失败的 TokenStore
	failingStore := &failingTokenStore{}
	manager, err := NewTokenManager(config, failingStore, log)
	require.NoError(t, err)

	ctx := context.Background()
	token, err := manager.GenerateAccessToken(ctx, "user123", nil)
	require.NoError(t, err)

	// 验证 Token（黑名单检查会失败）
	claims, err := manager.VerifyToken(ctx, token)
	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.Contains(t, err.Error(), "check blacklist failed")
}

// TestTokenManager_VerifyToken_UserBlacklistCheckError 测试用户黑名单检查错误
func TestTokenManager_VerifyToken_UserBlacklistCheckError(t *testing.T) {
	config := newTestConfig()
	config.Blacklist.Enabled = true

	log := logger.NewCtxZapLogger("yogan")

	// 创建一个会在用户黑名单检查时失败的 TokenStore
	failingStore := &failingUserBlacklistStore{}
	manager, err := NewTokenManager(config, failingStore, log)
	require.NoError(t, err)

	ctx := context.Background()
	token, err := manager.GenerateAccessToken(ctx, "user123", nil)
	require.NoError(t, err)

	// 验证 Token（用户黑名单检查会失败）
	claims, err := manager.VerifyToken(ctx, token)
	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.Contains(t, err.Error(), "check user blacklist failed")
}

// failingTokenStore 模拟失败的 TokenStore
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

// failingUserBlacklistStore 模拟用户黑名单检查失败的 TokenStore
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

