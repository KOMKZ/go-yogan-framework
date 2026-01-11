package jwt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"ErrTokenInvalid", ErrTokenInvalid, "jwt: token invalid"},
		{"ErrTokenExpired", ErrTokenExpired, "jwt: token expired"},
		{"ErrTokenNotYetValid", ErrTokenNotYetValid, "jwt: token not yet valid"},
		{"ErrInvalidSignature", ErrInvalidSignature, "jwt: invalid signature"},
		{"ErrTokenBlacklisted", ErrTokenBlacklisted, "jwt: token blacklisted"},
		{"ErrInvalidClaims", ErrInvalidClaims, "jwt: invalid claims"},
		{"ErrSecretEmpty", ErrSecretEmpty, "jwt: secret is empty"},
		{"ErrAlgorithmNotSupported", ErrAlgorithmNotSupported, "jwt: algorithm not supported"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.err.Error())
		})
	}
}

