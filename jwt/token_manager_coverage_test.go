package jwt

import (
	"context"
	"testing"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test for unsupported algorithms
func TestTokenManager_UnsupportedAlgorithm(t *testing.T) {
	config := newTestConfig()
	config.Algorithm = "ES256" // Unsupported algorithm

	log := logger.NewCtxZapLogger("yogan")
	tokenStore := NewMemoryTokenStore(0, log)
	defer tokenStore.Close()

	manager, err := NewTokenManager(config, tokenStore, log)
	assert.Error(t, err)
	assert.Nil(t, manager)
}

// Test token with NotBefore attribute
func TestTokenManager_TokenWithNotBefore(t *testing.T) {
	config := newTestConfig()
	config.Security.EnableNotBefore = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	token, err := manager.GenerateAccessToken(ctx, "user123", nil)
	require.NoError(t, err)

	// The token should be immediately valid (nbf = now)
	claims, err := manager.VerifyToken(ctx, token)
	assert.NoError(t, err)
	assert.NotNil(t, claims)
	assert.False(t, claims.NotBefore.IsZero())
}

// Test token without JTI
func TestTokenManager_TokenWithoutJTI(t *testing.T) {
	config := newTestConfig()
	config.Security.EnableJTI = false
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	token, err := manager.GenerateAccessToken(ctx, "user123", nil)
	require.NoError(t, err)

	claims, err := manager.VerifyToken(ctx, token)
	assert.NoError(t, err)
	assert.Empty(t, claims.JTI)
}

// Test without an Audience in the Token
func TestTokenManager_TokenWithoutAudience(t *testing.T) {
	config := newTestConfig()
	config.AccessToken.Audience = ""
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	token, err := manager.GenerateAccessToken(ctx, "user123", nil)
	require.NoError(t, err)

	claims, err := manager.VerifyToken(ctx, token)
	assert.NoError(t, err)
	assert.Empty(t, claims.Audience)
}

// Test all custom claims
func TestTokenManager_AllCustomClaims(t *testing.T) {
	config := newTestConfig()
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	customClaims := map[string]interface{}{
		"user_id":   int64(123),
		"username":  "testuser",
		"roles":     []string{"admin", "user"},
		"tenant_id": "tenant-001",
	}

	token, err := manager.GenerateAccessToken(ctx, "user123", customClaims)
	require.NoError(t, err)

	claims, err := manager.VerifyToken(ctx, token)
	assert.NoError(t, err)
	assert.Equal(t, int64(123), claims.UserID)
	assert.Equal(t, "testuser", claims.Username)
	assert.Equal(t, []string{"admin", "user"}, claims.Roles)
	assert.Equal(t, "tenant-001", claims.TenantID)
}

// Test the complete process of RefreshToken
func TestTokenManager_RefreshTokenFlow(t *testing.T) {
	config := newTestConfig()
	config.RefreshToken.Enabled = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	subject := "user123"
	customClaims := map[string]interface{}{
		"user_id":   int64(123),
		"username":  "testuser",
		"roles":     []string{"admin"},
		"tenant_id": "tenant-001",
	}

	// Generate Refresh Token
	refreshToken, err := manager.GenerateRefreshToken(ctx, subject)
	require.NoError(t, err)

	// Validate Refresh Token
	refreshClaims, err := manager.VerifyToken(ctx, refreshToken)
	assert.NoError(t, err)
	assert.Equal(t, "refresh", refreshClaims.TokenType)
	assert.Equal(t, subject, refreshClaims.Subject)

	// Use Refresh Token to obtain Access Token
	// Note: In actual applications, refreshToken will store custom claims
	// Here an Access Token with custom Claims is generated directly for testing purposes
	accessToken, err := manager.GenerateAccessToken(ctx, subject, customClaims)
	require.NoError(t, err)

	accessClaims, err := manager.VerifyToken(ctx, accessToken)
	assert.NoError(t, err)
	assert.Equal(t, "access", accessClaims.TokenType)
	assert.Equal(t, int64(123), accessClaims.UserID)
}

// Test validation when blacklist is not enabled
func TestTokenManager_VerifyToken_BlacklistDisabled(t *testing.T) {
	config := newTestConfig()
	config.Blacklist.Enabled = false
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	token, err := manager.GenerateAccessToken(ctx, "user123", nil)
	require.NoError(t, err)

	// Verify Token (blacklist not enabled, no check)
	claims, err := manager.VerifyToken(ctx, token)
	assert.NoError(t, err)
	assert.NotNil(t, claims)
}

// Test validation when TokenStore is nil
func TestTokenManager_VerifyToken_NilTokenStore(t *testing.T) {
	config := newTestConfig()
	config.Blacklist.Enabled = true

	log := logger.NewCtxZapLogger("yogan")
	manager, err := NewTokenManager(config, nil, log)
	require.NoError(t, err)

	ctx := context.Background()
	token, err := manager.GenerateAccessToken(ctx, "user123", nil)
	require.NoError(t, err)

	// Verify Token (TokenStore is nil, skip blacklist check)
	claims, err := manager.VerifyToken(ctx, token)
	assert.NoError(t, err)
	assert.NotNil(t, claims)
}

// Test format incorrect Token
func TestTokenManager_VerifyToken_MalformedToken(t *testing.T) {
	config := newTestConfig()
	manager := newTestTokenManager(t, config)

	ctx := context.Background()

	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"invalid_format", "invalid.token"},
		{"too_many_segments", "a.b.c.d.e"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := manager.VerifyToken(ctx, tt.token)
			assert.Error(t, err)
			assert.Nil(t, claims)
		})
	}
}

// Test Claim parsing errors
func TestTokenManager_ParseCustomClaims_InvalidTypes(t *testing.T) {
	config := newTestConfig()
	manager := newTestTokenManager(t, config)

	ctx := context.Background()

	// Generate a normal token, then verify
	token, err := manager.GenerateAccessToken(ctx, "user123", map[string]interface{}{
		"user_id":  "not_a_number", // Incorrect type
		"roles":    "not_an_array",  // Incorrect type
	})
	require.NoError(t, err)

	// Validate the Token (should be parseable, but some fields will be ignored)
	claims, err := manager.VerifyToken(ctx, token)
	assert.NoError(t, err)
	assert.NotNil(t, claims)
	assert.Equal(t, int64(0), claims.UserID) // Parsing failed, use zero value
	assert.Nil(t, claims.Roles)               // parse failed, use nil
}

// Test TruncateToken function
func TestTruncateToken(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		expected string
	}{
		{"short", "abc", "abc"},
		{"exact", "0123456789", "0123456789"},
		{"long", "01234567890123456789", "0123456789..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateToken(tt.token)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test the difference between RefreshToken and Access Token
func TestTokenManager_RefreshToken_vs_AccessToken(t *testing.T) {
	config := newTestConfig()
	config.RefreshToken.Enabled = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()

	// Generate two types of Tokens
	accessToken, err := manager.GenerateAccessToken(ctx, "user123", nil)
	require.NoError(t, err)

	refreshToken, err := manager.GenerateRefreshToken(ctx, "user123")
	require.NoError(t, err)

	// Validate two tokens
	accessClaims, err := manager.VerifyToken(ctx, accessToken)
	assert.NoError(t, err)
	assert.Equal(t, "access", accessClaims.TokenType)

	refreshClaims, err := manager.VerifyToken(ctx, refreshToken)
	assert.NoError(t, err)
	assert.Equal(t, "refresh", refreshClaims.TokenType)

	// The TTL for the Refresh Token should be longer
	assert.True(t, refreshClaims.ExpiresAt.After(accessClaims.ExpiresAt))
}

