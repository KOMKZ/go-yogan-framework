package jwt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestConfig_Validate_RS256 test RS256 configuration validation
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

// TestConfig_Validate_AllAlgorithms test all supported algorithms
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


// TestConfig_Validate_RSNotImplemented regression: RSA algorithms pass the
// key-path check but are not implemented in setupSigningMethod; they must be
// rejected at config time instead of failing at runtime.
func TestConfig_Validate_RSNotImplemented(t *testing.T) {
	for _, algo := range []string{"RS256", "RS384", "RS512"} {
		t.Run(algo, func(t *testing.T) {
			config := &Config{
				Enabled:       true,
				Algorithm:     algo,
				PrivateKeyPath: "/path/to/private.pem",
				PublicKeyPath:  "/path/to/public.pem",
				AccessToken: AccessTokenConfig{
					TTL: 2 * time.Hour,
				},
			}
			err := config.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "not yet implemented")
		})
	}
}
