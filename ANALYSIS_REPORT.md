# ERG-Go — Deep Codebase Analysis Report

> **Ngày phân tích:** 2026-04-02
> **Người thực hiện:** Claude Code (Deep Analysis Mode)
> **Phiên bản codebase:** erg-go (monorepo Go backend)

---

## 1. Project Overview

**ERG-Go** là một Go monolith phục vụ nền tảng công nghệ giáo dục **EduRise Global (ERG)**. Dự án được thiết kế theo phong cách lấy cảm hứng từ NestJS, sử dụng các module độc lập (dependency injection per module), nhưng triển khai hoàn toàn bằng Go thuần.

### Tech Stack chính

| Layer | Technology |
|---|---|
| Web Framework | Chi Router v5 (`github.com/go-chi/chi/v5`) |
| Config | Viper (`github.com/spf13/viper`) |
| Logging | zerolog (`github.com/rs/zerolog`) |
| Primary DB | pgx v5 (MySQL / PostgreSQL) |
| Secondary DB | MongoDB Driver v2 (`go.mongodb.org/mongo-driver/v2`) |
| Cache / Queue | Redis (`github.com/redis/go-redis/v9`) + Asynq |
| AI | Google Gemini (`ai.gemini.google.com`) |
| Observability | OpenTelemetry + Prometheus |
| Language | Go 1.21+ |

### Package Layout

```
erg-go/
├── cmd/server/          # Entry point: Run() bootstrap
├── internal/
│   ├── modules/
│   │   ├── crawler/     # Web scraping pipeline
│   │   ├── bot/         # Multi-platform messaging bot
│   │   ├── notifications/ # Notification dispatch
│   │   └── trending/    # Trending topics aggregation
│   └── routes/         # Route registration
└── pkg/
    ├── event/           # In-process + Redis pub/sub bus
    ├── cache/           # Redis client wrapper
    ├── queue/           # Asynq client/server
    ├── ai/              # Gemini AI client
    ├── scraper/         # Fetcher + robots.txt
    ├── rss/             # RSS/Atom/JSON Feed parser
    ├── sitemap/         # Sitemap discovery + fetch
    ├── dedup/           # SimHash + Hamming distance dedup
    ├── telemetry/       # OpenTelemetry + Prometheus
    └── http/middleware/ # Rate limit, CORS, auth middleware
```

---

## 2. Module Structure

### 2.1 Module Pattern (NestJS-style)

Mỗi module tuân theo pattern 4-phase lifecycle:

```
NewModule(cfg) → Setup(services) → RegisterRoutes(r) → Stop()
```

Điều này đảm bảo initialization order, tránh circular dependency, và cho phép graceful shutdown.

---

### 2.2 Module: `crawler`

**Responsibility:** Web scraping pipeline end-to-end — từ URL blacklist → robots.txt → fetch → quality gate → dedup → metadata extraction → tagging → content fingerprinting → reputation tracking → event publish.

#### Key Components

**`CrawlerService` (`internal/modules/crawler/crawler.service.go`)**

- **`SSEHub`**: In-memory channel-based SSE hub cho crawl progress streaming. Clients subscribe via `Subscribe(jobID) <-chan SSEProgress`, hub broadcast đến tất cả subscribers.
- **`RunPipeline(ctx, job)`**: 12-step synchronous pipeline:
  1. `checkBlacklist`
  2. `checkRobotsTxt`
  3. `fetchURL`
  4. `qualityGate`
  5. `checkDuplicate`
  6. `extractMetadata`
  7. `extractTags`
  8. `computeHash`
  9. `saveContent`
  10. `computeFingerprint`
  11. `updateReputation`
  12. `publishEvent`
- **`qualityGate()`**: 8-rule quality scorer — scoring rubric:
  - Body ≥ 500 chars: +1
  - Body ≤ 50,000 chars: +1
  - Has title: +1
  - Has meta description: +1
  - Has at least 1 heading: +1
  - No excessive links (> 100): +1
  - Valid HTML structure: +1
  - Has published date: +1
  - **Threshold ≥ 5**: content được chấp nhận
