package jwt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTokenManager_RefreshToken_AllCustomClaims 测试所有自定义 Claims 的刷新
func TestTokenManager_RefreshToken_AllCustomClaims(t *testing.T) {
	config := newTestConfig()
	config.RefreshToken.Enabled = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	subject := "user123"

	// 生成带所有自定义 Claims 的 Access Token
	customClaims := map[string]interface{}{
		"user_id":   int64(123),
		"username":  "testuser",
		"roles":     []string{"admin", "user"},
		"tenant_id": "tenant-001",
	}
	accessToken, err := manager.GenerateAccessToken(ctx, subject, customClaims)
	require.NoError(t, err)

	// 验证 Access Token
	accessClaims, err := manager.VerifyToken(ctx, accessToken)
	require.NoError(t, err)
	assert.Equal(t, int64(123), accessClaims.UserID)
	assert.Equal(t, "testuser", accessClaims.Username)
	assert.Equal(t, []string{"admin", "user"}, accessClaims.Roles)
	assert.Equal(t, "tenant-001", accessClaims.TenantID)

	// 生成 Refresh Token
	refreshToken, err := manager.GenerateRefreshToken(ctx, subject)
	require.NoError(t, err)

	// 使用 Refresh Token 获取新的 Access Token
	newAccessToken, err := manager.RefreshToken(ctx, refreshToken)
	require.NoError(t, err)

	// 验证新的 Access Token
	newAccessClaims, err := manager.VerifyToken(ctx, newAccessToken)
	require.NoError(t, err)
	assert.Equal(t, subject, newAccessClaims.Subject)
	assert.Equal(t, "access", newAccessClaims.TokenType)
}

// TestTokenManager_RefreshToken_OnlyUsername 测试只有 username 的刷新
func TestTokenManager_RefreshToken_OnlyUsername(t *testing.T) {
	config := newTestConfig()
	config.RefreshToken.Enabled = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	subject := "user123"

	// 生成带 username 的 Access Token
	customClaims := map[string]interface{}{
		"username": "testuser",
	}
	_, err := manager.GenerateAccessToken(ctx, subject, customClaims)
	require.NoError(t, err)

	// 生成 Refresh Token
	refreshToken, err := manager.GenerateRefreshToken(ctx, subject)
	require.NoError(t, err)

	// 使用 Refresh Token 获取新的 Access Token
	newAccessToken, err := manager.RefreshToken(ctx, refreshToken)
	require.NoError(t, err)

	// 验证新的 Access Token
	newAccessClaims, err := manager.VerifyToken(ctx, newAccessToken)
	require.NoError(t, err)
	assert.Equal(t, subject, newAccessClaims.Subject)
}

// TestTokenManager_RefreshToken_OnlyRoles 测试只有 roles 的刷新
func TestTokenManager_RefreshToken_OnlyRoles(t *testing.T) {
	config := newTestConfig()
	config.RefreshToken.Enabled = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	subject := "user123"

	// 生成带 roles 的 Access Token
	customClaims := map[string]interface{}{
		"roles": []string{"admin"},
	}
	_, err := manager.GenerateAccessToken(ctx, subject, customClaims)
	require.NoError(t, err)

	// 生成 Refresh Token
	refreshToken, err := manager.GenerateRefreshToken(ctx, subject)
	require.NoError(t, err)

	// 使用 Refresh Token 获取新的 Access Token
	newAccessToken, err := manager.RefreshToken(ctx, refreshToken)
	require.NoError(t, err)

	// 验证新的 Access Token
	newAccessClaims, err := manager.VerifyToken(ctx, newAccessToken)
	require.NoError(t, err)
	assert.Equal(t, subject, newAccessClaims.Subject)
}

// TestTokenManager_RefreshToken_OnlyTenantID 测试只有 tenant_id 的刷新
func TestTokenManager_RefreshToken_OnlyTenantID(t *testing.T) {
	config := newTestConfig()
	config.RefreshToken.Enabled = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	subject := "user123"

	// 生成带 tenant_id 的 Access Token
	customClaims := map[string]interface{}{
		"tenant_id": "tenant-001",
	}
	_, err := manager.GenerateAccessToken(ctx, subject, customClaims)
	require.NoError(t, err)

	// 生成 Refresh Token
	refreshToken, err := manager.GenerateRefreshToken(ctx, subject)
	require.NoError(t, err)

	// 使用 Refresh Token 获取新的 Access Token
	newAccessToken, err := manager.RefreshToken(ctx, refreshToken)
	require.NoError(t, err)

	// 验证新的 Access Token
	newAccessClaims, err := manager.VerifyToken(ctx, newAccessToken)
	require.NoError(t, err)
	assert.Equal(t, subject, newAccessClaims.Subject)
}

