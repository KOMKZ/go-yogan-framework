package event

// Configure event component settings
type Config struct {
	Enabled    bool `mapstructure:"enabled"`
	PoolSize   int  `mapstructure:"pool_size"`
	SetAllSync bool `mapstructure:"set_all_sync"`
}

// Return default configuration
func DefaultConfig() Config {
	return Config{
		Enabled:  true,
		PoolSize: 100,
	}
}
