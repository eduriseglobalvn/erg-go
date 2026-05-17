# Rate Limit and Session Hardening

## Distributed rate limits

HTTP rate limiting uses Redis-backed sliding windows implemented with a Lua script. Each decision removes expired entries, counts active entries, and records the current request in one atomic Redis operation. This keeps limits deterministic across multiple backend instances.

Local development without Redis falls back to an in-process limiter. That fallback is acceptable for local work only; production must run with Redis available.

## Route policies

- Auth login routes are checked across three dimensions: IP, identity, and device.
- Auth write routes fail closed if Redis cannot make a limiter decision.
- Upload, admin, and operations write routes fail closed.
- Public read routes use a higher bounded limit and fail open on Redis errors to avoid turning a cache outage into a public read outage.

Metrics are exported under:

- `erg_http_rate_limit_requests_total`
- `erg_http_rate_limit_decision_duration_seconds`

Labels include policy, route, backend, and outcome.

## Redis outage behavior

Auth-sensitive and write-heavy routes return `503 ERR_RATE_LIMITER_UNAVAILABLE` when Redis is configured but cannot answer in time. Public read routes continue and emit `redis_error_fail_open` metrics.

## Session persistence

Login creates the new session row before returning tokens. Previous-session revocation and login metadata updates run after the critical path, so slow remote databases do not leave the client waiting for non-critical cleanup. If session creation itself fails, login fails instead of returning a token that cannot resolve `/api/sessions/current`.

Use `/metrics` to alert on limiter Redis errors and login/session persistence warning logs.
