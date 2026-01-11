# Auth 认证组件

## 概述

Auth 组件提供统一的认证服务，支持多种认证方式（密码、OAuth2、API Key 等）。**认证成功后配合 JWT 组件颁发 token**。

## 核心功能

- ✅ **密码认证**：bcrypt 加密 + 密码策略验证
- ✅ **登录尝试限制**：防暴力破解（Redis/Memory 存储）
- ✅ **可扩展架构**：AuthProvider 接口支持多种认证方式
- 🚧 **OAuth2.0**：第三方登录（未来扩展）
- 🚧 **API Key**：服务间认证（未来扩展）

## 使用示例

### 1. 配置文件

```yaml
auth:
  enabled: true
  providers:
    - password  # 启用密码认证
  
  # 密码认证配置
  password:
    enabled: true
    bcrypt_cost: 12
    policy:
      min_length: 8
      max_length: 128
      require_uppercase: true
      require_lowercase: true
      require_digit: true
      require_special_char: false
      password_history: 5
      password_expiry_days: 90
      blacklist:
        - password
        - 123456
        - admin123
  
  # 登录尝试限制
  login_attempt:
    enabled: true
    max_attempts: 5
    lockout_duration: 30m
    storage: redis  # redis 或 memory
    redis_key_prefix: "auth:login_attempt:"
```

### 2. 注册组件

```go
// apps/user-api/internal/app/components.go
func (u *UserAPI) RegisterComponents(app *application.Application) {
    // 注册基础组件
    app.Register(database.NewComponent())
    app.Register(redis.NewComponent())
    app.Register(jwt.NewComponent())
    app.Register(auth.NewComponent())  // 注册认证组件
}
```

### 3. 注册认证提供者

```go
// apps/user-api/internal/app/app.go
func (u *UserAPI) onSetup(core *application.Application) error {
    // 获取组件
    authComp := core.MustGet(component.ComponentAuth).(*auth.Component)
    passwordService := authComp.GetPasswordService()
    attemptStore := authComp.GetAttemptStore()
    
    // 创建用户仓库
    db := core.GetDBManager().DB("master")
    userRepo := user.NewGORMRepository(db)
    
    // 创建并注册密码认证提供者
    passwordProvider := auth.NewPasswordAuthProvider(
        passwordService,
        userRepo,  // 业务层实现 UserRepository 接口
        attemptStore,
        5,    // maxAttempts
        30*time.Minute,  // lockoutDuration
    )
    authComp.RegisterProvider(passwordProvider)
    
    return nil
}
```

### 4. 实现 UserRepository 接口

```go
// domains/user/repository.go
type Repository interface {
    FindByUsername(ctx context.Context, username string) (*model.User, error)
    FindByEmail(ctx context.Context, email string) (*model.User, error)
    // ... 其他方法
}

// domains/user/repository_impl.go
func (r *GORMRepository) FindByUsername(ctx context.Context, username string) (*model.User, error) {
    var user model.User
    err := r.db.WithContext(ctx).
        Where("username = ? AND deleted_at IS NULL", username).
        First(&user).Error
    if err != nil {
        return nil, err
    }
    return &user, nil
}
```

### 5. 登录接口实现

