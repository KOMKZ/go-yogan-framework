package event

// Config 事件组件配置
type Config struct {
	Enabled  bool `mapstructure:"enabled"`
	PoolSize int  `mapstructure:"pool_size"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		Enabled:  true,
		PoolSize: 100,
	}
}
