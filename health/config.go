package health

import (
	"fmt"
	"net"
	"time"
)

// Health check configuration
type Config struct {
	Enabled bool          `mapstructure:"enabled"`
	Timeout time.Duration `mapstructure:"timeout"`
	// Listen 是非 HTTP 应用（worker/cron）的独立健康检查监听地址（如 "0.0.0.0:9203"）。
	// 空值 = 不启动独立 listener：HTTP 应用在主端口上挂 /health 路由，无需本字段。
	Listen string `mapstructure:"listen"`
}

// Return default configuration
func DefaultConfig() Config {
	return Config{
		Enabled: true,
		Timeout: 5 * time.Second,
	}
}

// Validate checks the health configuration; enabled 时 timeout 必须为正，
// listen 非空时必须是合法 host:port（worker/cron 独立 listener 的启动前置）。
func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("health timeout must be positive when enabled, got %s", c.Timeout)
	}
	if c.Listen != "" {
		if _, _, err := net.SplitHostPort(c.Listen); err != nil {
			return fmt.Errorf("health listen %q is not a valid host:port: %w", c.Listen, err)
		}
	}
	return nil
}
