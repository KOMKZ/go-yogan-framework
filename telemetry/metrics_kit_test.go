package telemetry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func setupTestMeterProvider(t *testing.T) (*sdkmetric.MeterProvider, *sdkmetric.ManualReader) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
	)
	return mp, reader
}

func TestMetricsBuilder_Counter(t *testing.T) {
	mp, reader := setupTestMeterProvider(t)
	defer mp.Shutdown(context.Background())

	meter := mp.Meter("test")
	builder := NewMetricsBuilder(meter, "app")

	counter, err := builder.Counter("requests_total", "Total requests")
	require.NoError(t, err)
	require.NotNil(t, counter)

	// Record some values
	ctx := context.Background()
	counter.Add(ctx, 1)
	counter.Add(ctx, 5)

	// Read metrics
	var rm metricdata.ResourceMetrics
	err = reader.Collect(ctx, &rm)
	require.NoError(t, err)

	// Verify
	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "app_requests_total" {
				found = true
				sum := m.Data.(metricdata.Sum[int64])
				assert.Equal(t, int64(6), sum.DataPoints[0].Value)
			}
		}
	}
	assert.True(t, found, "Expected metric 'app_requests_total' not found")
}

func TestMetricsBuilder_Histogram(t *testing.T) {
	mp, reader := setupTestMeterProvider(t)
	defer mp.Shutdown(context.Background())

	meter := mp.Meter("test")
	builder := NewMetricsBuilder(meter, "http")

	histogram, err := builder.DurationHistogram("request_duration_seconds", "Request duration")
	require.NoError(t, err)
	require.NotNil(t, histogram)

	// Record some values
	ctx := context.Background()
	histogram.Record(ctx, 0.1)
	histogram.Record(ctx, 0.5)
	histogram.Record(ctx, 1.2)

	// Read metrics
	var rm metricdata.ResourceMetrics
	err = reader.Collect(ctx, &rm)
	require.NoError(t, err)

	// Verify
	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "http_request_duration_seconds" {
				found = true
				hist := m.Data.(metricdata.Histogram[float64])
				assert.Equal(t, uint64(3), hist.DataPoints[0].Count)
			}
		}
	}
	assert.True(t, found, "Expected metric 'http_request_duration_seconds' not found")
}

func TestMetricsBuilder_NewRequestMetrics(t *testing.T) {
	mp, reader := setupTestMeterProvider(t)
	defer mp.Shutdown(context.Background())

	meter := mp.Meter("test")
	builder := NewMetricsBuilder(meter, "api")

	metrics, err := builder.NewRequestMetrics("http")
	require.NoError(t, err)
	require.NotNil(t, metrics)

	ctx := context.Background()

	// Record successful request
	metrics.Record(ctx, 0.1, nil, attribute.String("method", "GET"))

	// Record failed request
	metrics.Record(ctx, 0.5, errors.New("timeout"), attribute.String("method", "POST"))

	// Read metrics
	var rm metricdata.ResourceMetrics
	err = reader.Collect(ctx, &rm)
	require.NoError(t, err)

	// Verify metrics
	foundTotal := false
	foundDuration := false
	foundErrors := false

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch m.Name {
			case "api_http_requests_total":
				foundTotal = true
				sum := m.Data.(metricdata.Sum[int64])
				// Total should be 2 (1 GET + 1 POST)
				var total int64
				for _, dp := range sum.DataPoints {
					total += dp.Value
				}
				assert.Equal(t, int64(2), total)
			case "api_http_duration_seconds":
				foundDuration = true
			case "api_http_errors_total":
				foundErrors = true
				sum := m.Data.(metricdata.Sum[int64])
				var total int64
				for _, dp := range sum.DataPoints {
					total += dp.Value
				}
				assert.Equal(t, int64(1), total)
			}
		}
	}

	assert.True(t, foundTotal, "Expected metric 'api_http_requests_total' not found")
	assert.True(t, foundDuration, "Expected metric 'api_http_duration_seconds' not found")
	assert.True(t, foundErrors, "Expected metric 'api_http_errors_total' not found")
}

