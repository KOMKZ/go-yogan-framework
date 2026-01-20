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
	// instrumentationName instrument name
	instrumentationName = "gorm.io/plugin/opentelemetry"
	// instrumentationVersion instrument version
	instrumentationVersion = "0.1.0"
)

// OtelPlugin GORM OpenTelemetry plugin
type OtelPlugin struct {
	tracerProvider trace.TracerProvider
	tracer         trace.Tracer
	traceSQL       bool // Whether to log SQL statements to Span
	sqlMaxLen      int  // SQL maximum length
}

// Create NewOtelPlugin for OpenTelemetry plugin
// If tracerProvider is nil, use the global TracerProvider
func NewOtelPlugin(tracerProvider trace.TracerProvider) *OtelPlugin {
	if tracerProvider == nil {
		tracerProvider = otel.GetTracerProvider()
	}

	return &OtelPlugin{
		tracerProvider: tracerProvider,
		tracer:         tracerProvider.Tracer(instrumentationName, trace.WithInstrumentationVersion(instrumentationVersion)),
		traceSQL:       false, // By default, do not record SQL
		sqlMaxLen:      1000,  // Default maximum length 1000
	}
}

// WithTraceSQL setting whether to record SQL statements
func (p *OtelPlugin) WithTraceSQL(enabled bool) *OtelPlugin {
	p.traceSQL = enabled
	return p
}

// WithSQLMaxLen sets the maximum length of SQL
func (p *OtelPlugin) WithSQLMaxLen(maxLen int) *OtelPlugin {
	if maxLen > 0 {
		p.sqlMaxLen = maxLen
	}
	return p
}

// Name Plugin Name
func (p *OtelPlugin) Name() string {
	return "otel"
}

// Initialize plugin (register callbacks)
func (p *OtelPlugin) Initialize(db *gorm.DB) error {
	// Register Create callback
	if err := db.Callback().Create().Before("gorm:create").Register("otel:before_create", p.before); err != nil {
		return err
	}
	if err := db.Callback().Create().After("gorm:create").Register("otel:after_create", p.after); err != nil {
		return err
	}

	// Register Query callback
	if err := db.Callback().Query().Before("gorm:query").Register("otel:before_query", p.before); err != nil {
		return err
	}
	if err := db.Callback().Query().After("gorm:query").Register("otel:after_query", p.after); err != nil {
		return err
	}

	// Register Update callback
	if err := db.Callback().Update().Before("gorm:update").Register("otel:before_update", p.before); err != nil {
		return err
	}
	if err := db.Callback().Update().After("gorm:update").Register("otel:after_update", p.after); err != nil {
		return err
	}

	// Register Delete callback
	if err := db.Callback().Delete().Before("gorm:delete").Register("otel:before_delete", p.before); err != nil {
		return err
	}
	if err := db.Callback().Delete().After("gorm:delete").Register("otel:after_delete", p.after); err != nil {
		return err
	}

	// Register Row Callback
	if err := db.Callback().Row().Before("gorm:row").Register("otel:before_row", p.before); err != nil {
		return err
	}
	if err := db.Callback().Row().After("gorm:row").Register("otel:after_row", p.after); err != nil {
		return err
	}

	// Register Raw callback
	if err := db.Callback().Raw().Before("gorm:raw").Register("otel:before_raw", p.before); err != nil {
		return err
	}
	if err := db.Callback().Raw().After("gorm:raw").Register("otel:after_raw", p.after); err != nil {
		return err
	}

	return nil
}

// before creating Span for the operation
func (p *OtelPlugin) before(db *gorm.DB) {
	// Get the parent Span from context
	ctx := db.Statement.Context
	if ctx == nil {
		ctx = context.Background()
	}

	// Determine the operation type
	operation := p.determineOperation(db)

	// Create Span
	spanName := fmt.Sprintf("gorm.%s", operation)
	if db.Statement.Table != "" {
		spanName = fmt.Sprintf("gorm.%s %s", operation, db.Statement.Table)
	}

	ctx, span := p.tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))

	// Set base Span attributes
	attrs := []attribute.KeyValue{
		attribute.String("db.system", "gorm"),
		attribute.String("db.operation", operation),
	}

	if db.Statement.Table != "" {
		attrs = append(attrs, attribute.String("db.table", db.Statement.Table))
	}

	span.SetAttributes(attrs...)

	// Save Span to context (for after callback)
	db.Statement.Context = ctx
	db.InstanceSet("otel:span", span)
}

// English: end Span after operation
func (p *OtelPlugin) after(db *gorm.DB) {
	// Get Span
	spanVal, ok := db.InstanceGet("otel:span")
	if !ok {
		return
	}

	span, ok := spanVal.(trace.Span)
	if !ok {
		return
	}

	defer span.End()

	// 🎯 Determine whether to log SQL statements based on configuration
	if p.traceSQL {
		sql := db.Statement.SQL.String()
		if sql != "" {
			// The SQL statement may be long; trim according to configuration
			if len(sql) > p.sqlMaxLen {
				sql = sql[:p.sqlMaxLen] + "..."
			}
			span.SetAttributes(attribute.String("db.statement", sql))
		}

		// Record bound SQL variables (vars)
		if len(db.Statement.Vars) > 0 {
			span.SetAttributes(attribute.Int("db.vars_count", len(db.Statement.Vars)))
		}
	}

	// log the number of affected rows (always log, minimal performance impact)
	span.SetAttributes(
		attribute.Int64("db.rows_affected", db.Statement.RowsAffected),
	)

	// Record error if any
	if db.Error != nil && db.Error != gorm.ErrRecordNotFound {
		span.RecordError(db.Error)
		span.SetStatus(codes.Error, db.Error.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
}

// determineOperation determines the operation type based on Statement
func (p *OtelPlugin) determineOperation(db *gorm.DB) string {
	// Prioritize judgment from the SQL string
	sql := db.Statement.SQL.String()
	if sql != "" {
		// Extract the first word of the SQL (usually the operation type)
		for i, char := range sql {
			if char == ' ' || char == '\t' || char == '\n' {
				if i > 0 {
					return sql[:i]
				}
				break
			}
		}
	}

	// Fallback to default operation type
	return "query"
}
