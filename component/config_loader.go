package component

// ConfigLoader 配置加载器接口
//
// 提供统一的配置读取能力，组件通过此接口读取自己的配置
// 避免组件依赖具体的配置结构（如 AppConfig）
type ConfigLoader interface {
	// Get 获取配置项
	//
	// 参数：
	//   key: 配置键（如 "redis.main.host"）
	//
	// 返回：
	//   interface{}: 配置值
	Get(key string) interface{}

	// Unmarshal 将配置反序列化到结构体
	//
	// 参数：
	//   key: 配置键（如 "redis" 会读取整个 redis 配置段）
	//   v: 目标结构体指针
	//
	// 返回：
	//   error: 反序列化失败时返回错误
	//
	// 示例：
	//   var redisConfigs map[string]redis.Config
	//   if err := loader.Unmarshal("redis", &redisConfigs); err != nil {
	//       return err
	//   }
	Unmarshal(key string, v interface{}) error

	// GetString 获取字符串配置
	GetString(key string) string

	// GetInt 获取整数配置
	GetInt(key string) int

	// GetBool 获取布尔配置
	GetBool(key string) bool

	// IsSet 检查配置项是否存在
	IsSet(key string) bool
}

