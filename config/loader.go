package config

import (
	"fmt"
	"sort"

	"github.com/spf13/viper"
)

// Loader 配置加载器（支持多数据源）
type Loader struct {
	sources      []ConfigSource           // 数据源列表
	mergedConfig map[string]interface{}   // 合并后的配置
	v            *viper.Viper             // Viper 实例（用于兼容）
	loadedFiles  []string                 // 已加载的文件列表（用于日志）
}

// NewLoader 创建配置加载器
func NewLoader() *Loader {
	return &Loader{
		sources:      make([]ConfigSource, 0),
		mergedConfig: make(map[string]interface{}),
		v:            viper.New(),
		loadedFiles:  make([]string, 0),
	}
}

// AddSource 添加配置数据源
func (l *Loader) AddSource(source ConfigSource) {
	l.sources = append(l.sources, source)
}

// Load 加载并合并所有数据源
func (l *Loader) Load() error {
	// 1. 按优先级排序（从低到高）
	sort.Slice(l.sources, func(i, j int) bool {
		return l.sources[i].Priority() < l.sources[j].Priority()
	})

	// 2. 依次加载并合并
	l.mergedConfig = make(map[string]interface{})
	for _, source := range l.sources {
		data, err := source.Load()
		if err != nil {
			return fmt.Errorf("加载数据源 %s 失败: %w", source.Name(), err)
		}

		// 记录文件数据源
		if fileSource, ok := source.(*FileSource); ok {
			l.loadedFiles = append(l.loadedFiles, fileSource.path)
		}

		// 合并数据（高优先级覆盖低优先级）
		l.mergeFlat(data)
	}

	// 3. 将合并后的配置同步到 Viper（用于兼容现有代码）
	l.syncToViper()

	return nil
}

// mergeFlat 合并扁平化的配置（key 为点号分隔）
func (l *Loader) mergeFlat(data map[string]interface{}) {
	for key, value := range data {
		l.mergedConfig[key] = value
	}
}

// syncToViper 将合并后的配置同步到 Viper
func (l *Loader) syncToViper() {
	// 将扁平化的 map 转换为嵌套 map
	nested := l.unflattenMap(l.mergedConfig)

	// 清空 Viper 并重新设置
	l.v = viper.New()
	for key, value := range nested {
		l.v.Set(key, value)
	}
}

// unflattenMap 将扁平化的 map 转换为嵌套 map
// 例如：{"grpc.server.port": 9002} -> {"grpc": {"server": {"port": 9002}}}
func (l *Loader) unflattenMap(flat map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range flat {
		l.setNestedValue(result, key, value)
	}

	return result
}

// setNestedValue 设置嵌套 map 的值
func (l *Loader) setNestedValue(m map[string]interface{}, key string, value interface{}) {
	keys := splitKey(key)
	if len(keys) == 0 {
		return
	}

	// 最后一个 key
	if len(keys) == 1 {
		m[keys[0]] = value
		return
	}

	// 递归创建嵌套 map
	current := m
	for i := 0; i < len(keys)-1; i++ {
		k := keys[i]
		if _, ok := current[k]; !ok {
			current[k] = make(map[string]interface{})
		}

		// 类型断言
		if nested, ok := current[k].(map[string]interface{}); ok {
			current = nested
		} else {
			// 如果已存在的不是 map，覆盖它
			newMap := make(map[string]interface{})
			current[k] = newMap
			current = newMap
		}
	}

	// 设置最终值
	current[keys[len(keys)-1]] = value
}

// splitKey 分割配置 key
func splitKey(key string) []string {
	if key == "" {
		return []string{}
	}

	result := make([]string, 0)
	current := ""

	for _, ch := range key {
		if ch == '.' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}

	if current != "" {
		result = append(result, current)
	}

	return result
}

// Unmarshal 解析配置到结构体
func (l *Loader) Unmarshal(v interface{}) error {
	return l.v.Unmarshal(v)
}

// Get 获取配置值
func (l *Loader) Get(key string) interface{} {
	return l.v.Get(key)
}

// GetString 获取字符串配置
func (l *Loader) GetString(key string) string {
	return l.v.GetString(key)
}

// GetInt 获取整数配置
func (l *Loader) GetInt(key string) int {
	return l.v.GetInt(key)
}

// GetBool 获取布尔配置
func (l *Loader) GetBool(key string) bool {
	return l.v.GetBool(key)
}

// IsSet 检查配置项是否存在
func (l *Loader) IsSet(key string) bool {
	return l.v.IsSet(key)
}

// AllSettings 获取所有配置
func (l *Loader) AllSettings() map[string]interface{} {
	return l.v.AllSettings()
}

// GetLoadedFiles 获取已加载的配置文件列表
func (l *Loader) GetLoadedFiles() []string {
	return l.loadedFiles
}

// GetViper 获取底层 Viper 实例
func (l *Loader) GetViper() *viper.Viper {
	return l.v
}

// Reload 重新加载配置
func (l *Loader) Reload() error {
	return l.Load()
}

