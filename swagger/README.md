# Swagger 组件

基于 [swaggo/swag](https://github.com/swaggo/swag) 实现的 Swagger/OpenAPI 文档支持。

## 功能特性

- 🔧 **注释驱动**：在 Handler 上添加注释即可生成文档
- 🚀 **自动集成**：启用后 HTTP Application 自动挂载 Swagger UI 和 Spec 路由
- ⚙️ **配置化**：通过 YAML 配置启用/禁用和自定义路径
- 🔌 **DI 集成**：通过 `samber/do` 自动注入

## 快速开始

### 1. 配置启用

在应用配置文件中添加：

```yaml
swagger:
  enabled: true
  ui_path: "/swagger/*any"      # Swagger UI 路径
  spec_path: "/openapi.json"    # OpenAPI Spec 路径
  info:
    title: "My API"
    description: "API 接口文档"
    version: "1.0.0"
    base_path: "/api"
```

### 2. 添加注释

在 Handler 上添加 Swagger 注释：

```go
// GetUser 获取用户信息
//
//	@Summary		获取用户信息
//	@Description	根据 ID 获取用户详细信息
//	@Tags			用户
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int					true	"用户 ID"
//	@Success		200	{object}	httpx.Response{data=User}	"成功"
//	@Failure		404	{object}	httpx.Response			"用户不存在"
//	@Router			/users/{id} [get]
func (h *Handler) GetUser(c *gin.Context) {
    // ...
}
```

### 3. main.go 添加注释

在 `main.go` 添加全局 API 信息：

```go
package main

import (
    _ "your-app/docs" // 导入 swag 生成的 docs 包
)

// @title Your API
// @version 1.0.0
// @description API 接口文档

// @host localhost:8080
// @BasePath /api

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT Bearer Token

func main() {
    // ...
}
```

### 4. 生成文档

```bash
# 安装 swag CLI
go install github.com/swaggo/swag/cmd/swag@latest

# 生成文档
swag init --parseDependency --parseInternal -g main.go -o docs
```

### 5. 启动应用

启动应用后访问：

- **Swagger UI**: `http://localhost:8080/swagger/index.html`
- **OpenAPI Spec**: `http://localhost:8080/openapi.json`

## 配置项

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `enabled` | bool | `false` | 是否启用 Swagger |
| `ui_path` | string | `/swagger/*any` | Swagger UI 路由路径 |
| `spec_path` | string | `/openapi.json` | OpenAPI Spec 路由路径 |
| `deep_linking` | bool | `true` | 是否启用深度链接 |
| `persist_authorization` | bool | `true` | 是否持久化认证信息 |
| `info.title` | string | `API Documentation` | API 标题 |
| `info.description` | string | - | API 描述 |
| `info.version` | string | `1.0.0` | API 版本 |
| `info.base_path` | string | `/api` | API 基础路径 |

## API

### Manager

```go
// 创建 Manager
mgr := swagger.NewManager(cfg, info, logger)

// 注册路由
mgr.RegisterRoutes(engine)

// 检查是否启用
if mgr.IsEnabled() {
    // ...
}
```

### Helper 函数

```go
// 快捷设置（从 DI 获取 Manager 并注册路由）
swagger.Setup(injector, engine)

// 带 Info 设置（用于动态配置）
swagger.SetupWithInfo(injector, engine)

// Must 版本（失败时 panic）
swagger.MustSetup(injector, engine)
```

## 注意事项

1. **导入 docs 包**：main.go 必须导入 swag 生成的 docs 包
2. **注释格式**：遵循 swag 标准注释格式
3. **重新生成**：修改注释后需重新运行 `swag init`
4. **生产环境**：建议在生产环境禁用 Swagger

## 常见问题

### Q: 启动时提示 "swag.SwaggerInfo not initialized"

A: 确保 main.go 中导入了 `_ "your-app/docs"` 包。

### Q: 文档未更新

A: 重新运行 `swag init` 命令生成文档。

### Q: 如何支持多语言

A: 目前 swaggo 原生不支持多语言，可以通过配置不同的 docs 包实现。
