package config

import (
	"os"
	"strings"
)

// EnvSource 环境变量数据源
type EnvSource struct {
	prefix   string // 环境变量前缀，如 "APP"
	priority int
	bindings map[string]string // key 映射，如 "grpc.server.port" -> "GRPC_SERVER_PORT"
}

// NewEnvSource 创建环境变量数据源
func NewEnvSource(prefix string, priority int) *EnvSource {
	return &EnvSource{
		prefix:   prefix,
		priority: priority,
		bindings: make(map[string]string),
	}
}

// AddBinding 添加 key 映射
// 例如：AddBinding("grpc.server.port", "GRPC_SERVER_PORT")
func (s *EnvSource) AddBinding(key, envKey string) {
	s.bindings[key] = envKey
}

// Name 数据源名称
func (s *EnvSource) Name() string {
	return "env:" + s.prefix
}

// Priority 优先级
func (s *EnvSource) Priority() int {
	return s.priority
}

// Load 加载环境变量配置
func (s *EnvSource) Load() (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// 如果有明确的 bindings，使用 bindings
	if len(s.bindings) > 0 {
		for key, envKey := range s.bindings {
			fullEnvKey := envKey
			if s.prefix != "" && !strings.HasPrefix(envKey, s.prefix+"_") {
				fullEnvKey = s.prefix + "_" + envKey
			}

			if value := os.Getenv(fullEnvKey); value != "" {
				result[key] = value
			}
		}
		return result, nil
	}

	// 否则，扫描所有环境变量（前缀匹配）
	if s.prefix == "" {
		return result, nil
	}

	prefix := s.prefix + "_"
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		if strings.HasPrefix(key, prefix) {
			// 转换为配置 key：APP_GRPC_SERVER_PORT -> grpc.server.port
			configKey := strings.TrimPrefix(key, prefix)
			configKey = strings.ToLower(configKey)
			configKey = strings.ReplaceAll(configKey, "_", ".")
			result[configKey] = value
		}
	}

	return result, nil
}

