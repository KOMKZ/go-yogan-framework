package database

import (
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/trace"
)

// Test MySQL DSN (from admin-api config)
const testDSN = "root:123123@tcp(localhost:3306)/futurelz_db?charset=utf8mb4&parseTime=True&loc=Local"

func getTestLogger() *logger.CtxZapLogger {
	return logger.GetLogger("test")
}

// TestDefaultConfig tests default configuration
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "mysql", cfg.Driver)
	assert.Equal(t, 100, cfg.MaxOpenConns)
	assert.Equal(t, 10, cfg.MaxIdleConns)
	assert.Equal(t, 3600*time.Second, cfg.ConnMaxLifetime)
	assert.True(t, cfg.EnableLog)
	assert.Equal(t, 200*time.Millisecond, cfg.SlowThreshold)
	assert.True(t, cfg.EnableAudit)
	assert.False(t, cfg.TraceSQL)
	assert.Equal(t, 1000, cfg.TraceSQLMaxLen)
}

// TestConfig_Validate tests configuration validation
func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Driver: "mysql",
				DSN:    testDSN,
			},
			wantErr: false,
		},
		{
			name: "empty DSN",
			config: Config{
				Driver: "mysql",
				DSN:    "",
			},
			wantErr: true,
		},
		{
			name: "empty driver uses default",
			config: Config{
				Driver: "",
				DSN:    testDSN,
			},
			wantErr: false,
		},
		{
			name: "applies defaults for zero values",
			config: Config{
				Driver:          "mysql",
				DSN:             testDSN,
				MaxOpenConns:    0,
				MaxIdleConns:    0,
				ConnMaxLifetime: 0,
				SlowThreshold:   0,
				TraceSQLMaxLen:  0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Verify defaults are applied
				if tt.config.MaxOpenConns == 0 {
					assert.Equal(t, 100, tt.config.MaxOpenConns)
				}
			}
		})
	}
}

// TestNewManager_NilLogger tests that nil logger returns error
func TestNewManager_NilLogger(t *testing.T) {
	configs := map[string]Config{
		"test": {
			Driver: "mysql",
			DSN:    testDSN,
		},
	}

	manager, err := NewManager(configs, nil, nil)
	assert.Error(t, err)
	assert.Nil(t, manager)
	assert.Contains(t, err.Error(), "logger cannot be nil")
}

// TestNewManager_InvalidConfig tests invalid configuration
func TestNewManager_InvalidConfig(t *testing.T) {
	configs := map[string]Config{
		"test": {
			Driver: "mysql",
			DSN:    "", // Empty DSN
		},
	}

	manager, err := NewManager(configs, nil, getTestLogger())
	assert.Error(t, err)
	assert.Nil(t, manager)
	assert.Contains(t, err.Error(), "invalid config")
}

// TestNewManager_Success tests successful manager creation
func TestNewManager_Success(t *testing.T) {
	configs := map[string]Config{
		"master": {
			Driver:          "mysql",
			DSN:             testDSN,
			MaxOpenConns:    50,
			MaxIdleConns:    10,
			ConnMaxLifetime: 3600 * time.Second,
		},
	}

	manager, err := NewManager(configs, nil, getTestLogger())
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
	}
	require.NoError(t, err)
	require.NotNil(t, manager)
	defer manager.Close()

	// Verify DB instance exists
	db := manager.DB("master")
	assert.NotNil(t, db)

	// Verify non-existent DB returns nil
	nilDB := manager.DB("nonexistent")
	assert.Nil(t, nilDB)
}

// TestManager_DB tests DB retrieval
func TestManager_DB(t *testing.T) {
	configs := map[string]Config{
		"master": {
			Driver: "mysql",
			DSN:    testDSN,
		},
	}

	manager, err := NewManager(configs, nil, getTestLogger())
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
	}
	require.NoError(t, err)
	defer manager.Close()

	// Test existing DB
	db := manager.DB("master")
	assert.NotNil(t, db)

	// Test non-existent DB
	nilDB := manager.DB("nonexistent")
	assert.Nil(t, nilDB)
}

