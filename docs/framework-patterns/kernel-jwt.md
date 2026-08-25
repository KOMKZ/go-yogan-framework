# JWT Middleware

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
