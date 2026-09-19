# Structure Gate Report

- mode: `changed`
- files scanned: `407`
- functions scanned: `3329`
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
| block | naming | `auth/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `application/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
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
| warn | func | `application/base_app.go` | `*BaseApplication.registerComponentMetrics` | 150 | 130 | false | function line count exceeds threshold |
| warn | func | `application/http_server.go` | `newServer` | 135 | 130 | false | function line count exceeds threshold |
| warn | func | `retry/retry.go` | `DoWithData` | 113 | 110 | false | function line count exceeds threshold |
| warn | errlint | `event/dispatcher.go` | `LINT-ERR-006` | 2 | 0 | false | 禁止裸 `_ = func(...)` 吞错；除 Close/Unlock/Rollback/Shutdown 外必须显式处理 (lines 321,358) |
| warn | errlint | `logger/manager.go` | `LINT-ERR-006` | 2 | 0 | false | 禁止裸 `_ = func(...)` 吞错；除 Close/Unlock/Rollback/Shutdown 外必须显式处理 (lines 233,266) |
| warn | errlint | `di/component_providers.go` | `LINT-ERR-006` | 2 | 0 | false | 禁止裸 `_ = func(...)` 吞错；除 Close/Unlock/Rollback/Shutdown 外必须显式处理 (lines 76,457) |
| warn | errlint | `jwt/session_manager.go` | `LINT-ERR-006` | 1 | 0 | false | 禁止裸 `_ = func(...)` 吞错；除 Close/Unlock/Rollback/Shutdown 外必须显式处理 (lines 88) |
| warn | errlint | `breaker/metrics_impl.go` | `LINT-ERR-008` | 1 | 0 | false | 禁止把 err.Error() 直接赋值 / return 进 DB 列 / record 字段 / DTO：必须 errcode.SafeMessage(err) 或 LayeredError.Message() (lines 95) |
| warn | errlint | `health/aggregator.go` | `LINT-ERR-008` | 1 | 0 | false | 禁止把 err.Error() 直接赋值 / return 进 DB 列 / record 字段 / DTO：必须 errcode.SafeMessage(err) 或 LayeredError.Message() (lines 101) |
| warn | errlint | `auth/service.go` | `LINT-ERR-004` | 1 | 0 | false | 禁止 err.Error() 直接进响应 / DTO / 非日志函数参数；必须 errcode.SafeMessage(err) 或 LayeredError.Message() (lines 64) |
| warn | errlint | `jwt/session_store_memory.go` | `LINT-ERR-006` | 1 | 0 | false | 禁止裸 `_ = func(...)` 吞错；除 Close/Unlock/Rollback/Shutdown 外必须显式处理 (lines 184) |
| warn | errlint | `jwt/session_store_redis.go` | `LINT-ERR-006` | 1 | 0 | false | 禁止裸 `_ = func(...)` 吞错；除 Close/Unlock/Rollback/Shutdown 外必须显式处理 (lines 103) |
| warn | errlint | `jwt/token_manager.go` | `LINT-ERR-008` | 1 | 0 | false | 禁止把 err.Error() 直接赋值 / return 进 DB 列 / record 字段 / DTO：必须 errcode.SafeMessage(err) 或 LayeredError.Message() (lines 599) |
| warn | errlint | `logger/encoder.go` | `LINT-ERR-004` | 1 | 0 | false | 禁止 err.Error() 直接进响应 / DTO / 非日志函数参数；必须 errcode.SafeMessage(err) 或 LayeredError.Message() (lines 694) |
| warn | errlint | `httpx/sse.go` | `LINT-ERR-004` | 1 | 0 | false | 禁止 err.Error() 直接进响应 / DTO / 非日志函数参数；必须 errcode.SafeMessage(err) 或 LayeredError.Message() (lines 63) |
| warn | errlint | `middleware/jwt.go` | `LINT-ERR-004` | 1 | 0 | false | 禁止 err.Error() 直接进响应 / DTO / 非日志函数参数；必须 errcode.SafeMessage(err) 或 LayeredError.Message() (lines 222) |
| warn | errlint | `queue/capacity_guard.go` | `LINT-ERR-004` | 1 | 0 | false | 禁止 err.Error() 直接进响应 / DTO / 非日志函数参数；必须 errcode.SafeMessage(err) 或 LayeredError.Message() (lines 53) |
| warn | errlint | `validator/converter.go` | `LINT-ERR-008` | 1 | 0 | false | 禁止把 err.Error() 直接赋值 / return 进 DB 列 / record 字段 / DTO：必须 errcode.SafeMessage(err) 或 LayeredError.Message() (lines 37) |

## Remediation Template

- split plan: describe the target responsibility split, such as service/repository/model/policy/adapter/generator template/fixture/test scenario.
- impact: list changed packages, public APIs, imports, generated outputs, and callers that need review.
- verification: list exact commands, including `make structure-gate` plus focused build/test commands.
- baseline rule: refresh baseline only inside a governance ticket with `STRUCTURE_GATE_BASELINE_TICKET=<ticket-id>`.
- goal: do not chase zero warnings; keep new code from worsening, protect critical paths, and maintain a stock issue map.
