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

// DBMetrics 数据库层指标收集器
type DBMetrics struct {
	queriesTotal      metric.Int64Counter     // 查询总数
	queryDuration     metric.Float64Histogram // 查询耗时
	slowQueries       metric.Int64Counter     // 慢查询计数
	connectionsOpen   metric.Int64ObservableGauge // 打开的连接数
	connectionsIdle   metric.Int64ObservableGauge // 空闲连接数
	connectionsInUse  metric.Int64ObservableGauge // 使用中的连接数
	connectionsWait   metric.Int64Counter     // 等待连接次数
	db                *sql.DB
	recordSQLText     bool
	slowQueryThreshold float64 // 慢查询阈值（秒）
}

// NewDBMetrics 创建数据库指标收集器
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

	// 注册连接池指标（异步采集）
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

// GORMPlugin 返回 GORM 插件（用于注册回调）
func (m *DBMetrics) GORMPlugin() gorm.Plugin {
	return &metricsPlugin{metrics: m}
}

// metricsPlugin GORM 插件实现
type metricsPlugin struct {
	metrics *DBMetrics
}

func (p *metricsPlugin) Name() string {
	return "otel-db-metrics-plugin"
}

func (p *metricsPlugin) Initialize(db *gorm.DB) error {
	// 注册 before 回调（记录开始时间）
	db.Callback().Query().Before("gorm:query").Register("metrics:before_query", p.before)
	db.Callback().Create().Before("gorm:create").Register("metrics:before_create", p.before)
	db.Callback().Update().Before("gorm:update").Register("metrics:before_update", p.before)
	db.Callback().Delete().Before("gorm:delete").Register("metrics:before_delete", p.before)
	db.Callback().Row().Before("gorm:row").Register("metrics:before_row", p.before)
	db.Callback().Raw().Before("gorm:raw").Register("metrics:before_raw", p.before)

	// 注册 after 回调（记录指标）
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

	// 确定操作类型
	operation := getOperationType(db)

	// 获取表名
	table := db.Statement.Table
	if table == "" {
		table = "unknown"
	}

	// 构建标签
	attrs := []attribute.KeyValue{
		attribute.String("operation", operation),
		attribute.String("table", table),
	}

	// 可选：记录 SQL 文本（⚠️ 高基数，生产环境慎用）
	if p.metrics.recordSQLText && db.Statement.SQL.String() != "" {
		// 只记录前 100 个字符，避免标签过长
		sqlText := db.Statement.SQL.String()
		if len(sqlText) > 100 {
			sqlText = sqlText[:100] + "..."
		}
		attrs = append(attrs, attribute.String("sql_text", sqlText))
	}

	// 记录查询总数
	p.metrics.queriesTotal.Add(ctx, 1, metric.WithAttributes(attrs...))

	// 记录查询耗时
	p.metrics.queryDuration.Record(ctx, duration, metric.WithAttributes(attrs...))

	// 慢查询检测
	if duration >= p.metrics.slowQueryThreshold {
		p.metrics.slowQueries.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

// getOperationType 获取操作类型
func getOperationType(db *gorm.DB) string {
	// 根据 Statement 推断操作类型
	if db.Statement.ReflectValue.Kind() == reflect.Slice {
		return "select_many"
	}
	if db.Statement.ReflectValue.Kind() == reflect.Struct {
		return "select_one"
	}

	// 根据回调名称推断
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

