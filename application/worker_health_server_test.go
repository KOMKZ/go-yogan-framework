package application

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/KOMKZ/go-yogan-framework/config"
	"github.com/KOMKZ/go-yogan-framework/health"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeWorkerHealthChecker struct {
	name string
	err  error
}

func (f *fakeWorkerHealthChecker) Name() string { return f.name }

func (f *fakeWorkerHealthChecker) Check(context.Context) error { return f.err }

// newHealthTestLoader 构造带 health 段的测试 loader（绕过文件加载，直接写入 Viper）。
func newHealthTestLoader(t *testing.T, values map[string]any) *config.Loader {
	t.Helper()
	loader := config.NewLoader()
	for key, value := range values {
		loader.GetViper().Set(key, value)
	}
	return loader
}

func freeListenAddr(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())
	return addr
}

func TestLoadHealthConfig_DefaultsWhenUnset(t *testing.T) {
	cfg, err := loadHealthConfig(config.NewLoader())

	require.NoError(t, err)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 5*time.Second, cfg.Timeout)
	assert.Empty(t, cfg.Listen)
}

func TestLoadHealthConfig_DecodesListenAndRejectsInvalid(t *testing.T) {
	loader := newHealthTestLoader(t, map[string]any{
		"health": map[string]any{"enabled": true, "timeout": "3s", "listen": "0.0.0.0:9203"},
	})

	cfg, err := loadHealthConfig(loader)

	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0:9203", cfg.Listen)
	assert.Equal(t, 3*time.Second, cfg.Timeout)

	bad := newHealthTestLoader(t, map[string]any{
		"health": map[string]any{"enabled": true, "listen": "no-port"},
	})
	_, err = loadHealthConfig(bad)
	assert.Error(t, err)
}

func TestWorkerHealthMux_HealthyAndReadiness(t *testing.T) {
	agg := health.NewAggregator(time.Second)
	agg.Register(&fakeWorkerHealthChecker{name: "queue"})

	recorder := httptest.NewRecorder()
	newWorkerHealthMux(agg).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
	assert.Equal(t, http.StatusOK, recorder.Code)

	recorder = httptest.NewRecorder()
	newWorkerHealthMux(agg).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/readiness", nil))
	assert.Equal(t, http.StatusOK, recorder.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, "healthy", body["status"])
}

func TestWorkerHealthMux_UnhealthyFailsReadinessButKeepsLiveness(t *testing.T) {
	agg := health.NewAggregator(time.Second)
	agg.Register(&fakeWorkerHealthChecker{name: "queue", err: assert.AnError})

	recorder := httptest.NewRecorder()
	newWorkerHealthMux(agg).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)

	recorder = httptest.NewRecorder()
	newWorkerHealthMux(agg).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/readiness", nil))
	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)

	recorder = httptest.NewRecorder()
	newWorkerHealthMux(agg).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/liveness", nil))
	assert.Equal(t, http.StatusOK, recorder.Code)
}

func TestWorkerApplication_StartsAndStopsHealthServer(t *testing.T) {
	addr := freeListenAddr(t)
	configDir := t.TempDir()
	writeWorkerHealthConfigFile(t, configDir,
		"health:\n  enabled: true\n  timeout: 2s\n  listen: "+addr+"\n")
	app := NewWorker(configDir, "WORKER_HEALTH_TEST", "media", nil)

	// 只验证健康 listener 生命周期，不启动队列消费（queue server 需要真实 broker）。
	require.NoError(t, app.startWorkerHealthServer())
	require.NotNil(t, app.healthServer)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		assert.NoError(t, app.healthServer.shutdown(ctx))
	}()

	resp, err := pollWorkerHealthEndpoint(t, "http://"+addr+"/health/liveness")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// pollWorkerHealthEndpoint 轮询直到 listener 完成 bind（启动在独立 goroutine，存在竞态窗口）。
func pollWorkerHealthEndpoint(t *testing.T, url string) (*http.Response, error) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		time.Sleep(20 * time.Millisecond)
	}
	return nil, lastErr
}

func TestWorkerApplication_SkipsHealthServerWhenListenEmpty(t *testing.T) {
	configDir := t.TempDir()
	writeWorkerHealthConfigFile(t, configDir, "health:\n  enabled: true\n")
	app := NewWorker(configDir, "WORKER_HEALTH_TEST", "media", nil)

	require.NoError(t, app.startWorkerHealthServer())
	assert.Nil(t, app.healthServer)
}

func TestWorkerApplication_FailsFastOnInvalidHealthConfig(t *testing.T) {
	configDir := t.TempDir()
	writeWorkerHealthConfigFile(t, configDir,
		"health:\n  enabled: true\n  timeout: 0s\n  listen: 127.0.0.1:9203\n")
	app := NewWorker(configDir, "WORKER_HEALTH_TEST", "media", nil)

	err := app.startWorkerHealthServer()

	require.Error(t, err)
	assert.Nil(t, app.healthServer)
}

// writeWorkerHealthConfigFile 写入 loader 基础配置文件（<configPath>/config.yaml）。
func writeWorkerHealthConfigFile(t *testing.T, configDir, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(content), 0o644))
}