- **`UpdateReputation(domain, score)`**: Track domain reputation score (cumulative weighted average), lưu vào MongoDB `crawler_domains` collection.
- **`SSEHub.Broadcast(jobID, SSEProgress)`**: Broadcast progress ra tất cả subscribed clients.

**`CrawlerController`**: Chi routes — `POST /crawl`, `GET /crawl/jobs`, `GET /crawl/jobs/{id}`, `GET /crawl/jobs/{id}/progress`.

**`CrawlerRepository`**: MongoDB CRUD cho crawl jobs.

**Pipeline anti-blocking:**
- User-Agent rotation
- Proxy rotation
- Respect robots.txt (`pkg/scraper/robots.go`)
- Crawl delay từ robots.txt
- Block pattern detection (HTTP 403/429 detection → exponential backoff)

---

### 2.3 Module: `bot`

**Responsibility:** Multi-platform messaging bot — Discord (slash commands + webhooks), Telegram (webhooks), account linking, conversation wizards, workflow engine.

#### Key Components

**`CommandHandler` (`internal/modules/bot/services/command_handler.go`)**

- Switch-based command dispatch đến registered handlers
- **RBAC 5-level permission system**: `owner > admin > moderator > trusted > user`
- Cooldown map: per-command, per-user rate limiting
- Command aliases via `Aliases []string` field

**`ConversationService` (`internal/modules/bot/services/conversation.go`)**

- **Wizard pattern**: Multi-step conversation flows với TTL (5 phút)
- **Dual storage**: In-memory map (`wizards map[string]*WizardState`) + MongoDB persistence
- **`HandleWizardInput()`**: Process user input → validate → advance step → persist
- **`WizardTTL = 5 minutes`**, max data size 100 fields
- **RSS Add Wizard**: 4-step flow (input URL → input category → confirm → done)
- `detectWizardTemplate()`: Infer wizard type từ step structure (structural equality)
- `restoreWizardSteps()`: Rebuild wizard steps từ MongoDB state (with fallback inference)

**`WorkflowEngine` (`internal/modules/bot/services/workflow.go`)**

- **Workflow**: Named automation flows với typed step handlers
- **Step handlers**: `WorkflowStepHandlerFunc func(ctx, stepCtx) (string, error)`
- **Execution**: Sequential step processing, support pause/resume/cancel
- **Retry**: Per-step retry với max attempts
- **Persistence**: MongoDB `workflow_executions` collection

**`LinkService` (`internal/modules/bot/services/link.go`)**

- **6-character alphanumeric link code** (a-z, 0-9, uppercase letters) — 2.1 billion combinations
- **Redis TTL**: 5 phút expiration
- Flow: `CreateLinkCode(userID)` → store `{code: userID}` in Redis → user sends code via bot → `VerifyLinkCode(code)` → create `BotAccount` in MongoDB

**Webhook Verification:**
- **Discord**: HMAC-SHA256 body signature verification
- **Telegram**: Ed25519 bot API webhook verification

**`BotController` (`internal/modules/bot/handlers/bot_controller.go`)**

- `GET/POST /conversations` — list, get, send messages
- `GET /conversations/{id}/wizard` — wizard state inspection
- `POST /link`, `GET /link/{code}` — account linking
- `GET /accounts`, `DELETE /accounts/{id}` — linked account management

---

### 2.4 Module: `notifications`

**Responsibility:** Notification dispatch qua nhiều kênh (Discord, Telegram, WhatsApp, Email) với digest batching, rate limiting, và template rendering.

#### Key Components

**`NotificationService` (`internal/modules/notifications/service.go`)**

- **`Send()`**: Fan-out đến provider dựa trên `ChannelType`
- **`SendToChannels()`**: Gửi đồng thời đến nhiều channels
- **`SendDigest()`**: Batch multiple notifications thành digest (configurable interval)
- **Provider dispatch** via `provider.Send(ctx, msg)` interface

