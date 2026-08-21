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

## 多应用环境隔离

| 机制 | 口径 |
|------|------|
| 环境文件选择 | 优先取 `AppFlags.Env`（随 `di.ConfigOptions.Env` 传入 loader）；全局 `APP_ENV`/`ENV` 仅作直接使用 `config.LoaderBuilder` 时的旧式回退 |
| 环境变量前缀 | 每应用独立前缀：`*WithDefaults` 构造器从 appName 推导（`EnvPrefixFor`：`user-api` → `USER_API`）；显式构造时由调用方传入，**空前缀表示禁用 env source**，不再兜底 `"APP"` |
| `ParseFlags` | 只读每应用变量（`{APP}_ENV` 等），**绝不写全局 `APP_ENV`**——同一进程内多个应用互不串扰 |
| env 扫描 | `EnvSource` 只扫描 `{prefix}_` 开头的变量；不要依赖 `APP_*` 通用前缀 |
