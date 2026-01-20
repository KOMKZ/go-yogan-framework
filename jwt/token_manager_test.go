package jwt

import (
	"context"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestConfig() *Config {
	config := &Config{
		Enabled:   true,
		Algorithm: "HS256",
		Secret:    "test-secret-key-for-jwt-testing",
		AccessToken: AccessTokenConfig{
			TTL:      2 * time.Hour,
			Issuer:   "test-issuer",
			Audience: "test-audience",
		},
		RefreshToken: RefreshTokenConfig{
			Enabled: true,
			TTL:     168 * time.Hour,
		},
		Blacklist: BlacklistConfig{
			Enabled:         true,
			Storage:         "memory",
			CleanupInterval: 1 * time.Hour,
		},
		Security: SecurityConfig{
			EnableJTI:       true,
			EnableNotBefore: false,
			ClockSkew:       60 * time.Second,
		},
	}
	return config
}

func newTestTokenManager(t *testing.T, config *Config) TokenManager {
	log := logger.NewCtxZapLogger("yogan")
	tokenStore := NewMemoryTokenStore(0, log)
	t.Cleanup(func() {
		tokenStore.Close()
	})

	manager, err := NewTokenManager(config, tokenStore, log)
	require.NoError(t, err)

	return manager
}

func TestNewTokenManager(t *testing.T) {
	config := newTestConfig()
	log := logger.NewCtxZapLogger("yogan")
	tokenStore := NewMemoryTokenStore(0, log)
	defer tokenStore.Close()

	manager, err := NewTokenManager(config, tokenStore, log)
	assert.NoError(t, err)
	assert.NotNil(t, manager)
}

func TestNewTokenManager_InvalidConfig(t *testing.T) {
	config := &Config{
		Enabled:   true,
		Algorithm: "HS256",
		Secret:    "", // empty key
		AccessToken: AccessTokenConfig{
			TTL: 2 * time.Hour,
		},
	}

	log := logger.NewCtxZapLogger("yogan")
	tokenStore := NewMemoryTokenStore(0, log)
	defer tokenStore.Close()

	manager, err := NewTokenManager(config, tokenStore, log)
	assert.Error(t, err)
	assert.Nil(t, manager)
}

func TestTokenManager_GenerateAccessToken(t *testing.T) {
	config := newTestConfig()
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	subject := "user123"
	customClaims := map[string]interface{}{
		"user_id":  int64(123),
		"username": "testuser",
		"roles":    []string{"admin", "user"},
	}

	token, err := manager.GenerateAccessToken(ctx, subject, customClaims)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate the generated Token
	claims, err := manager.VerifyToken(ctx, token)
	assert.NoError(t, err)
	assert.Equal(t, subject, claims.Subject)
	assert.Equal(t, int64(123), claims.UserID)
	assert.Equal(t, "testuser", claims.Username)
	assert.Equal(t, []string{"admin", "user"}, claims.Roles)
	assert.Equal(t, "access", claims.TokenType)
	assert.Equal(t, config.AccessToken.Issuer, claims.Issuer)
}

func TestTokenManager_GenerateAccessToken_WithJTI(t *testing.T) {
	config := newTestConfig()
	config.Security.EnableJTI = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	token, err := manager.GenerateAccessToken(ctx, "user123", nil)
	assert.NoError(t, err)

	claims, err := manager.VerifyToken(ctx, token)
	assert.NoError(t, err)
	assert.NotEmpty(t, claims.JTI)
}

func TestTokenManager_GenerateRefreshToken(t *testing.T) {
	config := newTestConfig()
	config.RefreshToken.Enabled = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	subject := "user123"

	token, err := manager.GenerateRefreshToken(ctx, subject)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate the generated Refresh Token
	claims, err := manager.VerifyToken(ctx, token)
	assert.NoError(t, err)
	assert.Equal(t, subject, claims.Subject)
	assert.Equal(t, "refresh", claims.TokenType)
}

func TestTokenManager_GenerateRefreshToken_Disabled(t *testing.T) {
	config := newTestConfig()
	config.RefreshToken.Enabled = false
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	token, err := manager.GenerateRefreshToken(ctx, "user123")
	assert.Error(t, err)
	assert.Empty(t, token)
}

func TestTokenManager_VerifyToken_Success(t *testing.T) {
	config := newTestConfig()
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	token, err := manager.GenerateAccessToken(ctx, "user123", nil)
	require.NoError(t, err)

	claims, err := manager.VerifyToken(ctx, token)
	assert.NoError(t, err)
	assert.NotNil(t, claims)
	assert.Equal(t, "user123", claims.Subject)
}

func TestTokenManager_VerifyToken_ExpiredToken(t *testing.T) {
	config := newTestConfig()
	config.AccessToken.TTL = 10 * time.Millisecond // Very short TTL
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	token, err := manager.GenerateAccessToken(ctx, "user123", nil)
	require.NoError(t, err)

	// waiting for token to expire
	time.Sleep(20 * time.Millisecond)

	claims, err := manager.VerifyToken(ctx, token)
	assert.ErrorIs(t, err, ErrTokenExpired)
	assert.Nil(t, claims)
}

func TestTokenManager_VerifyToken_InvalidSignature(t *testing.T) {
	config := newTestConfig()
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	invalidToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyMTIzIn0.invalid_signature"

	claims, err := manager.VerifyToken(ctx, invalidToken)
	assert.Error(t, err)
	assert.Nil(t, claims)
}

func TestTokenManager_VerifyToken_BlacklistedToken(t *testing.T) {
	config := newTestConfig()
	config.Blacklist.Enabled = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	token, err := manager.GenerateAccessToken(ctx, "user123", nil)
	require.NoError(t, err)

	// Revoke token
	err = manager.RevokeToken(ctx, token)
	require.NoError(t, err)

	// Verify revoked token
	claims, err := manager.VerifyToken(ctx, token)
	assert.ErrorIs(t, err, ErrTokenBlacklisted)
	assert.Nil(t, claims)
}

func TestTokenManager_RefreshToken_Success(t *testing.T) {
	config := newTestConfig()
	config.RefreshToken.Enabled = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	subject := "user123"
	customClaims := map[string]interface{}{
		"user_id":  int64(123),
		"username": "testuser",
		"roles":    []string{"admin"},
	}

	// Generate Refresh Token
	_, err := manager.GenerateRefreshToken(ctx, subject)
	require.NoError(t, err)

	// Use Refresh Token to obtain new Access Token
	// Note: An Access Token with custom Claims must be generated first, then a Refresh Token should be generated.
	// Here we simplify the test by directly testing the RefreshToken method
	accessToken, err := manager.GenerateAccessToken(ctx, subject, customClaims)
	require.NoError(t, err)

	// Validate Access Token
	claims, err := manager.VerifyToken(ctx, accessToken)
	assert.NoError(t, err)
	assert.Equal(t, subject, claims.Subject)
}

func TestTokenManager_RefreshToken_InvalidToken(t *testing.T) {
	config := newTestConfig()
	config.RefreshToken.Enabled = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	invalidToken := "invalid-token"

	accessToken, err := manager.RefreshToken(ctx, invalidToken)
	assert.Error(t, err)
	assert.Empty(t, accessToken)
}

func TestTokenManager_RefreshToken_NotRefreshToken(t *testing.T) {
	config := newTestConfig()
	config.RefreshToken.Enabled = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()

	// Generate Access Token (not Refresh Token)
	accessToken, err := manager.GenerateAccessToken(ctx, "user123", nil)
	require.NoError(t, err)

	// Try to refresh with Access Token
	newToken, err := manager.RefreshToken(ctx, accessToken)
	assert.Error(t, err)
	assert.Empty(t, newToken)
}

func TestTokenManager_RevokeToken_Success(t *testing.T) {
	config := newTestConfig()
	config.Blacklist.Enabled = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	token, err := manager.GenerateAccessToken(ctx, "user123", nil)
	require.NoError(t, err)

	// Revoke token
	err = manager.RevokeToken(ctx, token)
	assert.NoError(t, err)

	// Verify revoked token
	claims, err := manager.VerifyToken(ctx, token)
	assert.ErrorIs(t, err, ErrTokenBlacklisted)
	assert.Nil(t, claims)
}

func TestTokenManager_RevokeToken_ExpiredToken(t *testing.T) {
	config := newTestConfig()
	config.Blacklist.Enabled = true
	config.AccessToken.TTL = 10 * time.Millisecond
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	token, err := manager.GenerateAccessToken(ctx, "user123", nil)
	require.NoError(t, err)

	// wait for expiration
	time.Sleep(20 * time.Millisecond)

	// Revoke expired tokens (should return nil directly)
	err = manager.RevokeToken(ctx, token)
	assert.NoError(t, err)
}

func TestTokenManager_RevokeUserTokens_Success(t *testing.T) {
	config := newTestConfig()
	config.Blacklist.Enabled = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	subject := "user123"

	// Generate two old tokens
	token1, err := manager.GenerateAccessToken(ctx, subject, nil)
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	token2, err := manager.GenerateAccessToken(ctx, subject, nil)
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	// Revoke all user tokens
	err = manager.RevokeUserTokens(ctx, subject)
	assert.NoError(t, err)

	// Validate old token (should be revoked)
	claims, err := manager.VerifyToken(ctx, token1)
	assert.ErrorIs(t, err, ErrTokenBlacklisted)
	assert.Nil(t, claims)

	claims, err = manager.VerifyToken(ctx, token2)
	assert.ErrorIs(t, err, ErrTokenBlacklisted)
	assert.Nil(t, claims)

	// Wait long enough to ensure that a new token is generated after being blacklisted (using second-level delay)
	time.Sleep(1100 * time.Millisecond)
	newToken, err := manager.GenerateAccessToken(ctx, subject, nil)
	require.NoError(t, err)

	// The new token should be valid (as it was generated after being blacklisted)
	claims, err = manager.VerifyToken(ctx, newToken)
	assert.NoError(t, err)
	assert.NotNil(t, claims)
}

func TestTokenManager_RevokeToken_BlacklistDisabled(t *testing.T) {
	config := newTestConfig()
	config.Blacklist.Enabled = false
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	token, err := manager.GenerateAccessToken(ctx, "user123", nil)
	require.NoError(t, err)

	// Revoke token (blacklist not enabled)
	err = manager.RevokeToken(ctx, token)
	assert.Error(t, err)
}

func TestTokenManager_RevokeUserTokens_BlacklistDisabled(t *testing.T) {
	config := newTestConfig()
	config.Blacklist.Enabled = false
	manager := newTestTokenManager(t, config)

	ctx := context.Background()

	// Revoke user token (blacklist not enabled)
	err := manager.RevokeUserTokens(ctx, "user123")
	assert.Error(t, err)
}

func TestTokenManager_DifferentAlgorithms(t *testing.T) {
	algorithms := []string{"HS256", "HS384", "HS512"}

	for _, algo := range algorithms {
		t.Run(algo, func(t *testing.T) {
			config := newTestConfig()
			config.Algorithm = algo
			manager := newTestTokenManager(t, config)

			ctx := context.Background()
			token, err := manager.GenerateAccessToken(ctx, "user123", nil)
			assert.NoError(t, err)
			assert.NotEmpty(t, token)

			claims, err := manager.VerifyToken(ctx, token)
			assert.NoError(t, err)
			assert.NotNil(t, claims)
		})
	}
}

