package jwt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestConfig_Validate_RS256 测试 RS256 配置验证
func TestConfig_Validate_RS256(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "RS256 missing private key",
			config: &Config{
				Enabled:   true,
				Algorithm: "RS256",
				PublicKeyPath: "/path/to/public.pem",
				AccessToken: AccessTokenConfig{
					TTL: 2 * time.Hour,
				},
			},
			wantErr: true,
		},
		{
			name: "RS256 missing public key",
			config: &Config{
				Enabled:   true,
				Algorithm: "RS256",
				PrivateKeyPath: "/path/to/private.pem",
				AccessToken: AccessTokenConfig{
					TTL: 2 * time.Hour,
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

// TestConfig_Validate_AllAlgorithms 测试所有支持的算法
func TestConfig_Validate_AllAlgorithms(t *testing.T) {
	algorithms := []string{"HS256", "HS384", "HS512"}

	for _, algo := range algorithms {
		t.Run(algo, func(t *testing.T) {
			config := &Config{
				Enabled:   true,
				Algorithm: algo,
				Secret:    "test-secret",
				AccessToken: AccessTokenConfig{
					TTL: 2 * time.Hour,
				},
			}

			err := config.Validate()
			assert.NoError(t, err)
		})
	}
}

