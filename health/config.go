package health

import "time"

// Config 健康检查配置
type Config struct {
	Enabled bool          `mapstructure:"enabled"`
	Timeout time.Duration `mapstructure:"timeout"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		Enabled: true,
		Timeout: 5 * time.Second,
	}
}