```go
// apps/user-api/internal/handler/auth_handler.go
package handler

import (
    "github.com/KOMKZ/go-yogan-framework/auth"
    "github.com/KOMKZ/go-yogan-framework/component"
    "github.com/KOMKZ/go-yogan-framework/httpx"
    "github.com/KOMKZ/go-yogan-framework/jwt"
    "github.com/gin-gonic/gin"
    "go.uber.org/zap"
)

type AuthHandler struct {
    authService *auth.AuthService
    jwtManager  jwt.TokenManager
    logger      *logger.CtxZapLogger
}

type LoginRequest struct {
    Username string `json:"username" binding:"required"`
    Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    ExpiresIn    int64  `json:"expires_in"`
    TokenType    string `json:"token_type"`
}

// Login 用户登录
func (h *AuthHandler) Login(c *gin.Context) {
    var req LoginRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        httpx.ErrorJson(c, err)
        return
    }
    
    ctx := c.Request.Context()
    
    // 1. 执行认证（使用 auth 组件）
    authResult, err := h.authService.Authenticate(ctx, "password", auth.Credentials{
        Username: req.Username,
        Password: req.Password,
    })
    if err != nil {
        h.logger.WarnCtx(ctx, "login failed",
            zap.String("username", req.Username),
            zap.Error(err))
        httpx.ErrorJson(c, err)
        return
    }
    
    // 2. 生成 JWT Token（使用 jwt 组件）
    accessToken, err := h.jwtManager.Generate(ctx, authResult.UserID, map[string]interface{}{
        "user_id":  authResult.UserID,
        "username": authResult.Username,
        "email":    authResult.Email,
        "roles":    authResult.Roles,
    })
    if err != nil {
        h.logger.ErrorCtx(ctx, "generate token failed", zap.Error(err))
        httpx.InternalErrorJson(c)
        return
    }
    
    // 3. 生成 Refresh Token
    refreshToken, err := h.jwtManager.GenerateRefreshToken(ctx, authResult.UserID)
    if err != nil {
        h.logger.ErrorCtx(ctx, "generate refresh token failed", zap.Error(err))
        httpx.InternalErrorJson(c)
        return
    }
    
    // 4. 记录登录日志
    h.logger.InfoCtx(ctx, "user logged in",
        zap.Int64("user_id", authResult.UserID),
        zap.String("username", authResult.Username),
        zap.String("ip", c.ClientIP()))
    
    // 5. 返回响应
    httpx.SuccessJson(c, LoginResponse{
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
        ExpiresIn:    3600, // 从 JWT 配置读取
        TokenType:    "Bearer",
    })
}

// Register 用户注册
func (h *AuthHandler) Register(c *gin.Context) {
    var req RegisterRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        httpx.ErrorJson(c, err)
        return
    }
    
    ctx := c.Request.Context()
    
    // 1. 验证密码策略（使用 auth 组件）
    if err := h.passwordService.ValidatePassword(req.Password); err != nil {
        httpx.ErrorJson(c, err)
        return
    }
    
    // 2. 加密密码
    passwordHash, err := h.passwordService.HashPassword(req.Password)
    if err != nil {
        h.logger.ErrorCtx(ctx, "hash password failed", zap.Error(err))
        httpx.InternalErrorJson(c)
        return
    }
    
    // 3. 创建用户（调用 user service）
    user, err := h.userService.Create(ctx, &user.CreateUserInput{
        Username:     req.Username,
        Email:        req.Email,
        PasswordHash: passwordHash,
        Roles:        []string{"user"},
    })
    if err != nil {
        httpx.ErrorJson(c, err)
        return
    }
    
    httpx.SuccessJson(c, user)
}
```

### 6. 路由注册

```go
// apps/user-api/internal/router/auth_router.go
func RegisterAuthRoutes(r *gin.RouterGroup, authHandler *handler.AuthHandler) {
    auth := r.Group("/auth")
    {
        auth.POST("/login", authHandler.Login)      // 登录
        auth.POST("/register", authHandler.Register) // 注册
        auth.POST("/logout", authHandler.Logout)    // 登出
        auth.POST("/refresh", authHandler.Refresh)  // 刷新 token
    }
}
```

## 架构设计

### 认证流程

```
Client                    Login API              Auth Component        JWT Component
  │                           │                        │                     │
  │  POST /auth/login         │                        │                     │
  │  {username, password}     │                        │                     │
  ├──────────────────────────>│                        │                     │
  │                           │                        │                     │
  │                           │  Authenticate()        │                     │
  │                           ├───────────────────────>│                     │
  │                           │                        │                     │
  │                           │  ├─ Check Attempts     │                     │
  │                           │  ├─ Query User         │                     │
  │                           │  ├─ Verify Password    │                     │
  │                           │  └─ Return AuthResult  │                     │
  │                           │<───────────────────────┤                     │
  │                           │                        │                     │
  │                           │  Generate Token        │                     │
  │                           ├───────────────────────────────────────────>│
  │                           │<───────────────────────────────────────────┤
  │                           │  {access_token, refresh_token}              │
  │                           │                        │                     │
  │  200 OK                   │                        │                     │
  │  {access_token, ...}      │                        │                     │
  │<──────────────────────────┤                        │                     │
```