**`EventConsumer` (`internal/modules/notifications/event_consumer.go`)**

- Subscribe to **14 event topics** từ EventBus:
  - `crawl.success`, `crawl.failed` → Discord
  - `trending.hot_topic`, `trending.daily` → Discord / Email
  - `system.alert`, `system.warning`, `system.recovery` → Discord
  - `queue.overload`, `queue.status` → Discord
  - `rss.added`, `rss.fetch_error` → Discord
  - `bot.account.linked`, `bot.account.unlinked` → Telegram
- Dual subscription: `SubscribeLocal` (in-process) + `Subscribe` (Redis cross-service)
- Payload extraction: user_id, recipient, body from `map[string]string`
- **`handleEvent()`**: Timeout 15s, unmarshal payload, dispatch to `SendFromEvent()`

**Provider Interface Pattern:**

```go
type Provider interface {
    Send(ctx context.Context, msg Message) error
    SendTemplate(ctx context.Context, msg Message, template string, data map[string]string) error
}
```

Implementations:
- **`DiscordProvider`**: Discord webhook API, `discord.go` — rate limit 10 req/10s, exponential backoff
- **`TelegramProvider`**: Telegram Bot API, `telegram.go` — rate limit 20 msg/60s, parse mode (MarkdownV2 / HTML)
- **`WhatsAppProvider`**: WhatsApp Business API, `whatsapp.go` — async send via queue, template requirement
- **`EmailProvider`**: SMTP + Go's `net/smtp`, `email.go` — TLS, HTML + plain text multipart

**Template Renderer** (`notifications/tpl/renderer.go`):
- Vietnamese-first template engine
- Template lookup by name + channel type
- Data injection: `{{.FieldName}}` placeholders
- HTML sanitization via `bluemonday`

---

### 2.5 Module: `trending`

**Responsibility:** Trending topics aggregation từ MongoDB, Redis-cached, cron-refreshed.

#### Key Components

**`TrendingService` (`internal/modules/trending/trending.service.go`)**

- **`AggregateTopics(ctx, timeframe)`**: MongoDB aggregation pipeline → hot topics (score = `view_count * recency_factor + engagement`)
- **`GetDiscoveryFeed(ctx, source, cursor)`**: Paginated discovery feed từ Redis list (`RPUSH/LRANGE`)
- **`GetNews(ctx, topic)`**: News articles by topic from MongoDB

**`TrendingRepository` (`repository.go`)**

- MongoDB `trending_topics` + `news_articles` collections
- Compound indexes: `{topic, date: -1}`, `{source, date: -1}`

**`Scheduler` (`scheduler.go`)**

- **robfig/cron** v3 scheduler
- Default: refresh `*/15 * * * *` (every 15 minutes)
- `Start()`, `Stop()` lifecycle management

**`RedisCache` (`cache/redis_cache.go`)**

- JSON serialization cache với TTL
- `GetOrFetch(key, fetcher)` pattern với TTL-based invalidation

---

## 3. Routing & Middleware

### Chi Router v5

**Global Middleware Stack** (từ `cmd/server/server.go`):

1. **Recovery** — panic recovery middleware
2. **RequestID** — inject `X-Request-ID` header
3. **Logger** — structured zerolog request logging
4. **Tracer** — OpenTelemetry span creation per request
5. **Timeout** — context timeout (30s default)
6. **CORS** — configurable allowed origins
7. **RateLimit** — per-IP token bucket (configurable QPS)
8. **Prometheus** — HTTP metrics collector

**Route Registration** (`internal/routes/routes.go`):

```
/crawl/*          → CrawlerModule
/bot/*            → BotModule
/notifications/*  → NotificationsModule
/trending/*       → TrendingModule
/metrics          → Prometheus handler
/health           → Health check
```

### HTTP Middleware (`pkg/http/middleware/`)

