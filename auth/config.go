package auth

import (
	"errors"
	"time"
)

// Configuration for authentication components
type Config struct {
	Enabled bool `yaml:"enabled" mapstructure:"enabled"` // Whether to enable authentication component

	// Supported authentication methods
	Providers []string `yaml:"providers" mapstructure:"providers"` // password, oauth2, api_key, basic_auth

	// password authentication configuration
	Password PasswordConfig `yaml:"password" mapstructure:"password"`

	// OAuth2.0 configuration (future expansion)
	// OAuth2 OAuth2Config `yaml:"oauth2" mapstructure:"oauth2"`

	// API Key configuration (future expansion)
	// APIKey APIKeyConfig `yaml:"api_key" mapstructure:"api_key"`

	// Login attempt limit
	LoginAttempt LoginAttemptConfig `yaml:"login_attempt" mapstructure:"login_attempt"`
}

// PasswordConfig password authentication configuration
type PasswordConfig struct {
	Enabled    bool           `yaml:"enabled" mapstructure:"enabled"`         // Whether password authentication is enabled
	Policy     PasswordPolicy `yaml:"policy" mapstructure:"policy"`           // password policy
	BcryptCost int            `yaml:"bcrypt_cost" mapstructure:"bcrypt_cost"` // bcrypt cost (10-14, recommended 12)
}

// PasswordPolicy password policy
type PasswordPolicy struct {
	// Complexity requirement
	MinLength          int  `yaml:"min_length" mapstructure:"min_length"`                     // minimum length
	MaxLength          int  `yaml:"max_length" mapstructure:"max_length"`                     // maximum length
	RequireUppercase   bool `yaml:"require_uppercase" mapstructure:"require_uppercase"`       // At least one uppercase letter
	RequireLowercase   bool `yaml:"require_lowercase" mapstructure:"require_lowercase"`       // At least one lowercase letter
	RequireDigit       bool `yaml:"require_digit" mapstructure:"require_digit"`               // At least one digit
	RequireSpecialChar bool `yaml:"require_special_char" mapstructure:"require_special_char"` // At least one special character

	// Security policy
	PasswordHistory    int `yaml:"password_history" mapstructure:"password_history"`         // Prohibit repeated passwords from the last N attempts
	PasswordExpiryDays int `yaml:"password_expiry_days" mapstructure:"password_expiry_days"` // Password expires in N days

	// weak password blacklist
	Blacklist []string `yaml:"blacklist" mapstructure:"blacklist"`
}

// LoginAttemptConfig login attempt restriction configuration
type LoginAttemptConfig struct {
	Enabled         bool          `yaml:"enabled" mapstructure:"enabled"`                   // Is Enabled
	MaxAttempts     int           `yaml:"max_attempts" mapstructure:"max_attempts"`         // Maximum number of attempts
	LockoutDuration time.Duration `yaml:"lockout_duration" mapstructure:"lockout_duration"` // lock duration
	Storage         string        `yaml:"storage" mapstructure:"storage"`                   // Storage method: redis, memory
	RedisKeyPrefix  string        `yaml:"redis_key_prefix" mapstructure:"redis_key_prefix"` // Redis key prefix
}

// ApplyDefaults Apply default values
func (c *Config) ApplyDefaults() {
	// Enable password authentication by default
	if len(c.Providers) == 0 {
		c.Providers = []string{"password"}
	}

	// password authentication default value
	if c.Password.BcryptCost == 0 {
		c.Password.BcryptCost = 12 // recommended value
	}

	// default password policy values
	policy := &c.Password.Policy
	if policy.MinLength == 0 {
		policy.MinLength = 8
	}
	if policy.MaxLength == 0 {
		policy.MaxLength = 128
	}
	if policy.PasswordHistory == 0 {
		policy.PasswordHistory = 5
	}
	if policy.PasswordExpiryDays == 0 {
		policy.PasswordExpiryDays = 90
	}

	// Login attempt limit default value
	if c.LoginAttempt.Enabled {
		if c.LoginAttempt.MaxAttempts == 0 {
			c.LoginAttempt.MaxAttempts = 5
		}
		if c.LoginAttempt.LockoutDuration == 0 {
			c.LoginAttempt.LockoutDuration = 30 * time.Minute
		}
		if c.LoginAttempt.Storage == "" {
			c.LoginAttempt.Storage = "redis"
		}
		if c.LoginAttempt.RedisKeyPrefix == "" {
			c.LoginAttempt.RedisKeyPrefix = "auth:login_attempt:"
		}
	}
}

// Validate configuration
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}

	// At least one authentication method must be enabled
	if len(c.Providers) == 0 {
		return errors.New("至少需要启用一种认证方式")
	}

	// Password policy validation
	if c.Password.Enabled {
		policy := c.Password.Policy
		if policy.MinLength < 1 || policy.MinLength > policy.MaxLength {
			return errors.New("密码最小长度配置无效")
		}

		if c.Password.BcryptCost < 4 || c.Password.BcryptCost > 31 {
			return errors.New("bcrypt cost 必须在 4-31 之间")
		}
	}

	// Login attempt limit validation
	if c.LoginAttempt.Enabled {
		if c.LoginAttempt.MaxAttempts < 1 {
			return errors.New("最大登录尝试次数必须 >= 1")
		}
		if c.LoginAttempt.Storage != "redis" && c.LoginAttempt.Storage != "memory" {
			return errors.New("登录尝试存储方式只支持 redis 或 memory")
		}
	}

	return nil
}

// Check if the specified authentication method is enabled
func (c *Config) IsProviderEnabled(provider string) bool {
	for _, p := range c.Providers {
		if p == provider {
			return true
		}
	}
	return false
}
