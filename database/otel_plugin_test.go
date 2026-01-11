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

// TestOtelPlugin_Initialize 测试插件初始化
func TestOtelPlugin_Initialize(t *testing.T) {
	// 创建内存数据库
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// 创建插件
	plugin := NewOtelPlugin(nil)
	assert.NotNil(t, plugin)

	// 初始化插件
	err = plugin.Initialize(db)
	assert.NoError(t, err)

	// 验证插件名称
	assert.Equal(t, "otel", plugin.Name())
}

// TestModel 测试用模型
type TestModel struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"size:100"`
}

// TestOtelPlugin_CreateSpan 测试创建 Span
func TestOtelPlugin_CreateSpan(t *testing.T) {
	// 创建 SpanRecorder 用于捕获 Span
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := trace.NewTracerProvider(trace.WithSpanProcessor(spanRecorder))
	otel.SetTracerProvider(tracerProvider)
	defer otel.SetTracerProvider(trace.NewTracerProvider())

	// 创建内存数据库
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// 自动迁移
	err = db.AutoMigrate(&TestModel{})
	require.NoError(t, err)

	// 注册插件
	plugin := NewOtelPlugin(tracerProvider)
	err = plugin.Initialize(db)
	require.NoError(t, err)

	// 执行查询（触发 Span 创建）
	ctx := context.Background()
	db.WithContext(ctx).First(&TestModel{})

	// 等待 Span 处理
	spans := spanRecorder.Ended()
	require.NotEmpty(t, spans, "应该至少有一个 Span")

	// 验证 Span 属性
	span := spans[0]
	assert.Contains(t, span.Name(), "gorm")
	
	// 验证属性
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

// TestOtelPlugin_QueryOperation 测试查询操作
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

	// 执行查询
	ctx := context.Background()
	var result TestModel
	db.WithContext(ctx).Where("id = ?", 1).First(&result)

	// 验证 Span
	spans := spanRecorder.Ended()
	require.NotEmpty(t, spans)

	t.Logf("✅ Query 操作创建了 %d 个 Span", len(spans))
}

// TestOtelPlugin_CreateOperation 测试创建操作
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

	// 执行创建
	ctx := context.Background()
	model := TestModel{Name: "test"}
	db.WithContext(ctx).Create(&model)

	// 验证 Span
	spans := spanRecorder.Ended()
	require.NotEmpty(t, spans)

	t.Logf("✅ Create 操作创建了 %d 个 Span", len(spans))
}

// TestOtelPlugin_UpdateOperation 测试更新操作
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

	// 先创建
	ctx := context.Background()
	model := TestModel{Name: "test"}
	db.WithContext(ctx).Create(&model)

	// 清空记录
	spanRecorder.Reset()

	// 执行更新
	db.WithContext(ctx).Model(&model).Update("Name", "updated")

	// 验证 Span
	spans := spanRecorder.Ended()
	require.NotEmpty(t, spans)

	t.Logf("✅ Update 操作创建了 %d 个 Span", len(spans))
}

// TestOtelPlugin_DeleteOperation 测试删除操作
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

	// 先创建
	ctx := context.Background()
	model := TestModel{Name: "test"}
	db.WithContext(ctx).Create(&model)

	// 清空记录
	spanRecorder.Reset()

	// 执行删除
	db.WithContext(ctx).Delete(&model)

	// 验证 Span
	spans := spanRecorder.Ended()
	require.NotEmpty(t, spans)

	t.Logf("✅ Delete 操作创建了 %d 个 Span", len(spans))
}

// TestOtelPlugin_WithParentSpan 测试继承父 Span
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

	// 创建父 Span
	tracer := tracerProvider.Tracer("test")
	ctx, parentSpan := tracer.Start(context.Background(), "parent-span")
	defer parentSpan.End()

	// 执行查询（应该继承父 Span）
	var result TestModel
	db.WithContext(ctx).First(&result)

	// 验证 Span 链路
	spans := spanRecorder.Ended()
	require.NotEmpty(t, spans)

	// 找到 GORM Span
	var gormSpan trace.ReadOnlySpan
	for _, span := range spans {
		if span.Name() != "parent-span" {
			gormSpan = span
			break
		}
	}

	require.NotNil(t, gormSpan, "应该找到 GORM Span")
	
	// 验证 Parent 关系
	parentSpanContext := parentSpan.SpanContext()
	assert.Equal(t, parentSpanContext.TraceID(), gormSpan.SpanContext().TraceID(), "应该共享相同的 TraceID")

	t.Logf("✅ GORM Span 成功继承父 Span")
	t.Logf("   Parent TraceID: %s", parentSpanContext.TraceID())
	t.Logf("   GORM TraceID:   %s", gormSpan.SpanContext().TraceID())
}

