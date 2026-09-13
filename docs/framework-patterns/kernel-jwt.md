# JWT Middleware

## Server-Side Sessions

JWT can run in server-side session mode. In this mode JWT tokens carry short ids
(`sid`, `ati`, `rti`) and the framework stores session state in Redis or memory.
This replaces blacklist-style revocation.

```yaml
jwt:
  enabled: true
  algorithm: "HS256"
  secret: "admin-jwt-secret"
  access_token:
    ttl: "2h"
    issuer: "hrise-admin-api"
  refresh_token:
    enabled: true
    ttl: "168h"
  session:
    enabled: true
    store: "memory" # redis | memory
    redis_client: "main"
    key_prefix: "hrise:admin:jwt:"
    max_sessions_per_subject: 3
    last_seen_update_interval: "1m"
    refresh_reuse_policy: "revoke_session"
    cleanup_interval: "1h"
```

Redis connections are owned by the application Redis manager. The JWT package
only consumes the named client from `jwt.session.redis_client`.

Use `jwt.SessionTokenManager` for login, refresh, and forced logout:

```go
pair, err := tokenManager.(jwt.SessionTokenManager).IssueTokenPair(ctx, jwt.IssueTokenInput{
	Subject: "123",
	Claims: map[string]interface{}{"user_id": int64(123), "roles": []string{"admin"}},
	Client: jwt.ClientInfo{IP: ip, UserAgent: ua},
})
```

`RefreshTokenPair` rotates refresh tokens. If an old refresh token is reused and
`refresh_reuse_policy` is `revoke_session`, the whole session is revoked.

Use `RevokeSession` for single-device logout and `RevokeSubjectSessions` for
all-device logout after account disable, password reset, role change, or
permission version change.

## Token Type Boundary

Protected business routes should use `middleware.JWT(tokenManager)`. The default middleware accepts only JWT claims with `token_type=access`.

Refresh tokens are intentionally rejected by default, even if their signature and expiration are valid. Refresh tokens should be sent only to application refresh endpoints, usually in the request body, where the handler can explicitly verify `token_type=refresh`.

## Custom Token Types

Use `JWTWithConfig` only when a route intentionally accepts a non-default token type:

```go
middleware.JWTWithConfig(tokenManager, middleware.JWTConfig{
	AllowedTokenTypes: []string{"refresh"},
})
```

Application business APIs should not use this override.

## Permission Gate

`PermissionGate` may resolve user identity from `Authorization` when a route has not injected `user_id` yet. That fallback also accepts only access tokens, so refresh tokens cannot be used for permission decisions.
