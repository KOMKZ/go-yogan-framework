package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Test MySQL DSN (from admin-api config)
const testMySQLDSN = "root:123123@tcp(localhost:3306)/futurelz_db?charset=utf8mb4&parseTime=True&loc=Local"

// TestModel test model
type TestModel struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"size:100"`
}

// TableName specify table name to avoid conflicts
func (TestModel) TableName() string {
	return "yogan_otel_plugin_test"
}

// setupTestDB creates a test database connection
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(mysql.Open(testMySQLDSN), &gorm.Config{})
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
	}

	// AutoMigrate test table
	err = db.AutoMigrate(&TestModel{})
	require.NoError(t, err)

	// Clean test data
	db.Exec("TRUNCATE TABLE yogan_otel_plugin_test")

	return db
}

// TestOtelPlugin_Initialize test plugin initialization
func TestOtelPlugin_Initialize(t *testing.T) {
	db := setupTestDB(t)

	// Create plugin
	plugin := NewOtelPlugin(nil)
	assert.NotNil(t, plugin)

	// Initialize plugin
	err := plugin.Initialize(db)
	assert.NoError(t, err)

	// Validate plugin name
	assert.Equal(t, "otel", plugin.Name())
}


// TestOtelPlugin_CreateSpan test creating Span
func TestOtelPlugin_CreateSpan(t *testing.T) {
	// Create SpanRecorder to capture Span
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))
	otel.SetTracerProvider(tracerProvider)
	defer otel.SetTracerProvider(trace.NewTracerProvider())

	db := setupTestDB(t)

	// Register plugin
	plugin := NewOtelPlugin(tracerProvider)
	err := plugin.Initialize(db)
	require.NoError(t, err)

	// Execute query (trigger Span creation)
	ctx := context.Background()
	db.WithContext(ctx).First(&TestModel{})

	// wait for Span processing
	spans := spanRecorder.Ended()
	require.NotEmpty(t, spans, "should have at least one Span")

	// Validate Span attributes
	span := spans[0]
	assert.Contains(t, span.Name(), "gorm")

	// Validate attributes
	attrs := span.Attributes()
	hasDBSystem := false
	for _, attr := range attrs {
		if string(attr.Key) == "db.system" {
			hasDBSystem = true
			assert.Equal(t, "gorm", attr.Value.AsString())
		}
	}
	assert.True(t, hasDBSystem, "should contain db.system attribute")
}

// TestOtelPlugin_QueryOperation test query operation
func TestOtelPlugin_QueryOperation(t *testing.T) {
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))

	db := setupTestDB(t)

	plugin := NewOtelPlugin(tracerProvider)
	err := plugin.Initialize(db)
	require.NoError(t, err)

	// Execute query
	ctx := context.Background()
	var result TestModel
	db.WithContext(ctx).Where("id = ?", 1).First(&result)

	// Validate Span
	spans := spanRecorder.Ended()
	require.NotEmpty(t, spans)

	t.Logf("✅ Query operation created %d Span(s)", len(spans))
}

// TestOtelPlugin_CreateOperation test create operation
func TestOtelPlugin_CreateOperation(t *testing.T) {
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))

	db := setupTestDB(t)

	plugin := NewOtelPlugin(tracerProvider)
	err := plugin.Initialize(db)
	require.NoError(t, err)

	// Execute creation
	ctx := context.Background()
	model := TestModel{Name: "test"}
	db.WithContext(ctx).Create(&model)

	// Validate Span
	spans := spanRecorder.Ended()
	require.NotEmpty(t, spans)

	t.Logf("✅ Create operation created %d Span(s)", len(spans))
}

// TestOtelPlugin_UpdateOperation test update operation
func TestOtelPlugin_UpdateOperation(t *testing.T) {
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))

	db := setupTestDB(t)

	plugin := NewOtelPlugin(tracerProvider)
	err := plugin.Initialize(db)
	require.NoError(t, err)

	// Create first
	ctx := context.Background()
	model := TestModel{Name: "test"}
	db.WithContext(ctx).Create(&model)

	// Clear records
	spanRecorder.Reset()

	// Execute update
	db.WithContext(ctx).Model(&model).Update("Name", "updated")

	// Validate Span
	spans := spanRecorder.Ended()
	require.NotEmpty(t, spans)

	t.Logf("✅ Update operation created %d Span(s)", len(spans))
}

// TestOtelPlugin_DeleteOperation test delete operation
func TestOtelPlugin_DeleteOperation(t *testing.T) {
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))

	db := setupTestDB(t)

	plugin := NewOtelPlugin(tracerProvider)
	err := plugin.Initialize(db)
	require.NoError(t, err)

	// Create first
	ctx := context.Background()
	model := TestModel{Name: "test"}
	db.WithContext(ctx).Create(&model)

	// Clear records
	spanRecorder.Reset()

	// Execute deletion
	db.WithContext(ctx).Delete(&model)

	// Validate Span
	spans := spanRecorder.Ended()
	require.NotEmpty(t, spans)

	t.Logf("✅ Delete operation created %d Span(s)", len(spans))
}

// TestOtelPlugin_WithParentSpan test with parent Span
func TestOtelPlugin_WithParentSpan(t *testing.T) {
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))

	db := setupTestDB(t)

	plugin := NewOtelPlugin(tracerProvider)
	err := plugin.Initialize(db)
	require.NoError(t, err)

	// Create parent Span
	tracer := tracerProvider.Tracer("test")
	ctx, parentSpan := tracer.Start(context.Background(), "parent-span")
	defer parentSpan.End()

	// Execute query (should inherit from parent Span)
	var result TestModel
	db.WithContext(ctx).First(&result)

	// Validate Span link
	spans := spanRecorder.Ended()
	require.NotEmpty(t, spans)

	// Find GORM Span
	var gormSpan trace.ReadOnlySpan
	for _, span := range spans {
		if span.Name() != "parent-span" {
			gormSpan = span
			break
		}
	}

	require.NotNil(t, gormSpan, "should find GORM Span")

	// Validate Parent relationship
	parentSpanContext := parentSpan.SpanContext()
	assert.Equal(t, parentSpanContext.TraceID(), gormSpan.SpanContext().TraceID(), "should share the same TraceID")

	t.Logf("✅ GORM Span successfully inherited parent Span")
	t.Logf("   Parent TraceID: %s", parentSpanContext.TraceID())
	t.Logf("   GORM TraceID:   %s", gormSpan.SpanContext().TraceID())
}
