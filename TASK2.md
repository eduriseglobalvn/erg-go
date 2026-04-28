# Audit Report — erg-go (2026-04-01)

> **Scope**: ~20,331 lines Go | 4 modules + 14 pkg + cmd/server
> **Scan**: TODOs, FIXMEs, stubs, performance, security, architectural gaps, test coverage
> **Status**: MOSTLY FIXED IN SESSION — còn lại vài mục optimization/architecture dài hơi

---

## 🚨 CRITICAL

### [CRIT-A] `isAdmin()` always returns `false` — auth bypass

**Status**: FIXED (2026-04-01)

**File**: `internal/modules/bot/middleware/permission.go:161`
```go
func (s *PermissionService) isAdmin(ctx context.Context, userID string) bool {
    return false // TODO: load from config/env BOT_ADMIN_IDS
}
```

**Root cause**: `BotConfig.AdminIDs` tồn tại trong `config.go:232` nhưng **không được wire vào** `PermissionService`.

**Impact**: Không ai có admin access qua bot commands — bypass không hoạt động.

**Fix**: Inject `BotConfig.AdminIDs` vào `PermissionService`, check userID presence.

---

### [CRIT-B] Error swallowing — 76 occurrences

**Status**: PARTIALLY FIXED (2026-04-01)

**Files affected**: 40 files, pattern: `_ = err`, `// err`, `err =` without check/log.

Top files by count:

| File | Count |
|---|---|
| `pkg/telemetry/prometheus.go` | 5 |
| `internal/modules/trending/trending.service.go` | 5 |
| `internal/modules/crawler/crawler.service.go` | 6 |
| `pkg/ai/gemini.go` | 1 |
| `pkg/database/mongo.go` | 2 |
| `internal/routes/routes.go` | 4 |
| `pkg/cache/redis.go` | 2 |
| `pkg/http/middleware/recovery.go` | 3 |
| `internal/modules/bot/services/conversation.go` | 3 |
| `internal/modules/bot/services/link.go` | 3 |
| ... | ... |

**Impact**: Silent failures → debugging impossible → data loss.

**Fix**: Replace `_ = err` với proper error logging hoặc propagate. Use `log.Error().Err(err)` at minimum.

**Session note**: Đã fix các nhóm swallow lỗi nổi bật trong `trending.service.go`, `crawler.service.go`, `gemini.go`, `mongo.go`, `routes.go`, `recovery.go`, `link.go`, và một phần `conversation.go`. Vẫn còn một số occurrence rải rác ngoài top-path cần cleanup tiếp nếu muốn đóng triệt để toàn bộ thống kê.

---

### [CRIT-C] RSS refresh endpoint không enqueue job thật

**Status**: FIXED (2026-04-01)

**File**: `internal/modules/crawler/rss.controller.go:196-202`
```go
// TODO: Enqueue a feed refresh job.
// For now, return accepted.
c.writeJSON(w, http.StatusAccepted, map[string]string{
    "status":  "refresh_enqueued",
    "feed_id": id,
    "feed":    feed.URL,
})
```

**Impact**: `POST /api/rss/{id}/refresh` trả về 202 nhưng không tạo ASYNQ job → feed không được refresh.

**Fix**: Enqueue `refresh_feed` job qua ASYNQ client (pattern giống `refresh_feed_job.go`).

---

## ⚠️ HIGH

### [HIGH-A] Bot platform clients không được inject — messages không gửi được

**Status**: FIXED (2026-04-01)

**File**: `internal/modules/bot/handlers/bot_controller.go:170`
```go
// Send via platform client (injected via command handler — simplified here).
// In production, inject the platform clients into this handler.
_ = update // TODO: Inject platform clients to send messages.
```

**Impact**: Reply messages qua bot không hoạt động.

**Fix**: Inject Discord/Telegram platform clients vào `BotController` từ `bot.module.go`.

---

### [HIGH-B] CORS `"*"` in production config

**Status**: FIXED (2026-04-01)

**File**: `pkg/config/config.go:498`
```go
AllowedOrigins: []string{"*"},
```

**Status**: Warning log có trong `server.go` nhưng không block — attacker có thể exploit.

**Fix**: Block startup if `app.env == "production"` AND `AllowedOrigins` contains `"*"`.

---

### [HIGH-C] Event bus Redis backend không được sử dụng

**Status**: FIXED (2026-04-01)

**File**: `pkg/event/bus.go`

- Bus hỗ trợ Redis pub/sub qua `WithRedisBackend`
- `routes.go` tạo bus với Redis backend
- **Không có module nào gọi `bus.Subscribe()`** (cross-service) — chỉ `SubscribeLocal`

**Impact**: Redis pub/sub infrastructure có sẵn nhưng dead code. Nếu muốn scale multi-instance, event không propagate.

