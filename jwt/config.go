package jwt

import (
	"fmt"
	"time"
)

// Config JWT 配置
type Config struct {
	Enabled bool `yaml:"enabled" mapstructure:"enabled"` // 是否启用

	// 签名算法
	Algorithm string `yaml:"algorithm" mapstructure:"algorithm"` // HS256, HS384, HS512, RS256, RS384, RS512

	// 密钥配置
	Secret         string `yaml:"secret" mapstructure:"secret"`                     // 对称密钥（HS256）
	PrivateKeyPath string `yaml:"private_key_path" mapstructure:"private_key_path"` // 私钥路径（RS256）
	PublicKeyPath  string `yaml:"public_key_path" mapstructure:"public_key_path"`   // 公钥路径（RS256）

	// Token 配置
	AccessToken  AccessTokenConfig  `yaml:"access_token" mapstructure:"access_token"`
	RefreshToken RefreshTokenConfig `yaml:"refresh_token" mapstructure:"refresh_token"`

	// 黑名单配置
	Blacklist BlacklistConfig `yaml:"blacklist" mapstructure:"blacklist"`

	// 安全配置
	Security SecurityConfig `yaml:"security" mapstructure:"security"`
}

// AccessTokenConfig Access Token 配置
type AccessTokenConfig struct {
	TTL      time.Duration `yaml:"ttl" mapstructure:"ttl"`           // 有效期
	Issuer   string        `yaml:"issuer" mapstructure:"issuer"`     // 签发者
	Audience string        `yaml:"audience" mapstructure:"audience"` // 接收方
}

// RefreshTokenConfig Refresh Token 配置
type RefreshTokenConfig struct {
	Enabled bool          `yaml:"enabled" mapstructure:"enabled"` // 是否启用
	TTL     time.Duration `yaml:"ttl" mapstructure:"ttl"`         // 有效期
}

// BlacklistConfig 黑名单配置
type BlacklistConfig struct {
	Enabled         bool          `yaml:"enabled" mapstructure:"enabled"`                   // 是否启用
	Storage         string        `yaml:"storage" mapstructure:"storage"`                   // redis / memory
	RedisKeyPrefix  string        `yaml:"redis_key_prefix" mapstructure:"redis_key_prefix"` // Redis key 前缀
	CleanupInterval time.Duration `yaml:"cleanup_interval" mapstructure:"cleanup_interval"` // 内存模式清理间隔
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	EnableJTI       bool          `yaml:"enable_jti" mapstructure:"enable_jti"`               // 启用 JTI（防重放）
	EnableNotBefore bool          `yaml:"enable_not_before" mapstructure:"enable_not_before"` // 启用 NBF（延迟生效）
	ClockSkew       time.Duration `yaml:"clock_skew" mapstructure:"clock_skew"`               // 时钟偏移容忍
}

// Validate 验证配置
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}

	// 验证算法
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

	// 验证 TTL
	if c.AccessToken.TTL <= 0 {
		return fmt.Errorf("jwt: access token ttl must be positive")
	}

	if c.RefreshToken.Enabled && c.RefreshToken.TTL <= 0 {
		return fmt.Errorf("jwt: refresh token ttl must be positive")
	}

	// 验证黑名单存储
	if c.Blacklist.Enabled {
		if c.Blacklist.Storage != "redis" && c.Blacklist.Storage != "memory" {
			return fmt.Errorf("jwt: blacklist storage must be redis or memory")
		}
	}

	return nil
}

// ApplyDefaults 应用默认值
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
