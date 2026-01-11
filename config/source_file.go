package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// FileSource 文件配置数据源
type FileSource struct {
	path     string
	priority int
}

// NewFileSource 创建文件数据源
func NewFileSource(path string, priority int) *FileSource {
	return &FileSource{
		path:     path,
		priority: priority,
	}
}

// Name 数据源名称
func (s *FileSource) Name() string {
	return "file:" + s.path
}

// Priority 优先级
func (s *FileSource) Priority() int {
	return s.priority
}

// Load 加载文件配置
func (s *FileSource) Load() (map[string]interface{}, error) {
	// 检查文件是否存在
	if _, err := os.Stat(s.path); err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，返回空配置（非错误）
			return make(map[string]interface{}), nil
		}
		return nil, fmt.Errorf("访问配置文件失败 %s: %w", s.path, err)
	}

	// 使用 Viper 加载文件
	v := viper.New()
	v.SetConfigFile(s.path)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败 %s: %w", s.path, err)
	}

	// 转换为 flat map（带点号的 key）
	return flattenMap("", v.AllSettings()), nil
}

// flattenMap 将嵌套 map 展平为点号分隔的 key
// 例如：{"grpc": {"server": {"port": 9002}}} -> {"grpc.server.port": 9002}
func flattenMap(prefix string, data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range data {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		switch v := value.(type) {
		case map[string]interface{}:
			// 递归处理嵌套 map
			nested := flattenMap(fullKey, v)
			for nestedKey, nestedValue := range nested {
				result[nestedKey] = nestedValue
			}
		default:
			result[fullKey] = value
		}
	}

	return result
}

