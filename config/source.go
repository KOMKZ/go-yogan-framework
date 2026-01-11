package config

// ConfigSource 配置数据源接口
// 所有配置来源（文件、环境变量、命令行参数等）都实现此接口
type ConfigSource interface {
	// Name 数据源名称（用于日志和调试）
	Name() string

	// Priority 优先级（数字越大优先级越高）
	// 建议值：
	//   - 默认值：1
	//   - 配置文件(config.yaml)：10
	//   - 环境配置文件(dev.yaml)：20
	//   - 环境变量：50
	//   - 命令行参数：100
	Priority() int

	// Load 加载配置数据
	// 返回的 map 使用点号分隔的 key，如 "grpc.server.port"
	Load() (map[string]interface{}, error)
}

