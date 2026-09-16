# Error Boundary（治理 ticket 000105）

## 目的

在 service 入口用 `defer errcode.CaptureInto(&err, "<scope>")` 给返回值补 origin stack + operation，保证错误经过统一收口（`httpx.HandleError` / `asynq ErrorHandler` / `clierrors.Handle`）时一定带可定位的现场信息。

## 框架能力

| 能力 | 位置 |
|---|---|
| `errcode.CaptureInto(&err, op)` | `framework/errcode/boundary.go` |
| `errcode.Capture(err, op)` | `framework/errcode/error.go` |
| `LayeredError.Wrap / Wrapf` | `framework/errcode/error.go` |
| `errcode.SafeMessage(err)` | `framework/errcode/safe_msg.go` |

## 模式

### 模式 A：service 公共方法入口

```go
func (s *UserService) BindPhone(ctx context.Context, input BindPhoneInput) (item *UserItem, err error) {
    defer errcode.CaptureInto(&err, "users.service.bind_phone")

    phone := normalizePhone(input.Phone)
    if phone == nil {
        return nil, domainerrors.ErrInvalidPhone
    }
    user, e := s.repo.FindByID(ctx, input.ID)
    if e != nil {
        return nil, errcode.Capture(e, "users.repo.find_by_id")
    }
    ...
}
```

**约束**：
- 第一行 `defer errcode.CaptureInto(&err, "...")` 必须紧跟方法签名。
- `operation` 命名格式：`<domain>.<service>.<method>`（如 `users.service.bind_phone`）。
- repo 错误用 `errcode.Capture` 锚定 I/O 边界操作。
- 业务码错误直接 return `domainerrors.ErrXxx`。

### 模式 B：worker / cron tick（非请求路径）

```go
func (w *Worker) Process(ctx context.Context, job Job) (err error) {
    defer errcode.Boundary(&err, "worker.process")

    if err := w.process(ctx, job); err != nil {
        return errcode.Capture(err, "worker.process.inner")
    }
    return nil
}
```

### 模式 C：客户端字段错误文案

```go
func buildDTO(err error) (dto DTO, logFields map[string]any) {
    publicMsg, details := errcode.SafeMessage(err)
    dto.Msg = publicMsg          // 客户端只看到安全文案
    logFields = details         // 服务端日志保留全部排障信息
    return
}
```

## CaptureInto 语义

```
CaptureInto(&err, "scope.op"):

  *errp == nil                                     → noop
  *errp is *LayeredError with OriginStack() != ""  → 保留 I/O 边界栈 + 补 operation（如缺）
  *errp is *LayeredError with OriginStack() == ""  → 补 operation + 重算 stack
  *errp is 普通 error                              → Capture(err, op)
```

**关键**：保留"最内层 origin"而不是"service 入口 origin"——排查时第一现场永远是 I/O 边界（DB / OSS / RPC / SDK），service 只是包装。service 入口的 `operation` 字段会稳定定位业务层。

### operation 归属规则

`LayeredError.Wrap(cause)` 内部 `cloneOrigin` 只在 wrapper 自己 `operation` 为空时采用 cause 的 operation——wrapper 的业务 op 永远不被覆盖。例如：

```go
// 业务层用 media-jobs/errors.Wrap(ErrCallFailed, "llm generate", providerErr)：
//   wrapped.cause        = captured(providerErr)，ProviderError 可 errors.As 拿到
//   wrapped.operation    = "llm generate"（业务 op，不会被 captured 的 "llm generate.cause" 覆盖）
//   wrapped.originStack  = captured.OriginStack()（I/O 边界栈）
//   wrapped.Error()      = "llm generate: provider=openai code=429: rate limit exceeded"
```

如果业务 op 需要进一步细分，建议在 service 入口用 `defer errcode.CaptureInto(&err, "<domain>.<sub>.<method>")` 单独设一个更精确的 op，而不是依赖 `Wrap` 时被覆盖。

## SafeMessage 语义

| 输入 | 客户端返回 | 日志保留 |
|---|---|---|
| `*LayeredError` | `Message()`（注册时审核过） | code / module / msg_key / http_status / operation / origin_stack / cause_type+message / root_type+message |
| 普通 error | 固定"内部错误，请稍后再试" | cause_type + cause_message |

**禁止**：`SafeMessage(err).Message()` 直接拿到的是注册文案，已无 `err.Error()` 字符串泄漏。

## 关联

- [kernel-httpx-error.md](kernel-httpx-error.md) — 11 字段 schema
- [kernel-queue.md](kernel-queue.md) — asynq 错误处理
- 治理 ticket 000105
