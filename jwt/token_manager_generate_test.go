package jwt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTokenManager_GenerateAccessToken_AllCombinations 测试所有组合
func TestTokenManager_GenerateAccessToken_AllCombinations(t *testing.T) {
	tests := []struct {
		name          string
		enableJTI     bool
		enableNBF     bool
		audience      string
		customClaims  map[string]interface{}
	}{
		{
			name:      "with JTI and NBF",
			enableJTI: true,
			enableNBF: true,
			audience:  "test-audience",
		},
		{
			name:      "without JTI",
			enableJTI: false,
			enableNBF: true,
			audience:  "test-audience",
		},
		{
			name:      "without NBF",
			enableJTI: true,
			enableNBF: false,
			audience:  "test-audience",
		},
		{
			name:      "without audience",
			enableJTI: true,
			enableNBF: true,
			audience:  "",
		},
		{
			name:      "minimal config",
			enableJTI: false,
			enableNBF: false,
			audience:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := newTestConfig()
			config.Security.EnableJTI = tt.enableJTI
			config.Security.EnableNotBefore = tt.enableNBF
			config.AccessToken.Audience = tt.audience
			manager := newTestTokenManager(t, config)

			ctx := context.Background()
			token, err := manager.GenerateAccessToken(ctx, "user123", tt.customClaims)
			assert.NoError(t, err)
			assert.NotEmpty(t, token)

			// 验证 Token
			claims, err := manager.VerifyToken(ctx, token)
			assert.NoError(t, err)
			assert.NotNil(t, claims)

			// 验证 JTI
			if tt.enableJTI {
				assert.NotEmpty(t, claims.JTI)
			} else {
				assert.Empty(t, claims.JTI)
			}

			// 验证 NBF
			if tt.enableNBF {
				assert.False(t, claims.NotBefore.IsZero())
			} else {
				assert.True(t, claims.NotBefore.IsZero())
			}

			// 验证 Audience
			if tt.audience != "" {
				assert.Equal(t, tt.audience, claims.Audience)
			} else {
				assert.Empty(t, claims.Audience)
			}
		})
	}
}

// TestTokenManager_GenerateRefreshToken_AllCombinations 测试 Refresh Token 的所有组合
func TestTokenManager_GenerateRefreshToken_AllCombinations(t *testing.T) {
	config := newTestConfig()
	config.RefreshToken.Enabled = true
	manager := newTestTokenManager(t, config)

	ctx := context.Background()

	// 生成多个 Refresh Token
	tokens := make([]string, 3)
	for i := 0; i < 3; i++ {
		token, err := manager.GenerateRefreshToken(ctx, "user123")
		require.NoError(t, err)
		tokens[i] = token
	}

	// 验证所有 Token 都不同（包含唯一的 JTI）
	for i := 0; i < len(tokens); i++ {
		for j := i + 1; j < len(tokens); j++ {
			assert.NotEqual(t, tokens[i], tokens[j])
		}
	}

	// 验证所有 Token 都有效
	for _, token := range tokens {
		claims, err := manager.VerifyToken(ctx, token)
		assert.NoError(t, err)
		assert.Equal(t, "refresh", claims.TokenType)
		assert.NotEmpty(t, claims.JTI) // Refresh Token 总是有 JTI
	}
}

// TestTokenManager_ParseCustomClaims_EmptyRoles 测试空 roles 数组
func TestTokenManager_ParseCustomClaims_EmptyRoles(t *testing.T) {
	config := newTestConfig()
	manager := newTestTokenManager(t, config)

	ctx := context.Background()
	customClaims := map[string]interface{}{
		"roles": []string{}, // 空数组
	}

	token, err := manager.GenerateAccessToken(ctx, "user123", customClaims)
	require.NoError(t, err)

	claims, err := manager.VerifyToken(ctx, token)
	assert.NoError(t, err)
	assert.Empty(t, claims.Roles)
}

