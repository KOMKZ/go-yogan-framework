package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// MetricsBuilder 指标构建器，减少样板代码
type MetricsBuilder struct {
	meter     metric.Meter
	namespace string
}

// NewMetricsBuilder 创建指标构建器
func NewMetricsBuilder(meter metric.Meter, namespace string) *MetricsBuilder {
	return &MetricsBuilder{
		meter:     meter,
		namespace: namespace,
	}
}

// fullName 生成完整的指标名称
func (b *MetricsBuilder) fullName(name string) string {
	if b.namespace == "" {
		return name
	}
	return b.namespace + "_" + name
}

// ========== 基础指标创建方法 ==========

// Counter 创建 Int64Counter
func (b *MetricsBuilder) Counter(name, desc string) (metric.Int64Counter, error) {
	return b.meter.Int64Counter(
		b.fullName(name),
		metric.WithDescription(desc),
		metric.WithUnit("{count}"),
	)
}

// CounterWithUnit 创建带自定义单位的 Int64Counter
func (b *MetricsBuilder) CounterWithUnit(name, desc, unit string) (metric.Int64Counter, error) {
	return b.meter.Int64Counter(
		b.fullName(name),
		metric.WithDescription(desc),
		metric.WithUnit(unit),
	)
}

// Histogram 创建 Float64Histogram
func (b *MetricsBuilder) Histogram(name, desc, unit string) (metric.Float64Histogram, error) {
	return b.meter.Float64Histogram(
		b.fullName(name),
		metric.WithDescription(desc),
		metric.WithUnit(unit),
	)
}

// DurationHistogram 创建时间分布直方图（秒）
func (b *MetricsBuilder) DurationHistogram(name, desc string) (metric.Float64Histogram, error) {
	return b.Histogram(name, desc, "s")
}

// BytesHistogram 创建字节大小直方图
func (b *MetricsBuilder) BytesHistogram(name, desc string) (metric.Int64Histogram, error) {
	return b.meter.Int64Histogram(
		b.fullName(name),
		metric.WithDescription(desc),
		metric.WithUnit("By"),
	)
}

// Gauge 创建可观测 Gauge（需要回调函数）
func (b *MetricsBuilder) Gauge(name, desc string, callback func(context.Context) (int64, error)) (metric.Int64ObservableGauge, error) {
	return b.meter.Int64ObservableGauge(
		b.fullName(name),
		metric.WithDescription(desc),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			val, err := callback(ctx)
			if err != nil {
				return err
			}
			o.Observe(val)
			return nil
		}),
	)
}

// GaugeWithAttrs 创建带属性的可观测 Gauge
func (b *MetricsBuilder) GaugeWithAttrs(name, desc string, callback func(context.Context) (int64, []attribute.KeyValue, error)) (metric.Int64ObservableGauge, error) {
	return b.meter.Int64ObservableGauge(
		b.fullName(name),
		metric.WithDescription(desc),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			val, attrs, err := callback(ctx)
			if err != nil {
				return err
			}
			o.Observe(val, metric.WithAttributes(attrs...))
			return nil
		}),
	)
}

// UpDownCounter 创建可增减计数器
func (b *MetricsBuilder) UpDownCounter(name, desc string) (metric.Int64UpDownCounter, error) {
	return b.meter.Int64UpDownCounter(
		b.fullName(name),
		metric.WithDescription(desc),
		metric.WithUnit("{count}"),
	)
}

// ========== 预定义模板 ==========

// RequestMetrics 请求类指标模板
type RequestMetrics struct {
	Total    metric.Int64Counter     // 请求总数
	Duration metric.Float64Histogram // 请求耗时分布
	Errors   metric.Int64Counter     // 错误总数
}

// NewRequestMetrics 创建请求类指标（一行创建 3 个指标）
func (b *MetricsBuilder) NewRequestMetrics(prefix string) (*RequestMetrics, error) {
	total, err := b.Counter(prefix+"_requests_total", "Total number of "+prefix+" requests")
	if err != nil {
		return nil, err
	}

	duration, err := b.DurationHistogram(prefix+"_duration_seconds", prefix+" request duration distribution")
	if err != nil {
		return nil, err
	}

	errors, err := b.Counter(prefix+"_errors_total", "Total number of "+prefix+" errors")
	if err != nil {
		return nil, err
	}

	return &RequestMetrics{
		Total:    total,
		Duration: duration,
		Errors:   errors,
	}, nil
}