- **`ratelimit.go`**: Token bucket algorithm, atomic counter, configurable burst/limit
- **`cors.go`**: Allow origin list, credential support, preflight handling
- **`auth.go`**: Bearer token JWT validation

### Hot-Reload

- **SIGHUP signal** handler → atomic config reload
- `atomic.Pointer[Config]` — zero-allocation atomic config swap
- Rate limit + CORS params hot-reloaded via `atomic.Value`

---

## 4. Database Strategy

### Dual Database Architecture

```
┌─────────────────────────────────────────────────────┐
│                    ERG-Go Application                │
│                                                      │
│  ┌──────────────────┐     ┌──────────────────────┐  │
│  │  pgx v5          │     │  MongoDB Driver v2   │  │
│  │  (MySQL/Postgres)│    │  (Document store)    │  │
│  │                  │     │                      │  │
│  │  • Crawl jobs    │     │  • Bot conversations │  │
│  │  • RSS sources   │     │  • Workflow states   │  │
│  │  • User data     │     │  • Wizard states     │  │
│  │  • Trending tops │     │  • Bot accounts      │  │
│  │  • Notifications │     │  • Crawl metadata   │  │
│  │  • Queue jobs    │     │  • Domain reps       │  │
│  └──────────────────┘     └──────────────────────┘  │
└─────────────────────────────────────────────────────┘
```

### pgx v5 (relational data)

- Prepared statement caching
- Connection pool: `pgxpool.New()`
- Batch queries for bulk inserts
- Scan to struct với custom type converters

### MongoDB v2 (document + session data)

- CRUD qua `mongo.Collection`
- Aggregation pipelines cho trending topics
- Change streams support (future)
- Indexes:
  - `bot_conversations`: `{platform_conv_id}` (unique), `{user_id}`, `{state, updated_at}`
  - `trending_topics`: `{topic, date: -1}`, `{source, date: -1}`
  - `workflow_executions`: `{workflow_id}`, `{status}`
  - `crawler_domains`: `{domain}` (unique)

---

## 5. Queue / Async Strategy

### Asynq (Redis-backed job queue)

**Priority Queues** (`pkg/queue/asynq.go`):

| Queue | Priority | Use Case |
|---|---|---|
| `critical` | 6 | Webhook delivery, bot responses |
| `high` | 5 | Crawl initiation |
| `default` | 3 | Content processing |
| `low` | 1 | Cleanup, digest sending |

**Retry Policy** — stable jitter:
```
delay = min(base * 2^attempt + jitter, maxDelay)
jitter = random(0, base/2)
```

**`AsynqServer`**: Registered handlers:
- `crawl:fetch` — URL fetching
- `crawl:process` — Content processing
- `crawl:fingerprint` — SimHash fingerprinting
- `notify:send` — Notification dispatch
- `notify:digest` — Digest batching

**`AsynqClient`** (`Enqueue`): Auto-serialization of task payload to JSON.

### SSE Hub (in-process real-time)

- In-memory channel-based pub/sub
- `SSEHub.Subscribe(jobID)` → exclusive channel
- `SSEHub.Broadcast(jobID, SSEProgress)` → fan-out to all subscribers
- Graceful cleanup: `Unsubscribe(jobID)` closes channel

---

## 6. Caching Strategy

### Redis Client (`pkg/cache/redis.go`)

**`DistributedLock`** — Lua script atomic acquire/release:

```lua
-- Acquire: SET key unique_value NX PX ttl_ms
-- Release: EVALSHA + check value matches
```

**Cache Patterns:**

| Pattern | Implementation | TTL |
|---|---|---|
| Simple cache | `SET key json.Marshal(v), EX ttl` | Configurable |
| Pub/Sub | `PUBLISH channel payload` | N/A |
| Link codes | `SET code userID EX 300` | 5 phút |
| Wizard state | MongoDB (see above) | 5 phút |
| Content hash | `SET hash urlKey EX 24h` | 24 giờ |
| Trending data | `SET key json EX 15m` | 15 phút |
| Regex cache | LRU list (256 entries) | unbounded |
| Pipeline | `redis.Pipeline()` for batch ops | N/A |

