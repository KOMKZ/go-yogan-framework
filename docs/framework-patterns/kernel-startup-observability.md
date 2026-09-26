# HTTP 启动分段可观测性

## 目的

HTTP 应用启动时会输出统一的 `HTTP startup phase completed` 结构化日志，用于区分配置/DI、路由、TCP listener 和业务 `OnReady` 的耗时。消费方不应根据日志出现顺序猜测 listener 是否已经绑定。

## 稳定字段

| 字段 | 含义 |
|---|---|
| `startup_phase` | 稳定的阶段标识 |
| `phase_duration_ms` | 当前阶段耗时；checkpoint 阶段可以没有该字段 |
| `startup_elapsed_ms` | 从 `NewBase` 起表到当前日志的累计毫秒数 |
| `port` | listener 相关阶段的实际监听端口 |
| `bind_duration_ms` | `net.Listen` 调用耗时，仅 `listener_bound` 提供 |

框架当前阶段依次为：

1. `application_initialization`：`NewBase`、配置与应用构造完成，到进入运行流程为止。
2. `setup`：框架 setup 与应用 `OnSetup`。
3. `http_routes`：HTTP server、中间件、健康路由、业务路由与 Swagger 装配。
4. `listener_bound`：`net.Listen` 已成功，端口已经被进程占用；这是判断 listener 前后故障窗口的精确边界。
5. `listener_start_confirmation`：Serve goroutine 启动后的确认等待完成。
6. `on_ready`：应用 `OnReady` 全部返回。

`RunInProcess` 不绑定 TCP，因此不会输出两个 listener 阶段，其余阶段保持一致。

## 应用侧扩展

应用可以在 `OnReady` 内继续使用相同字段记录业务子阶段，例如权限快照同步或启动 seed 整理。应用不得伪造 framework 阶段名，也不得把凭据、DSN 或密文写入阶段日志。

## 兼容性

原有最终日志 `✅ HTTP application started` 与 `startup_time` 字段保持不变。新阶段日志是附加可观测性，不改变生命周期顺序、健康检查或公共 Go API。
