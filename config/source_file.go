package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// FileSource file configuration data source
type FileSource struct {
	path     string
	priority int
}

// NewFileSource creates file data source
func NewFileSource(path string, priority int) *FileSource {
	return &FileSource{
		path:     path,
		priority: priority,
	}
}

// Data source name
func (s *FileSource) Name() string {
	return "file:" + s.path
}

// Priority
func (s *FileSource) Priority() int {
	return s.priority
}

// Load file configuration
func (s *FileSource) Load() (map[string]interface{}, error) {
	// Check if the file exists
	if _, err := os.Stat(s.path); err != nil {
		if os.IsNotExist(err) {
			// File does not exist, return empty configuration (not an error)
			return make(map[string]interface{}), nil
		}
		return nil, fmt.Errorf("访问配置文件失败 %s: %w", s.path, err)
	}

	// Use Viper to load file
	v := viper.New()
	v.SetConfigFile(s.path)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败 %s: %w", s.path, err)
	}

	// Convert to flat map (keys with dots)
	return flattenMap("", v.AllSettings()), nil
}

// flattenMap flattens nested maps into dot-separated keys
// For example: {"grpc": {"server": {"port": 9002}}} -> {"grpc.server.port": 9002}
func flattenMap(prefix string, data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range data {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		switch v := value.(type) {
		case map[string]interface{}:
			// Recursively process nested maps
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

