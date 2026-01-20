package database

import (
	"context"
	"database/sql"
	"reflect"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"gorm.io/gorm"
)

// DBMetrics database layer metrics collector
type DBMetrics struct {
	queriesTotal      metric.Int64Counter     // Query total count
	queryDuration     metric.Float64Histogram // Query duration
	slowQueries       metric.Int64Counter     // Slow query count
	connectionsOpen   metric.Int64ObservableGauge // number of open connections
	connectionsIdle   metric.Int64ObservableGauge // number of idle connections
	connectionsInUse  metric.Int64ObservableGauge // Number of active connections
	connectionsWait   metric.Int64Counter     // wait connection attempts
	db                *sql.DB
	recordSQLText     bool
	slowQueryThreshold float64 // Slow query threshold (seconds)
}

// NewDBMetrics creates a database metrics collector
func NewDBMetrics(db *gorm.DB, recordSQLText bool, slowQueryThreshold float64) (*DBMetrics, error) {
	meter := otel.Meter("database")
	sqlDB, _ := db.DB()

	queriesTotal, err := meter.Int64Counter(
		"db_queries_total",
		metric.WithDescription("数据库查询总数"),
		metric.WithUnit("{query}"),
	)
	if err != nil {
		return nil, err
	}

	queryDuration, err := meter.Float64Histogram(
		"db_query_duration_seconds",
		metric.WithDescription("数据库查询耗时分布"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	slowQueries, err := meter.Int64Counter(
		"db_slow_queries_total",
		metric.WithDescription("慢查询总数"),
		metric.WithUnit("{query}"),
	)
	if err != nil {
		return nil, err
	}

	connectionsWait, err := meter.Int64Counter(
		"db_connections_wait_total",
		metric.WithDescription("等待连接的次数"),
		metric.WithUnit("{wait}"),
	)
	if err != nil {
		return nil, err
	}

	dbMetrics := &DBMetrics{
		queriesTotal:       queriesTotal,
		queryDuration:      queryDuration,
		slowQueries:        slowQueries,
		connectionsWait:    connectionsWait,
		db:                 sqlDB,
		recordSQLText:      recordSQLText,
		slowQueryThreshold: slowQueryThreshold,
	}

	// Register connection pool metrics (asynchronous collection)
	_, err = meter.Int64ObservableGauge(
		"db_connections_open",
		metric.WithDescription("当前打开的数据库连接数"),
		metric.WithUnit("{connection}"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			stats := sqlDB.Stats()
			o.Observe(int64(stats.OpenConnections))
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}

	_, err = meter.Int64ObservableGauge(
		"db_connections_idle",
		metric.WithDescription("当前空闲连接数"),
		metric.WithUnit("{connection}"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			stats := sqlDB.Stats()
			o.Observe(int64(stats.Idle))
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}

	_, err = meter.Int64ObservableGauge(
		"db_connections_in_use",
		metric.WithDescription("当前使用中的连接数"),
		metric.WithUnit("{connection}"),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			stats := sqlDB.Stats()
			o.Observe(int64(stats.InUse))
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}

	return dbMetrics, nil
}

// GORMPlugin returns the GORM plugin (for registering callbacks)
func (m *DBMetrics) GORMPlugin() gorm.Plugin {
	return &metricsPlugin{metrics: m}
}

// metricsPlugin GORM plugin implementation
type metricsPlugin struct {
	metrics *DBMetrics
}

func (p *metricsPlugin) Name() string {
	return "otel-db-metrics-plugin"
}

func (p *metricsPlugin) Initialize(db *gorm.DB) error {
	// Register before callback (record start time)
	db.Callback().Query().Before("gorm:query").Register("metrics:before_query", p.before)
	db.Callback().Create().Before("gorm:create").Register("metrics:before_create", p.before)
	db.Callback().Update().Before("gorm:update").Register("metrics:before_update", p.before)
	db.Callback().Delete().Before("gorm:delete").Register("metrics:before_delete", p.before)
	db.Callback().Row().Before("gorm:row").Register("metrics:before_row", p.before)
	db.Callback().Raw().Before("gorm:raw").Register("metrics:before_raw", p.before)

	// register after callback (record metrics)
	db.Callback().Query().After("gorm:query").Register("metrics:after_query", p.after)
	db.Callback().Create().After("gorm:create").Register("metrics:after_create", p.after)
	db.Callback().Update().After("gorm:update").Register("metrics:after_update", p.after)
	db.Callback().Delete().After("gorm:delete").Register("metrics:after_delete", p.after)
	db.Callback().Row().After("gorm:row").Register("metrics:after_row", p.after)
	db.Callback().Raw().After("gorm:raw").Register("metrics:after_raw", p.after)

	return nil
}

func (p *metricsPlugin) before(db *gorm.DB) {
	db.Set("metrics:start_time", time.Now())
}

func (p *metricsPlugin) after(db *gorm.DB) {
	startTime, ok := db.Get("metrics:start_time")
	if !ok {
		return
	}

	duration := time.Since(startTime.(time.Time)).Seconds()
	ctx := db.Statement.Context

	// Determine the operation type
	operation := getOperationType(db)

	// Get table name
	table := db.Statement.Table
	if table == "" {
		table = "unknown"
	}

	// Build label
	attrs := []attribute.KeyValue{
		attribute.String("operation", operation),
		attribute.String("table", table),
	}

	// Optional: Log SQL text (⚠ High cardinality, use with caution in production)
	if p.metrics.recordSQLText && db.Statement.SQL.String() != "" {
		// Only record the first 100 characters to avoid overly long tags
		sqlText := db.Statement.SQL.String()
		if len(sqlText) > 100 {
			sqlText = sqlText[:100] + "..."
		}
		attrs = append(attrs, attribute.String("sql_text", sqlText))
	}

	// Record total query count
	p.metrics.queriesTotal.Add(ctx, 1, metric.WithAttributes(attrs...))

	// Record query duration
	p.metrics.queryDuration.Record(ctx, duration, metric.WithAttributes(attrs...))

	// Slow query detection
	if duration >= p.metrics.slowQueryThreshold {
		p.metrics.slowQueries.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

// Get operation type
func getOperationType(db *gorm.DB) string {
	// Infer operation type based on Statement
	if db.Statement.ReflectValue.Kind() == reflect.Slice {
		return "select_many"
	}
	if db.Statement.ReflectValue.Kind() == reflect.Struct {
		return "select_one"
	}

	// Infer based on callback name
	switch {
	case db.Statement.SQL.String() == "":
		return "unknown"
	case len(db.Statement.SQL.String()) > 6:
		sqlPrefix := db.Statement.SQL.String()[:6]
		switch sqlPrefix {
		case "SELECT", "select":
			return "select"
		case "INSERT", "insert":
			return "insert"
		case "UPDATE", "update":
			return "update"
		case "DELETE", "delete":
			return "delete"
		}
	}

	return "unknown"
}

