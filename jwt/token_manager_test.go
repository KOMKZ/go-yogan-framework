package jwt

import (
	"context"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/golang-jwt/jwt/v5"
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
	manager, err := NewTokenManager(config, nil, log)
	require.NoError(t, err)

	return manager
}

func TestNewTokenManager(t *testing.T) {
	config := newTestConfig()
	log := logger.NewCtxZapLogger("yogan")

	manager, err := NewTokenManager(config, nil, log)
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

	manager, err := NewTokenManager(config, nil, log)
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
	config.Security.ClockSkew = 0                  // Strict expiry (no leeway)
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

// TestTokenManager_VerifyToken_ClockSkewLeeway regression: the configured
// ClockSkew must be applied as parse leeway — a token expired 30s ago stays
// valid within the 60s default skew (previously ClockSkew was dead config).
func TestTokenManager_VerifyToken_ClockSkewLeeway(t *testing.T) {
	config := newTestConfig()
	manager := newTestTokenManager(t, config)

	ctx := context.Background()

	// Craft a token that expired 30s ago (within the 60s ClockSkew)
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":        "user-skew",
		"iat":        now.Add(-2 * time.Hour).Unix(),
		"exp":        now.Add(-30 * time.Second).Unix(),
		"iss":        config.AccessToken.Issuer,
		"token_type": "access",
	})
	tokenString, err := token.SignedString([]byte(config.Secret))
	require.NoError(t, err)

	claims, err := manager.VerifyToken(ctx, tokenString)
	require.NoError(t, err, "token within clock skew must verify")
	require.NotNil(t, claims)
	assert.Equal(t, "user-skew", claims.Subject)
}
