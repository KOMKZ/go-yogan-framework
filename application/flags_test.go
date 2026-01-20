package application

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAppFlags_Struct test AppFlags struct
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

// TestParseFlags test the ParseFlags function
func TestParseFlags(t *testing.T) {
	// Reset flag status (flag package is global state)
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	// Save and restore environment variables
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

	// Set environment variables
	os.Setenv("PARSE_TEST_CONFIG_DIR", "/env/config")
	os.Setenv("PARSE_TEST_ENV", "staging")
	os.Setenv("PARSE_TEST_PORT", "7070")
	os.Setenv("PARSE_TEST_ADDRESS", "10.0.0.1")

	// Call ParseFlags
	flags := ParseFlags("parse-test", "/default/config")

	// Verify the results (the environment variables should take effect)
	assert.Equal(t, "/env/config", flags.ConfigDir)
	assert.Equal(t, "staging", flags.Env)
	assert.Equal(t, 7070, flags.Port)
	assert.Equal(t, "10.0.0.1", flags.Address)
}

// TestParseFlags_DefaultValues test default values
func TestParseFlags_DefaultValues(t *testing.T) {
	// Reset flag status
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	// Clear related environment variables
	os.Unsetenv("DEFAULT_TEST_CONFIG_DIR")
	os.Unsetenv("DEFAULT_TEST_ENV")
	os.Unsetenv("DEFAULT_TEST_PORT")
	os.Unsetenv("DEFAULT_TEST_ADDRESS")

	flags := ParseFlags("default-test", "/my/default/path")

	// Without environment variables, default values should be used
	assert.Equal(t, "/my/default/path", flags.ConfigDir)
	assert.Equal(t, "", flags.Env)
	assert.Equal(t, 0, flags.Port)
	assert.Equal(t, "", flags.Address)
}