**Fix**: Không cần fix nếu 1 instance. Nếu scale → implement `Subscribe()` trong các module.

**Session note**: `notifications.EventConsumer` đã subscribe cả local bus lẫn Redis-backed bus; event bus cũng skip self-originated Redis messages để tránh duplicate trên cùng instance.

---

### [HIGH-D] JSON allocation pressure on event bus hot paths

**Status**: OPEN

**File**: `pkg/event/bus.go`

```go
// Line 81: marshal payload
payloadBytes, err := json.Marshal(payload)

// Line 98: marshal envelope (payload already marshaled → marshal again)
envelopeBytes, err := json.Marshal(envelope)

// Line 112: marshal AGAIN in PublishLocal
payloadBytes, err := json.Marshal(payload)
```

**Impact**: JSON marshal mỗi publish call, allocate []byte mới mỗi lần. Trên high-throughput event bus → GC pressure.

**Fix**: Buffer pool cho `[]byte` reuse, hoặc reuse `json.Encoder` per subscriber.

---

## 🔧 MEDIUM

### [MED-A] Parser functions thiếu context support

**Status**: FIXED (2026-04-01)

**File**: `pkg/scraper/parser.go`

- `ExtractLinks()`, `ExtractBySelector()`, `ExtractJSONLD()` không nhận `context.Context`
- Không có cancellation khi parsing large HTML

**Fix**: Add `ctx context.Context` parameter.

---

### [MED-B] Conversation wizard rebuild là placeholder

**Status**: FIXED (2026-04-01)

**File**: `internal/modules/bot/services/conversation.go:259`
```go
// Rebuild wizard steps from wizard_step field (placeholder — in production,
// you'd store the full wizard template in the conversation or look it up).
```

**Impact**: Wizard resume có thể không restore đúng step data.

**Fix**: Store full wizard template in conversation document hoặc lookup từ template registry.

---

### [MED-C] ASYNQ retry không có exponential backoff

**Status**: FIXED (2026-04-01)

**File**: `pkg/queue/asynq.go`

```go
RetryDelay: 10 * time.Second,  // fixed — no exponential backoff
```

**Impact**: Retry storm khi external API down (10s delay × 3 retries = fast burst).

**Fix**: Exponential backoff với jitter:
```go
delay := time.Duration(math.Pow(2, float64(retry))) * time.Second
jitter := time.Duration(rand.Int63n(int64(delay / 2)))
```

---

### [MED-D] Missing request ID tracing

**Status**: FIXED (2026-04-01)

**Files**: `internal/routes/routes.go`, `pkg/http/middleware/`

- Không có `X-Request-ID` middleware
- Multi-service trace không thể correlate
- Không có request ID trong log fields

**Fix**: Add `RequestIDMiddleware` in `pkg/http/middleware/`:
1. Extract or generate UUID v4
2. Set `r.Header.Set("X-Request-ID", id)`
3. Add to log context: `log = log.With().Str("request_id", id).Logger()`
4. Return in response header

---

### [MED-E] Unused code: `pathDir()` function

**Status**: FIXED (2026-04-01)

**File**: `pkg/scraper/robots.go:236`
```go
// pathDir returns the directory portion of a URL path.
func pathDir(p string) string {
    ...
}
```

**Status**: Defined but **never called** anywhere.

**Fix**: Remove dead code.

---

## 💡 LOW

### [LOW-A] `regexCache` unbounded growth

**Status**: FIXED (2026-04-01)

**File**: `pkg/scraper/robots.go`

Compiled regex cache grow vô hạn. Với 1000 unique robots.txt patterns → 1000 compiled regex in memory forever.

**Fix**: Add LRU eviction (e.g., `lru` package) hoặc `ttlCache` với max entries.

**Session note**: Đã thêm bounded LRU cache cho compiled regex patterns.

---

### [LOW-B] `SubscribeByReflection` reflection overhead

**Status**: PARTIALLY FIXED (2026-04-01)

**File**: `pkg/event/bus.go:269`

Reflect trên mỗi call, allocate reflection objects.

**Fix**: Convenience feature — không nên dùng trong hot path. Document as such.

**Session note**: Đã document rõ trong code rằng API này không nên dùng trên hot path.

---

### [LOW-C] Missing graceful degradation

**Status**: PARTIALLY FIXED

- MongoDB/Redis down → toàn bộ app crash
- Không có circuit breaker cho external calls (Gemini AI, etc.)

**Fix**: Add circuit breaker cho external service calls (e.g., `sony/gobreaker`).

**Session note**: Repo đã có `pkg/http/client.go` với circuit breaker cho HTTP calls, nhưng app vẫn khởi động fail-fast nếu MongoDB/Redis unavailable và Gemini client hiện chưa được migrate sang wrapper này.

