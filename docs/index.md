# Yogan Framework Docs Index

## P0

| 文档 | 用途 |
|------|------|
| [../CLAUDE.md](../CLAUDE.md) | AI 开发门禁、工作区规则、验证命令 |
| [framework-patterns/index.md](framework-patterns/index.md) | Provider、DI、配置、健康检查、关闭、缓存和组件清单 |

## 文档同步门禁

| 场景 | 动作 |
|------|------|
| 新增框架能力 | 新增或更新 `docs/` 对应文档 |
| 修改框架行为 | 检查并更新已有文档 |
| 新增文档 | 更新本索引和子目录索引 |
| 不需要文档变更 | 在交付说明中写明原因 |

## 开发入口

| 场景 | 必读 |
|------|------|
| 新增内核组件 | `framework-patterns/kernel-provider-template.md`、`framework-patterns/kernel-provider-register.md`、`framework-patterns/kernel-config.md` |
| 接入依赖注入 | `framework-patterns/kernel-do.md`、`framework-patterns/kernel-do-provide.md`、`framework-patterns/kernel-do-invoke.md` |
| 组件生命周期 | `framework-patterns/kernel-healthcheck.md`、`framework-patterns/kernel-shutdown.md` |
| 缓存失效 | `framework-patterns/kernel-cache.md` |
| HTTP 限速 | `framework-patterns/kernel-limiter.md` |
| 队列任务 | `framework-patterns/kernel-queue.md` |
| 应用侧访问内核 | `framework-patterns/kernel-apputil.md` |
| 查已有能力 | `framework-patterns/kernel-components-list.md` |

## 命令

```bash
go test ./...
go build ./...
```
