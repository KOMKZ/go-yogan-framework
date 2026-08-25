# Kernel Limiter

## 目标

`limiter` 是框架级限速组件，HTTP 应用启动时会在全局 middleware 中自动接入。应用只需要配置 `limiter.enabled=true`，并声明全局策略或规则策略。

## 配置文件

框架默认加载以下限速配置文件：

| 文件 | 优先级 | 用途 |
|------|--------|------|
| `rate_limiter.yaml` | 15 | 限速基础配置 |

优先级关系：`config.yaml` < `rate_limiter.yaml` < `{env}.yaml` < 环境变量 < flags。

## 全局限速

全局限速由 `limiter.key_func` 和 `limiter.default` 控制。面向用户 API 建议使用 `path_ip`，按接口路径和客户端 IP 分桶。

```yaml
limiter:
  enabled: true
  store_type: "redis"
  redis:
    instance: "limiter"
    key_prefix: "hrise:user-api:limiter:"
  key_func: "path_ip"
  skip_paths: ["/health"]
  default:
    algorithm: "token_bucket"
    rate: 100
    capacity: 200
    init_tokens: 200

redis:
  instances:
    limiter:
      mode: "standalone"
      addrs: ["127.0.0.1:6379"]
      db: 7
```

## 规则限速

规则限速由 `limiter.rules` 控制，适合只保护某些接口，或为重要接口追加更细维度。

支持的规则 key：

| key_func | 维度 | 说明 |
|----------|------|------|
| `path_ip` | method + path + ip | 登录、注册、短信发送等匿名接口 |
| `user_path` | user_id + method + path | 登录态高成本接口 |

`user_path` 默认先读 Gin context 的 `user_id`。当 `identity_source=jwt_context_or_token` 且应用注册了 `jwt.TokenManager` 时，限速 middleware 会在路由 JWT middleware 之前从 Bearer token 解析 `user_id`。

```yaml
limiter:
  enabled: true
  store_type: "redis"
  rules:
    admin_login:
      key_func: "path_ip"
      match:
        - method: "POST"
          path: "/api/auth/login"
      limit:
        algorithm: "token_bucket"
        rate: 1
        capacity: 5
        init_tokens: 5
    user_profile:
      key_func: "user_path"
      identity_source: "jwt_context_or_token"
      user_id_key: "user_id"
      match:
        - method: "GET"
          path: "/api/user/profile"
      limit:
        algorithm: "token_bucket"
        rate: 10
        capacity: 20
        init_tokens: 20
```

## 治理原则

- `admin-api` 默认只保护登录接口，后台业务接口优先通过 IP 白名单、权限和审计治理。
- `user-api` 面向公网用户，优先开启 Redis + 全局 `path_ip` 默认限速。
- 高成本、强用户属性的登录态接口，再追加 `user_path` 规则。
- 生产环境不要使用 memory store，多实例部署必须使用 Redis store。
- 新增限速规则要显式写出 method、path、key_func、limit，并说明为什么需要这个维度。