func TestMetricsBuilder_NewPoolMetrics(t *testing.T) {
	mp, reader := setupTestMeterProvider(t)
	defer mp.Shutdown(context.Background())

	meter := mp.Meter("test")
	builder := NewMetricsBuilder(meter, "db")

	// Mock pool stats
	poolActive := int64(10)
	poolIdle := int64(3)
	poolInUse := int64(7)

	statsFunc := func() (active, idle, inUse int64) {
		return poolActive, poolIdle, poolInUse
	}

	metrics, err := builder.NewPoolMetrics("postgres", statsFunc)
	require.NoError(t, err)
	require.NotNil(t, metrics)

	ctx := context.Background()

	// Record wait
	metrics.RecordWait(ctx, attribute.String("pool", "main"))

	// Read metrics
	var rm metricdata.ResourceMetrics
	err = reader.Collect(ctx, &rm)
	require.NoError(t, err)

	// Verify gauge values
	foundActive := false
	foundIdle := false
	foundInUse := false

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch m.Name {
			case "db_postgres_connections_active":
				foundActive = true
				gauge := m.Data.(metricdata.Gauge[int64])
				assert.Equal(t, int64(10), gauge.DataPoints[0].Value)
			case "db_postgres_connections_idle":
				foundIdle = true
				gauge := m.Data.(metricdata.Gauge[int64])
				assert.Equal(t, int64(3), gauge.DataPoints[0].Value)
			case "db_postgres_connections_in_use":
				foundInUse = true
				gauge := m.Data.(metricdata.Gauge[int64])
				assert.Equal(t, int64(7), gauge.DataPoints[0].Value)
			}
		}
	}

	assert.True(t, foundActive, "Expected metric 'db_postgres_connections_active' not found")
	assert.True(t, foundIdle, "Expected metric 'db_postgres_connections_idle' not found")
	assert.True(t, foundInUse, "Expected metric 'db_postgres_connections_in_use' not found")
}

func TestMetricsBuilder_NewOperationMetrics(t *testing.T) {
	mp, reader := setupTestMeterProvider(t)
	defer mp.Shutdown(context.Background())

	meter := mp.Meter("test")
	builder := NewMetricsBuilder(meter, "redis")

	metrics, err := builder.NewOperationMetrics("command")
	require.NoError(t, err)
	require.NotNil(t, metrics)

	ctx := context.Background()

	// Record successful operation
	metrics.Record(ctx, 0.001, nil, attribute.String("cmd", "GET"))

	// Record failed operation
	metrics.Record(ctx, 0.002, errors.New("connection refused"), attribute.String("cmd", "SET"))

	// Read metrics
	var rm metricdata.ResourceMetrics
	err = reader.Collect(ctx, &rm)
	require.NoError(t, err)

	// Verify
	foundTotal := false
	foundSuccess := false
	foundErrors := false

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch m.Name {
			case "redis_command_operations_total":
				foundTotal = true
				sum := m.Data.(metricdata.Sum[int64])
				var total int64
				for _, dp := range sum.DataPoints {
					total += dp.Value
				}
				assert.Equal(t, int64(2), total)
			case "redis_command_operation_success_total":
				foundSuccess = true
				sum := m.Data.(metricdata.Sum[int64])
				var total int64
				for _, dp := range sum.DataPoints {
					total += dp.Value
				}
				assert.Equal(t, int64(1), total)
			case "redis_command_operation_errors_total":
				foundErrors = true
				sum := m.Data.(metricdata.Sum[int64])
				var total int64
				for _, dp := range sum.DataPoints {
					total += dp.Value
				}
				assert.Equal(t, int64(1), total)
			}
		}
	}

	assert.True(t, foundTotal, "Expected metric 'redis_command_operations_total' not found")
	assert.True(t, foundSuccess, "Expected metric 'redis_command_operation_success_total' not found")
	assert.True(t, foundErrors, "Expected metric 'redis_command_operation_errors_total' not found")
}

func TestMetricsBuilder_NewTokenMetrics(t *testing.T) {
	mp, reader := setupTestMeterProvider(t)
	defer mp.Shutdown(context.Background())

	meter := mp.Meter("test")
	builder := NewMetricsBuilder(meter, "jwt")

	metrics, err := builder.NewTokenMetrics("access")
	require.NoError(t, err)
	require.NotNil(t, metrics)

	ctx := context.Background()

	// Record token operations
	metrics.RecordGenerated(ctx, attribute.String("type", "access"))
	metrics.RecordVerified(ctx, 0.001, true)
	metrics.RecordRefreshed(ctx)
	metrics.RecordRevoked(ctx)

	// Read metrics
	var rm metricdata.ResourceMetrics
	err = reader.Collect(ctx, &rm)
	require.NoError(t, err)

	// Verify
	foundGenerated := false
	foundVerified := false
	foundRefreshed := false
	foundRevoked := false

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch m.Name {
			case "jwt_access_tokens_generated_total":
				foundGenerated = true
			case "jwt_access_tokens_verified_total":
				foundVerified = true
			case "jwt_access_tokens_refreshed_total":
				foundRefreshed = true
			case "jwt_access_tokens_revoked_total":
				foundRevoked = true
			}
		}
	}

	assert.True(t, foundGenerated, "Expected metric 'jwt_access_tokens_generated_total' not found")
	assert.True(t, foundVerified, "Expected metric 'jwt_access_tokens_verified_total' not found")
	assert.True(t, foundRefreshed, "Expected metric 'jwt_access_tokens_refreshed_total' not found")
	assert.True(t, foundRevoked, "Expected metric 'jwt_access_tokens_revoked_total' not found")
}

