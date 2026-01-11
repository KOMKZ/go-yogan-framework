package jwt

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTokenManager_RefreshToken_FullFlow 测试完整的 RefreshToken 流程
func TestTokenManager_RefreshToken_FullFlow(t *testing.T) {
	config := newTestConfig()
	config.RefreshToken.Enabled = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	subject := "user123"

	// 1. 生成 Refresh Token
	refreshToken, err := manager.GenerateRefreshToken(ctx, subject)
	require.NoError(t, err)
	assert.NotEmpty(t, refreshToken)

	// 2. 验证 Refresh Token
	refreshClaims, err := manager.VerifyToken(ctx, refreshToken)
	require.NoError(t, err)
	assert.Equal(t, subject, refreshClaims.Subject)
	assert.Equal(t, "refresh", refreshClaims.TokenType)

	// 3. 使用 Refresh Token 获取新的 Access Token
	newAccessToken, err := manager.RefreshToken(ctx, refreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, newAccessToken)

	// 4. 验证新的 Access Token
	accessClaims, err := manager.VerifyToken(ctx, newAccessToken)
	require.NoError(t, err)
	assert.Equal(t, subject, accessClaims.Subject)
	assert.Equal(t, "access", accessClaims.TokenType)
}

// TestTokenManager_RefreshToken_WithCustomClaims 测试带自定义 Claims 的 RefreshToken
func TestTokenManager_RefreshToken_WithCustomClaims(t *testing.T) {
	config := newTestConfig()
	config.RefreshToken.Enabled = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	subject := "user123"

	// 生成带自定义 Claims 的 Access Token
	customClaims := map[string]interface{}{
		"user_id":   int64(123),
		"username":  "testuser",
		"roles":     []string{"admin"},
		"tenant_id": "tenant-001",
	}
	accessToken, err := manager.GenerateAccessToken(ctx, subject, customClaims)
	require.NoError(t, err)

	// 验证 Access Token
	accessClaims, err := manager.VerifyToken(ctx, accessToken)
	require.NoError(t, err)
	assert.Equal(t, int64(123), accessClaims.UserID)
	assert.Equal(t, "testuser", accessClaims.Username)
	assert.Equal(t, []string{"admin"}, accessClaims.Roles)
	assert.Equal(t, "tenant-001", accessClaims.TenantID)

	// 生成 Refresh Token
	refreshToken, err := manager.GenerateRefreshToken(ctx, subject)
	require.NoError(t, err)

	// 使用 Refresh Token 获取新的 Access Token
	// 注意：实际应用中，Refresh Token 应该存储自定义 Claims
	// 这里为了测试，我们验证 RefreshToken 方法能正确处理
	newAccessToken, err := manager.RefreshToken(ctx, refreshToken)
	require.NoError(t, err)

	// 验证新的 Access Token（自定义 Claims 不会自动继承）
	newAccessClaims, err := manager.VerifyToken(ctx, newAccessToken)
	require.NoError(t, err)
	assert.Equal(t, subject, newAccessClaims.Subject)
	assert.Equal(t, "access", newAccessClaims.TokenType)
}

// TestTokenManager_RefreshToken_BlacklistedRefreshToken 测试被撤销的 Refresh Token
func TestTokenManager_RefreshToken_BlacklistedRefreshToken(t *testing.T) {
	config := newTestConfig()
	config.RefreshToken.Enabled = true
	config.Blacklist.Enabled = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	subject := "user123"

	// 生成 Refresh Token
	refreshToken, err := manager.GenerateRefreshToken(ctx, subject)
	require.NoError(t, err)

	// 撤销 Refresh Token
	err = manager.RevokeToken(ctx, refreshToken)
	require.NoError(t, err)

	// 尝试使用被撤销的 Refresh Token
	newAccessToken, err := manager.RefreshToken(ctx, refreshToken)
	assert.Error(t, err)
	assert.Empty(t, newAccessToken)
	assert.Contains(t, err.Error(), "invalid refresh token")
}

// TestTokenManager_RefreshToken_ExpiredRefreshToken 测试过期的 Refresh Token
func TestTokenManager_RefreshToken_ExpiredRefreshToken(t *testing.T) {
	config := newTestConfig()
	config.RefreshToken.Enabled = true
	config.RefreshToken.TTL = 10 * time.Millisecond // 极短 TTL
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	subject := "user123"

	// 生成 Refresh Token
	refreshToken, err := manager.GenerateRefreshToken(ctx, subject)
	require.NoError(t, err)

	// 等待 Token 过期
	time.Sleep(20 * time.Millisecond)

	// 尝试使用过期的 Refresh Token
	newAccessToken, err := manager.RefreshToken(ctx, refreshToken)
	assert.Error(t, err)
	assert.Empty(t, newAccessToken)
}

