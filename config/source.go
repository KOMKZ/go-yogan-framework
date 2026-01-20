package config

// Configuration Source interface for configuration data sources
// All configuration sources (files, environment variables, command-line arguments, etc.) implement this interface
type ConfigSource interface {
	// Data source name (for logs and debugging)
	Name() string

	// Priority (Higher numerical value indicates higher priority)
	// Suggested value:
	// - Default value: 1
	// - Configuration file (config.yaml): 10
	// - Environment configuration file (dev.yaml): 20
	// - Environment variable: 50
	// - Command line argument: 100
	Priority() int

	// Load configuration data
	// The returned map uses keys separated by dots, such as "grpc.server.port"
	Load() (map[string]interface{}, error)
}

