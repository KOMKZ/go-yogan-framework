package jwt

import (
	"context"
	"testing"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTokenManager_ParseJWTError_AllCases 测试所有 JWT 错误解析
func TestTokenManager_ParseJWTError_AllCases(t *testing.T) {
	config := newTestConfig()
	manager := newTestTokenManager(t, config)
	impl := manager.(*tokenManagerImpl)

	tests := []struct {
		name     string
		err      error
		expected error
	}{
		{"nil error", nil, nil},
		{"ErrTokenExpired", jwt.ErrTokenExpired, ErrTokenExpired},
		{"ErrTokenNotValidYet", jwt.ErrTokenNotValidYet, ErrTokenNotYetValid},
		{"ErrTokenSignatureInvalid", jwt.ErrTokenSignatureInvalid, ErrInvalidSignature},
		{"unknown error", jwt.ErrTokenMalformed, ErrTokenInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := impl.parseJWTError(tt.err)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestTokenManager_ParseJWTError_UnknownError 测试未知错误
func TestTokenManager_ParseJWTError_UnknownError(t *testing.T) {
	config := newTestConfig()
	manager := newTestTokenManager(t, config)
	impl := manager.(*tokenManagerImpl)

	// 测试未知错误
	err := jwt.ErrTokenMalformed
	result := impl.parseJWTError(err)
	assert.Equal(t, ErrTokenInvalid, result)
}

// TestContains 测试 contains 函数
func TestContains(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		substr string
		want   bool
	}{
		{"empty substr", "hello", "", true},
		{"found at start", "hello world", "hello", true},
		{"found in middle", "hello world", "lo wo", true},
		{"found at end", "hello world", "world", true},
		{"not found", "hello world", "xyz", false},
		{"empty string", "", "hello", false},
		{"exact match", "hello", "hello", true},
		{"substr longer", "hi", "hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contains(tt.s, tt.substr)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestTokenManager_VerifyToken_ErrorCases 测试各种验证错误情况
func TestTokenManager_VerifyToken_ErrorCases(t *testing.T) {
	config := newTestConfig()
	manager := newTestTokenManager(t, config)

	ctx := context.Background()

	tests := []struct {
		name     string
		token    string
		wantErr  bool
		contains string
	}{
		{
			name:     "empty token",
			token:    "",
			wantErr:  true,
			contains: "",
		},
		{
			name:     "invalid format",
			token:    "invalid",
			wantErr:  true,
			contains: "",
		},
		{
			name:     "wrong signature",
			token:    "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyMTIzIn0.wrong",
			wantErr:  true,
			contains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := manager.VerifyToken(ctx, tt.token)
			assert.Error(t, err)
			assert.Nil(t, claims)
		})
	}
}

// TestTokenManager_RevokeToken_NilTokenStore 测试 TokenStore 为 nil 时的撤销
func TestTokenManager_RevokeToken_NilTokenStore(t *testing.T) {
	config := newTestConfig()
	config.Blacklist.Enabled = true

	log := logger.NewCtxZapLogger("yogan")
	manager, err := NewTokenManager(config, nil, log)
	require.NoError(t, err)

	ctx := context.Background()
	token, err := manager.GenerateAccessToken(ctx, "user123", nil)
	require.NoError(t, err)

	// 撤销 Token（TokenStore 为 nil）
	err = manager.RevokeToken(ctx, token)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "blacklist not enabled")
}

// TestTokenManager_RevokeUserTokens_NilTokenStore 测试 TokenStore 为 nil 时的用户撤销
func TestTokenManager_RevokeUserTokens_NilTokenStore(t *testing.T) {
	config := newTestConfig()
	config.Blacklist.Enabled = true

	log := logger.NewCtxZapLogger("yogan")
	manager, err := NewTokenManager(config, nil, log)
	require.NoError(t, err)

	ctx := context.Background()

	// 撤销用户 Token（TokenStore 为 nil）
	err = manager.RevokeUserTokens(ctx, "user123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "blacklist not enabled")
}