// TestManager_Ping tests database ping
func TestManager_Ping(t *testing.T) {
	configs := map[string]Config{
		"master": {
			Driver: "mysql",
			DSN:    testDSN,
		},
	}

	manager, err := NewManager(configs, nil, getTestLogger())
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
	}
	require.NoError(t, err)
	defer manager.Close()

	// Test ping all DBs
	err = manager.Ping()
	assert.NoError(t, err)
}

// TestManager_Stats tests database stats
func TestManager_Stats(t *testing.T) {
	configs := map[string]Config{
		"master": {
			Driver: "mysql",
			DSN:    testDSN,
		},
	}

	manager, err := NewManager(configs, nil, getTestLogger())
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
	}
	require.NoError(t, err)
	defer manager.Close()

	// Test stats for existing DB
	stats, err := manager.Stats("master")
	assert.NoError(t, err)
	assert.NotNil(t, stats)

	// Test stats for non-existent DB
	_, err = manager.Stats("nonexistent")
	assert.Error(t, err)
}

// TestManager_GetDBNames tests getting all DB names
func TestManager_GetDBNames(t *testing.T) {
	configs := map[string]Config{
		"master": {
			Driver: "mysql",
			DSN:    testDSN,
		},
		"slave": {
			Driver: "mysql",
			DSN:    testDSN,
		},
	}

	manager, err := NewManager(configs, nil, getTestLogger())
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
	}
	require.NoError(t, err)
	defer manager.Close()

	names := manager.GetDBNames()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "master")
	assert.Contains(t, names, "slave")
}

// TestManager_Close tests manager close
func TestManager_Close(t *testing.T) {
	configs := map[string]Config{
		"master": {
			Driver: "mysql",
			DSN:    testDSN,
		},
	}

	manager, err := NewManager(configs, nil, getTestLogger())
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
	}
	require.NoError(t, err)

	err = manager.Close()
	assert.NoError(t, err)
}

// TestManager_Shutdown tests manager shutdown
func TestManager_Shutdown(t *testing.T) {
	configs := map[string]Config{
		"master": {
			Driver: "mysql",
			DSN:    testDSN,
		},
	}

	manager, err := NewManager(configs, nil, getTestLogger())
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
	}
	require.NoError(t, err)

	err = manager.Shutdown()
	assert.NoError(t, err)
}

// TestManager_SetOtelPlugin tests setting OTel plugin
func TestManager_SetOtelPlugin(t *testing.T) {
	configs := map[string]Config{
		"master": {
			Driver: "mysql",
			DSN:    testDSN,
		},
	}

	manager, err := NewManager(configs, nil, getTestLogger())
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
	}
	require.NoError(t, err)
	defer manager.Close()

	// Create tracer provider
	tp := trace.NewTracerProvider()
	plugin := NewOtelPlugin(tp)

	// Set plugin
	err = manager.SetOtelPlugin(plugin)
	assert.NoError(t, err)
}

// TestManager_UnsupportedDriver tests unsupported driver
func TestManager_UnsupportedDriver(t *testing.T) {
	configs := map[string]Config{
		"test": {
			Driver: "unsupported",
			DSN:    "some-dsn",
		},
	}

	manager, err := NewManager(configs, nil, getTestLogger())
	assert.Error(t, err)
	assert.Nil(t, manager)
	assert.Contains(t, err.Error(), "unsupported driver")
}

// TestManager_SetMetricsPlugin tests setting metrics plugin
func TestManager_SetMetricsPlugin(t *testing.T) {
	configs := map[string]Config{
		"master": {
			Driver: "mysql",
			DSN:    testDSN,
		},
	}

	manager, err := NewManager(configs, nil, getTestLogger())
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
	}
	require.NoError(t, err)
	defer manager.Close()

	// Create metrics
	db := manager.DB("master")
	metrics, err := NewDBMetrics(db, false, 0.2)
	require.NoError(t, err)

	// Set metrics plugin
	err = manager.SetMetricsPlugin("master", metrics)
	assert.NoError(t, err)

	// Test with non-existent DB
	err = manager.SetMetricsPlugin("nonexistent", metrics)
	assert.Error(t, err)
}
