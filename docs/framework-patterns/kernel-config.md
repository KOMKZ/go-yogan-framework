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
├── config.yaml          # manifest/bootstrap
├── runtime.yaml         # 启动、日志、中间件等运行时基础配置
├── runtime.test.yaml    # runtime 的 test profile 覆盖
├── database.yaml        # 数据库配置
├── database.test.yaml   # database 的 test profile 覆盖
├── job.yaml             # 队列和任务配置
├── media.yaml           # 媒体和外部 provider 配置
└── rate_limiter.yaml    # 限速配置，manifest mode 下必须显式导入
```

`config.yaml` 可以使用 manifest mode 显式声明要导入的配置文件：

```yaml
yogan:
  config:
    mode: "manifest"
    imports:
      - runtime.yaml
      - database.yaml
      - job.yaml
      - media.yaml
      - rate_limiter.yaml
```

加载规则：

| 模式 | 规则 |
|------|------|
| legacy | `config.yaml`、`rate_limiter.yaml`、`{env}.yaml`、env vars、flags |
| manifest | `config.yaml`、`imports`、`imports` 的 profile 覆盖、env vars、flags |

manifest mode 下不自动加载全局 `{env}.yaml`。环境差异跟随被导入文件命名，例如 `database.yaml` 的 test 覆盖文件是 `database.test.yaml`。多个 profile 用逗号分隔，后面的 profile 覆盖前面的 profile。

基础 import 之间不得重复配置 key。profile 覆盖文件只能覆盖对应基础文件拥有的顶层 section，不能把其他模块配置塞进当前文件。

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
| port/address 绑定 | builder 按 appType 注入显式绑定：http → `{PREFIX}_PORT`→`api_server.port`、`{PREFIX}_ADDRESS`→`api_server.host`；grpc → `grpc.server.port/address`。绑定与通用扫描合并生效，绑定变量不再产生顶层杂键 |
| 实际端口 | `HTTPServer` 先绑 listener 再 Serve（消除预检 TOCTOU）；`port: 0` 时用 `GetActualPort()` 取系统分配端口 |
