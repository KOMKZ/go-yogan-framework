package health

import "time"

// Health check configuration
type Config struct {
	Enabled bool          `mapstructure:"enabled"`
	Timeout time.Duration `mapstructure:"timeout"`
}

// Return default configuration
func DefaultConfig() Config {
	return Config{
		Enabled: true,
		Timeout: 5 * time.Second,
	}
}
