package component

// ConfigLoader configuration loader interface
//
// Provides unified configuration reading capability, components read their own configurations through this interface
// Avoid component dependencies on specific configuration structures (such as AppConfig)
type ConfigLoader interface {
	// Get configuration item
	//
	// Parameters:
	// key: configuration key (e.g., "redis.main.host")
	//
	// Return:
	// interface{}: configuration value
	Get(key string) interface{}

	// Unmarshal deserializes the configuration into a struct
	//
	// Parameters:
	// key: Configuration key (e.g., "redis" reads the entire redis configuration section)
	// v: pointer to the target struct
	//
	// Return:
	// error: return error on deserialization failure
	//
	// Example:
	//   var redisConfigs map[string]redis.Config
	//   if err := loader.Unmarshal("redis", &redisConfigs); err != nil {
	//       return err
	//   }
	Unmarshal(key string, v interface{}) error

	// GetString Get string configuration
	GetString(key string) string

	// Get integer configuration
	GetInt(key string) int

	// GetBool Get boolean configuration
	GetBool(key string) bool

	// Check if the configuration item exists
	IsSet(key string) bool
}

