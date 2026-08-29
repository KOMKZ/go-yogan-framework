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

## Rules

- Capture I/O and SDK boundary errors with `errcode.Capture(err, "stable.operation")`.
- Wrap business semantics with `LayeredError.Wrap(err)`.
- Do not add per-handler logging glue for provider details.
- Do not log tokens, passwords, Authorization headers, DSNs, or access secrets.