// Record 记录请求指标
func (m *RequestMetrics) Record(ctx context.Context, durationSec float64, err error, attrs ...attribute.KeyValue) {
	opt := metric.WithAttributes(attrs...)
	m.Total.Add(ctx, 1, opt)
	m.Duration.Record(ctx, durationSec, opt)
	if err != nil {
		m.Errors.Add(ctx, 1, opt)
	}
}

// PoolMetrics 连接池类指标模板
type PoolMetrics struct {
	Active    metric.Int64ObservableGauge // 活跃连接数
	Idle      metric.Int64ObservableGauge // 空闲连接数
	InUse     metric.Int64ObservableGauge // 使用中连接数
	WaitTotal metric.Int64Counter         // 等待获取连接总数
}

// PoolStatsFunc 连接池统计回调
type PoolStatsFunc func() (active, idle, inUse int64)

// NewPoolMetrics 创建连接池类指标
func (b *MetricsBuilder) NewPoolMetrics(prefix string, statsFunc PoolStatsFunc) (*PoolMetrics, error) {
	active, err := b.Gauge(prefix+"_connections_active", "Number of active connections", func(ctx context.Context) (int64, error) {
		a, _, _ := statsFunc()
		return a, nil
	})
	if err != nil {
		return nil, err
	}

	idle, err := b.Gauge(prefix+"_connections_idle", "Number of idle connections", func(ctx context.Context) (int64, error) {
		_, i, _ := statsFunc()
		return i, nil
	})
	if err != nil {
		return nil, err
	}

	inUse, err := b.Gauge(prefix+"_connections_in_use", "Number of in-use connections", func(ctx context.Context) (int64, error) {
		_, _, u := statsFunc()
		return u, nil
	})
	if err != nil {
		return nil, err
	}

	waitTotal, err := b.Counter(prefix+"_connections_wait_total", "Total number of connection waits")
	if err != nil {
		return nil, err
	}

	return &PoolMetrics{
		Active:    active,
		Idle:      idle,
		InUse:     inUse,
		WaitTotal: waitTotal,
	}, nil
}

