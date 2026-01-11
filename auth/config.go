package auth

import (
	"errors"
	"time"
)

// Config 认证组件配置
type Config struct {
	Enabled bool `yaml:"enabled" mapstructure:"enabled"` // 是否启用认证组件

	// 支持的认证方式
	Providers []string `yaml:"providers" mapstructure:"providers"` // password, oauth2, api_key, basic_auth

	// 密码认证配置
	Password PasswordConfig `yaml:"password" mapstructure:"password"`

	// OAuth2.0 配置（未来扩展）
	// OAuth2 OAuth2Config `yaml:"oauth2" mapstructure:"oauth2"`

	// API Key 配置（未来扩展）
	// APIKey APIKeyConfig `yaml:"api_key" mapstructure:"api_key"`

	// 登录尝试限制
	LoginAttempt LoginAttemptConfig `yaml:"login_attempt" mapstructure:"login_attempt"`
}

// PasswordConfig 密码认证配置
type PasswordConfig struct {
	Enabled    bool           `yaml:"enabled" mapstructure:"enabled"`         // 是否启用密码认证
	Policy     PasswordPolicy `yaml:"policy" mapstructure:"policy"`           // 密码策略
	BcryptCost int            `yaml:"bcrypt_cost" mapstructure:"bcrypt_cost"` // bcrypt cost（10-14，推荐 12）
}

// PasswordPolicy 密码策略
type PasswordPolicy struct {
	// 复杂度要求
	MinLength          int  `yaml:"min_length" mapstructure:"min_length"`                     // 最小长度
	MaxLength          int  `yaml:"max_length" mapstructure:"max_length"`                     // 最大长度
	RequireUppercase   bool `yaml:"require_uppercase" mapstructure:"require_uppercase"`       // 至少 1 个大写字母
	RequireLowercase   bool `yaml:"require_lowercase" mapstructure:"require_lowercase"`       // 至少 1 个小写字母
	RequireDigit       bool `yaml:"require_digit" mapstructure:"require_digit"`               // 至少 1 个数字
	RequireSpecialChar bool `yaml:"require_special_char" mapstructure:"require_special_char"` // 至少 1 个特殊字符

	// 安全策略
	PasswordHistory    int `yaml:"password_history" mapstructure:"password_history"`         // 禁止重复最近 N 次密码
	PasswordExpiryDays int `yaml:"password_expiry_days" mapstructure:"password_expiry_days"` // 密码 N 天过期

	// 弱密码黑名单
	Blacklist []string `yaml:"blacklist" mapstructure:"blacklist"`
}

// LoginAttemptConfig 登录尝试限制配置
type LoginAttemptConfig struct {
	Enabled         bool          `yaml:"enabled" mapstructure:"enabled"`                   // 是否启用
	MaxAttempts     int           `yaml:"max_attempts" mapstructure:"max_attempts"`         // 最大尝试次数
	LockoutDuration time.Duration `yaml:"lockout_duration" mapstructure:"lockout_duration"` // 锁定时长
	Storage         string        `yaml:"storage" mapstructure:"storage"`                   // 存储方式：redis, memory
	RedisKeyPrefix  string        `yaml:"redis_key_prefix" mapstructure:"redis_key_prefix"` // Redis 键前缀
}

// ApplyDefaults 应用默认值
func (c *Config) ApplyDefaults() {
	// 默认启用密码认证
	if len(c.Providers) == 0 {
		c.Providers = []string{"password"}
	}

	// 密码认证默认值
	if c.Password.BcryptCost == 0 {
		c.Password.BcryptCost = 12 // 推荐值
	}

	// 密码策略默认值
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

	// 登录尝试限制默认值
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

// Validate 验证配置
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}

	// 至少启用一种认证方式
	if len(c.Providers) == 0 {
		return errors.New("至少需要启用一种认证方式")
	}

	// 密码策略验证
	if c.Password.Enabled {
		policy := c.Password.Policy
		if policy.MinLength < 1 || policy.MinLength > policy.MaxLength {
			return errors.New("密码最小长度配置无效")
		}

		if c.Password.BcryptCost < 4 || c.Password.BcryptCost > 31 {
			return errors.New("bcrypt cost 必须在 4-31 之间")
		}
	}

	// 登录尝试限制验证
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

// IsProviderEnabled 检查指定认证方式是否启用
func (c *Config) IsProviderEnabled(provider string) bool {
	for _, p := range c.Providers {
		if p == provider {
			return true
		}
	}
	return false
}
