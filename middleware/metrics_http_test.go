package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestNewHTTPMetrics(t *testing.T) {
	t.Run("creates with config", func(t *testing.T) {
		cfg := HTTPMetricsConfig{
			Enabled:            true,
			RecordRequestSize:  true,
			RecordResponseSize: false,
		}
		m := NewHTTPMetrics(cfg)
		
		assert.NotNil(t, m)
		assert.True(t, m.config.Enabled)
		assert.True(t, m.config.RecordRequestSize)
		assert.False(t, m.config.RecordResponseSize)
		assert.False(t, m.registered)
	})
}

func TestHTTPMetrics_MetricsProvider(t *testing.T) {
	t.Run("MetricsName returns http", func(t *testing.T) {
		m := NewHTTPMetrics(HTTPMetricsConfig{Enabled: true})
		assert.Equal(t, "http", m.MetricsName())
	})

	t.Run("IsMetricsEnabled reflects config", func(t *testing.T) {
		m1 := NewHTTPMetrics(HTTPMetricsConfig{Enabled: true})
		assert.True(t, m1.IsMetricsEnabled())

		m2 := NewHTTPMetrics(HTTPMetricsConfig{Enabled: false})
		assert.False(t, m2.IsMetricsEnabled())
	})
}

func TestHTTPMetrics_RegisterMetrics(t *testing.T) {
	t.Run("registers basic metrics", func(t *testing.T) {
		mp := noop.NewMeterProvider()
		meter := mp.Meter("test")
		
		m := NewHTTPMetrics(HTTPMetricsConfig{Enabled: true})
		err := m.RegisterMetrics(meter)
		
		require.NoError(t, err)
		assert.True(t, m.IsRegistered())
		assert.NotNil(t, m.requestsTotal)
		assert.NotNil(t, m.requestDuration)
		assert.NotNil(t, m.requestsInFlight)
	})

	t.Run("registers optional size metrics", func(t *testing.T) {
		mp := noop.NewMeterProvider()
		meter := mp.Meter("test")
		
		m := NewHTTPMetrics(HTTPMetricsConfig{
			Enabled:            true,
			RecordRequestSize:  true,
			RecordResponseSize: true,
		})
		err := m.RegisterMetrics(meter)
		
		require.NoError(t, err)
		assert.NotNil(t, m.requestSize)
		assert.NotNil(t, m.responseSize)
	})

	t.Run("skips optional metrics when disabled", func(t *testing.T) {
		mp := noop.NewMeterProvider()
		meter := mp.Meter("test")
		
		m := NewHTTPMetrics(HTTPMetricsConfig{
			Enabled:            true,
			RecordRequestSize:  false,
			RecordResponseSize: false,
		})
		err := m.RegisterMetrics(meter)
		
		require.NoError(t, err)
		assert.Nil(t, m.requestSize)
		assert.Nil(t, m.responseSize)
	})

	t.Run("idempotent registration", func(t *testing.T) {
		mp := noop.NewMeterProvider()
		meter := mp.Meter("test")
		
		m := NewHTTPMetrics(HTTPMetricsConfig{Enabled: true})
		
		err1 := m.RegisterMetrics(meter)
		require.NoError(t, err1)
		
		err2 := m.RegisterMetrics(meter)
		require.NoError(t, err2)
	})
}

func TestHTTPMetrics_Handler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("skips when not registered", func(t *testing.T) {
		m := NewHTTPMetrics(HTTPMetricsConfig{Enabled: true})
		// Not registered
		
		router := gin.New()
		router.Use(m.Handler())
		router.GET("/test", func(c *gin.Context) {
			c.String(http.StatusOK, "ok")
		})
		
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("records metrics when registered", func(t *testing.T) {
		mp := noop.NewMeterProvider()
		meter := mp.Meter("test")
		
		m := NewHTTPMetrics(HTTPMetricsConfig{Enabled: true})
		err := m.RegisterMetrics(meter)
		require.NoError(t, err)
		
		router := gin.New()
		router.Use(m.Handler())
		router.GET("/test", func(c *gin.Context) {
			c.String(http.StatusOK, "ok")
		})
		
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestGetStatusClass(t *testing.T) {
	tests := []struct {
		code     int
		expected string
	}{
		{200, "2xx"},
		{201, "2xx"},
		{299, "2xx"},
		{301, "3xx"},
		{304, "3xx"},
		{400, "4xx"},
		{404, "4xx"},
		{500, "5xx"},
		{503, "5xx"},
		{100, "unknown"},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.code), func(t *testing.T) {
			result := getStatusClass(tt.code)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Ensure HTTPMetrics implements the MetricsProvider-like interface
func TestHTTPMetrics_Interface(t *testing.T) {
	m := NewHTTPMetrics(HTTPMetricsConfig{Enabled: true})
	
	// Check interface methods exist
	_ = m.MetricsName()
	_ = m.IsMetricsEnabled()
	
	mp := noop.NewMeterProvider()
	meter := mp.Meter("test")
	_ = m.RegisterMetrics(meter)
}

// mockMeter for testing error paths
type errorMeter struct {
	metric.Meter
}

func (m *errorMeter) Int64Counter(name string, opts ...metric.Int64CounterOption) (metric.Int64Counter, error) {
	return nil, assert.AnError
}
