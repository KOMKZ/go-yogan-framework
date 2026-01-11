package jwt

import (
	"context"
	"testing"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 测试不支持的算法
func TestTokenManager_UnsupportedAlgorithm(t *testing.T) {
	config := newTestConfig()
	config.Algorithm = "ES256" // 不支持的算法

	log := logger.NewCtxZapLogger("yogan")
	tokenStore := NewMemoryTokenStore(0, log)
	defer tokenStore.Close()

	manager, err := NewTokenManager(config, tokenStore, log)
	assert.Error(t, err)
	assert.Nil(t, manager)
}

// 测试带 NotBefore 的 Token
func TestTokenManager_TokenWithNotBefore(t *testing.T) {
	config := newTestConfig()
	config.Security.EnableNotBefore = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	token, err := manager.GenerateAccessToken(ctx, "user123", nil)
	require.NoError(t, err)

	// Token 应该立即有效（nbf = now）
	claims, err := manager.VerifyToken(ctx, token)
	assert.NoError(t, err)
	assert.NotNil(t, claims)
	assert.False(t, claims.NotBefore.IsZero())
}

// 测试不带 JTI 的 Token
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

// 测试不带 Audience 的 Token
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

// 测试所有自定义 Claims
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

// 测试 RefreshToken 完整流程
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

	// 生成 Refresh Token
	refreshToken, err := manager.GenerateRefreshToken(ctx, subject)
	require.NoError(t, err)

	// 验证 Refresh Token
	refreshClaims, err := manager.VerifyToken(ctx, refreshToken)
	assert.NoError(t, err)
	assert.Equal(t, "refresh", refreshClaims.TokenType)
	assert.Equal(t, subject, refreshClaims.Subject)

	// 使用 Refresh Token 获取 Access Token
	// 注意：实际应用中，refreshToken 会存储自定义 Claims
	// 这里为了测试，直接生成带自定义 Claims 的 Access Token
	accessToken, err := manager.GenerateAccessToken(ctx, subject, customClaims)
	require.NoError(t, err)

	accessClaims, err := manager.VerifyToken(ctx, accessToken)
	assert.NoError(t, err)
	assert.Equal(t, "access", accessClaims.TokenType)
	assert.Equal(t, int64(123), accessClaims.UserID)
}

// 测试黑名单未启用时的验证
func TestTokenManager_VerifyToken_BlacklistDisabled(t *testing.T) {
	config := newTestConfig()
	config.Blacklist.Enabled = false
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	token, err := manager.GenerateAccessToken(ctx, "user123", nil)
	require.NoError(t, err)

	// 验证 Token（黑名单未启用，不检查）
	claims, err := manager.VerifyToken(ctx, token)
	assert.NoError(t, err)
	assert.NotNil(t, claims)
}

// 测试 TokenStore 为 nil 时的验证
func TestTokenManager_VerifyToken_NilTokenStore(t *testing.T) {
	config := newTestConfig()
	config.Blacklist.Enabled = true

	log := logger.NewCtxZapLogger("yogan")
	manager, err := NewTokenManager(config, nil, log)
	require.NoError(t, err)

	ctx := context.Background()
	token, err := manager.GenerateAccessToken(ctx, "user123", nil)
	require.NoError(t, err)

	// 验证 Token（TokenStore 为 nil，跳过黑名单检查）
	claims, err := manager.VerifyToken(ctx, token)
	assert.NoError(t, err)
	assert.NotNil(t, claims)
}

// 测试格式错误的 Token
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

// 测试 Claims 解析错误
func TestTokenManager_ParseCustomClaims_InvalidTypes(t *testing.T) {
	config := newTestConfig()
	manager := newTestTokenManager(t, config)

	ctx := context.Background()

	// 生成一个正常的 Token，然后验证
	token, err := manager.GenerateAccessToken(ctx, "user123", map[string]interface{}{
		"user_id":  "not_a_number", // 错误的类型
		"roles":    "not_an_array",  // 错误的类型
	})
	require.NoError(t, err)

	// 验证 Token（应该能解析，但某些字段会被忽略）
	claims, err := manager.VerifyToken(ctx, token)
	assert.NoError(t, err)
	assert.NotNil(t, claims)
	assert.Equal(t, int64(0), claims.UserID) // 解析失败，使用零值
	assert.Nil(t, claims.Roles)               // 解析失败，使用 nil
}

// 测试 TruncateToken 函数
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

// 测试 RefreshToken 与 Access Token 的区别
func TestTokenManager_RefreshToken_vs_AccessToken(t *testing.T) {
	config := newTestConfig()
	config.RefreshToken.Enabled = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()

	// 生成两种 Token
	accessToken, err := manager.GenerateAccessToken(ctx, "user123", nil)
	require.NoError(t, err)

	refreshToken, err := manager.GenerateRefreshToken(ctx, "user123")
	require.NoError(t, err)

	// 验证两种 Token
	accessClaims, err := manager.VerifyToken(ctx, accessToken)
	assert.NoError(t, err)
	assert.Equal(t, "access", accessClaims.TokenType)

	refreshClaims, err := manager.VerifyToken(ctx, refreshToken)
	assert.NoError(t, err)
	assert.Equal(t, "refresh", refreshClaims.TokenType)

	// Refresh Token 的 TTL 应该更长
	assert.True(t, refreshClaims.ExpiresAt.After(accessClaims.ExpiresAt))
}

