package jwt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTokenManager_RefreshToken_AllCustomClaims test refresh with all custom claims
func TestTokenManager_RefreshToken_AllCustomClaims(t *testing.T) {
	config := newTestConfig()
	config.RefreshToken.Enabled = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	subject := "user123"

	// Generate an Access Token with all custom Claims
	customClaims := map[string]interface{}{
		"user_id":   int64(123),
		"username":  "testuser",
		"roles":     []string{"admin", "user"},
		"tenant_id": "tenant-001",
	}
	accessToken, err := manager.GenerateAccessToken(ctx, subject, customClaims)
	require.NoError(t, err)

	// Verify Access Token
	accessClaims, err := manager.VerifyToken(ctx, accessToken)
	require.NoError(t, err)
	assert.Equal(t, int64(123), accessClaims.UserID)
	assert.Equal(t, "testuser", accessClaims.Username)
	assert.Equal(t, []string{"admin", "user"}, accessClaims.Roles)
	assert.Equal(t, "tenant-001", accessClaims.TenantID)

	// Generate Refresh Token
	refreshToken, err := manager.GenerateRefreshToken(ctx, subject)
	require.NoError(t, err)

	// Use Refresh Token to obtain new Access Token
	newAccessToken, err := manager.RefreshToken(ctx, refreshToken)
	require.NoError(t, err)

	// Verify new Access Token
	newAccessClaims, err := manager.VerifyToken(ctx, newAccessToken)
	require.NoError(t, err)
	assert.Equal(t, subject, newAccessClaims.Subject)
	assert.Equal(t, "access", newAccessClaims.TokenType)
}

// TestTokenManager_RefreshToken_OnlyUsername test refresh with username only
func TestTokenManager_RefreshToken_OnlyUsername(t *testing.T) {
	config := newTestConfig()
	config.RefreshToken.Enabled = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	subject := "user123"

	// Generate Access Token with username
	customClaims := map[string]interface{}{
		"username": "testuser",
	}
	_, err := manager.GenerateAccessToken(ctx, subject, customClaims)
	require.NoError(t, err)

	// Generate Refresh Token
	refreshToken, err := manager.GenerateRefreshToken(ctx, subject)
	require.NoError(t, err)

	// Use Refresh Token to obtain new Access Token
	newAccessToken, err := manager.RefreshToken(ctx, refreshToken)
	require.NoError(t, err)

	// Validate new Access Token
	newAccessClaims, err := manager.VerifyToken(ctx, newAccessToken)
	require.NoError(t, err)
	assert.Equal(t, subject, newAccessClaims.Subject)
}

// TestTokenManager_RefreshToken_OnlyRoles test refresh with only roles
func TestTokenManager_RefreshToken_OnlyRoles(t *testing.T) {
	config := newTestConfig()
	config.RefreshToken.Enabled = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	subject := "user123"

	// Generate Access Token with roles
	customClaims := map[string]interface{}{
		"roles": []string{"admin"},
	}
	_, err := manager.GenerateAccessToken(ctx, subject, customClaims)
	require.NoError(t, err)

	// Generate Refresh Token
	refreshToken, err := manager.GenerateRefreshToken(ctx, subject)
	require.NoError(t, err)

	// Use Refresh Token to obtain new Access Token
	newAccessToken, err := manager.RefreshToken(ctx, refreshToken)
	require.NoError(t, err)

	// Verify new Access Token
	newAccessClaims, err := manager.VerifyToken(ctx, newAccessToken)
	require.NoError(t, err)
	assert.Equal(t, subject, newAccessClaims.Subject)
}

// TestTokenManager_RefreshToken_OnlyTenantID Test refresh with only tenant_id
func TestTokenManager_RefreshToken_OnlyTenantID(t *testing.T) {
	config := newTestConfig()
	config.RefreshToken.Enabled = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	subject := "user123"

	// Generate Access Token with tenant_id
	customClaims := map[string]interface{}{
		"tenant_id": "tenant-001",
	}
	_, err := manager.GenerateAccessToken(ctx, subject, customClaims)
	require.NoError(t, err)

	// Generate Refresh Token
	refreshToken, err := manager.GenerateRefreshToken(ctx, subject)
	require.NoError(t, err)

	// Use Refresh Token to obtain new Access Token
	newAccessToken, err := manager.RefreshToken(ctx, refreshToken)
	require.NoError(t, err)

	// Validate new Access Token
	newAccessClaims, err := manager.VerifyToken(ctx, newAccessToken)
	require.NoError(t, err)
	assert.Equal(t, subject, newAccessClaims.Subject)
}

