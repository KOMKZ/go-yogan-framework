package jwt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClaims_Valid(t *testing.T) {
	tests := []struct {
		name    string
		claims  *Claims
		wantErr error
	}{
		{
			name: "valid claims",
			claims: &Claims{
				Subject:   "user123",
				IssuedAt:  time.Now().Add(-1 * time.Hour),
				ExpiresAt: time.Now().Add(1 * time.Hour),
			},
			wantErr: nil,
		},
		{
			name: "expired token",
			claims: &Claims{
				Subject:   "user123",
				IssuedAt:  time.Now().Add(-2 * time.Hour),
				ExpiresAt: time.Now().Add(-1 * time.Hour),
			},
			wantErr: ErrTokenExpired,
		},
		{
			name: "not yet valid",
			claims: &Claims{
				Subject:   "user123",
				IssuedAt:  time.Now(),
				ExpiresAt: time.Now().Add(2 * time.Hour),
				NotBefore: time.Now().Add(1 * time.Hour),
			},
			wantErr: ErrTokenNotYetValid,
		},
		{
			name: "no expiry",
			claims: &Claims{
				Subject:  "user123",
				IssuedAt: time.Now(),
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.claims.Valid()
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClaims_IsExpired(t *testing.T) {
	tests := []struct {
		name   string
		claims *Claims
		want   bool
	}{
		{
			name: "not expired",
			claims: &Claims{
				ExpiresAt: time.Now().Add(1 * time.Hour),
			},
			want: false,
		},
		{
			name: "expired",
			claims: &Claims{
				ExpiresAt: time.Now().Add(-1 * time.Hour),
			},
			want: true,
		},
		{
			name: "no expiry",
			claims: &Claims{
				ExpiresAt: time.Time{},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.claims.IsExpired()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClaims_TTL(t *testing.T) {
	tests := []struct {
		name   string
		claims *Claims
		want   time.Duration
	}{
		{
			name: "1 hour remaining",
			claims: &Claims{
				ExpiresAt: time.Now().Add(1 * time.Hour),
			},
			want: 1 * time.Hour,
		},
		{
			name: "expired",
			claims: &Claims{
				ExpiresAt: time.Now().Add(-1 * time.Hour),
			},
			want: -1 * time.Hour,
		},
		{
			name: "no expiry",
			claims: &Claims{
				ExpiresAt: time.Time{},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.claims.TTL()
			// 允许 1 秒误差
			assert.InDelta(t, tt.want.Seconds(), got.Seconds(), 1.0)
		})
	}
}