func TestMetricsBuilder_NewCacheMetrics(t *testing.T) {
	mp, reader := setupTestMeterProvider(t)
	defer mp.Shutdown(context.Background())

	meter := mp.Meter("test")
	builder := NewMetricsBuilder(meter, "redis")

	metrics, err := builder.NewCacheMetrics("session")
	require.NoError(t, err)
	require.NotNil(t, metrics)

	ctx := context.Background()

	// Record cache operations
	metrics.RecordHit(ctx, attribute.String("key_type", "user_session"))
	metrics.RecordMiss(ctx, attribute.String("key_type", "user_session"))
	metrics.RecordSet(ctx)
	metrics.RecordDelete(ctx)

	// Read metrics
	var rm metricdata.ResourceMetrics
	err = reader.Collect(ctx, &rm)
	require.NoError(t, err)

	// Verify
	foundHits := false
	foundMisses := false
	foundGets := false
	foundSets := false
	foundDeletes := false

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch m.Name {
			case "redis_session_cache_hits_total":
				foundHits = true
			case "redis_session_cache_misses_total":
				foundMisses = true
			case "redis_session_cache_gets_total":
				foundGets = true
				sum := m.Data.(metricdata.Sum[int64])
				var total int64
				for _, dp := range sum.DataPoints {
					total += dp.Value
				}
				// 1 hit + 1 miss = 2 gets
				assert.Equal(t, int64(2), total)
			case "redis_session_cache_sets_total":
				foundSets = true
			case "redis_session_cache_deletes_total":
				foundDeletes = true
			}
		}
	}

	assert.True(t, foundHits, "Expected metric 'redis_session_cache_hits_total' not found")
	assert.True(t, foundMisses, "Expected metric 'redis_session_cache_misses_total' not found")
	assert.True(t, foundGets, "Expected metric 'redis_session_cache_gets_total' not found")
	assert.True(t, foundSets, "Expected metric 'redis_session_cache_sets_total' not found")
	assert.True(t, foundDeletes, "Expected metric 'redis_session_cache_deletes_total' not found")
}

func TestMetricsBuilder_EmptyNamespace(t *testing.T) {
	mp, reader := setupTestMeterProvider(t)
	defer mp.Shutdown(context.Background())

	meter := mp.Meter("test")
	builder := NewMetricsBuilder(meter, "") // Empty namespace

	counter, err := builder.Counter("requests_total", "Total requests")
	require.NoError(t, err)
	require.NotNil(t, counter)

	ctx := context.Background()
	counter.Add(ctx, 1)

	// Read metrics
	var rm metricdata.ResourceMetrics
	err = reader.Collect(ctx, &rm)
	require.NoError(t, err)

	// Verify - name should be without prefix
	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "requests_total" {
				found = true
			}
		}
	}
	assert.True(t, found, "Expected metric 'requests_total' not found (without namespace prefix)")
}

func BenchmarkRequestMetrics_Record(b *testing.B) {
	mp, _ := setupTestMeterProvider(&testing.T{})
	defer mp.Shutdown(context.Background())

	meter := mp.Meter("bench")
	builder := NewMetricsBuilder(meter, "http")
	metrics, _ := builder.NewRequestMetrics("api")

	ctx := context.Background()
	attrs := []attribute.KeyValue{
		attribute.String("method", "GET"),
		attribute.String("path", "/api/users"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics.Record(ctx, 0.001*float64(i%100), nil, attrs...)
	}
}

func BenchmarkOperationMetrics_Record(b *testing.B) {
	mp, _ := setupTestMeterProvider(&testing.T{})
	defer mp.Shutdown(context.Background())

	meter := mp.Meter("bench")
	builder := NewMetricsBuilder(meter, "redis")
	metrics, _ := builder.NewOperationMetrics("command")

	ctx := context.Background()
	attrs := []attribute.KeyValue{
		attribute.String("cmd", "GET"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var err error
		if i%10 == 0 {
			err = errors.New("test error")
		}
		metrics.Record(ctx, float64(time.Microsecond*100)/float64(time.Second), err, attrs...)
	}
}
