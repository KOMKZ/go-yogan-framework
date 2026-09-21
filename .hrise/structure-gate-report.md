# Structure Gate Report

- mode: `changed`
- files scanned: `410`
- functions scanned: `3357`
- issues: `43`
- blocking: `0`

## Issues

| severity | kind | path | name | value | threshold | changed | reason |
|---|---|---|---|---:|---:|---|---|
| block | naming | `httpx/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `config/validator.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `kafka/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `telemetry/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `breaker/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `queue/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `application/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `auth/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `jwt/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `cache/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `auth/provider.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `swagger/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `config/builder.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `database/repository.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `redis/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `grpc/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `auth/service.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `health/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `swagger/provider.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `httpx/handler.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `logger/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `limiter/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `database/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `event/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `application/router.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| warn | func | `application/base_app.go` | `*BaseApplication.registerComponentMetrics` | 150 | 130 | true | function line count exceeds threshold |
| warn | func | `application/http_server.go` | `newServer` | 135 | 130 | false | function line count exceeds threshold |
| warn | func | `retry/retry.go` | `DoWithData` | 113 | 110 | false | function line count exceeds threshold |
| warn | errlint | `event/dispatcher.go` | `LINT-ERR-006` | 2 | 0 | false | 禁止裸 _ = call() 吞错。修复：可传播→errcode.Capture 后返回；无法返回的旁路→logger.GetLogger(域).Warn 或 gin c.Error 显式告警；解码容错→if err != nil 显式降级零值；装配期错误→fail-fast panic (lines 321,358) |
| warn | errlint | `logger/manager.go` | `LINT-ERR-006` | 2 | 0 | false | 禁止裸 _ = call() 吞错。修复：可传播→errcode.Capture 后返回；无法返回的旁路→logger.GetLogger(域).Warn 或 gin c.Error 显式告警；解码容错→if err != nil 显式降级零值；装配期错误→fail-fast panic (lines 233,266) |
| warn | errlint | `di/component_providers.go` | `LINT-ERR-006` | 2 | 0 | false | 禁止裸 _ = call() 吞错。修复：可传播→errcode.Capture 后返回；无法返回的旁路→logger.GetLogger(域).Warn 或 gin c.Error 显式告警；解码容错→if err != nil 显式降级零值；装配期错误→fail-fast panic (lines 76,457) |
| warn | errlint | `jwt/session_manager.go` | `LINT-ERR-006` | 1 | 0 | false | 禁止裸 _ = call() 吞错。修复：可传播→errcode.Capture 后返回；无法返回的旁路→logger.GetLogger(域).Warn 或 gin c.Error 显式告警；解码容错→if err != nil 显式降级零值；装配期错误→fail-fast panic (lines 88) |
| warn | errlint | `breaker/metrics_impl.go` | `LINT-ERR-008` | 1 | 0 | false | err.Error() 不得直接赋值/return 进 DB 列/record/DTO。修复：持久化诊断 errcode.Redact(err.Error())；用户可见文案 errcode.SafeMessage(err).PublicMessage (lines 95) |
| warn | errlint | `health/aggregator.go` | `LINT-ERR-008` | 1 | 0 | false | err.Error() 不得直接赋值/return 进 DB 列/record/DTO。修复：持久化诊断 errcode.Redact(err.Error())；用户可见文案 errcode.SafeMessage(err).PublicMessage (lines 101) |
| warn | errlint | `auth/service.go` | `LINT-ERR-004` | 1 | 0 | false | err.Error() 不得进响应/DTO/持久化。修复：用户面 errcode.SafeMessage(err).PublicMessage 或 PublicMessageOf；运维/日志诊断面 errcode.Redact(err.Error())；内部分类改 errors.As 类型化判别 (lines 64) |
| warn | errlint | `jwt/session_store_memory.go` | `LINT-ERR-006` | 1 | 0 | false | 禁止裸 _ = call() 吞错。修复：可传播→errcode.Capture 后返回；无法返回的旁路→logger.GetLogger(域).Warn 或 gin c.Error 显式告警；解码容错→if err != nil 显式降级零值；装配期错误→fail-fast panic (lines 184) |
| warn | errlint | `jwt/session_store_redis.go` | `LINT-ERR-006` | 1 | 0 | false | 禁止裸 _ = call() 吞错。修复：可传播→errcode.Capture 后返回；无法返回的旁路→logger.GetLogger(域).Warn 或 gin c.Error 显式告警；解码容错→if err != nil 显式降级零值；装配期错误→fail-fast panic (lines 103) |
| warn | errlint | `jwt/token_manager.go` | `LINT-ERR-008` | 1 | 0 | false | err.Error() 不得直接赋值/return 进 DB 列/record/DTO。修复：持久化诊断 errcode.Redact(err.Error())；用户可见文案 errcode.SafeMessage(err).PublicMessage (lines 599) |
| warn | errlint | `logger/encoder.go` | `LINT-ERR-004` | 1 | 0 | false | err.Error() 不得进响应/DTO/持久化。修复：用户面 errcode.SafeMessage(err).PublicMessage 或 PublicMessageOf；运维/日志诊断面 errcode.Redact(err.Error())；内部分类改 errors.As 类型化判别 (lines 694) |
| warn | errlint | `httpx/sse.go` | `LINT-ERR-004` | 1 | 0 | false | err.Error() 不得进响应/DTO/持久化。修复：用户面 errcode.SafeMessage(err).PublicMessage 或 PublicMessageOf；运维/日志诊断面 errcode.Redact(err.Error())；内部分类改 errors.As 类型化判别 (lines 63) |
| warn | errlint | `middleware/jwt.go` | `LINT-ERR-004` | 1 | 0 | false | err.Error() 不得进响应/DTO/持久化。修复：用户面 errcode.SafeMessage(err).PublicMessage 或 PublicMessageOf；运维/日志诊断面 errcode.Redact(err.Error())；内部分类改 errors.As 类型化判别 (lines 222) |
| warn | errlint | `queue/capacity_guard.go` | `LINT-ERR-004` | 1 | 0 | false | err.Error() 不得进响应/DTO/持久化。修复：用户面 errcode.SafeMessage(err).PublicMessage 或 PublicMessageOf；运维/日志诊断面 errcode.Redact(err.Error())；内部分类改 errors.As 类型化判别 (lines 53) |
| warn | errlint | `validator/converter.go` | `LINT-ERR-008` | 1 | 0 | false | err.Error() 不得直接赋值/return 进 DB 列/record/DTO。修复：持久化诊断 errcode.Redact(err.Error())；用户可见文案 errcode.SafeMessage(err).PublicMessage (lines 37) |

## Remediation Template

- split plan: describe the target responsibility split, such as service/repository/model/policy/adapter/generator template/fixture/test scenario.
- impact: list changed packages, public APIs, imports, generated outputs, and callers that need review.
- verification: list exact commands, including `make structure-gate` plus focused build/test commands.
- baseline rule: refresh baseline only inside a governance ticket with `STRUCTURE_GATE_BASELINE_TICKET=<ticket-id>`.
- goal: do not chase zero warnings; keep new code from worsening, protect critical paths, and maintain a stock issue map.
