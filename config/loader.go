package config

import (
	"fmt"
	"sort"

	"github.com/spf13/viper"
)

// Loader configuration loader (supporting multiple data sources)
type Loader struct {
	sources      []ConfigSource           // data source list
	mergedConfig map[string]interface{}   // merged configuration
	v            *viper.Viper             // Viper instance (for compatibility)
	loadedFiles  []string                 // List of loaded files (for logging)
}

// Create configuration loader
func NewLoader() *Loader {
	return &Loader{
		sources:      make([]ConfigSource, 0),
		mergedConfig: make(map[string]interface{}),
		v:            viper.New(),
		loadedFiles:  make([]string, 0),
	}
}

// AddSource add configuration data source
func (l *Loader) AddSource(source ConfigSource) {
	l.sources = append(l.sources, source)
}

// Load and merge all data sources
func (l *Loader) Load() error {
	// 1. Sort by priority (from low to high)
	sort.Slice(l.sources, func(i, j int) bool {
		return l.sources[i].Priority() < l.sources[j].Priority()
	})

	// 2. Load and merge in sequence
	l.mergedConfig = make(map[string]interface{})
	for _, source := range l.sources {
		data, err := source.Load()
		if err != nil {
			return fmt.Errorf("加载数据源 %s 失败: %w", source.Name(), err)
		}

		// Log file data source
		if fileSource, ok := source.(*FileSource); ok {
			l.loadedFiles = append(l.loadedFiles, fileSource.path)
		}

		// Merge data (higher priority overrides lower priority)
		l.mergeFlat(data)
	}

	// 3. Sync the merged configuration to Viper (for compatibility with existing code)
	l.syncToViper()

	return nil
}

// mergeFlat merges flattened configuration (keys are dot-separated)
func (l *Loader) mergeFlat(data map[string]interface{}) {
	for key, value := range data {
		l.mergedConfig[key] = value
	}
}

// syncToViper synchronizes the merged configuration to Viper
func (l *Loader) syncToViper() {
	// Convert the flat map to a nested map
	nested := l.unflattenMap(l.mergedConfig)

	// Clear Viper and reset
	l.v = viper.New()
	for key, value := range nested {
		l.v.Set(key, value)
	}
}

// unflattenMap converts a flattened map to a nested map
// For example: {"grpc.server.port": 9002} -> {"grpc": {"server": {"port": 9002}}}
func (l *Loader) unflattenMap(flat map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range flat {
		l.setNestedValue(result, key, value)
	}

	return result
}

// Set the value of a nested map
func (l *Loader) setNestedValue(m map[string]interface{}, key string, value interface{}) {
	keys := splitKey(key)
	if len(keys) == 0 {
		return
	}

	// last key
	if len(keys) == 1 {
		m[keys[0]] = value
		return
	}

	// Recursively create nested maps
	current := m
	for i := 0; i < len(keys)-1; i++ {
		k := keys[i]
		if _, ok := current[k]; !ok {
			current[k] = make(map[string]interface{})
		}

		// type assertion
		if nested, ok := current[k].(map[string]interface{}); ok {
			current = nested
		} else {
			// If it is not already a map, override it
			newMap := make(map[string]interface{})
			current[k] = newMap
			current = newMap
		}
	}

	// Set final value
	current[keys[len(keys)-1]] = value
}

// splitKey configuration key split
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

// Unmarshal parse configuration into struct
func (l *Loader) Unmarshal(v interface{}) error {
	return l.v.Unmarshal(v)
}

// Get configuration value
func (l *Loader) Get(key string) interface{} {
	return l.v.Get(key)
}

// GetString Get string configuration
func (l *Loader) GetString(key string) string {
	return l.v.GetString(key)
}

// Get integer configuration
func (l *Loader) GetInt(key string) int {
	return l.v.GetInt(key)
}

// GetBool Get boolean configuration
func (l *Loader) GetBool(key string) bool {
	return l.v.GetBool(key)
}

// Check if the configuration item exists
func (l *Loader) IsSet(key string) bool {
	return l.v.IsSet(key)
}

// Get all settings
func (l *Loader) AllSettings() map[string]interface{} {
	return l.v.AllSettings()
}

// GetLoadedFiles Retrieve the list of loaded configuration files
func (l *Loader) GetLoadedFiles() []string {
	return l.loadedFiles
}

// GetViper获取底层Viper实例
func (l *Loader) GetViper() *viper.Viper {
	return l.v
}

// Reload reload configuration
func (l *Loader) Reload() error {
	return l.Load()
}

