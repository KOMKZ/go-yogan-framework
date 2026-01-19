package application

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// AppFlags 应用启动标志
type AppFlags struct {
	ConfigDir string // 配置目录路径
	Env       string // 运行环境
	Port      int    // 服务端口（0表示使用配置文件中的值）
	Address   string // 服务地址（空表示使用配置文件中的值）
}

// ParseFlags 解析命令行标志和环境变量
//
// 参数：
//   - appName: 应用名称（如 "user-api"），用于构造环境变量前缀
//   - defaultConfigDir: 默认配置目录（如 "../configs/user-api"）
//
// 返回：
//   - *AppFlags: 解析后的标志值
//
// 优先级：
//  1. 命令行参数 --config-dir、--env、--port、--address（最高优先级）
//  2. 环境变量 {APP_NAME}_CONFIG_DIR、{APP_NAME}_ENV、{APP_NAME}_PORT、{APP_NAME}_ADDRESS
//  3. 配置文件中的值（默认）
//
// 示例：
//
//	// user-api 应用
//	flags := application.ParseFlags("user-api", "../configs/user-api")
//	// 环境变量：USER_API_CONFIG_DIR、USER_API_ENV、USER_API_PORT、USER_API_ADDRESS
//	// 命令行：--port 8081 --address 0.0.0.0
//
//	// auth-app 应用（启动第二个实例）
//	flags := application.ParseFlags("auth-app", "../configs/auth-app")
//	// go run main.go --port 9003 --address 192.168.1.100
//	// 环境变量：AUTH_APP_PORT、AUTH_APP_ADDRESS
func ParseFlags(appName string, defaultConfigDir string) *AppFlags {
	// 构造环境变量前缀（转大写，- 替换为 _）
	envPrefix := strings.ToUpper(strings.ReplaceAll(appName, "-", "_"))
	configDirEnvKey := envPrefix + "_CONFIG_DIR"
	envEnvKey := envPrefix + "_ENV"
	portEnvKey := envPrefix + "_PORT"
	addressEnvKey := envPrefix + "_ADDRESS"

	var configDir string
	var env string
	var port int
	var address string

	// 定义命令行标志
	flag.StringVar(&configDir, "config-dir", "",
		"配置目录路径（默认：从 "+configDirEnvKey+" 环境变量读取，或 "+defaultConfigDir+"）")
	flag.StringVar(&env, "env", "",
		"运行环境（dev/test/prod，默认从 "+envEnvKey+" 环境变量读取）")
	flag.IntVar(&port, "port", 0,
		"服务端口（0表示使用配置文件，默认从 "+portEnvKey+" 环境变量读取）")
	flag.StringVar(&address, "address", "",
		"服务地址（空表示使用配置文件，默认从 "+addressEnvKey+" 环境变量读取）")

	// 解析命令行参数
	flag.Parse()

	// 读取环境变量
	envConfigDir := os.Getenv(configDirEnvKey)
	envEnv := os.Getenv(envEnvKey)
	envPort := os.Getenv(portEnvKey)
	envAddress := os.Getenv(addressEnvKey)

	// 确定最终配置目录
	// 优先级：命令行参数 > 环境变量 > 默认值
	finalConfigDir := configDir
	if finalConfigDir == "" {
		if envConfigDir != "" {
			finalConfigDir = envConfigDir
		} else {
			finalConfigDir = defaultConfigDir
		}
	}

	// 确定最终环境
	// 优先级：命令行参数 > 环境变量
	finalEnv := env
	if finalEnv == "" {
		finalEnv = envEnv
	}

	// 确定最终端口
	// 优先级：命令行参数 > 环境变量 > 0（表示使用配置文件）
	finalPort := port
	if finalPort == 0 && envPort != "" {
		// 尝试解析环境变量中的端口
		fmt.Sscanf(envPort, "%d", &finalPort)
	}

	// 确定最终地址
	// 优先级：命令行参数 > 环境变量 > 空（表示使用配置文件）
	finalAddress := address
	if finalAddress == "" {
		finalAddress = envAddress
	}

	// 如果指定了环境，设置到 APP_ENV（供 config.Loader 使用）
	if finalEnv != "" {
		os.Setenv("APP_ENV", finalEnv)
	}

	// 调试输出最终结果
	if os.Getenv("APP_DEBUG") == "1" {
		//fmt.Printf("[DEBUG] 最终配置目录: %s\n", finalConfigDir)
		//fmt.Printf("[DEBUG] 最终环境: %s\n", finalEnv)
		//fmt.Printf("[DEBUG] 最终端口: %d\n", finalPort)
		//fmt.Printf("[DEBUG] 最终地址: %s\n", finalAddress)
		//fmt.Println("---")
	}

	return &AppFlags{
		ConfigDir: finalConfigDir,
		Env:       finalEnv,
		Port:      finalPort,
		Address:   finalAddress,
	}
}
