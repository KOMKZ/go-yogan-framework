package logger

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGinLogWriter 测试 Gin 日志适配器
func TestGinLogWriter(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "gin")

	globalManager = nil
	managerOnce = sync.Once{}

	InitManager(ManagerConfig{
		BaseLogDir:            logDir,
		Level:                 "debug",
		Encoding:              "json",
		EnableConsole:         false,
		EnableLevelInFilename: true,
		EnableDateInFilename:  false,
		MaxSize:               10,
	})

	// 创建 GinLogWriter
	writer := NewGinLogWriter("gin")
	assert.NotNil(t, writer)

	// 写入日志
	n, err := writer.Write([]byte("[GIN-debug] GET /api/users --> handler.GetUsers (3 handlers)"))
	assert.NoError(t, err)
	assert.Greater(t, n, 0)

	n, err = writer.Write([]byte("[GIN] 2026/01/20 - 16:30:45 | 200 | 1.234ms | 127.0.0.1 | GET \"/api/health\""))
	assert.NoError(t, err)
	assert.Greater(t, n, 0)

	CloseAll()

	// 验证日志文件
	content, _ := os.ReadFile(filepath.Join(logDir, "gin", "gin-info.log"))
	contentStr := string(content)
	assert.Contains(t, contentStr, "GIN-debug")
	assert.Contains(t, contentStr, "GIN")
}
