package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestOtelPlugin_Initialize test plugin initialization
func TestOtelPlugin_Initialize(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Create plugin
	plugin := NewOtelPlugin(nil)
	assert.NotNil(t, plugin)

	// Initialize plugin
	err = plugin.Initialize(db)
	assert.NoError(t, err)

	// Validate plugin name
	assert.Equal(t, "otel", plugin.Name())
}

// TestModel test model
type TestModel struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"size:100"`
}

// TestOtelPlugin_CreateSpan test creating Span
func TestOtelPlugin_CreateSpan(t *testing.T) {
	// Create SpanRecorder to capture Span
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))
	otel.SetTracerProvider(tracerProvider)
	defer otel.SetTracerProvider(trace.NewTracerProvider())

	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Automatic migration
	err = db.AutoMigrate(&TestModel{})
	require.NoError(t, err)

	// Register plugin
	plugin := NewOtelPlugin(tracerProvider)
	err = plugin.Initialize(db)
	require.NoError(t, err)

	// Execute query (trigger Span creation)
	ctx := context.Background()
	db.WithContext(ctx).First(&TestModel{})

	// wait for Span processing
	spans := spanRecorder.Ended()
	require.NotEmpty(t, spans, "应该至少有一个 Span")

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
	assert.True(t, hasDBSystem, "应该包含 db.system 属性")
}

// TestOtelPlugin_QueryOperation test query operation
func TestOtelPlugin_QueryOperation(t *testing.T) {
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&TestModel{})
	require.NoError(t, err)

	plugin := NewOtelPlugin(tracerProvider)
	err = plugin.Initialize(db)
	require.NoError(t, err)

	// Execute query
	ctx := context.Background()
	var result TestModel
	db.WithContext(ctx).Where("id = ?", 1).First(&result)

	// Validate Span
	spans := spanRecorder.Ended()
	require.NotEmpty(t, spans)

	t.Logf("✅ Query 操作创建了 %d 个 Span", len(spans))
}

// TestOtelPlugin_CreateOperation test create operation
func TestOtelPlugin_CreateOperation(t *testing.T) {
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&TestModel{})
	require.NoError(t, err)

	plugin := NewOtelPlugin(tracerProvider)
	err = plugin.Initialize(db)
	require.NoError(t, err)

	// Execute creation
	ctx := context.Background()
	model := TestModel{Name: "test"}
	db.WithContext(ctx).Create(&model)

	// Validate Span
	spans := spanRecorder.Ended()
	require.NotEmpty(t, spans)

	t.Logf("✅ Create 操作创建了 %d 个 Span", len(spans))
}

// TestOtelPlugin_UpdateOperation test update operation
func TestOtelPlugin_UpdateOperation(t *testing.T) {
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&TestModel{})
	require.NoError(t, err)

	plugin := NewOtelPlugin(tracerProvider)
	err = plugin.Initialize(db)
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

	t.Logf("✅ Update 操作创建了 %d 个 Span", len(spans))
}

// TestOtelPlugin_DeleteOperation test delete operation
func TestOtelPlugin_DeleteOperation(t *testing.T) {
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&TestModel{})
	require.NoError(t, err)

	plugin := NewOtelPlugin(tracerProvider)
	err = plugin.Initialize(db)
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

	t.Logf("✅ Delete 操作创建了 %d 个 Span", len(spans))
}

// TestOtelPlugin_WithParentSpan test with parent Span
func TestOtelPlugin_WithParentSpan(t *testing.T) {
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&TestModel{})
	require.NoError(t, err)

	plugin := NewOtelPlugin(tracerProvider)
	err = plugin.Initialize(db)
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

	require.NotNil(t, gormSpan, "应该找到 GORM Span")
	
	// Validate Parent relationship
	parentSpanContext := parentSpan.SpanContext()
	assert.Equal(t, parentSpanContext.TraceID(), gormSpan.SpanContext().TraceID(), "应该共享相同的 TraceID")

	t.Logf("✅ GORM Span 成功继承父 Span")
	t.Logf("   Parent TraceID: %s", parentSpanContext.TraceID())
	t.Logf("   GORM TraceID:   %s", gormSpan.SpanContext().TraceID())
}

