package jwt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid HS256 config",
			config: &Config{
				Enabled:   true,
				Algorithm: "HS256",
				Secret:    "test-secret",
				AccessToken: AccessTokenConfig{
					TTL: 2 * time.Hour,
				},
			},
			wantErr: false,
		},
		{
			name: "disabled config",
			config: &Config{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name: "missing secret for HS256",
			config: &Config{
				Enabled:   true,
				Algorithm: "HS256",
				Secret:    "",
				AccessToken: AccessTokenConfig{
					TTL: 2 * time.Hour,
				},
			},
			wantErr: true,
		},
		{
			name: "unsupported algorithm",
			config: &Config{
				Enabled:   true,
				Algorithm: "ES256",
				Secret:    "test-secret",
				AccessToken: AccessTokenConfig{
					TTL: 2 * time.Hour,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid access token ttl",
			config: &Config{
				Enabled:   true,
				Algorithm: "HS256",
				Secret:    "test-secret",
				AccessToken: AccessTokenConfig{
					TTL: 0,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid refresh token ttl",
			config: &Config{
				Enabled:   true,
				Algorithm: "HS256",
				Secret:    "test-secret",
				AccessToken: AccessTokenConfig{
					TTL: 2 * time.Hour,
				},
				RefreshToken: RefreshTokenConfig{
					Enabled: true,
					TTL:     0,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid blacklist storage",
			config: &Config{
				Enabled:   true,
				Algorithm: "HS256",
				Secret:    "test-secret",
				AccessToken: AccessTokenConfig{
					TTL: 2 * time.Hour,
				},
				Blacklist: BlacklistConfig{
					Enabled: true,
					Storage: "mysql",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_ApplyDefaults(t *testing.T) {
	config := &Config{}
	config.ApplyDefaults()

	assert.Equal(t, "HS256", config.Algorithm)
	assert.Equal(t, 2*time.Hour, config.AccessToken.TTL)
	assert.Equal(t, "yogan-api", config.AccessToken.Issuer)
	assert.Equal(t, 168*time.Hour, config.RefreshToken.TTL)
	assert.Equal(t, "jwt:blacklist:", config.Blacklist.RedisKeyPrefix)
	assert.Equal(t, 1*time.Hour, config.Blacklist.CleanupInterval)
	assert.Equal(t, 60*time.Second, config.Security.ClockSkew)
}