### LRU Regex Cache (`pkg/scraper/robots.go`)

- Bounded LRU cache: `maxPathRegexEntries = 256`
- `container/list` for O(1) move-to-front + eviction
- Thread-safe via `sync.Mutex`
- Pattern → compiled `*regexp.Regexp`

---

## 7. AI Integration

### Gemini Client (`pkg/ai/gemini.go`)

**Features:**
- **Content analysis**: `AnalyzeContent(ctx, text)` → structured insights
- **Tag extraction**: `SuggestTags(ctx, content)` → keyword extraction
- **CSS selector suggestion**: `SuggestSelectors(ctx, pageContent)` → for scraper optimization
- **Image understanding**: `AnalyzeImage(ctx, imageURL)` → alt text, descriptions

**Cache Strategy — Dual-tier:**
1. **L1**: In-memory `sync.Map` (process-local)
2. **L2**: Redis `GET/SET` with TTL (distributed)
3. Cache key: deterministic hash of request payload

**Rate Limiting**: `semaphore` with configurable concurrency limit.

---

## 8. Monitoring & Observability

### OpenTelemetry (`pkg/telemetry/otel.go`)

- **`InitTracer()`**: SDK-style setup với `otelsdktrace.Provider`
- **W3C Trace Context propagation** via `otelpropagation.New()` (B3 + W3C)
- Spans: `Publish`, `Subscribe`, `HTTP handler`, `DB query`, `Redis op`
- Resource labels: `service.name`, `service.version`, `host.name`

### Prometheus (`pkg/telemetry/prometheus.go`)

**`PrometheusRegistry`** aggregates:

| Metric Type | Metrics |
|---|---|
| HTTP | `http_requests_total{method, path, status}`, `http_request_duration_seconds{path}` |
| Crawler | `crawl_jobs_total{status}`, `crawl_fetch_duration_seconds`, `crawl_content_size_bytes`, `crawl_quality_score` |
| Notifications | `notifications_sent_total{channel, status}`, `notifications_failed_total{channel}`, `notification_send_duration_seconds{channel}` |

- **`NewPrometheusHTTP()`**: Wraps chi router, auto-instruments all routes
- **`NewCrawlerMetrics()`**: Specialized crawler metrics
- **`NewNotificationMetrics()`**: Per-channel notification metrics
- **`Provide()`**: Inject via chi router via middleware

### Health Monitor (`internal/health/`)

- **`HealthChecker`**: Aggregates Redis, MongoDB, MySQL, Asynq health checks
- `GET /health`: Returns `{"status": "ok/degraded/down", "components": {...}}`
- Periodic check every 30s

---

## 9. Code Quality Assessment

### Strengths

1. **Consistent module pattern** — NestJS-inspired lifecycle (`New → Setup → RegisterRoutes → Stop`) across all modules makes the codebase predictable
2. **Interface-based design** — Notification providers, cache backend, AI client all use interfaces, enabling easy mocking and swapping
3. **Dual caching** — Redis + in-memory LRU in robots parser; Redis + in-memory in AI client; clear separation of concerns
4. **Graceful shutdown** — All modules return `stop func()` for clean teardown
5. **Error wrapping** — Consistent `fmt.Errorf("module: operation: %w", err)` pattern
6. **Context propagation** — All long-running operations respect context cancellation/timeout
7. **Structured logging** — zerolog with consistent field naming (`Str`, `Err`, `Int`)
8. **RBAC system** — Clean permission enum with explicit level checks

### Concerns

1. **`sameWizardSteps()` structural comparison** — Uses struct field comparison (Name, NextStep, OnComplete only), missing Prompt/Validate. May cause false positives in template detection.

2. **`SubscribeByReflection` hot path** — Uses `reflect` per-message, allocates `reflect.Value` objects on every event. Marked as "not for hot paths" but still available.

