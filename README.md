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

```go
package main

import (
    "github.com/KOMKZ/go-yogan-framework/application"
    "github.com/KOMKZ/go-yogan-framework/database"
    "github.com/KOMKZ/go-yogan-framework/redis"
)

func main() {
    app := application.New("./configs", "MY_APP", nil)
    
    app.Register(
        database.NewComponent(),
        redis.NewComponent(),
    )
    
    app.Run()
}
```

## License

[MIT License](LICENSE)
