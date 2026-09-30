package application

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCronApplication_StartsAndStopsHealthServer 验证 cron 应用与 worker 同口径的健康 listener 生命周期。
func TestCronApplication_StartsAndStopsHealthServer(t *testing.T) {
	configDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.yaml"),
		[]byte("health:\n  enabled: true\n  timeout: 2s\n  listen: "+freeListenAddr(t)+"\n"), 0o644))
	app, err := NewCron(configDir, "CRON_HEALTH_TEST")
	require.NoError(t, err)

	// 只验证健康 listener 生命周期，不启动调度器（需要任务注册与真实依赖）。
	require.NoError(t, app.startCronHealthServer())
	require.NotNil(t, app.healthServer)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		assert.NoError(t, app.healthServer.shutdown(ctx))
	}()

	resp, err := pollWorkerHealthEndpoint(t, "http://"+app.healthServer.addr+"/health/liveness")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestCronApplication_SkipsHealthServerWhenListenEmpty(t *testing.T) {
	configDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.yaml"),
		[]byte("health:\n  enabled: true\n"), 0o644))
	app, err := NewCron(configDir, "CRON_HEALTH_TEST")
	require.NoError(t, err)

	require.NoError(t, app.startCronHealthServer())
	assert.Nil(t, app.healthServer)
}
