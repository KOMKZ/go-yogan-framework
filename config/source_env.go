package config

import (
	"os"
	"strings"
)

// EnvSource environment variable data source
type EnvSource struct {
	prefix   string // Environment variable prefix, such as "APP"
	priority int
	bindings map[string]string // key mapping, such as "grpc.server.port" -> "GRPC_SERVER_PORT"
}

// NewEnvSource creates an environment variable data source
func NewEnvSource(prefix string, priority int) *EnvSource {
	return &EnvSource{
		prefix:   prefix,
		priority: priority,
		bindings: make(map[string]string),
	}
}

// AddBinding add key mapping
// For example: AddBinding("grpc.server.port", "GRPC_SERVER_PORT")
func (s *EnvSource) AddBinding(key, envKey string) {
	s.bindings[key] = envKey
}

// Data source name
func (s *EnvSource) Name() string {
	return "env:" + s.prefix
}

// Priority
func (s *EnvSource) Priority() int {
	return s.priority
}

// Load environment variable configuration
func (s *EnvSource) Load() (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// If there are explicit bindings, use the bindings
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

	// Otherwise, scan all environment variables (prefix matching)
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
			// Convert configuration key: APP_GRPC_SERVER_PORT -> grpc.server.port
			configKey := strings.TrimPrefix(key, prefix)
			configKey = strings.ToLower(configKey)
			configKey = strings.ReplaceAll(configKey, "_", ".")
			result[configKey] = value
		}
	}

	return result, nil
}