3. **No retry on MongoDB writes** — `persistWizard`, `deleteWizard` have no retry logic. A transient MongoDB error loses wizard state.

4. **SSEHub memory unbounded** — No max client limit; a malicious client can subscribe to many jobIDs and hold open channels.

5. **Race condition in `SubscribeLocal`** — The returned cancel closure captures `cancelFn` and `cancelID` correctly, but the in-memory `wizards` map in `ConversationService` is accessed without the write lock in `buildWizardResult` (read path uses `RLock` but `buildWizardResult` is called within write lock in `HandleWizardInput`).

6. **Goroutine leak in `Subscribe`** — `pubsub.Close()` on context cancellation is not deferred; if `ReceiveMessage` blocks forever, goroutine may not exit cleanly. Should use `defer pubsub.Close()`.

7. **Magic numbers** — Quality threshold (5/8), TTL values (5 min, 15 min, 24h), burst sizes, rate limits — many are defined as constants but scattered across files.

8. **Missing integration tests** — No test files found in the read batches (`.go` files without `_test.go` companions).

---

## 10. ASCII Architecture Diagram

```
                         ┌─────────────────────────────────────────────────────────┐
                         │                     ERG-Go Server                       │
                         │                     (Chi Router v5)                     │
                         │                                                         │
                         │  ┌─────────────────────────────────────────────────────┐  │
                         │  │               MIDDLEWARE STACK                      │  │
                         │  │  Recovery → RequestID → Logger → Tracer            │  │
                         │  │  Timeout → CORS → RateLimit → Prometheus             │  │
                         │  └─────────────────────────────────────────────────────┘  │
                         │                                                         │
  ┌─────────────────┐    │  ┌──────────────┐  ┌───────────────┐                    │
  │   SSE Clients   │────┼──│   SSEHub      │  │  HealthCheck  │                    │
  │  (Browser/CLI)  │    │  │  (in-memory)  │  │  /health       │                    │
  └─────────────────┘    │  └──────────────┘  └───────────────┘                    │
                         │         ↕                                               │
                         │  ┌─────────────────────────────────────────────────────┐  │
                         │  │                 MODULE LAYER                         │  │
                         │  │                                                       │  │
                         │  │  ┌───────────┐  ┌───────────┐  ┌────────────────┐   │  │
                         │  │  │  CRAWLER  │  │   BOT     │  │ NOTIFICATIONS  │   │  │
                         │  │  │  Module   │  │  Module   │  │    Module      │   │  │
                         │  │  │           │  │           │  │                │   │  │
                         │  │  │ Controller│  │Controller│  │ EventConsumer  │   │  │
                         │  │  │ Service   │  │ Service  │  │   Service     │   │  │
                         │  │  │Repository │  │Repository│  │  + Providers  │   │  │
                         │  │  │   SSEHub  │  │Commands  │  │  Discord       │   │  │
                         │  │  │  Pipeline │  │Wizards   │  │  Telegram      │   │  │
                         │  │  │           │  │Workflows │  │  WhatsApp      │   │  │
                         │  │  │           │  │ LinkSvc  │  │  Email         │   │  │
                         │  │  └───────────┘  └───────────┘  └────────────────┘   │  │
                         │  │                                                       │  │
                         │  │  ┌──────────────────────────────────────────────┐   │  │
                         │  │  │              TRENDING Module                  │   │  │
                         │  │  │  Service → Repository → RedisCache → Scheduler│   │  │
                         │  │  └──────────────────────────────────────────────┘   │  │
                         │  └─────────────────────────────────────────────────────┘  │
                         │         │                │                │                  │
                         │         ↓                ↓                ↓                  │
                         │  ┌─────────────────────────────────────────────────────┐  │
                         │  │                   PKG LAYER                        │  │
                         │  │                                                       │  │
                         │  │  ┌──────────┐  ┌──────────┐  ┌────────────────┐    │  │
                         │  │  │  Event   │  │  Cache   │  │     Queue      │    │  │
                         │  │  │   Bus    │  │  (Redis) │  │   (Asynq)      │    │  │
                         │  │  │ local +  │  │  Lock +  │  │  Priority Qs   │    │  │
                         │  │  │  Redis   │  │  PubSub  │  │  retry+jitter  │    │  │
                         │  │  └──────────┘  └──────────┘  └────────────────┘    │  │
                         │  │                                                       │  │
                         │  │  ┌──────────┐  ┌──────────┐  ┌────────────────┐    │  │
                         │  │  │    AI    │  │  Scraper │  │  Telemetry     │    │  │
                         │  │  │ (Gemini) │  │          │  │  OTel + Prom   │    │  │
                         │  │  │ Dual     │  │ Fetch +  │  │  Metrics +     │    │  │
                         │  │  │  cache   │  │ robots   │  │  Traces        │    │  │
                         │  │  └──────────┘  └──────────┘  └────────────────┘    │  │
                         │  │                                                       │  │
                         │  │  ┌──────────┐  ┌──────────┐  ┌────────────────┐    │  │
                         │  │  │   RSS    │  │ Sitemap  │  │     Dedup      │    │  │
                         │  │  │  Parser  │  │  Parser  │  │  SimHash+FNV   │    │  │
                         │  │  │ 2.0/Atom │  │ bounded  │  │  Hamming dist  │    │  │
                         │  │  │ JSONFeed │  │ goroutine│  │  16-bit bucket │    │  │
                         │  │  └──────────┘  └──────────┘  └────────────────┘    │  │
                         │  └─────────────────────────────────────────────────────┘  │
                         └─────────────────────────────────────────────────────────┘
                                            │                      │
                         ┌──────────────────┘                      └──────────────────┐
                         │                                                             │
                         ▼                                                             ▼
              ┌──────────────────────┐                              ┌──────────────────┐
              │       MongoDB        │                              │       Redis       │
              │                      │                              │                  │
              │ bot_conversations    │                              │ pub/sub channels │
              │ bot_accounts         │                              │ job queues       │
              │ workflow_executions  │                              │ link codes (TTL) │
              │ crawler_domains      │                              │ trending cache   │
              │ trending_topics      │                              │ AI L1/L2 cache   │
              │ news_articles        │                              │ regex LRU cache  │
              └──────────────────────┘                              └──────────────────┘
                                                                          │
                                            ┌─────────────────────────────┘
                                            │
                                            ▼
                               ┌─────────────────────────┐
                               │     Asynq Workers        │
                               │  critical/high/default/ │
                               │  low priority queues    │
                               │  stable-jitter retry    │
                               └─────────────────────────┘
                                            │
                         ┌──────────────────┴──────────────────┐
                         ▼                                     ▼
              ┌──────────────────┐               ┌──────────────────────┐
              │    Gemini AI     │               │   External Services   │
              │  (Google Cloud)  │               │                      │
              │                  │               │ Discord webhooks      │
              │                  │               │ Telegram Bot API      │
              │                  │               │ WhatsApp Business API │
              │                  │               │ SMTP Email servers    │
              └──────────────────┘               └──────────────────────┘
```

