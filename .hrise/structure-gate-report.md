# Structure Gate Report

- mode: `changed`
- files scanned: `395`
- functions scanned: `3240`
- issues: `28`
- blocking: `0`

## Issues

| severity | kind | path | name | value | threshold | changed | reason |
|---|---|---|---|---:|---:|---|---|
| block | naming | `limiter/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `logger/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `kafka/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `telemetry/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `breaker/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `queue/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `application/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `auth/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `cache/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `auth/provider.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `swagger/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `config/builder.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `jwt/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `database/repository.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `redis/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `grpc/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `auth/service.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `database/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `swagger/provider.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `httpx/handler.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `httpx/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `config/validator.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `health/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `event/config.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| block | naming | `application/router.go` | `` | 1 | 0 | false | weak file name; split/new business files should use business_role.go |
| warn | func | `application/base_app.go` | `*BaseApplication.registerComponentMetrics` | 150 | 130 | false | function line count exceeds threshold |
| warn | func | `application/http_server.go` | `newServer` | 135 | 130 | false | function line count exceeds threshold |
| warn | func | `retry/retry.go` | `DoWithData` | 113 | 110 | false | function line count exceeds threshold |

## Remediation Template

- split plan: describe the target responsibility split, such as service/repository/model/policy/adapter/generator template/fixture/test scenario.
- impact: list changed packages, public APIs, imports, generated outputs, and callers that need review.
- verification: list exact commands, including `make structure-gate` plus focused build/test commands.
- baseline rule: refresh baseline only inside a governance ticket with `STRUCTURE_GATE_BASELINE_TICKET=<ticket-id>`.
- goal: do not chase zero warnings; keep new code from worsening, protect critical paths, and maintain a stock issue map.
