package redis

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "有效的单机配置",
			config: Config{
				Mode:  "standalone",
				Addrs: []string{"localhost:6379"},
				DB:    0,
			},
			wantErr: false,
		},
		{
			name: "有效的集群配置",
			config: Config{
				Mode: "cluster",
				Addrs: []string{
					"localhost:7000",
					"localhost:7001",
					"localhost:7002",
				},
			},
			wantErr: false,
		},
		{
			name: "无效的模式",
			config: Config{
				Mode:  "invalid",
				Addrs: []string{"localhost:6379"},
			},
			wantErr: true,
			errMsg:  "invalid mode",
		},
		{
			name: "空地址列表",
			config: Config{
				Mode:  "standalone",
				Addrs: []string{},
			},
			wantErr: true,
			errMsg:  "addrs cannot be empty",
		},
		{
			name: "单机模式 DB 超出范围",
			config: Config{
				Mode:  "standalone",
				Addrs: []string{"localhost:6379"},
				DB:    16,
			},
			wantErr: true,
			errMsg:  "db must be between 0 and 15",
		},
		{
			name: "负数连接池大小",
			config: Config{
				Mode:     "standalone",
				Addrs:    []string{"localhost:6379"},
				PoolSize: -1,
			},
			wantErr: true,
			errMsg:  "pool_size must be >= 0",
		},
		{
			name: "负数最小空闲连接数",
			config: Config{
				Mode:         "standalone",
				Addrs:        []string{"localhost:6379"},
				MinIdleConns: -1,
			},
			wantErr: true,
			errMsg:  "min_idle_conns must be >= 0",
		},
		{
			name: "集群模式 DB 可以任意值",
			config: Config{
				Mode:  "cluster",
				Addrs: []string{"localhost:7000", "localhost:7001"},
				DB:    16, // cluster 模式不验证 DB
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_ApplyDefaults(t *testing.T) {
	cfg := Config{
		Mode:  "",
		Addrs: []string{"localhost:6379"},
	}

	cfg.ApplyDefaults()

	assert.Equal(t, "standalone", cfg.Mode)
	assert.Equal(t, 10, cfg.PoolSize)
	assert.Equal(t, 5, cfg.MinIdleConns)
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 5*time.Second, cfg.DialTimeout)
	assert.Equal(t, 3*time.Second, cfg.ReadTimeout)
	assert.Equal(t, 3*time.Second, cfg.WriteTimeout)
}

func TestConfig_ApplyDefaults_PreservesExisting(t *testing.T) {
	cfg := Config{
		Mode:         "cluster",
		Addrs:        []string{"localhost:7000"},
		PoolSize:     20,
		MinIdleConns: 10,
		MaxRetries:   5,
		DialTimeout:  10 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	cfg.ApplyDefaults()

	// The set value should not be overwritten
	assert.Equal(t, "cluster", cfg.Mode)
	assert.Equal(t, 20, cfg.PoolSize)
	assert.Equal(t, 10, cfg.MinIdleConns)
	assert.Equal(t, 5, cfg.MaxRetries)
	assert.Equal(t, 10*time.Second, cfg.DialTimeout)
	assert.Equal(t, 5*time.Second, cfg.ReadTimeout)
	assert.Equal(t, 5*time.Second, cfg.WriteTimeout)
}

func TestConfig_ApplyDefaults_AddrBackwardCompat(t *testing.T) {
	// 测试 Addr (单数) 到 Addrs (复数) 的向后兼容
	cfg := Config{
		Addr: "localhost:6379", // 使用旧的 Addr 字段
	}

	cfg.ApplyDefaults()

	assert.Equal(t, []string{"localhost:6379"}, cfg.Addrs)
	assert.Equal(t, "standalone", cfg.Mode)
}

func TestConfig_ApplyDefaults_AddrsHasPriority(t *testing.T) {
	// 如果同时设置了 Addr 和 Addrs，Addrs 优先
	cfg := Config{
		Addr:  "localhost:6379",
		Addrs: []string{"localhost:7000", "localhost:7001"},
	}

	cfg.ApplyDefaults()

	// Addrs 不应被覆盖
	assert.Equal(t, []string{"localhost:7000", "localhost:7001"}, cfg.Addrs)
}