---

## 11. Key Insights

### 1. NestJS-Inspired Module Pattern — Go-idiomatic

Dự án áp dụng NestJS module lifecycle (`New → Setup → RegisterRoutes → Stop`) bằng pure Go functions. Không có framework ràng buộc — chỉ là convention. Điều này mang lại:
- **Testability**: mỗi module có thể instantiate với mock dependencies
- **Composability**: modules được compose trong `routes.Register()`
- **Graceful shutdown**: mọi module đều trả về `stop func()`

### 2. Dual Database là Design Choice quan trọng

- **pgx (SQL)**: Dùng cho dữ liệu có schema rõ ràng, cần transaction, relational integrity (crawl jobs, RSS sources, user data, notification logs)
- **MongoDB**: Dùng cho document data, flexible schema, wizard state, workflow executions, conversation history

Đây là trade-off giữa **consistency (SQL)** và **flexibility (MongoDB)**. Không dùng ORM — dùng raw driver với prepared statements.

### 3. Event Bus là Cross-Cutting Communication Layer

`pkg/event/bus.go` implement 2-tier pub/sub:
- **Local** (`SubscribeLocal`): Synchronous, in-process — zero network overhead, perfect for same-process coupling
- **Redis** (`Subscribe`): Asynchronous, cross-service — enables microservices decomposition in future

