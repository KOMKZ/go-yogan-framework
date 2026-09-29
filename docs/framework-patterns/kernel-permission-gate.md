# Kernel Permission Gate（路由权限门禁）

## 职责

`middleware/permission_gate.go` + `permission/` 提供基于路由策略的 HTTP 权限强制：
认证之后、业务 handler 之前，按"请求路由 → 权限码 → 用户权限集合"判定放行 / 403。

## 匹配语义（两级，ticket 000277）

| 顺序 | 输入 | 用途 |
|------|------|------|
| 1 | `RequestMeta.Path`（gin `FullPath()`，含 `:param` 段） | 精确命中**参数化声明**（如 `/api/items/:id` → `item:read`） |
| 2 | `RequestMeta.ConcretePath`（`c.Request.URL.Path`，真实请求路径） | 顺序 1 miss 时兜底，命中**按具体值展开的声明**（如同一路由按 scope 注册多条具体策略） |

- 两级都是精确匹配（`METHOD:PATH` 组合键），不做通配或段级模式匹配。
- 顺序 1 命中后不再看顺序 2，参数化声明语义不变。
- 两级都 miss → `NotConfigured`，由 `DefaultPolicy` 决定放行或拒绝（当前应用配置为放行，即未声明路由 fail-open）。
- 命中策略后用户权限集合含 `*:*` 或对应权限码 → Allow；否则 Deny（403）。

## 声明规范

| 场景 | 声明形态 | 示例 |
|------|----------|------|
| 路由路径固定 | 具体路径 | `POST /api/system-resources/video-style-profiles` |
| 路由含路径参数且各参数值同权限 | gin 参数模板 | `PUT /api/items/:id` |
| 路由含路径参数且**不同参数值对应不同权限码** | 按参数值展开多条具体策略 | 分类路由按 scope 展开：`POST /api/system-resources/categories/bgm` → `category:bgm:write`、`POST /api/system-resources/categories/video_style_profile` → `category:video_style_profile:write` |

第三种形态依赖 `ConcretePath` 兜底匹配：门禁收到的是模板 `categories/:resource`，
精确 miss 后由真实请求路径命中具体声明。声明侧必须写具体值，不能写 `:resource`。

## 使用与配置

```go
engine.Use(PermissionGate(PermissionGateConfig{
    PolicyEngine:  policyEngine,        // 实现 permission.PolicyEngine，通常为 CachedPolicyEngine
    DefaultPolicy: permission.Allow,    // NotConfigured 时的默认决策
}))
```

- `CachedPolicyEngine` 通过 `PolicyLoader` 从持久层加载 `RoutePolicy{Method, PathPattern, PermissionCode}`
  （admin-api 存于 `sys_api_policies`，由各 domain `DeclaredPolicies` 启动 upsert）。
- `PermissionLoader` 加载用户权限码集合，内置 TTL 缓存；`InvalidateUser/InvalidateAllUsers` 主动失效。
- 用户身份优先取 context `user_id`（JWT 中间件写入），否则按 `TokenLookup` 配置解析 token。

## 边界

- 本门禁只做"路由→权限码→用户集合"判定；权限码语义、策略声明与 RBAC 管理在各应用/domain。
- 认证（JWT）独立于本门禁；未认证请求在 JWT 中间件已被拦截。
- 新增路由若不声明策略，则处于 fail-open（`DefaultPolicy=Allow`）；敏感路由必须显式声明。

## 验证

```bash
go test ./permission/... ./middleware/...
```