// RecordWait 记录等待获取连接
func (m *PoolMetrics) RecordWait(ctx context.Context, attrs ...attribute.KeyValue) {
	m.WaitTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// OperationMetrics 操作类指标模板（适用于 CRUD、命令执行等）
type OperationMetrics struct {
	Total    metric.Int64Counter     // 操作总数
	Duration metric.Float64Histogram // 操作耗时
	Errors   metric.Int64Counter     // 错误数
	Success  metric.Int64Counter     // 成功数
}

// NewOperationMetrics 创建操作类指标
func (b *MetricsBuilder) NewOperationMetrics(prefix string) (*OperationMetrics, error) {
	total, err := b.Counter(prefix+"_operations_total", "Total number of "+prefix+" operations")
	if err != nil {
		return nil, err
	}

	duration, err := b.DurationHistogram(prefix+"_operation_duration_seconds", prefix+" operation duration distribution")
	if err != nil {
		return nil, err
	}

	errors, err := b.Counter(prefix+"_operation_errors_total", "Total number of "+prefix+" operation errors")
	if err != nil {
		return nil, err
	}

	success, err := b.Counter(prefix+"_operation_success_total", "Total number of successful "+prefix+" operations")
	if err != nil {
		return nil, err
	}

	return &OperationMetrics{
		Total:    total,
		Duration: duration,
		Errors:   errors,
		Success:  success,
	}, nil
}

// Record 记录操作指标
func (m *OperationMetrics) Record(ctx context.Context, durationSec float64, err error, attrs ...attribute.KeyValue) {
	opt := metric.WithAttributes(attrs...)
	m.Total.Add(ctx, 1, opt)
	m.Duration.Record(ctx, durationSec, opt)
	if err != nil {
		m.Errors.Add(ctx, 1, opt)
	} else {
		m.Success.Add(ctx, 1, opt)
	}
}

// TokenMetrics Token 类指标模板（适用于 JWT、Session 等）
type TokenMetrics struct {
	Generated metric.Int64Counter     // 生成数
	Verified  metric.Int64Counter     // 验证数
	Refreshed metric.Int64Counter     // 刷新数
	Revoked   metric.Int64Counter     // 撤销数
	Duration  metric.Float64Histogram // 验证耗时
}

// NewTokenMetrics 创建 Token 类指标
func (b *MetricsBuilder) NewTokenMetrics(prefix string) (*TokenMetrics, error) {
	generated, err := b.Counter(prefix+"_tokens_generated_total", "Total number of "+prefix+" tokens generated")
	if err != nil {
		return nil, err
	}

	verified, err := b.Counter(prefix+"_tokens_verified_total", "Total number of "+prefix+" tokens verified")
	if err != nil {
		return nil, err
	}

	refreshed, err := b.Counter(prefix+"_tokens_refreshed_total", "Total number of "+prefix+" tokens refreshed")
	if err != nil {
		return nil, err
	}

	revoked, err := b.Counter(prefix+"_tokens_revoked_total", "Total number of "+prefix+" tokens revoked")
	if err != nil {
		return nil, err
	}

	duration, err := b.DurationHistogram(prefix+"_verification_duration_seconds", prefix+" verification duration distribution")
	if err != nil {
		return nil, err
	}

	return &TokenMetrics{
		Generated: generated,
		Verified:  verified,
		Refreshed: refreshed,
		Revoked:   revoked,
		Duration:  duration,
	}, nil
}

// RecordGenerated 记录 Token 生成
func (m *TokenMetrics) RecordGenerated(ctx context.Context, attrs ...attribute.KeyValue) {
	m.Generated.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordVerified 记录 Token 验证
func (m *TokenMetrics) RecordVerified(ctx context.Context, durationSec float64, success bool, attrs ...attribute.KeyValue) {
	opt := metric.WithAttributes(attrs...)
	m.Verified.Add(ctx, 1, opt)
	m.Duration.Record(ctx, durationSec, opt)
}

// RecordRefreshed 记录 Token 刷新
func (m *TokenMetrics) RecordRefreshed(ctx context.Context, attrs ...attribute.KeyValue) {
	m.Refreshed.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordRevoked 记录 Token 撤销
func (m *TokenMetrics) RecordRevoked(ctx context.Context, attrs ...attribute.KeyValue) {
	m.Revoked.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// CacheMetrics 缓存类指标模板
type CacheMetrics struct {
	Hits     metric.Int64Counter // 命中数
	Misses   metric.Int64Counter // 未命中数
	Gets     metric.Int64Counter // 获取总数
	Sets     metric.Int64Counter // 设置总数
	Deletes  metric.Int64Counter // 删除总数
	Duration metric.Float64Histogram // 操作耗时
}

// NewCacheMetrics 创建缓存类指标
func (b *MetricsBuilder) NewCacheMetrics(prefix string) (*CacheMetrics, error) {
	hits, err := b.Counter(prefix+"_cache_hits_total", "Total number of "+prefix+" cache hits")
	if err != nil {
		return nil, err
	}

	misses, err := b.Counter(prefix+"_cache_misses_total", "Total number of "+prefix+" cache misses")
	if err != nil {
		return nil, err
	}

	gets, err := b.Counter(prefix+"_cache_gets_total", "Total number of "+prefix+" cache get operations")
	if err != nil {
		return nil, err
	}

	sets, err := b.Counter(prefix+"_cache_sets_total", "Total number of "+prefix+" cache set operations")
	if err != nil {
		return nil, err
	}

	deletes, err := b.Counter(prefix+"_cache_deletes_total", "Total number of "+prefix+" cache delete operations")
	if err != nil {
		return nil, err
	}

	duration, err := b.DurationHistogram(prefix+"_cache_duration_seconds", prefix+" cache operation duration")
	if err != nil {
		return nil, err
	}

	return &CacheMetrics{
		Hits:     hits,
		Misses:   misses,
		Gets:     gets,
		Sets:     sets,
		Deletes:  deletes,
		Duration: duration,
	}, nil
}

// RecordHit 记录缓存命中
func (m *CacheMetrics) RecordHit(ctx context.Context, attrs ...attribute.KeyValue) {
	opt := metric.WithAttributes(attrs...)
	m.Gets.Add(ctx, 1, opt)
	m.Hits.Add(ctx, 1, opt)
}

// RecordMiss 记录缓存未命中
func (m *CacheMetrics) RecordMiss(ctx context.Context, attrs ...attribute.KeyValue) {
	opt := metric.WithAttributes(attrs...)
	m.Gets.Add(ctx, 1, opt)
	m.Misses.Add(ctx, 1, opt)
}

// RecordSet 记录缓存设置
func (m *CacheMetrics) RecordSet(ctx context.Context, attrs ...attribute.KeyValue) {
	m.Sets.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordDelete 记录缓存删除
func (m *CacheMetrics) RecordDelete(ctx context.Context, attrs ...attribute.KeyValue) {
	m.Deletes.Add(ctx, 1, metric.WithAttributes(attrs...))
}
