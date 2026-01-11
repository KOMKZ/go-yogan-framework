package database

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

const (
	// instrumentationName 仪器名称
	instrumentationName = "gorm.io/plugin/opentelemetry"
	// instrumentationVersion 仪器版本
	instrumentationVersion = "0.1.0"
)

// OtelPlugin GORM OpenTelemetry 插件
type OtelPlugin struct {
	tracerProvider trace.TracerProvider
	tracer         trace.Tracer
	traceSQL       bool // 是否记录 SQL 语句到 Span
	sqlMaxLen      int  // SQL 最大长度
}

// NewOtelPlugin 创建 OpenTelemetry 插件
// 如果 tracerProvider 为 nil，使用全局 TracerProvider
func NewOtelPlugin(tracerProvider trace.TracerProvider) *OtelPlugin {
	if tracerProvider == nil {
		tracerProvider = otel.GetTracerProvider()
	}

	return &OtelPlugin{
		tracerProvider: tracerProvider,
		tracer:         tracerProvider.Tracer(instrumentationName, trace.WithInstrumentationVersion(instrumentationVersion)),
		traceSQL:       false, // 默认不记录 SQL
		sqlMaxLen:      1000,  // 默认最大长度 1000
	}
}

// WithTraceSQL 设置是否记录 SQL 语句
func (p *OtelPlugin) WithTraceSQL(enabled bool) *OtelPlugin {
	p.traceSQL = enabled
	return p
}

// WithSQLMaxLen 设置 SQL 最大长度
func (p *OtelPlugin) WithSQLMaxLen(maxLen int) *OtelPlugin {
	if maxLen > 0 {
		p.sqlMaxLen = maxLen
	}
	return p
}

// Name 插件名称
func (p *OtelPlugin) Name() string {
	return "otel"
}

// Initialize 初始化插件（注册回调）
func (p *OtelPlugin) Initialize(db *gorm.DB) error {
	// 注册 Create 回调
	if err := db.Callback().Create().Before("gorm:create").Register("otel:before_create", p.before); err != nil {
		return err
	}
	if err := db.Callback().Create().After("gorm:create").Register("otel:after_create", p.after); err != nil {
		return err
	}

	// 注册 Query 回调
	if err := db.Callback().Query().Before("gorm:query").Register("otel:before_query", p.before); err != nil {
		return err
	}
	if err := db.Callback().Query().After("gorm:query").Register("otel:after_query", p.after); err != nil {
		return err
	}

	// 注册 Update 回调
	if err := db.Callback().Update().Before("gorm:update").Register("otel:before_update", p.before); err != nil {
		return err
	}
	if err := db.Callback().Update().After("gorm:update").Register("otel:after_update", p.after); err != nil {
		return err
	}

	// 注册 Delete 回调
	if err := db.Callback().Delete().Before("gorm:delete").Register("otel:before_delete", p.before); err != nil {
		return err
	}
	if err := db.Callback().Delete().After("gorm:delete").Register("otel:after_delete", p.after); err != nil {
		return err
	}

	// 注册 Row 回调
	if err := db.Callback().Row().Before("gorm:row").Register("otel:before_row", p.before); err != nil {
		return err
	}
	if err := db.Callback().Row().After("gorm:row").Register("otel:after_row", p.after); err != nil {
		return err
	}

	// 注册 Raw 回调
	if err := db.Callback().Raw().Before("gorm:raw").Register("otel:before_raw", p.before); err != nil {
		return err
	}
	if err := db.Callback().Raw().After("gorm:raw").Register("otel:after_raw", p.after); err != nil {
		return err
	}

	return nil
}

// before 在操作之前创建 Span
func (p *OtelPlugin) before(db *gorm.DB) {
	// 从 context 获取父 Span
	ctx := db.Statement.Context
	if ctx == nil {
		ctx = context.Background()
	}

	// 确定操作类型
	operation := p.determineOperation(db)

	// 创建 Span
	spanName := fmt.Sprintf("gorm.%s", operation)
	if db.Statement.Table != "" {
		spanName = fmt.Sprintf("gorm.%s %s", operation, db.Statement.Table)
	}

	ctx, span := p.tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))

	// 设置基础 Span 属性
	attrs := []attribute.KeyValue{
		attribute.String("db.system", "gorm"),
		attribute.String("db.operation", operation),
	}

	if db.Statement.Table != "" {
		attrs = append(attrs, attribute.String("db.table", db.Statement.Table))
	}

	span.SetAttributes(attrs...)

	// 将 Span 保存到 context（用于 after 回调）
	db.Statement.Context = ctx
	db.InstanceSet("otel:span", span)
}

// after 在操作之后结束 Span
func (p *OtelPlugin) after(db *gorm.DB) {
	// 获取 Span
	spanVal, ok := db.InstanceGet("otel:span")
	if !ok {
		return
	}

	span, ok := spanVal.(trace.Span)
	if !ok {
		return
	}

	defer span.End()

	// 🎯 根据配置决定是否记录 SQL 语句
	if p.traceSQL {
		sql := db.Statement.SQL.String()
		if sql != "" {
			// SQL 语句可能很长，根据配置截取
			if len(sql) > p.sqlMaxLen {
				sql = sql[:p.sqlMaxLen] + "..."
			}
			span.SetAttributes(attribute.String("db.statement", sql))
		}

		// 记录绑定的 SQL 变量（vars）
		if len(db.Statement.Vars) > 0 {
			span.SetAttributes(attribute.Int("db.vars_count", len(db.Statement.Vars)))
		}
	}

	// 记录影响行数（始终记录，性能影响小）
	span.SetAttributes(
		attribute.Int64("db.rows_affected", db.Statement.RowsAffected),
	)

	// 记录错误（如果有）
	if db.Error != nil && db.Error != gorm.ErrRecordNotFound {
		span.RecordError(db.Error)
		span.SetStatus(codes.Error, db.Error.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
}

// determineOperation 根据 Statement 判断操作类型
func (p *OtelPlugin) determineOperation(db *gorm.DB) string {
	// 优先从 SQL 字符串判断
	sql := db.Statement.SQL.String()
	if sql != "" {
		// 提取 SQL 的第一个单词（通常是操作类型）
		for i, char := range sql {
			if char == ' ' || char == '\t' || char == '\n' {
				if i > 0 {
					return sql[:i]
				}
				break
			}
		}
	}

	// 回退到默认操作类型
	return "query"
}
