package jwt

import (
	"context"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

// TestTokenManager_ParseJWTError_AllCases_Test all JWT error parsing scenarios
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

// TestTokenManager_ParseJWTError_UnknownError Test unknown error
func TestTokenManager_ParseJWTError_UnknownError(t *testing.T) {
	config := newTestConfig()
	manager := newTestTokenManager(t, config)
	impl := manager.(*tokenManagerImpl)

	// Test unknown error
	err := jwt.ErrTokenMalformed
	result := impl.parseJWTError(err)
	assert.Equal(t, ErrTokenInvalid, result)
}

// TestContains test the contains function
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

// TestTokenManager_VerifyToken_ErrorCases Test various verification error cases
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