### 组件职责

| 组件 | 职责 |
|------|------|
| **Auth Component** | 认证逻辑（密码验证、登录限制） |
| **JWT Component** | Token 生成/验证/刷新/撤销 |
| **Login Handler** | 协调认证和 Token 颁发 |
| **User Repository** | 用户数据访问（业务层实现） |

## 测试

### 单元测试

```go
// auth/password_test.go
func TestPasswordService_ValidatePassword(t *testing.T) {
    policy := PasswordPolicy{
        MinLength:          8,
        MaxLength:          128,
        RequireUppercase:   true,
        RequireLowercase:   true,
        RequireDigit:       true,
        RequireSpecialChar: false,
        Blacklist:          []string{"password", "123456"},
    }
    
    service := NewPasswordService(policy, 12)
    
    tests := []struct {
        name     string
        password string
        wantErr  error
    }{
        {"valid password", "Abc123def", nil},
        {"too short", "Abc1", ErrPasswordTooShort},
        {"no uppercase", "abc123def", ErrPasswordRequireUppercase},
        {"in blacklist", "Password123", ErrPasswordInBlacklist},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := service.ValidatePassword(tt.password)
            if err != tt.wantErr {
                t.Errorf("ValidatePassword() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### 集成测试

```go
// apps/user-api/test/integration/auth_test.go
func TestLogin(t *testing.T) {
    // 1. 创建测试用户
    passwordHash, _ := bcrypt.GenerateFromPassword([]byte("Test123456"), 12)
    user := &model.User{
        Username:     "testuser",
        Email:        "test@example.com",
        PasswordHash: string(passwordHash),
        Status:       "active",
        Roles:        []string{"user"},
    }
    db.Create(user)
    
    // 2. 登录请求
    resp := testutil.Request(t, app, testutil.RequestOptions{
        Method: "POST",
        Path:   "/api/v1/auth/login",
        Body: map[string]interface{}{
            "username": "testuser",
            "password": "Test123456",
        },
    })
    
    // 3. 验证响应
    assert.Equal(t, 200, resp.Code)
    assert.NotEmpty(t, resp.Body["access_token"])
    assert.NotEmpty(t, resp.Body["refresh_token"])
}
```

## 错误码

| 错误 | 说明 |
|------|------|
| `ErrInvalidCredentials` | 用户名或密码错误 |
| `ErrUserNotFound` | 用户不存在 |
| `ErrAccountDisabled` | 账户已禁用 |
| `ErrTooManyAttempts` | 登录尝试次数过多 |
| `ErrPasswordTooWeak` | 密码过于简单 |

## 最佳实践

1. ✅ **密码加密**：始终使用 bcrypt（cost >= 12）
2. ✅ **登录限制**：启用登录尝试限制防暴力破解
3. ✅ **密码策略**：强制复杂密码（大小写+数字+特殊字符）
4. ✅ **错误消息**：登录失败统一返回"用户名或密码错误"（防用户名枚举）
5. ✅ **审计日志**：记录所有登录尝试（成功/失败）
6. ✅ **Token 管理**：配合 JWT 组件实现 Token 生命周期管理

## 与其他组件的关系

- **JWT Component**：认证成功后颁发 Token
- **Redis Component**：存储登录尝试次数
- **Logger Component**：记录认证日志
- **Database Component**：查询用户信息

## 未来扩展

- OAuth2.0 认证提供者
- API Key 认证提供者
- LDAP/AD 认证提供者
- 短信验证码认证
- 邮箱验证码认证
- 多因素认证（MFA）

---

**文档版本**: v1.0  
**最后更新**: 2026-01-06