`EventConsumer` trong notifications module là consumption layer — subscribe tất cả 14 topics, dispatch notification dựa trên payload. Nếu tách notifications thành service riêng, chỉ cần enable Redis backend.

### 4. Crawler Pipeline là Core Business Logic

12-step pipeline là trái tim của ERG-Go. Các điểm đáng chú ý:
- **Quality gate** (8 rules, threshold 5) là content filtering layer đầu tiên
- **SimHash + Hamming distance** cho semantic deduplication (không chỉ exact hash)
- **Domain reputation tracking** tích lũy điểm theo thời gian
- **SSEHub** cung cấp real-time progress streaming — không polling, không WebSocket overhead
- **Anti-blocking** layers: UA rotation, proxy rotation, robots.txt respect, crawl delay

### 5. Bot Module là Polyglot Gateway

Bot là điểm entry cho users trên nhiều nền tảng:
- Discord (slash commands + webhooks + HMAC verification)
- Telegram (webhooks + Ed25519 verification)

Wizard system cho phép structured user flows (RSS add, account linking) với state persistence. Workflow engine mở rộng được cho automation sequences.

### 6. AI Integration với Cache-First Strategy

Gemini client sử dụng **dual-tier cache** (L1 in-memory + L2 Redis) để tránh:
- Rate limitExceeded errors
- Redundant API calls cho cùng content
- Latency cho repeated analyses

Content hashing → deterministic cache key → cache-aside pattern.

### 7. Observability là First-Class Citizen

- **OpenTelemetry**: W3C trace context propagation, spans for all DB/Redis/queue ops
- **Prometheus**: Purpose-built metric families (crawler-specific, notification-specific)
- **Health checks**: Component-level health với `/health` endpoint cho load balancer integration
- **zerolog structured logging**: Machine-parseable, zero allocation

### 8. Security Considerations

- **HMAC-SHA256** (Discord) và **Ed25519** (Telegram) cho webhook verification
- **Distributed lock** (Lua atomic) cho critical sections
- **Rate limiting** per-IP với token bucket
- **JWT Bearer token** validation middleware
- **Input validation** ở mọi controller entry point

### 9. Production-Ready Gaps

| Area | Gap |
|---|---|
| Testing | Không có test files trong codebase |
| Circuit breaker | Không có — cascade failure possible khi Redis/MongoDB down |
| Dead letter queue | Asynq errors logged nhưng không có DLQ tracking |
| Metrics export | Prometheuspushgateway not configured |
| Secrets management | Viper `ENV` support nhưng không có Vault/ASM integration |
| Horizontal scaling | SSEHub (in-memory) không shareable across instances |
| Connection pooling | pgx pool configured nhưng không có max lifetime / health check |
| Migration tooling | Không có flyway/ goose migration runner |

### 10. Extensibility Points

| Pattern | Implementation |
|---|---|
| New notification channel | Implement `Provider` interface + add to `eventTopics` map |
| New crawler step | Add func field to pipeline + call in `RunPipeline` |
| New bot platform | Implement `PlatformClient` interface + add to `BotController` switch |
| New AI provider | Swap `aiClient` in module setup (interface-based) |
| New wizard type | Add `Register<Name>Wizard()` + `detectWizardTemplate` case |
| New workflow action | Register `WorkflowStepHandler` in `WorkflowEngine` |

---

*Report generated by Claude Code Deep Analysis*
