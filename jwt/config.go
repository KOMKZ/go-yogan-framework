package jwt

import (
	"fmt"
	"time"
)

// Configure JWT settings
type Config struct {
	Enabled bool `yaml:"enabled" mapstructure:"enabled"` // Is enabled

	// Signature algorithm
	Algorithm string `yaml:"algorithm" mapstructure:"algorithm"` // HS256, HS384, HS512, RS256, RS384, RS512

	// Key configuration
	Secret         string `yaml:"secret" mapstructure:"secret"`                     // Symmetric key (HS256)
	PrivateKeyPath string `yaml:"private_key_path" mapstructure:"private_key_path"` // Private key path (RS256)
	PublicKeyPath  string `yaml:"public_key_path" mapstructure:"public_key_path"`   // Public key path (RS256)

	// Token configuration
	AccessToken  AccessTokenConfig  `yaml:"access_token" mapstructure:"access_token"`
	RefreshToken RefreshTokenConfig `yaml:"refresh_token" mapstructure:"refresh_token"`

	// Session configuration
	Session SessionConfig `yaml:"session" mapstructure:"session"`

	// Security configuration
	Security SecurityConfig `yaml:"security" mapstructure:"security"`
}

// AccessToken Configuration
type AccessTokenConfig struct {
	TTL      time.Duration `yaml:"ttl" mapstructure:"ttl"`           // valid period
	Issuer   string        `yaml:"issuer" mapstructure:"issuer"`     // issuer
	Audience string        `yaml:"audience" mapstructure:"audience"` // receiver
}

// RefreshTokenConfig Refresh token configuration
type RefreshTokenConfig struct {
	Enabled bool          `yaml:"enabled" mapstructure:"enabled"` // Is enabled
	TTL     time.Duration `yaml:"ttl" mapstructure:"ttl"`         // valid period
}

// SessionConfig configures server-side JWT session state.
type SessionConfig struct {
	Enabled                bool          `yaml:"enabled" mapstructure:"enabled"`                                     // Is enabled
	Store                  string        `yaml:"store" mapstructure:"store"`                                         // redis / memory
	RedisClient            string        `yaml:"redis_client" mapstructure:"redis_client"`                           // Named redis client
	KeyPrefix              string        `yaml:"key_prefix" mapstructure:"key_prefix"`                               // Redis key prefix
	MaxSessionsPerSubject  int           `yaml:"max_sessions_per_subject" mapstructure:"max_sessions_per_subject"`   // Max active sessions per subject
	LastSeenUpdateInterval time.Duration `yaml:"last_seen_update_interval" mapstructure:"last_seen_update_interval"` // Throttle last_seen writes
	RefreshReusePolicy     string        `yaml:"refresh_reuse_policy" mapstructure:"refresh_reuse_policy"`           // revoke_session / reject
	CleanupInterval        time.Duration `yaml:"cleanup_interval" mapstructure:"cleanup_interval"`                   // Memory cleanup interval
}

// SecurityConfig security configuration
type SecurityConfig struct {
	EnableJTI       bool          `yaml:"enable_jti" mapstructure:"enable_jti"`               // Enable JTI (anti-replay)
	EnableNotBefore bool          `yaml:"enable_not_before" mapstructure:"enable_not_before"` // Enable NBF (delayed activation)
	ClockSkew       time.Duration `yaml:"clock_skew" mapstructure:"clock_skew"`               // clock skew tolerance
}

// Validate configuration
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}

	// Validate algorithm
	switch c.Algorithm {
	case "HS256", "HS384", "HS512":
		if c.Secret == "" {
			return ErrSecretEmpty
		}
	case "RS256", "RS384", "RS512":
		// 🎯 Only HS is implemented today; reject RS at config time instead
		// of passing validation and failing later inside setupSigningMethod.
		return fmt.Errorf("jwt: algorithm %s is not yet implemented", c.Algorithm)
	default:
		return ErrAlgorithmNotSupported
	}

	// Validate TTL
	if c.AccessToken.TTL <= 0 {
		return fmt.Errorf("jwt: access token ttl must be positive")
	}

	if c.RefreshToken.Enabled && c.RefreshToken.TTL <= 0 {
		return fmt.Errorf("jwt: refresh token ttl must be positive")
	}

	// Validate session store
	if c.Session.Enabled {
		if c.Session.Store != "redis" && c.Session.Store != "memory" {
			return fmt.Errorf("jwt: session store must be redis or memory")
		}
		if c.Session.Store == "redis" && c.Session.RedisClient == "" {
			return fmt.Errorf("jwt: session redis_client is required")
		}
		if c.Session.RefreshReusePolicy != "revoke_session" && c.Session.RefreshReusePolicy != "reject" {
			return fmt.Errorf("jwt: session refresh_reuse_policy must be revoke_session or reject")
		}
	}

	return nil
}

// Apply defaults
func (c *Config) ApplyDefaults() {
	if c.Algorithm == "" {
		c.Algorithm = "HS256"
	}

	if c.AccessToken.TTL == 0 {
		c.AccessToken.TTL = 2 * time.Hour
	}

	if c.AccessToken.Issuer == "" {
		c.AccessToken.Issuer = "yogan-api"
	}

	if c.RefreshToken.TTL == 0 {
		c.RefreshToken.TTL = 168 * time.Hour // 7 days
	}

	if c.Session.Store == "" {
		c.Session.Store = "memory"
	}
	if c.Session.RedisClient == "" {
		c.Session.RedisClient = "main"
	}
	if c.Session.KeyPrefix == "" {
		c.Session.KeyPrefix = "jwt:session:"
	}
	if c.Session.RefreshReusePolicy == "" {
		c.Session.RefreshReusePolicy = "revoke_session"
	}
	if c.Session.LastSeenUpdateInterval == 0 {
		c.Session.LastSeenUpdateInterval = 1 * time.Minute
	}
	if c.Session.CleanupInterval == 0 {
		c.Session.CleanupInterval = 1 * time.Hour
	}

	if c.Security.ClockSkew == 0 {
		c.Security.ClockSkew = 60 * time.Second
	}
}
