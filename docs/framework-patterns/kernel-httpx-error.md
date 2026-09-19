# HTTPX Error Logging

## Boundary

`httpx.HandleError` is the unified HTTP error exit. Application handlers should return errors and let `httpx` decide the response envelope and logging fields.

## LayeredError Fields

For `errcode.LayeredError`, `httpx` logs these diagnostic fields when logging is enabled:

| Field | Meaning |
|-------|---------|
| `error_code` | Public business error code returned to clients |
| `error_msg` | Public business message returned to clients |
| `error_origin_stack` | First captured business or I/O boundary stack |
| `error_operation` | Stable operation name from `errcode.Capture` |
| `error_cause_type` / `error_cause_message` | First non-`LayeredError` contextual cause, useful for troubleshooting |
| `error_root_type` / `error_root_message` | Deepest unwrapped root cause, useful for sentinel or technical classification |
| `error_chain` | Compact complete chain string |

`error_cause_message` and `error_root_message` are intentionally different. A provider error may unwrap to a sentinel such as `provider rejected`; the cause keeps provider code and context, while the root stays stable for `errors.Is` semantics.

## Redaction (ticket 000128)

`error_cause_message`, `error_root_message`, `error_chain`, and the full-chain `error` field are passed through `errcode.Redact` before being written to logs. The sanitizer covers URL/DSN credentials, Bearer tokens, token/api_key/signature/password-style key/value pairs (query, form, and JSON shapes), `sk-` style keys, emails, and CN mobile numbers. Client safety does not imply log safety; the redaction regression tests live in `errcode/redact_test.go` and `httpx/response_test.go`.

## Response Data Boundary (ticket 000128 P0-4)

- `httpx.HandleError` writes `LayeredErr.PublicData()` into the response `Data` field — never the private `Data()` map. Diagnostic data added with `WithData`/`WithFields` stays server-side.
- `httpx.BadRequestJson` never writes `err.Error()`: registered business codes use their registered message; everything else degrades to the fixed text `请求无效，请检查参数`. Use `httpx.ErrorJson` when a caller needs an explicit reviewed string.
- To expose data to clients, mark it explicitly with `LayeredError.WithPublicData(key, value)`.

## Rules

- Capture I/O and SDK boundary errors with `errcode.Capture(err, "stable.operation")`.
- Wrap business semantics with `LayeredError.Wrap(err)`.
- Do not add per-handler logging glue for provider details.
- Do not log tokens, passwords, Authorization headers, DSNs, or access secrets; every error text that reaches logs goes through `errcode.Redact`.
- Do not read `LayeredError.Data()` in HTTP/DTO layers; use `PublicData()`.
