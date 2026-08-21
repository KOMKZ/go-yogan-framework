# 配置结构规范

## 配置结构

```go
// mypackage/config.go

type Config struct {
    Enabled bool          `mapstructure:"enabled"`
    Timeout time.Duration `mapstructure:"timeout"`
}

func (c *Config) ApplyDefaults() {
    if c.Timeout == 0 {
        c.Timeout = 30 * time.Second
    }
}

func (c *Config) Validate() error {
    if c.Enabled && c.Timeout <= 0 {
        return fmt.Errorf("timeout must be positive")
    }
    return nil
}
```

## 配置文件模式

```
src/apps/user-api/config/
├── config.yaml      # 主配置
├── dev.yaml         # 开发环境覆盖
├── test.yaml        # 测试环境覆盖
└── prod.yaml        # 生产环境覆盖
```

## 配置示例

```yaml
database:
  connections:
    master:
      driver: "mysql"
      dsn: "root:password@tcp(localhost:3306)/app?charset=utf8mb4"
      max_open_conns: 100
      max_idle_conns: 10

redis:
  instances:
    main:
      mode: "standalone"
      addrs:
        - "localhost:6379"
      pool_size: 10
```
