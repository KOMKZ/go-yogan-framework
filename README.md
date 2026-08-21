# Yogan Framework

**[中文](README_zh.md)** | English

**No duplicate code, no infrastructure headaches.** Register components and they just work. Config auto-loads. Tracing out of the box. You write business logic, the framework handles the rest.

📖 **Documentation**: [go-yogan-doc-portal.pages.dev](https://go-yogan-doc-portal.pages.dev/)

> ⚠️ **Note**: This project is under active development. APIs may change.

## Installation

```bash
go get github.com/KOMKZ/go-yogan-framework
```

## Scaffolding Tool: go-ygctl

One command, project ready:

```bash
# Install
go install github.com/KOMKZ/go-ygctl@latest

# Create HTTP project
go-ygctl new http my-api

# Create gRPC / CLI / Cron project
go-ygctl new grpc my-service
go-ygctl new cli my-tool
go-ygctl new cron my-scheduler
```

Generated projects are complete and runnable: config files, routes, health checks, Docker Compose included.

**List available components**:

```bash
go-ygctl component list
```

**Get component integration guide**:

```bash
go-ygctl component add database
go-ygctl component add redis
go-ygctl component add kafka
```

No need to dig through docs—the CLI tells you how to integrate.

## Core Components

| Component | Description |
|-----------|-------------|
| application | Application lifecycle management (HTTP/gRPC/CLI/Cron) |
| component | Component interface definitions |
| config | Configuration loading (YAML + environment variables) |
| logger | Structured logging (Zap) |
| database | GORM database connection pool |
| redis | Redis client management |
| grpc | gRPC server/client |
| kafka | Kafka producer/consumer |
| auth | Authentication service (password/OAuth) |
| jwt | JWT token management |
| middleware | HTTP middleware (CORS/TraceID/logging) |
| telemetry | OpenTelemetry distributed tracing |
| health | Health checks |
| limiter | Rate limiting (token bucket/sliding window) |
| breaker | Circuit breaker |
| retry | Retry strategies |

## Quick Start

Create `./config/config.yaml` with your `api_server` settings, then:

```go
package main

import (
    "github.com/KOMKZ/go-yogan-framework/application"
    "github.com/KOMKZ/go-yogan-framework/logger"
    "github.com/gin-gonic/gin"
    "go.uber.org/zap"
)

// routerRegistrar implements application.RouterRegistrar to register business routes
type routerRegistrar struct{}

func (r routerRegistrar) RegisterRoutes(engine *gin.Engine, app *application.Application) {
    engine.GET("/api/hello", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "hello"})
    })
}

func main() {
    // Parse per-app flags and environment variables (USER_API_PORT, USER_API_ENV, ...)
    flags := application.ParseFlags("user-api", "./config")

    app := application.NewWithFlags("./config", "USER_API", flags).
        WithVersion("0.1.0").
        RegisterRoutes(routerRegistrar{})

    if err := app.Run(); err != nil {
        logger.Fatal("main", "Application start failed", zap.Error(err))
    }
}
```

All components are wired through samber/do providers — see `docs/` for the full API.

## License

[MIT License](LICENSE)
