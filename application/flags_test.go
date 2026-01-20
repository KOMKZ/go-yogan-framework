package application

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAppFlags_Struct 测试 AppFlags 结构体
func TestAppFlags_Struct(t *testing.T) {
	flags := &AppFlags{
		ConfigDir: "/path/to/config",
		Env:       "test",
		Port:      8080,
		Address:   "0.0.0.0",
	}

	assert.Equal(t, "/path/to/config", flags.ConfigDir)
	assert.Equal(t, "test", flags.Env)
	assert.Equal(t, 8080, flags.Port)
	assert.Equal(t, "0.0.0.0", flags.Address)
}

// TestParseFlags 测试 ParseFlags 函数
func TestParseFlags(t *testing.T) {
	// 重置 flag 状态（flag 包是全局状态）
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	// 保存和恢复环境变量
	origConfigDir := os.Getenv("PARSE_TEST_CONFIG_DIR")
	origEnv := os.Getenv("PARSE_TEST_ENV")
	origPort := os.Getenv("PARSE_TEST_PORT")
	origAddress := os.Getenv("PARSE_TEST_ADDRESS")
	origAppEnv := os.Getenv("APP_ENV")
	defer func() {
		os.Setenv("PARSE_TEST_CONFIG_DIR", origConfigDir)
		os.Setenv("PARSE_TEST_ENV", origEnv)
		os.Setenv("PARSE_TEST_PORT", origPort)
		os.Setenv("PARSE_TEST_ADDRESS", origAddress)
		os.Setenv("APP_ENV", origAppEnv)
	}()

	// 设置环境变量
	os.Setenv("PARSE_TEST_CONFIG_DIR", "/env/config")
	os.Setenv("PARSE_TEST_ENV", "staging")
	os.Setenv("PARSE_TEST_PORT", "7070")
	os.Setenv("PARSE_TEST_ADDRESS", "10.0.0.1")

	// 调用 ParseFlags
	flags := ParseFlags("parse-test", "/default/config")

	// 验证结果（环境变量应该生效）
	assert.Equal(t, "/env/config", flags.ConfigDir)
	assert.Equal(t, "staging", flags.Env)
	assert.Equal(t, 7070, flags.Port)
	assert.Equal(t, "10.0.0.1", flags.Address)
}

// TestParseFlags_DefaultValues 测试默认值
func TestParseFlags_DefaultValues(t *testing.T) {
	// 重置 flag 状态
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	// 清除相关环境变量
	os.Unsetenv("DEFAULT_TEST_CONFIG_DIR")
	os.Unsetenv("DEFAULT_TEST_ENV")
	os.Unsetenv("DEFAULT_TEST_PORT")
	os.Unsetenv("DEFAULT_TEST_ADDRESS")

	flags := ParseFlags("default-test", "/my/default/path")

	// 没有环境变量，应该使用默认值
	assert.Equal(t, "/my/default/path", flags.ConfigDir)
	assert.Equal(t, "", flags.Env)
	assert.Equal(t, 0, flags.Port)
	assert.Equal(t, "", flags.Address)
}
