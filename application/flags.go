package application

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// Application startup flags
type AppFlags struct {
	ConfigDir string // Configure directory path
	Env       string // runtime environment
	Port      int    // service port (0 indicates using the value from the configuration file)
	Address   string // Service address (empty means use the value from the configuration file)
}

// ParseFlags parse command line flags and environment variables
//
// Parameters:
// - appName: Application name (e.g., "user-api"), used for constructing environment variable prefixes
// - defaultConfigDir: Default configuration directory (e.g., "../configs/user-api")
//
// Return:
// - *AppFlags: parsed flag values
//
// Priority:
// 1. Command line arguments --config-dir, --env, --port, --address (highest priority)
// 2. Environment variables {APP_NAME}_CONFIG_DIR, {APP_NAME}_ENV, {APP_NAME}_PORT, {APP_NAME}_ADDRESS
// 3. Default values in configuration file
//
// Example:
//
// // user-api application
//	flags := application.ParseFlags("user-api", "../configs/user-api")
// // Environment variables: USER_API_CONFIG_DIR, USER_API_ENV, USER_API_PORT, USER_API_ADDRESS
// // Command line: --port 8081 --address 0.0.0.0
//
// // auth-app application (launching a second instance)
//	flags := application.ParseFlags("auth-app", "../configs/auth-app")
//	// go run main.go --port 9003 --address 192.168.1.100
// // Environment variables: AUTH_APP_PORT, AUTH_APP_ADDRESS
func ParseFlags(appName string, defaultConfigDir string) *AppFlags {
	// Construct environment variable prefix (convert to uppercase, replace - with _)
	envPrefix := strings.ToUpper(strings.ReplaceAll(appName, "-", "_"))
	configDirEnvKey := envPrefix + "_CONFIG_DIR"
	envEnvKey := envPrefix + "_ENV"
	portEnvKey := envPrefix + "_PORT"
	addressEnvKey := envPrefix + "_ADDRESS"

	var configDir string
	var env string
	var port int
	var address string

	// Define command line flags
	flag.StringVar(&configDir, "config-dir", "",
		"配置目录路径（默认：从 "+configDirEnvKey+" 环境变量读取，或 "+defaultConfigDir+"）")
	flag.StringVar(&env, "env", "",
		"运行环境（dev/test/prod，默认从 "+envEnvKey+" 环境变量读取）")
	flag.IntVar(&port, "port", 0,
		"服务端口（0表示使用配置文件，默认从 "+portEnvKey+" 环境变量读取）")
	flag.StringVar(&address, "address", "",
		"服务地址（空表示使用配置文件，默认从 "+addressEnvKey+" 环境变量读取）")

	// Parse command line arguments
	flag.Parse()

	// Read environment variables
	envConfigDir := os.Getenv(configDirEnvKey)
	envEnv := os.Getenv(envEnvKey)
	envPort := os.Getenv(portEnvKey)
	envAddress := os.Getenv(addressEnvKey)

	// Determine final configuration directory
	// Priority: Command line arguments > Environment variables > Default values
	finalConfigDir := configDir
	if finalConfigDir == "" {
		if envConfigDir != "" {
			finalConfigDir = envConfigDir
		} else {
			finalConfigDir = defaultConfigDir
		}
	}

	// Determine final environment
	// Priority: Command-line arguments > Environment variables
	finalEnv := env
	if finalEnv == "" {
		finalEnv = envEnv
	}

	// Determine final port
	// Priority: Command line arguments > Environment variables > 0 (indicating use of configuration file)
	finalPort := port
	if finalPort == 0 && envPort != "" {
		// Try to parse the port from environment variables
		fmt.Sscanf(envPort, "%d", &finalPort)
	}

	// Determine final address
	// Priority: command line arguments > environment variables > none (indicating use of configuration file)
	finalAddress := address
	if finalAddress == "" {
		finalAddress = envAddress
	}

	// If an environment is specified, set it to APP_ENV (for use by config.Loader)
	if finalEnv != "" {
		os.Setenv("APP_ENV", finalEnv)
	}

	// Debug output of final result
	if os.Getenv("APP_DEBUG") == "1" {
		// fmt.Printf("[DEBUG] Final configuration directory: %s\n", finalConfigDir)
		// fmt.Printf("[DEBUG] Final environment: %s\n", finalEnv)
		// fmt.Printf("[DEBUG] Final port: %d\n", finalPort)
		// fmt.Printf("[DEBUG] Final address: %s\n", finalAddress)
		//fmt.Println("---")
	}

	return &AppFlags{
		ConfigDir: finalConfigDir,
		Env:       finalEnv,
		Port:      finalPort,
		Address:   finalAddress,
	}
}
