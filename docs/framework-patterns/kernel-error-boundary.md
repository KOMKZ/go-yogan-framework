# Error Boundary（治理 ticket 000105；000128 修订 public/private 边界）

## 目的

在 service 入口用 `defer errcode.CaptureInto(&err, "<scope>")` 给返回值补 origin stack + operation，保证错误经过统一收口（`httpx.HandleError` / `asynq ErrorHandler` / hrise-cli `main.go` 的 `fatalCLI`，三者均走 `errcode.ErrorLogFields`）时一定带可定位的现场信息。

## 框架能力

| 能力 | 位置 |
|---|---|
| `errcode.CaptureInto(&err, op)` | `framework/errcode/boundary.go` |
| `errcode.Capture(err, op)` | `framework/errcode/error.go` |
| `LayeredError.Wrap / Wrapf` | `framework/errcode/error.go` |
| `errcode.SafeMessage(err)` → `SafeMessageResult` | `framework/errcode/safe_msg.go` |
| `errcode.Redact(text)` 统一脱敏 | `framework/errcode/redact.go` |
| `LayeredError.WithPublicData / PublicData` | `framework/errcode/error.go` |
| `errcode.HasRegisteredBusinessCode(err)` | `framework/errcode/error.go` |

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
func buildDTO(err error) (dto DTO, log loggerFields) {
    safe := errcode.SafeMessage(err)   // typed result（000128 P0-3）
    dto.Msg = safe.PublicMessage       // 客户端只看到安全文案
    dto.Data = safe.PublicData         // 只有显式 WithPublicData 标注的数据
    log = safe.Diagnostics             // 服务端日志保留脱敏后的排障信息
    return
}
```

**禁止** `_ = details` 式静默丢弃：typed result 让诊断丢弃必须显式写出来，review 时可直接拒绝。

## CaptureInto 语义

```
CaptureInto(&err, "scope.op"):

  *errp == nil                                     → noop
  *errp is *LayeredError with OriginStack() != ""  → 保留 I/O 边界栈 + 补 operation（如缺）
  *errp is *LayeredError with OriginStack() == ""  → 补 operation + 重算 stack
  *errp is 普通 error                              → Capture(err, op)
```

`Capture` 为普通错误建立 `LayeredError` 时会同时保留 message 与 cause。为避免同一技术错误
在 `Error()` 中输出两遍，当 `Message() == Cause().Error()` 时只渲染一次；业务码包装的注册文案
与 cause 不同，仍保持 `"业务文案: 技术原因"`。该规则只改变字符串渲染，不改变
`Unwrap`、`errors.Is/As`、origin stack 或 operation。

**关键**：保留"最内层 origin"而不是"service 入口 origin"——排查时第一现场永远是 I/O 边界（DB / OSS / RPC / SDK），service 只是包装。service 入口的 `operation` 字段会稳定定位业务层。

### operation 归属规则

`LayeredError.Wrap(cause)` 内部 `cloneOrigin` 只在 wrapper 自己 `operation` 为空时采用 cause 的 operation——wrapper 的业务 op 永远不被覆盖。例如：

```go
// 业务层用 media-jobs/errors.Wrap(ErrCallFailed, "llm generate", providerErr)：
//   wrapped.cause        = captured(providerErr)，ProviderError 可 errors.As 拿到
//   wrapped.operation    = "llm generate"（业务 op，不会被 captured 的 "llm generate.cause" 覆盖）
//   wrapped.originStack  = captured.OriginStack()（I/O 边界栈）
//   wrapped.Message()    = "media job provider call failed"（注册文案，op 不再覆盖——000128 P0-2）
//   wrapped.Error()      = "media job provider call failed: provider=openai code=429: rate limit exceeded"（仅日志）
```

如果业务 op 需要进一步细分，建议在 service 入口用 `defer errcode.CaptureInto(&err, "<domain>.<sub>.<method>")` 单独设一个更精确的 op，而不是依赖 `Wrap` 时被覆盖。

## SafeMessage 语义（000128 P0-3 修订）

`SafeMessage(err)` 返回 `SafeMessageResult{PublicMessage, PublicData, Diagnostics}`：

| 输入 | PublicMessage | PublicData | Diagnostics |
|---|---|---|---|
| 注册业务码 `*LayeredError`（Code()>0 且 Module()!=""） | `Message()`（注册时审核过） | 仅 `WithPublicData` 显式标注的数据 | code / module / msg_key / http_status / operation / origin_stack / cause+root（**已 Redact 脱敏**） |
| 裸 Capture / 未注册 LayeredError（code=0） | 固定"内部错误，请稍后再试" | nil | 同上（动态 message 只留在诊断） |
| 普通 error | 固定"内部错误，请稍后再试" | nil | cause_type + cause_message（已脱敏） |

**关键规则**：
- 裸 `Capture` 的动态 message 含 cause 原文，永远不能作为公开文案（000128 P0-3 守卫）。
- `Data()` 是私有诊断数据，只能进日志；响应/DTO 只允许 `PublicData()`（000128 拆分 public/private 边界）。
- cause/root/chain 写日志前必须经过 `errcode.Redact`：覆盖 DSN/URL 凭据、Bearer token、token/api_key/auth_key/OSSAccessKeyId/signature/password 等敏感键值、sk- 密钥、邮箱、手机号。客户端安全不等于日志安全。

**禁止**：`SafeMessage(err).PublicMessage` 直接拿到的是注册文案，已无 `err.Error()` 字符串泄漏。

## 关联

- [kernel-httpx-error.md](kernel-httpx-error.md) — 11 字段 schema 与 HTTP 出口 public/private data 契约
- [kernel-queue.md](kernel-queue.md) — asynq 错误处理
- 治理 ticket 000105、000128
