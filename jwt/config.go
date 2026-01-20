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

	// Blacklist configuration
	Blacklist BlacklistConfig `yaml:"blacklist" mapstructure:"blacklist"`

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

// BlacklistConfig blacklist configuration
type BlacklistConfig struct {
	Enabled         bool          `yaml:"enabled" mapstructure:"enabled"`                   // Is enabled
	Storage         string        `yaml:"storage" mapstructure:"storage"`                   // redis / memory
	RedisKeyPrefix  string        `yaml:"redis_key_prefix" mapstructure:"redis_key_prefix"` // Redis key prefix
	CleanupInterval time.Duration `yaml:"cleanup_interval" mapstructure:"cleanup_interval"` // Memory mode cleanup interval
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
		if c.PrivateKeyPath == "" || c.PublicKeyPath == "" {
			return fmt.Errorf("jwt: RSA keys not configured")
		}
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

	// Validate blacklist storage
	if c.Blacklist.Enabled {
		if c.Blacklist.Storage != "redis" && c.Blacklist.Storage != "memory" {
			return fmt.Errorf("jwt: blacklist storage must be redis or memory")
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

	if c.Blacklist.RedisKeyPrefix == "" {
		c.Blacklist.RedisKeyPrefix = "jwt:blacklist:"
	}

	if c.Blacklist.CleanupInterval == 0 {
		c.Blacklist.CleanupInterval = 1 * time.Hour
	}

	if c.Security.ClockSkew == 0 {
		c.Security.ClockSkew = 60 * time.Second
	}
}
