package jwt

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTokenManager_RefreshToken_FullFlow Test the complete RefreshToken flow
func TestTokenManager_RefreshToken_FullFlow(t *testing.T) {
	config := newTestConfig()
	config.RefreshToken.Enabled = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	subject := "user123"

	// Generate Refresh Token
	refreshToken, err := manager.GenerateRefreshToken(ctx, subject)
	require.NoError(t, err)
	assert.NotEmpty(t, refreshToken)

	// Verify Refresh Token
	refreshClaims, err := manager.VerifyToken(ctx, refreshToken)
	require.NoError(t, err)
	assert.Equal(t, subject, refreshClaims.Subject)
	assert.Equal(t, "refresh", refreshClaims.TokenType)

	// 3. Use Refresh Token to obtain new Access Token
	newAccessToken, err := manager.RefreshToken(ctx, refreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, newAccessToken)

	// 4. Verify new Access Token
	accessClaims, err := manager.VerifyToken(ctx, newAccessToken)
	require.NoError(t, err)
	assert.Equal(t, subject, accessClaims.Subject)
	assert.Equal(t, "access", accessClaims.TokenType)
}

// TestTokenManager_RefreshToken_WithCustomClaims Test RefreshToken with custom claims
func TestTokenManager_RefreshToken_WithCustomClaims(t *testing.T) {
	config := newTestConfig()
	config.RefreshToken.Enabled = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	subject := "user123"

	// Generate an Access Token with custom Claims
	customClaims := map[string]interface{}{
		"user_id":   int64(123),
		"username":  "testuser",
		"roles":     []string{"admin"},
		"tenant_id": "tenant-001",
	}
	accessToken, err := manager.GenerateAccessToken(ctx, subject, customClaims)
	require.NoError(t, err)

	// Verify Access Token
	accessClaims, err := manager.VerifyToken(ctx, accessToken)
	require.NoError(t, err)
	assert.Equal(t, int64(123), accessClaims.UserID)
	assert.Equal(t, "testuser", accessClaims.Username)
	assert.Equal(t, []string{"admin"}, accessClaims.Roles)
	assert.Equal(t, "tenant-001", accessClaims.TenantID)

	// Generate Refresh Token
	refreshToken, err := manager.GenerateRefreshToken(ctx, subject)
	require.NoError(t, err)

	// Use Refresh Token to obtain new Access Token
	// Note: In actual applications, Refresh Tokens should store custom claims
	// Here we verify that the RefreshToken method correctly handles
	newAccessToken, err := manager.RefreshToken(ctx, refreshToken)
	require.NoError(t, err)

	// Verify the new Access Token (custom claims will not be automatically inherited)
	newAccessClaims, err := manager.VerifyToken(ctx, newAccessToken)
	require.NoError(t, err)
	assert.Equal(t, subject, newAccessClaims.Subject)
	assert.Equal(t, "access", newAccessClaims.TokenType)
}

// TestTokenManager_RefreshToken_BlacklistedRefreshToken Test Revoked Refresh Token
func TestTokenManager_RefreshToken_BlacklistedRefreshToken(t *testing.T) {
	config := newTestConfig()
	config.RefreshToken.Enabled = true
	config.Blacklist.Enabled = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	subject := "user123"

	// Generate Refresh Token
	refreshToken, err := manager.GenerateRefreshToken(ctx, subject)
	require.NoError(t, err)

	// Revoke Refresh Token
	err = manager.RevokeToken(ctx, refreshToken)
	require.NoError(t, err)

	// Try to use revoked Refresh Token
	newAccessToken, err := manager.RefreshToken(ctx, refreshToken)
	assert.Error(t, err)
	assert.Empty(t, newAccessToken)
	assert.Contains(t, err.Error(), "invalid refresh token")
}

// TestTokenManager_RefreshToken_ExpiredRefreshToken test expired Refresh Token
func TestTokenManager_RefreshToken_ExpiredRefreshToken(t *testing.T) {
	config := newTestConfig()
	config.RefreshToken.Enabled = true
	config.RefreshToken.TTL = 10 * time.Millisecond // Very short TTL
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	subject := "user123"

	// Generate Refresh Token
	refreshToken, err := manager.GenerateRefreshToken(ctx, subject)
	require.NoError(t, err)

	// waiting for token to expire
	time.Sleep(20 * time.Millisecond)

	// Try to use an expired Refresh Token
	newAccessToken, err := manager.RefreshToken(ctx, refreshToken)
	assert.Error(t, err)
	assert.Empty(t, newAccessToken)
}

