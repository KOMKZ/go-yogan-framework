# go-yogan-framework

## P0

| 项 | 规则 |
|----|------|
| 文档索引 | 修改框架前先读 [docs/index.md](docs/index.md) |
| 依赖注入 | 使用 `samber/do` Provider 模式，不恢复旧 Registry |
| 项目边界 | 框架层禁止 import `yogan-domains`、`rong-admin-api` 或业务应用包 |
| 配置 | 组件配置必须有 defaults 和 validate |
| 生命周期 | 持有连接、goroutine、worker、client 的组件必须考虑 Shutdown |
| 健康检查 | 可探活组件实现 `component.HealthChecker` |
| 日志 | 使用框架 logger，保留 ctx 和结构化字段 |
| 文档同步 | 新增或修改框架能力时，必须同步更新 `docs/` 对应文档和索引 |

## 常用命令

```bash
go test ./...
go build ./...
```

## 文档维护

- 新增框架能力、组件、Provider、配置项、生命周期规则或推荐用法时，写入 `docs/` 对应文档。
- 新增文档后更新 `docs/index.md` 和相关子目录索引。
- 修改已有能力时，同步检查对应文档是否过时；过时必须本次更新。
- 如果代码变更不需要文档更新，在交付说明中写明原因。
- 不把具体框架知识沉淀到 skill；skill 只保留流程和门禁。