---

### [LOW-D] Missing index hints in MongoDB queries

**Status**: OPEN

**File**: `internal/modules/crawler/repository/repository.go`, `internal/modules/trending/`

MongoDB queries không có `Hint()` — không đảm bảo sử dụng đúng index.

**Fix**: Analyze query plans, add compound indexes, use `Hint()` where needed.

---

## ✅ ĐIỂM TỐT (đã có sẵn)

- `pkg/cache/redis.go` — Lua script cho atomic lock release ✅
- `pkg/scraper/fetcher.go` — retry + backoff + UA rotation ✅
- Module pattern NestJS-style nhất quán ✅
- SIGHUP hot-reload + DB health monitor ✅
- Prometheus /metrics endpoint ✅
- HMAC verification cho Discord/Telegram webhooks ✅
- SSE với proper drain + timeout ✅
- Distributed lock với unique ownership value + Lua release ✅
- Config hot-reload atomic swap ✅
- Workflow engine wired into bot ✅

---

## ✅ ĐÃ FIX TRONG SESSION NÀY (2026-04-01)

| Item | Fix |
|---|---|
| [CRIT-A] Admin bypass | `BotConfig.AdminIDs` wired vào `PermissionService` + test |
| [CRIT-C] RSS refresh enqueue | `POST /api/rss/feeds/{id}/refresh` enqueue Asynq job thật |
| [HIGH-A] Bot platform clients | `BotController` gửi thật qua Discord/Telegram clients |
| [HIGH-B] CORS production wildcard | Config loader block `AllowedOrigins=["*"]` ở production |
| [HIGH-C] Redis event bus usage | Notifications consumer dùng `Subscribe()` + self-event dedupe |
| [MED-A] Parser context support | `ExtractLinks/ExtractBySelector/ExtractJSONLD` nhận `context.Context` |
| [MED-B] Wizard rebuild | Persist/restore wizard template key cho RSS wizard |
| [MED-C] ASYNQ backoff | Retry delay có exponential-style backoff + stable jitter |
| [MED-D] Request ID tracing | Middleware inject request ID vào context/header cho logging |
| [MED-E] Dead code `pathDir()` | Removed |
| [LOW-A] Unbounded regex cache | Bounded LRU cache cho compiled regex |
| [LOW-B] Reflection hot path note | `SubscribeByReflection` documented as non-hot-path API |
| [HIGH-8] `FetchRobotsTxt` context leak | Context-aware deadline + Transport pooling |
| `matchesPath` regex recompile | `sync.Map` regex cache |
| `matchesPathSpecificity` regex recompile | Reuse `getPathRegex()` |
| `RobotsCache` no TTL eviction | `expires` map + lazy eviction + `Evict()` |
| `ExtractJSONLD` stub | Full JSON array/object + `@graph` unwrap |

---

## 📋 ACTION PLAN

| Priority | Task | Effort | Owner |
|---|---|---|---|
| **P0** | Wire `BotConfig.AdminIDs` → `PermissionService` | 30 phút | Agent |
| **P0** | Fix RSS refresh enqueue ASYNQ job | 20 phút | Agent |
| **P0** | Fix bot platform clients injection | 1 giờ | Agent |
| **P1** | JSON buffer pool cho event bus | 2 giờ | Agent |
| **P1** | Add `X-Request-ID` middleware | 1 giờ | Agent |
| **P1** | Fix error swallowing (top 10 files) | 2 giờ | Agent |
| **P1** | Block CORS `"*"` in production | 30 phút | Agent |
| **P2** | Context support trong parser functions | 1 giờ | Agent |
| **P2** | Exponential backoff cho ASYNQ retries | 1 giờ | Agent |
| **P2** | Wizard rebuild full template | 2 giờ | Agent |
| **P3** | `regexCache` LRU eviction | 1 giờ | Agent |
| **P3** | Circuit breaker cho external calls | 2 giờ | Agent |
| **P3** | MongoDB index hints | 1 giờ | Agent |
| **P3** | Remove dead code `pathDir()` | 5 phút | Agent |

## 📌 REMAINING OPEN ITEMS

- [CRIT-B] Finish cleanup của toàn bộ swallow-error occurrences còn sót lại ngoài các hot paths đã fix.
- [HIGH-D] Giảm JSON allocation pressure trong `pkg/event/bus.go` bằng buffer pool / envelope reuse.
- [LOW-C] Decide whether app should degrade gracefully khi MongoDB/Redis unavailable, và migrate external HTTP-heavy clients sang shared circuit-breaker wrapper nếu cần.
- [LOW-D] Review query plans + indexes trước khi thêm `Hint()` để tránh hard-code sai execution plan.
