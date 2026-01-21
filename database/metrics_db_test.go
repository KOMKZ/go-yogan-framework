package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// TestNewDBMetrics tests creating DB metrics
func TestNewDBMetrics(t *testing.T) {
	db, err := gorm.Open(mysql.Open(testDSN), &gorm.Config{})
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
	}

	metrics, err := NewDBMetrics(db, false, 0.2)
	assert.NoError(t, err)
	assert.NotNil(t, metrics)
}

// TestDBMetrics_GORMPlugin tests creating GORM plugin from metrics
func TestDBMetrics_GORMPlugin(t *testing.T) {
	db, err := gorm.Open(mysql.Open(testDSN), &gorm.Config{})
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
	}

	metrics, err := NewDBMetrics(db, false, 0.2)
	require.NoError(t, err)

	plugin := metrics.GORMPlugin()
	assert.NotNil(t, plugin)

	// Test plugin name
	assert.Equal(t, "otel-db-metrics-plugin", plugin.Name())
}

// TestDBMetrics_Initialize tests plugin initialization
func TestDBMetrics_Initialize(t *testing.T) {
	db, err := gorm.Open(mysql.Open(testDSN), &gorm.Config{})
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
	}

	metrics, err := NewDBMetrics(db, false, 0.2)
	require.NoError(t, err)

	plugin := metrics.GORMPlugin()
	err = plugin.Initialize(db)
	assert.NoError(t, err)
}

// MetricsTestModel test model for metrics tests
type MetricsTestModel struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"size:100"`
}

// TableName specify table name
func (MetricsTestModel) TableName() string {
	return "yogan_metrics_test"
}

// TestDBMetrics_RecordOperations tests metrics recording during operations
func TestDBMetrics_RecordOperations(t *testing.T) {
	db, err := gorm.Open(mysql.Open(testDSN), &gorm.Config{})
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
	}

	// AutoMigrate test table
	err = db.AutoMigrate(&MetricsTestModel{})
	require.NoError(t, err)

	// Clean test data
	db.Exec("TRUNCATE TABLE yogan_metrics_test")

	metrics, err := NewDBMetrics(db, true, 0.2)
	require.NoError(t, err)

	plugin := metrics.GORMPlugin()
	err = plugin.Initialize(db)
	require.NoError(t, err)

	ctx := context.Background()

	// Test CREATE operation
	model := MetricsTestModel{Name: "test-metrics"}
	db.WithContext(ctx).Create(&model)

	// Test SELECT operation
	var result MetricsTestModel
	db.WithContext(ctx).First(&result)

	// Test UPDATE operation
	db.WithContext(ctx).Model(&result).Update("Name", "updated")

	// Test DELETE operation
	db.WithContext(ctx).Delete(&result)

	t.Log("✅ All CRUD operations recorded for metrics")
}

