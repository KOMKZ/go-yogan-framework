# Framework Patterns Index

## 文档

| 文档 | 内容 |
|------|------|
| [kernel-provider-template.md](kernel-provider-template.md) | Provider 函数模板 |
| [kernel-provider-register.md](kernel-provider-register.md) | Provider 注册和组件依赖层级 |
| [kernel-do.md](kernel-do.md) | samber/do 直接使用模式 |
| [kernel-do-provide.md](kernel-do-provide.md) | do.Provide 注册应用层 Provider |
| [kernel-do-invoke.md](kernel-do-invoke.md) | do.Invoke / do.MustInvoke 获取组件 |
| [kernel-config.md](kernel-config.md) | 配置结构、默认值、校验和配置文件模式 |
| [kernel-healthcheck.md](kernel-healthcheck.md) | HealthChecker 接口 |
| [kernel-shutdown.md](kernel-shutdown.md) | do.Shutdownable 接口 |
| [kernel-cache.md](kernel-cache.md) | 缓存事件驱动失效 |
| [kernel-limiter.md](kernel-limiter.md) | HTTP 限速配置、规则限速和治理原则 |
| [kernel-jwt.md](kernel-jwt.md) | JWT 中间件 access-only 默认边界和 refresh token 使用约束 |
| [kernel-queue.md](kernel-queue.md) | Queue/Asynq worker 抽象、配置、注册方式 |
| [kernel-apputil.md](kernel-apputil.md) | apputil 快捷访问内核组件 |
| [kernel-components-list.md](kernel-components-list.md) | 内核组件清单 |

## 使用规则

- 修改框架代码前，先读对应文档。
- 文档与代码冲突时，以代码为准，同时更新文档。
- 新增可复用模式、Provider 模式、配置规则、生命周期规则或组件清单项时，补充对应文档和本索引。
- 删除或改名模式文档时，同步更新 `docs/index.md`。
