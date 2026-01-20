package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfig_ApplyDefaults(t *testing.T) {
	t.Run("empty config gets defaults", func(t *testing.T) {
		cfg := &Config{}
		cfg.ApplyDefaults()

		assert.Equal(t, []string{"password"}, cfg.Providers)
		assert.Equal(t, 12, cfg.Password.BcryptCost)
		assert.Equal(t, 8, cfg.Password.Policy.MinLength)
		assert.Equal(t, 128, cfg.Password.Policy.MaxLength)
		assert.Equal(t, 5, cfg.Password.Policy.PasswordHistory)
		assert.Equal(t, 90, cfg.Password.Policy.PasswordExpiryDays)
	})

	t.Run("existing values are preserved", func(t *testing.T) {
		cfg := &Config{
			Providers: []string{"oauth2"},
			Password: PasswordConfig{
				BcryptCost: 14,
				Policy: PasswordPolicy{
					MinLength:          10,
					MaxLength:          64,
					PasswordHistory:    3,
					PasswordExpiryDays: 30,
				},
			},
		}
		cfg.ApplyDefaults()

		assert.Equal(t, []string{"oauth2"}, cfg.Providers)
		assert.Equal(t, 14, cfg.Password.BcryptCost)
		assert.Equal(t, 10, cfg.Password.Policy.MinLength)
		assert.Equal(t, 64, cfg.Password.Policy.MaxLength)
		assert.Equal(t, 3, cfg.Password.Policy.PasswordHistory)
		assert.Equal(t, 30, cfg.Password.Policy.PasswordExpiryDays)
	})

	t.Run("login attempt defaults when enabled", func(t *testing.T) {
		cfg := &Config{
			LoginAttempt: LoginAttemptConfig{
				Enabled: true,
			},
		}
		cfg.ApplyDefaults()

		assert.Equal(t, 5, cfg.LoginAttempt.MaxAttempts)
		assert.Equal(t, 30*time.Minute, cfg.LoginAttempt.LockoutDuration)
		assert.Equal(t, "redis", cfg.LoginAttempt.Storage)
		assert.Equal(t, "auth:login_attempt:", cfg.LoginAttempt.RedisKeyPrefix)
	})

	t.Run("login attempt preserves existing values", func(t *testing.T) {
		cfg := &Config{
			LoginAttempt: LoginAttemptConfig{
				Enabled:         true,
				MaxAttempts:     3,
				LockoutDuration: 10 * time.Minute,
				Storage:         "memory",
				RedisKeyPrefix:  "custom:",
			},
		}
		cfg.ApplyDefaults()

		assert.Equal(t, 3, cfg.LoginAttempt.MaxAttempts)
		assert.Equal(t, 10*time.Minute, cfg.LoginAttempt.LockoutDuration)
		assert.Equal(t, "memory", cfg.LoginAttempt.Storage)
		assert.Equal(t, "custom:", cfg.LoginAttempt.RedisKeyPrefix)
	})
}

func TestConfig_Validate(t *testing.T) {
	t.Run("disabled config is valid", func(t *testing.T) {
		cfg := &Config{Enabled: false}
		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("no providers error", func(t *testing.T) {
		cfg := &Config{
			Enabled:   true,
			Providers: []string{},
		}
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "At least one authentication method must be enabled.")
	})

	t.Run("invalid password min length", func(t *testing.T) {
		cfg := &Config{
			Enabled:   true,
			Providers: []string{"password"},
			Password: PasswordConfig{
				Enabled:    true,
				BcryptCost: 12,
				Policy: PasswordPolicy{
					MinLength: 0,
					MaxLength: 100,
				},
			},
		}
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "English: Invalid configuration for minimum password length")
	})

	t.Run("min length greater than max length", func(t *testing.T) {
		cfg := &Config{
			Enabled:   true,
			Providers: []string{"password"},
			Password: PasswordConfig{
				Enabled:    true,
				BcryptCost: 12,
				Policy: PasswordPolicy{
					MinLength: 100,
					MaxLength: 50,
				},
			},
		}
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "English: Invalid configuration for minimum password length")
	})

	t.Run("invalid bcrypt cost too low", func(t *testing.T) {
		cfg := &Config{
			Enabled:   true,
			Providers: []string{"password"},
			Password: PasswordConfig{
				Enabled:    true,
				BcryptCost: 3,
				Policy: PasswordPolicy{
					MinLength: 8,
					MaxLength: 100,
				},
			},
		}
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "bcrypt cost The bcrypt cost must be between 4 and 31 4-31 The bcrypt cost must be between 4 and 31")
	})

	t.Run("invalid bcrypt cost too high", func(t *testing.T) {
		cfg := &Config{
			Enabled:   true,
			Providers: []string{"password"},
			Password: PasswordConfig{
				Enabled:    true,
				BcryptCost: 32,
				Policy: PasswordPolicy{
					MinLength: 8,
					MaxLength: 100,
				},
			},
		}
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "bcrypt cost The bcrypt cost must be between 4 and 31 4-31 The bcrypt cost must be between 4 and 31")
	})

	t.Run("invalid login attempt max attempts", func(t *testing.T) {
		cfg := &Config{
			Enabled:   true,
			Providers: []string{"password"},
			LoginAttempt: LoginAttemptConfig{
				Enabled:     true,
				MaxAttempts: 0,
				Storage:     "redis",
			},
		}
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "The maximum number of login attempts must be >= 1 >= 1")
	})

	t.Run("invalid login attempt storage", func(t *testing.T) {
		cfg := &Config{
			Enabled:   true,
			Providers: []string{"password"},
			LoginAttempt: LoginAttemptConfig{
				Enabled:     true,
				MaxAttempts: 5,
				Storage:     "invalid",
			},
		}
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "The login attempt storage method only supports redis or memory redis The login attempt storage method only supports redis or memory memory")
	})

	t.Run("valid config", func(t *testing.T) {
		cfg := &Config{
			Enabled:   true,
			Providers: []string{"password"},
			Password: PasswordConfig{
				Enabled:    true,
				BcryptCost: 12,
				Policy: PasswordPolicy{
					MinLength: 8,
					MaxLength: 128,
				},
			},
			LoginAttempt: LoginAttemptConfig{
				Enabled:     true,
				MaxAttempts: 5,
				Storage:     "memory",
			},
		}
		err := cfg.Validate()
		assert.NoError(t, err)
	})
}

func TestConfig_IsProviderEnabled(t *testing.T) {
	cfg := &Config{
		Providers: []string{"password", "oauth2"},
	}

	assert.True(t, cfg.IsProviderEnabled("password"))
	assert.True(t, cfg.IsProviderEnabled("oauth2"))
	assert.False(t, cfg.IsProviderEnabled("api_key"))
	assert.False(t, cfg.IsProviderEnabled(""))
}
