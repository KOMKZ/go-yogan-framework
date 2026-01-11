package config

import (
	"testing"
)

// TestLoader_Basic 测试基本加载
func TestLoader_Basic(t *testing.T) {
	loader := NewLoader()

	// 添加文件数据源
	loader.AddSource(NewFileSource("testdata/config.yaml", 10))

	// 加载
	if err := loader.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// 验证配置值
	if loader.GetString("app.name") != "test-app" {
		t.Errorf("app.name = %s, want test-app", loader.GetString("app.name"))
	}

	if loader.GetInt("grpc.server.port") != 9002 {
		t.Errorf("grpc.server.port = %d, want 9002", loader.GetInt("grpc.server.port"))
	}
}

// TestLoader_MultipleFiles 测试多文件加载（优先级）
func TestLoader_MultipleFiles(t *testing.T) {
	loader := NewLoader()

	// 添加基础配置（低优先级）
	loader.AddSource(NewFileSource("testdata/config.yaml", 10))

	// 添加环境配置（高优先级）
	loader.AddSource(NewFileSource("testdata/dev.yaml", 20))

	// 加载
	if err := loader.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// dev.yaml 应该覆盖 config.yaml
	if loader.GetInt("grpc.server.port") != 9999 {
		t.Errorf("grpc.server.port = %d, want 9999 (from dev.yaml)", loader.GetInt("grpc.server.port"))
	}

	if loader.GetInt("api_server.port") != 8888 {
		t.Errorf("api_server.port = %d, want 8888 (from dev.yaml)", loader.GetInt("api_server.port"))
	}

	// config.yaml 中的其他配置应该保留
	if loader.GetString("app.name") != "test-app" {
		t.Errorf("app.name = %s, want test-app", loader.GetString("app.name"))
	}
}

// TestLoader_WithFlags 测试命令行参数覆盖
func TestLoader_WithFlags(t *testing.T) {
	type TestFlags struct {
		Port    int
		Address string
	}

	loader := NewLoader()

	// 添加文件配置（低优先级）
	loader.AddSource(NewFileSource("testdata/config.yaml", 10))

	// 添加命令行参数（最高优先级）
	loader.AddSource(NewFlagSource(&TestFlags{
		Port:    7777,
		Address: "10.0.0.1",
	}, "grpc", 100))

	// 加载
	if err := loader.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// 命令行参数应该覆盖文件配置
	if loader.GetInt("grpc.server.port") != 7777 {
		t.Errorf("grpc.server.port = %d, want 7777 (from flags)", loader.GetInt("grpc.server.port"))
	}

	if loader.GetString("grpc.server.address") != "10.0.0.1" {
		t.Errorf("grpc.server.address = %s, want 10.0.0.1 (from flags)", loader.GetString("grpc.server.address"))
	}

	// 文件中的其他配置应该保留
	if loader.GetString("app.name") != "test-app" {
		t.Errorf("app.name = %s, want test-app", loader.GetString("app.name"))
	}
}

// TestLoader_AllSources 测试所有数据源（完整优先级）
func TestLoader_AllSources(t *testing.T) {
	type TestFlags struct {
		Port int
	}

	loader := NewLoader()

	// 1. 基础配置文件（优先级 10）
	loader.AddSource(NewFileSource("testdata/config.yaml", 10))

	// 2. 环境配置文件（优先级 20）
	loader.AddSource(NewFileSource("testdata/dev.yaml", 20))

	// 3. 环境变量（优先级 50）- 暂时跳过，避免污染测试环境

	// 4. 命令行参数（优先级 100）
	loader.AddSource(NewFlagSource(&TestFlags{
		Port: 6666,
	}, "grpc", 100))

	// 加载
	if err := loader.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// 最终应该使用命令行参数
	if loader.GetInt("grpc.server.port") != 6666 {
		t.Errorf("grpc.server.port = %d, want 6666 (from flags, highest priority)", loader.GetInt("grpc.server.port"))
	}
}

// TestLoader_Unmarshal 测试反序列化
func TestLoader_Unmarshal(t *testing.T) {
	type AppConfig struct {
		App struct {
			Name    string `mapstructure:"name"`
			Version string `mapstructure:"version"`
		} `mapstructure:"app"`
		Grpc struct {
			Server struct {
				Port    int    `mapstructure:"port"`
				Address string `mapstructure:"address"`
			} `mapstructure:"server"`
		} `mapstructure:"grpc"`
	}

	loader := NewLoader()
	loader.AddSource(NewFileSource("testdata/config.yaml", 10))

	if err := loader.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	var cfg AppConfig
	if err := loader.Unmarshal(&cfg); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	// 验证反序列化结果
	if cfg.App.Name != "test-app" {
		t.Errorf("cfg.App.Name = %s, want test-app", cfg.App.Name)
	}

	if cfg.Grpc.Server.Port != 9002 {
		t.Errorf("cfg.Grpc.Server.Port = %d, want 9002", cfg.Grpc.Server.Port)
	}
}

// TestLoader_GetLoadedFiles 测试获取已加载文件列表
func TestLoader_GetLoadedFiles(t *testing.T) {
	loader := NewLoader()

	loader.AddSource(NewFileSource("testdata/config.yaml", 10))
	loader.AddSource(NewFileSource("testdata/dev.yaml", 20))

	if err := loader.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	files := loader.GetLoadedFiles()
	if len(files) != 2 {
		t.Errorf("GetLoadedFiles() = %d files, want 2", len(files))
	}

	// 验证文件路径
	expectedFiles := []string{
		"testdata/config.yaml",
		"testdata/dev.yaml",
	}

	for i, expected := range expectedFiles {
		if files[i] != expected {
			t.Errorf("files[%d] = %s, want %s", i, files[i], expected)
		}
	}
}

// TestLoader_Reload 测试重新加载
func TestLoader_Reload(t *testing.T) {
	loader := NewLoader()
	loader.AddSource(NewFileSource("testdata/config.yaml", 10))

	// 第一次加载
	if err := loader.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	port1 := loader.GetInt("grpc.server.port")

	// 重新加载
	if err := loader.Reload(); err != nil {
		t.Fatalf("Reload() error: %v", err)
	}

	port2 := loader.GetInt("grpc.server.port")

	// 应该保持一致
	if port1 != port2 {
		t.Errorf("port changed after reload: %d -> %d", port1, port2)
	}
}

// TestSplitKey 测试 key 分割
func TestSplitKey(t *testing.T) {
	tests := []struct {
		key      string
		expected []string
	}{
		{"grpc.server.port", []string{"grpc", "server", "port"}},
		{"app.name", []string{"app", "name"}},
		{"simple", []string{"simple"}},
		{"", []string{}},
		{"a.b.c.d.e", []string{"a", "b", "c", "d", "e"}},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := splitKey(tt.key)

			if len(result) != len(tt.expected) {
				t.Errorf("splitKey(%s) = %v, want %v", tt.key, result, tt.expected)
				return
			}

			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("splitKey(%s)[%d] = %s, want %s", tt.key, i, v, tt.expected[i])
				}
			}
		})
	}
}

