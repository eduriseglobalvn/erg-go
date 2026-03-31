# Migration to Go — Task Status Tracker

> **Monorepo root**: `D:\ERG\go-erg\`
> **Last updated**: 2026-03-31
> **Router**: go-chi/chi/v5 (xuyên suốt toàn bộ)
> **Language**: Go 1.22+

---

## Tổng quan phases

| Phase | Service | Trạng thái | Agent |
|---|---|---|---|
| Phase 1 | Foundation (shared packages) | ✅ DONE | Agent #1 (đã xong) |
| Phase 2 | BOT Service | 🔄 IN PROGRESS | Agent #2 (đang chạy) |
| Phase 3 | Notification Service | 🔄 IN PROGRESS | Agent #3 (đang chạy) |
| Phase 4 | Crawler Service | 🔄 IN PROGRESS | Agent #4 (đang chạy) |
| Phase 5 | Trending Service | ⬜ NOT STARTED | Agent #5 |
| Phase 6 | Integration + Docs | ⬜ NOT STARTED | Agent #6 |

---

## ✅ PHASE 1 — Foundation (DONE)

### Cấu trúc đã tạo

```
D:\ERG\go-erg\
├── cmd/
│   ├── bot-service/main.go          ✅ (skeleton, cần expand trong Phase 2)
│   ├── notification-service/main.go ✅ (skeleton, cần expand trong Phase 3)
│   ├── crawler-service/main.go      ✅ (skeleton, cần expand trong Phase 4)
│   └── trending-service/main.go     ✅ (skeleton, cần expand trong Phase 5)
├── pkg/
│   ├── config/config.go             ✅ + config_test.go
│   ├── database/mongo.go            ✅ + mongo_test.go
│   ├── database/mysql.go            ✅ + mysql_test.go
│   ├── cache/redis.go               ✅ + redis_test.go
│   ├── queue/asynq.go               ✅ + asynq_test.go
│   ├── event/bus.go                ✅ + bus_test.go
│   ├── logger/log.go                ✅ + log_test.go
│   ├── http/
│   │   ├── server.go                 ✅
│   │   ├── client.go                 ✅
│   │   └── middleware/
│   │       ├── recovery.go           ✅
│   │       ├── requestid.go          ✅
│   │       ├── realip.go             ✅
│   │       ├── logger.go             ✅
│   │       ├── cors.go               ✅
│   │       ├── ratelimit.go          ✅
│   │       └── auth.go               ✅
│   ├── auth/jwt.go                   ✅ + jwt_test.go
│   ├── notification/interfaces.go    ✅ + notification_test.go
│   ├── scraper/
│   │   ├── fetcher.go               ✅
│   │   ├── parser.go                ✅
│   │   ├── robots.go                ✅
│   │   └── scraper_test.go          ✅
│   ├── dedup/simhash.go              ✅ + simhash_test.go
│   ├── ai/gemini.go                  ✅ + gemini_test.go
│   ├── rss/parser.go                 ✅ + parser_test.go
│   ├── sitemap/parser.go             ✅ + parser_test.go
│   └── telemetry/
│       ├── otel.go                   ✅
│       └── prometheus.go             ✅
├── migrations/
│   ├── 001_bot_indexes.go            ✅
│   ├── 002_notification_indexes.go   ✅
│   ├── 003_crawler_indexes.go        ✅
│   └── 004_trending_indexes.go       ✅
├── proto/events.proto                ✅
├── scripts/
│   ├── docker-build.sh              ✅
│   ├── db-migrate.sh                 ✅
│   └── run_migrations.go             ✅
├── Dockerfile.base                   ✅
├── docker-compose.yml                ✅
├── Makefile                          ✅
├── go.work                           ✅
├── go.mod (root + 4 service modules) ✅
├── README.md                          ✅
└── .github/workflows/
    ├── ci.yml                        ✅
    └── build-deploy.yml              ✅
```

### Đã xác nhận compile: ✅ `go build ./...` — zero errors

---

## 🔄 PHASE 2 — BOT Service (Agent #2 đang làm)

**Module**: `github.com/erg.ninja/bot-service`
**Port**: `:8081`
**go.mod tại**: `D:\ERG\go-erg\cmd\bot-service\go.mod`

### Đã làm

```
cmd/bot-service/main.go        ✅ skeleton đã tạo
cmd/bot-service/wire.go        ✅ đã tạo
```

### Cần hoàn thiện — DANH SÁCH CHI TIẾT

#### `internal/models/` (4 files — CHƯA TẠO)

```
internal/models/
├── bot_conversation.go     ❌ CHƯA — MongoDB model: user_id, platform, state, wizard_data, context, TTL 30 ngày
├── bot_linked_account.go   ❌ CHƯA — platform_user_id, internal_user_id, link_code, verified_at
├── bot_workflow.go         ❌ CHƯA — workflow_steps[], current_step, status, started_at/completed_at
└── bot_command.go          ❌ CHƯA — command registry entry (in-memory)
```

**bot_conversation.go** cần:
```go
type ConversationState string
const (
    StateActive   ConversationState = "active"
    StatePending  ConversationState = "pending"  // wizard đang chờ input
    StateCompleted ConversationState = "completed"
    StateExpired ConversationState = "expired"
)

type BotConversation struct {
    ID             primitive.ObjectID     `bson:"_id,omitempty"`
    UserID        string                 `bson:"user_id"`
    Platform      string                 `bson:"platform"` // discord, telegram
    PlatformConvID string               `bson:"platform_conv_id"`
    State         ConversationState      `bson:"state"`
    WizardStep    string                 `bson:"wizard_step,omitempty"`
    WizardData    map[string]string     `bson:"wizard_data,omitempty"`
    Context       map[string]interface{} `bson:"context,omitempty"`
    CreatedAt     time.Time             `bson:"created_at"`
    UpdatedAt     time.Time             `bson:"updated_at"`
    ExpiresAt     time.Time             `bson:"expires_at,omitempty"`
}
```

#### `internal/services/` (4 files — CHƯA TẠO)

```
internal/services/
├── command_handler.go    ❌ CHƯA — Route commands → handlers, RBAC permission check
├── conversation.go      ❌ CHƯA — Thread-safe wizard state machine (sync.RWMutex)
├── workflow.go          ❌ CHƯA — Step execution, resume-from-checkpoint, branching
└── link.go              ❌ CHƯA — 6-char code generation, Redis TTL 5 phút
```

**command_handler.go** cần:
- `PlatformUpdate` struct: Platform, UserID, ConversationID, MessageID, Command, Args, RawText, Timestamp
- `CommandService.Handle(ctx, update)` — lookup handler → check permission → execute → return string
- Map 36+ commands → handler functions

**conversation.go** cần:
- `sync.RWMutex` protecting `map[string]*WizardState`
- `AdvanceStep()` — validate → transition → persist MongoDB (upsert)
- `persistWizard()` + `LoadWizard()` — durability across restarts
- `renderPrompt()` — platform-specific message cho step hiện tại
- Wizard steps: confirm → input_url → input_category → confirm → done
- 5 phút TTL, extendable on activity

**link.go** cần:
- `CreateLinkCode(ctx, userID) → code`: charset `ABCDEFGHJKLMNPQRSTUVWXYZ23456789`, SETEX Redis 5min
- `VerifyLinkCode(ctx, code) → (userID, error)`: GET Redis → delete key → upsert BotLinkedAccount
- `GetLinkedAccounts(ctx, userID) → []*BotLinkedAccount`

#### `internal/commands/` (7 files — CHƯA TẠO)

```
internal/commands/
├── base.go              ❌ CHƯA — Command interface: Name(), Description(), Handle()
├── registry.go          ❌ CHƯA — command registry map: string → CommandHandler
├── rss_commands.go     ❌ CHƯA — /rss add, /rss list, /rss remove, /rss sync
├── crawl_commands.go   ❌ CHƯA — /crawl start, /crawl status, /crawl stop, /crawl batch
├── trending_commands.go ❌ CHƯA — /trending top, /trending keyword
├── draft_commands.go   ❌ CHƯA — /draft list, /draft publish, /draft delete
├── stats_commands.go   ❌ CHƯA — /stats users, /stats crawler, /stats queue
└── system_commands.go  ❌ CHƯA — /system health, /system ping, /system reload
```

**rss_commands.go** cần:
- `/rss add <url>`: validate URL → check robots.txt via pkg/scraper → add to MongoDB → send confirmation
- `/rss list`: query `rss_feeds` collection by user_id → format list
- `/rss remove <url>`: delete from MongoDB
- `/rss sync`: enqueue `crawl:refresh_feed` Asynq job

**crawl_commands.go** cần:
- `/crawl start <url>`: validate URL → enqueue `crawl:run` Asynq job → return job_id
- `/crawl status <job_id>`: check Asynq Redis job info
- `/crawl stop <job_id>`: cancel Asynq job if pending
- `/crawl batch <urls>`: enqueue multiple URLs

**trending_commands.go** cần:
- `/trending top`: HTTP call `http://trending-service:8084/trending/topics` → format response
- `/trending keyword <topic>`: HTTP call → topic detail → format response

**stats_commands.go** cần:
- `/stats users`: count users in MongoDB `users` collection
- `/stats crawler`: Redis LLEN on asynq queues
- `/stats queue`: Asynq server stats

**system_commands.go** cần:
- `/system health`: return health check từ healthz endpoint
- `/system ping`: return "pong"
- `/system reload` (admin only): reload config from file

#### `internal/handlers/` (4 files — CHƯA TẠO)

```
internal/handlers/
├── discord_webhook.go    ❌ CHƯA — POST /webhooks/discord (Ed25519 + HMAC-SHA256 verify)
├── telegram_webhook.go  ❌ CHƯA — POST /webhooks/telegram (HMAC-SHA256 verify)
├── bot_controller.go    ❌ CHƯA — REST API: conversations, link
└── health.go            ❌ CHƯA — GET /healthz, GET /ready
```

**discord_webhook.go** cần:
- Route: `POST /webhooks/discord`
- Parse Discord interaction body (json)
- Verify Ed25519: `X-Signature-Ed25519` + `X-Signature-Timestamp` + body
- Fallback HMAC-SHA256: `X-Hub-Signature-256 = "sha256=<hex>"`
- Route to command handler → send Discord API response

**telegram_webhook.go** cần:
- Route: `POST /webhooks/telegram`
- Parse Telegram Update (json)
- Verify HMAC-SHA256: data-check-string = sorted params + bot token
- Route to command handler → Telegram sendMessage API

**bot_controller.go** cần REST endpoints:
```
GET  /conversations
GET  /conversations/:id
POST /conversations/:id/send   (body: {message})
POST /link                      (body: {user_id})
GET  /link/:code
GET  /accounts
DELETE /accounts/:id
```

#### `internal/middleware/` (1 file — CHƯA TẠO)

```
internal/middleware/
└── permission.go  ❌ CHƯA — RBAC: viewer=1, editor=2, crawler=3, moderator=4, admin=5
```

Cần:
- `PermissionService.Check(ctx, userID, command) error`
- `commandPermissions` map: command → required level
- Fetch user from MongoDB → compare levels
- Middleware for protected REST routes

#### `internal/platform/` (2 files — CHƯA TẠO)

```
internal/platform/
├── discord.go    ❌ CHƯA — Discord API client: send DM, edit message, create embed
└── telegram.go   ❌ CHƯA — Telegram Bot API: sendMessage, editMessageText, answerCallbackQuery
```

#### `cmd/bot-service/main.go` — CẦN EXPAND

Hiện tại là skeleton. Cần thêm:
- Full middleware chain: Recovery → RequestID → RealIP → Logger → CORS → RateLimit → Auth
- Mount webhook routes WITHOUT auth (HMAC verification thay thế)
- Mount REST API routes WITH auth
- Asynq client khởi tạo + job enqueue helpers
- Start EventBus subscriber (subscribe `events:rss.added`)

---

## 🔄 PHASE 3 — Notification Service (Agent #3 đang làm)

**Module**: `github.com/erg.ninja/notification-service`
**Port**: `:8082`
**go.mod tại**: `D:\ERG\go-erg\cmd\notification-service\go.mod`

### Đã làm

```
cmd/notification-service/main.go  ✅ skeleton đã tạo
```

### Cần hoàn thiện — DANH SÁCH CHI TIẾT

#### `internal/models/` (3 files — CHƯA TẠO)

```
internal/models/
├── notification.go            ❌ CHƯA
├── notification_preference.go ❌ CHƯA
└── notification_template.go  ❌ CHƯA — Vietnamese templates (Full implementation)
```

**notification.go** cần:
```go
type NotificationStatus string
const (
    NotifStatusPending   NotificationStatus = "pending"
    NotifStatusSending   NotificationStatus = "sending"
    NotifStatusSent      NotificationStatus = "sent"
    NotifStatusFailed    NotificationStatus = "failed"
    NotifStatusCancelled NotificationStatus = "cancelled"
)

type Notification struct {
    ID            primitive.ObjectID           `bson:"_id,omitempty"`
    RecipientID  string                       `bson:"recipient_id"`
    Channel      notification.ChannelType     `bson:"channel"`
    Template     string                       `bson:"template"`
    TemplateData map[string]string            `bson:"template_data,omitempty"`
    Body         string                       `bson:"body,omitempty"`
    Subject      string                       `bson:"subject,omitempty"`
    Status       NotificationStatus           `bson:"status"`
    Priority     int                          `bson:"priority"` // 1=high, 5=low
    ScheduledAt  time.Time                    `bson:"scheduled_at,omitempty"`
    SentAt       time.Time                    `bson:"sent_at,omitempty"`
    RetryCount   int                          `bson:"retry_count"`
    MaxRetries   int                          `bson:"max_retries"`
    LastError    string                       `bson:"last_error,omitempty"`
    ProviderRef  string                       `bson:"provider_ref,omitempty"`
    CreatedAt    time.Time                    `bson:"created_at"`
    UpdatedAt    time.Time                    `bson:"updated_at"`
}
```

**notification_preference.go** cần:
```go
type DigestFrequency string
const (DigestNone DigestFrequency = "none"; DigestDaily DigestFrequency = "daily"; DigestWeekly DigestFrequency = "weekly"; DigestMonthly DigestFrequency = "monthly")

type NotificationPreference struct {
    ID         primitive.ObjectID `bson:"_id,omitempty"`
    UserID     string             `bson:"user_id"`
    Discord    bool               `bson:"discord"`
    Telegram   bool               `bson:"telegram"`
    WhatsApp   bool               `bson:"whatsapp"`
    Email      bool               `bson:"email"`
    MuteUntil  time.Time         `bson:"mute_until,omitempty"`
    DigestFreq DigestFrequency   `bson:"digest_freq"`
    CreatedAt  time.Time         `bson:"created_at"`
    UpdatedAt  time.Time         `bson:"updated_at"`
}
```

**notification_template.go** — Full Vietnamese templates:
```go
FormatCrawlSuccess  = `🎉 Crawl thành công!\n📰 {{.Title}}\n🌐 {{.URL}}\n⏱ Thời gian: {{.Duration}}`
FormatCrawlFailed   = `❌ Crawl thất bại!\n🌐 {{.URL}}\n⚠️ Lý do: {{.Error}}\n🔄 Thử lại: {{.RetryAt}}`
FormatHotTopicAlert = `🔥 Topic hot: {{.Topic}}\n📊 Volume: {{.Volume}}\n🔗 {{.URL}}`
FormatDailyDigest  = `📬 Daily Digest — {{.Date}}\nBài viết nổi bật trong ngày:\n{{range .Items}}• {{.}}\n{{end}}\nXem thêm: {{.DashboardURL}}`
FormatSystemAlert   = `⚠️ {{.AlertType}}\n{{.Message}}\n🕐 {{.Timestamp}}`
FormatQueueStatus   = `📊 Queue Status\n⏳ Depth: {{.Depth}}\n⚡ Processing: {{.ProcessingRate}}/min\n❌ Errors: {{.ErrorRate}}/min`
FormatRssAdded       = `✅ RSS đã thêm!\n📡 {{.FeedName}}\n🔗 {{.FeedURL}}`
```

#### `internal/services/` (4 files — CHƯA TẠO)

```
internal/services/
├── notification.go       ❌ CHƯA — Send, BatchSend, Cancel, Resend
├── template_renderer.go ❌ CHƯA — Go text/template Vietnamese interpolation
├── digest.go           ❌ CHƯA — Daily (8AM Asia/Ho_Chi_Minh), Weekly (Sun 9AM), Monthly (1st 10AM)
└── delivery_tracker.go ❌ CHƯA — Retry logic, exponential backoff, delivery receipts
```

**notification.go** cần:
- `Send(ctx, msg)` → render template → find provider → check rate limit → send → track
- `BatchSend(ctx, msgs)` → concurrent goroutines + WaitGroup
- `Cancel(ctx, id)` → update status to cancelled in MongoDB
- `Resend(ctx, id)` → reset status, re-enqueue Asynq job

**template_renderer.go** cần:
- `Render(templateName string, data map[string]string) (string, error)`
- Lookup template by name from `notification_template.go`
- Parse with `text/template`
- Execute with data map
- Return rendered string

**digest.go** cần:
- `DigestScheduler` với cron: `0 8 * * *` (daily Vietnam), `0 9 * * 0` (weekly), `0 10 1 * *` (monthly)
- Aggregate pending notifications into digest batches
- Render digest templates
- Enqueue aggregated notifications

**delivery_tracker.go** cần:
- `DeliveryReceipt` struct: notification_id, provider, status, sent_at, latency_ms, error
- `Track(receipt)` → insert to MongoDB
- `ShouldRetry(err)` → classify retryable vs permanent errors
- `BackoffDuration(retryCount)` → 1s, 2s, 4s, 8s, 16s, max 5min

#### `internal/providers/` (7 files — CHƯA TẠO)

```
internal/providers/
├── discord.go    ❌ CHƯA — Discord webhook embeds, rate limit 200/min
├── telegram.go   ❌ CHƯA — Telegram Bot API, 30 msg/sec, editMessage
├── whatsapp.go  ❌ CHƯA — WhatsApp Business API, template messages
├── email.go      ❌ CHƯA — SMTP, HTML+plain text multipart
├── slack.go      ❌ CHƯA — Slack webhook, Block kit
├── sms.go        ❌ CHƯA — SMS interface + mock implementation
└── notifier.go  ❌ CHƯA — Provider registry + dispatch logic
```

**discord.go** cần:
- `DiscordProvider` struct implements `NotifierProvider`
- `Send(ctx, msg)` → Discord webhook embeds (max 6000 chars, split if needed)
- Token bucket rate limiter: 200 req/min
- Respect Retry-After on 429
- Fields: title, url, description, color, timestamp in embed

**telegram.go** cần:
- `TelegramProvider` struct implements `NotifierProvider`
- `Send(ctx, msg)` → sendMessage with parseMode=MarkdownV2 or HTML
- Max 4096 chars, support reply_markup for inline buttons
- 30 msg/sec rate limit
- `editMessageText` for updating existing messages

**whatsapp.go** cần:
- `WhatsAppProvider` implements `NotifierProvider`
- Template messages (required for non-initiated conversations)
- Support text and image templates
- Mark as read webhook handler

**email.go** cần:
- `EmailProvider` implements `NotifierProvider`
- SMTP connection via `net/smtp`
- HTML + plain text multipart (use `crypto/tls` for TLS)
- Configurable: host, port, username, password, from address
- DKIM signing (optional, comment-based for MVP)

**notifier.go** cần:
- `NotifierRegistry` struct: []NotifierProvider
- `Dispatch(ctx, msg)` → find provider by `Supports(channel)`, call `Send()`
- Register all providers in constructor

#### `internal/event/consumer.go` — CHƯA TẠO

Subscribe to Redis pub/sub topics:
- `events:crawl.success`
- `events:crawl.failed`
- `events:trending.alert`
- `events:system.warning`
- `events:queue.status`
- `events:rss.added`

On event:
1. Parse event envelope `{event_type, source_service, payload, timestamp}`
2. Map `event_type` → template + data
3. Lookup user preferences from MongoDB
4. Build `Notification` per user/channel
5. Enqueue to Asynq with priority

#### `internal/queue/send_notification_job.go` — CHƯA TẠO

- Asynq job type: `"notification:send"`
- Priority queue: `"high"` (1-3), `"default"` (4-6), `"low"` (7-10)
- Max retries: 5
- Timeout: 30s
- Dead letter queue: `"notification:dlq"`
- Idempotency key: `notification_id` (prevent duplicate send on retry)
- On failure: increment retry count, schedule with backoff

#### `internal/handlers/` (4 files — CHƯA TẠO)

```
internal/handlers/
├── notification_controller.go ❌ CHƯA — Main REST API
├── channel_controller.go     ❌ CHƯA — Connect/disconnect channels, test webhooks
├── webhook_controller.go     ❌ CHƯA — Incoming webhooks from providers
└── health.go                 ❌ CHƯA — GET /healthz, GET /ready
```

REST API endpoints:
```
GET  /notifications                    (paginated: page, limit, status, channel, user_id)
GET  /notifications/:id
POST /notifications/send              (body: {recipient_id, channel, template, data})
POST /notifications/batch              (body: {notifications: []})
GET  /notifications/preferences       (query: user_id)
PUT  /notifications/preferences       (body: preference update)
POST /notifications/:id/cancel
POST /notifications/:id/resend
GET  /notifications/stats             (sent, failed, pending counts)
POST /channels/discord/test          (body: {webhook_url, message})
POST /channels/telegram/test         (body: {chat_id, message})
POST /channels/whatsapp/test          (body: {phone, template})
POST /channels/email/test            (body: {to, subject, body})
GET  /channels/status                 (all channel connection status)
```

---

## 🔄 PHASE 4 — Crawler Service (Agent #4 đang làm)

**Module**: `github.com/erg.ninja/crawler-service`
**Port**: `:8083`
**go.mod tại**: `D:\ERG\go-erg\cmd\crawler-service\go.mod`

### Đã làm

```
cmd/crawler-service/main.go  ✅ skeleton đã tạo
pkg/scraper/fetcher.go      ✅ (foundation)
pkg/scraper/robots.go       ✅ (foundation)
pkg/scraper/parser.go       ✅ (foundation)
pkg/dedup/simhash.go        ✅ (foundation)
```

### Cần hoàn thiện — DANH SÁCH CHI TIẾT

#### `cmd/crawler-service/main.go` — CẦN EXPAND LỚN

Cần thêm:
- Asynq worker pool khởi tạo (20 workers, configurable)
- SSE broadcaster khởi tạo
- Asynq handler registration
- EventBus publisher khởi tạo
- Feed scheduler cron khởi tạo
- All routes mount

#### `cmd/crawler-service/wire.go` — CHƯA TẠO

#### `internal/models/` (6 files — CHƯA TẠO)

```
internal/models/
├── rss_feed.go              ❌ CHƯA — URL, title, category, language, frequency, last_fetch, ETag, LastModified, status
├── scraper_config.go        ❌ CHƯA — domain → CSS selectors, requires_js, use_smart_selector
├── crawl_history.go         ❌ CHƯA — url, status, score, quality_pass, duration_ms, items, job_id, fingerprint
├── domain_reputation.go     ❌ CHƯA — domain, success_rate, block_count, last_seen, proxies_used
├── content_fingerprint.go   ❌ CHƯA — url, simhash, simhash_bucket, sha256[32], created_at
└── content_blacklist.go    ❌ CHƯA — type (url/domain/keyword), pattern, reason, active, added_by/at
```

#### `internal/services/` (9 files — CHƯA TẠO)

```
internal/services/
├── orchestrator.go     ❌ CHƯA — Main crawl pipeline coordinator (orchestrates all steps)
├── feed_fetcher.go    ❌ CHƯA — Concurrent RSS/Atom/JSON fetch, ETag/Last-Modified
├── scraper.go         ❌ CHƯA — HTML fetch: robots → proxy → UA → delay → fetch
├── anti_block.go      ❌ CHƯA — Proxy pool, UA rotation, adaptive delay (3s min), backoff
├── robots_parser.go   ❌ CHƯA — robots.txt respect (crawl-delay, allow/disallow)
├── quality_gate.go    ❌ CHƯA — 8-rule scoring (≥70 = publishable), Full implementation
├── content_dedup.go   ❌ CHƯA — SimHash + SHA-256 dedup wrapper (pkg/dedup đã có)
├── smart_selector.go  ❌ CHƯA — Gemini AI → CSS selector suggestions, domain caching
├── sitemap.go         ❌ CHƯA — Sitemap discovery (robots.txt + standard paths), recursive parse
└── blacklist.go       ❌ CHƯA — Aho-Corasick keyword matching, URL/domain/keyword checking
```

**orchestrator.go** — QUAN TRỌNG NHẤT. Cần implement full pipeline:
```go
func (o *CrawlerService) RunCrawl(ctx context.Context, payload *jobs.CrawlJobPayload) error {
    // 1. URL Discovery (input is already URL)
    // 2. Blacklist Check → if blocked, return ErrBlacklisted
    // 3. Domain Reputation → if block_count > 10, skip
    // 4. Robots.txt Check → if disallowed, return ErrRobotsDisallowed
    // 5. Anti-Block → adaptive delay, proxy rotation, UA cycling
    // 6. Fetch Content → ScraperService
    // 7. Quality Gate → if score < 70, return ErrQualityTooLow
    // 8. Content Dedup → if duplicate, return ErrDuplicate
    // 9. AI SEO (parallel goroutine) → SmartSelector + Gemini
    // 10. Save to MongoDB (CrawlHistory + Fingerprint)
    // 11. Publish "crawl.success" or "crawl.failed" event
    // 12. SSE Broadcast progress
}
```

**quality_gate.go** — Full 8-rule implementation:
```go
// mỗi rule đóng góp tối đa 12.5 điểm → total 0-100
// Threshold: ≥70 = publishable

type QualityScore struct {
    Total       float64
    Length      float64  // ≥500 words = 12.5, <200 = 0
    Originality float64  // keyword stuffing check, AI-detect >0.7 = 0
    Freshness   float64  // ≤30 days = 12.5, >90 = 0
    Readability float64  // Flesch reading ease ≥60 = 12.5
    Media       float64  // has image = 6, has alt = 6.5
    Structure   float64  // has H1-H6 = 7.5, has lists = 5
    SpamSignals float64  // spam hit = −12.5, ads-heavy = −12.5
}

func (q *QualityGate) Score(ctx context.Context, html, url string) (*QualityScore, error)
func (q *QualityGate) ShouldPublish(score *QualityScore) bool
```

**blacklist.go** — Aho-Corasick implementation:
```go
// Build automaton from keyword blacklist (load on startup, rebuild on change)
// Match URL + page text against automaton
// O(n) matching
type BlacklistChecker struct {
    automaton *ahocorasick.Automaton  // keyword trie
    domains   map[string]bool          // exact domain matches
    urls      map[string]bool          // exact URL matches
    mu        sync.RWMutex
}

func (c *BlacklistChecker) IsAllowed(url, domain, text string) (bool, string)
// Returns (allowed, reason_if_blocked)
```

**smart_selector.go**:
```go
// Gemini AI → CSS selector suggestions
// Cache per domain: in-memory LRU (100 entries) + Redis (1 hour TTL)
// Batch up to 10 URLs per Gemini call
// Fallback to goquery heuristics if AI unavailable
// Suggests: title_selector, body_selector, author_selector, date_selector, image_selector
```

**feed_fetcher.go**:
```go
// Fetch concurrently: RSS 2.0, Atom 1.0, JSON Feed 1.1
// ETag/Last-Modified support: if unchanged, skip parsing
// Respect feed.frequency (don't re-fetch if not due)
// Parse → []RssItem{title, url, description, pub_date, author, categories}
```

#### `internal/jobs/` (4 files — CHƯA TẠO)

```
internal/jobs/
├── crawl_job.go        ❌ CHƯA — CrawlJobPayload + Asynq handler
├── refresh_feed_job.go ❌ CHƯA — RefreshFeedJobPayload + Asynq handler
├── reindex_job.go      ❌ CHƯA — ReindexJobPayload + Asynq handler
└── job_handlers.go     ❌ CHƯA — Asynq handler registration + worker pool
```

#### `internal/handlers/` (5 files — CHƯA TẠO)

```
internal/handlers/
├── crawler_controller.go  ❌ CHƯA — Main crawl REST API
├── blacklist_controller.go ❌ CHƯA — Blacklist CRUD
├── feed_controller.go    ❌ CHƯA — RSS feed CRUD
├── sse_gateway.go        ❌ CHƯA — GET /crawl/stream/:job_id SSE endpoint
└── health.go              ❌ CHƯA — GET /healthz, GET /ready
```

**crawler_controller.go** endpoints:
```
GET  /crawl/stats
GET  /crawl/ai-quota
GET  /crawl/quality-stats
GET  /crawl/dedup-stats
GET  /crawl/history             (paginated)
POST /crawl/url                 (body: {url, priority, source})
POST /crawl/batch               (body: {urls: [], priority})
```

**feed_controller.go** endpoints:
```
GET  /rss/feeds                 (paginated)
POST /rss/feeds                 (body: {url, category, language, frequency})
PUT  /rss/feeds/:id
DELETE /rss/feeds/:id
POST /rss/sync                  (body: {feed_id})
GET  /rss/preview/:url
```

**blacklist_controller.go** endpoints:
```
GET  /blacklist                 (paginated)
POST /blacklist                 (body: {type, pattern, reason})
DELETE /blacklist/:id
```

**sse_gateway.go**:
```go
// GET /crawl/stream/:job_id
// - SSE content-type: text/event-stream
// - Header: X-Accel-Buffering: no (disable nginx buffering)
// - Register client to Broadcaster
// - Send initial state
// - Stream events as JSON: {job_id, url, status, progress_pct, items_discovered, items_scraped, errors[], timestamp}
// - Heartbeat: ping comment (: ping\n\n) every 30s
// - Close channel on client disconnect
```

#### `internal/sse/broadcaster.go` — CHƯA TẠO

```go
// Thread-safe SSE connection manager
type ClientRegistration struct {
    JobID   string
    Channel chan SSEEvent
}

type SSEEvent struct {
    JobID   string `json:"job_id"`
    URL     string `json:"url,omitempty"`
    Status  string `json:"status,omitempty"`
    Progress int   `json:"progress_pct,omitempty"`
    ItemsDiscovered int `json:"items_discovered,omitempty"`
    ItemsScraped int   `json:"items_scraped,omitempty"`
    Error   string `json:"error,omitempty"`
    Timestamp string `json:"timestamp"`
}

type Broadcaster struct {
    clients      map[string]map[chan SSEEvent]struct{} // jobID → channels
    register     chan *ClientRegistration
    unregister   chan *ClientUnregistration
    broadcast    chan *SSEEvent
    done         chan struct{}
    mu           sync.RWMutex
}

func NewBroadcaster() *Broadcaster
func (b *Broadcaster) Start()
func (b *Broadcaster) Stop()
func (b *Broadcaster) Register(jobID string) <-chan SSEEvent
func (b *Broadcaster) Unregister(jobID string, ch chan SSEEvent)
func (b *Broadcaster) Broadcast(jobID string, event SSEEvent)
// Max 10,000 concurrent connections (reject with 503 if exceeded)
```

#### `internal/asynq/worker.go` — CHƯA TẠO

```go
// Asynq server setup
// Worker pool: 20 workers (WORKER_COUNT env, default 20)
// Queues: "high" (priority 1-3), "default" (4-6), "low" (7-10)
// Register handlers for: crawl:run, crawl:refresh_feed, crawl:reindex
// Graceful shutdown: receive signal → stop accepting new tasks → wait for in-flight (up to 5min)
```

---

## ⬜ PHASE 5 — Trending Service (Agent #5)

**Module**: `github.com/erg.ninja/trending-service`
**Port**: `:8084`
**go.mod tại**: `D:\ERG\go-erg\cmd\trending-service\go.mod`

### Đã làm

```
cmd/trending-service/main.go  ✅ skeleton đã tạo (chỉ có health endpoint)
```

### Cần hoàn thiện — DANH SÁCH CHI TIẾT

#### `cmd/trending-service/wire.go` — CHƯA TẠO

#### `internal/models/` (3 files — CHƯA TẠO)

```
internal/models/
├── trending_topic.go     ❌ CHƯA — topic, score, volume, source, keywords[], timestamp
├── news_article.go       ❌ CHƯA — headline, source, url, published_at, relevance_score
└── trending_snapshot.go  ❌ CHƯA — Point-in-time snapshot for historical trend charts
```

**trending_topic.go**:
```go
type TrendingTopic struct {
    ID           primitive.ObjectID `bson:"_id,omitempty"`
    Topic        string             `bson:"topic"`
    Score        float64            `bson:"score"`
    Volume       int                `bson:"volume"` // search volume
    Source       string             `bson:"source"` // google_trends, news_api
    Keywords     []string           `bson:"keywords"`
    URLs         []string           `bson:"urls,omitempty"` // discovered article URLs
    Country      string             `bson:"country"`
    Language     string             `bson:"language"`
    CreatedAt    time.Time         `bson:"created_at"`
    UpdatedAt    time.Time         `bson:"updated_at"`
}
```

#### `internal/services/` (4 files — CHƯA TẠO)

```
internal/services/
├── google_trends.go    ❌ CHƯA — Google Trends API (serpapi or scrape fallback)
├── news_api.go         ❌ CHƯA — NewsAPI.org integration
├── aggregator.go       ❌ CHƯA — Merge + rank + dedupe results from all sources
└── scheduler.go       ❌ CHƯA — robfig/cron v3: every 30 min
```

**google_trends.go**:
- HTTP GET to SerpAPI or Google Trends scrape
- Parse trending topics: topic name, volume, related queries
- Cache responses in Redis (25 min TTL)
- Fallback gracefully if API unavailable

**news_api.go**:
- HTTP GET to NewsAPI.org `/top-headlines` and `/everything`
- Parse response: headline, source, url, published_at
- Cache in Redis (25 min TTL)
- Handle NewsAPI 100 req/day limit: aggressive caching, batch lookups

**aggregator.go**:
```go
type TrendingAggregator struct {
    trendsAPI  *GoogleTrendsService
    newsAPI    *NewsApiService
    mongo      MongoDB
    redis      RedisCache
}

func (a *TrendingAggregator) Refresh(ctx context.Context) ([]*TrendingTopic, error) {
    // Concurrent fetch: goroutine for Google Trends + goroutine for NewsAPI
    // Merge results
    // Deduplicate by topic name
    // Rank by score (weighted: Trends volume + News volume)
    // Take top 20
    // Store in MongoDB trending_topics collection
    // Push discovered URLs to Redis: LPUSH trending:urls → LTRIM to 10,000
}

func (a *TrendingAggregator) GetTop(ctx context.Context, limit int) ([]*TrendingTopic, error)
func (a *TrendingAggregator) GetTopic(ctx context.Context, topic string) (*TrendingTopic, []string, error)
```

**scheduler.go**:
```go
func (s *Scheduler) Start() {
    // Every 30 minutes
    s.cron.AddFunc("*/30 * * * *", func() {
        ctx := context.Background()
        topics, err := s.aggregator.Refresh(ctx)
        if err != nil {
            slog.Error("trending refresh failed", "err", err)
            return
        }
        // Push URLs to Redis
        for _, url := range extractURLs(topics) {
            s.redis.LPush(ctx, "trending:urls", url)
        }
        s.redis.LTrim(ctx, "trending:urls", 0, 9999)
    })
}
```

#### `internal/cache/redis_cache.go` — CHƯA TẠO

```go
// Redis cache with 25-min TTL per data source
type TrendingCache struct {
    redis RedisCache
}

func (c *TrendingCache) Get(key string) ([]byte, error)
func (c *TrendingCache) Set(key string, val []byte, ttl time.Duration) error
// Keys: trending:topics:vietnam, trending:news:vietnam, trending:feeds
```

#### `internal/handlers/` (3 files — CHƯA TẠO)

```
internal/handlers/
├── trending_controller.go ❌ CHƯA — Main REST API
├── feed_controller.go   ❌ CHƯA — URL discovery feed (for crawler-service)
└── health.go             ❌ CHƯA — GET /healthz, GET /ready (báo last_refresh timestamp)
```

**trending_controller.go** endpoints:
```
GET  /trending/topics           → Top 20 (cached, < 200ms response)
GET  /trending/topics/:topic   → Topic detail + related keywords + timeline
GET  /trending/news            → Latest news articles (query: topic, country, limit)
GET  /trending/history         → Historical snapshots (query: topic, from, to)
GET  /trending/sources         → Google Trends + NewsAPI health status
POST /trending/refresh          → Admin: trigger immediate full refresh (auth required)
```

**feed_controller.go** — CRITICAL inter-service contract:
```
GET  /trending/feeds           → URL discovery feed (crawler-service polls this every 5 min)
                            Query params: ?since=<timestamp>&limit=100
                            Returns: {urls: [...], count: N}
```
Crawler-service calls: `LRANGE trending:urls 0 99` → `LTRIM trending:urls 100 -1`
Fallback: HTTP call to this endpoint if Redis unavailable

**health.go** — Special: báo cả `last_refresh` timestamp:
```json
{"status":"ok","mongo":"ok","redis":"ok","last_refresh":"2026-03-31T07:00:00Z","sources":{"google_trends":"ok","news_api":"ok"}}
```

#### `cmd/trending-service/main.go` — CẦN EXPAND

Cần thêm:
- TrendingAggregator khởi tạo
- Scheduler khởi tạo + Start()
- TrendingCache khởi tạo
- All routes mount
- Graceful shutdown

---

## ⬜ PHASE 6 — Integration & Docs (Agent #6)

### Cần làm

#### `integration/` directory

```
integration/
├── api_gateway/
│   └── nginx.conf            ❌ CHƯA — Nginx reverse proxy routing (see plan section 9)
├── e2e/
│   └── e2e_test.go           ❌ CHƯA — Full end-to-end test: trending → crawler → notification
└── benchmarks/
    └── bench_test.go         ❌ CHƯA — Performance comparison: NestJS vs Go services
```

**nginx.conf** cần:
```nginx
# API Gateway routing
location /api/bot/ {
    proxy_pass http://bot-service:8081/;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
}

location /api/notifications/ {
    proxy_pass http://notification-service:8082/;
}

location /api/crawler/ {
    proxy_pass http://crawler-service:8083/;
}

location /api/trending/ {
    proxy_pass http://trending-service:8084/;
}
# Rate limiting, TLS termination, logging
```

**e2e_test.go**:
```go
// Test: trending → crawler → notification delivery
// 1. Trigger trending refresh → get URLs
// 2. Enqueue crawl job → wait for completion
// 3. Assert crawl_history has entry with quality_pass=true
// 4. Assert notification was sent (check notification_history)
// 5. Run on every PR via GitHub Actions
```

#### `docs/` directory

```
docs/
├── architecture.md           ❌ CHƯA — Service diagrams, data flow, inter-service contracts
├── api/
│   ├── bot-service.md       ❌ CHƯA — OpenAPI spec
│   ├── notification-service.md ❌ CHƯA — OpenAPI spec
│   ├── crawler-service.md   ❌ CHƯA — OpenAPI spec
│   └── trending-service.md  ❌ CHƯA — OpenAPI spec
└── runbook.md               ❌ CHƯA — Deployment, rollback, alerting procedures
```

#### NestJS Decommission Checklist (docs section)

```
docs/
└── decommission.md           ❌ CHƯA — NestJS monolith decommission checklist
```

Checklist cần:
- [ ] All 4 modules removed from NestJS source tree
- [ ] NestJS only runs auth gateway + admin dashboard
- [ ] All env vars/secrets migrated to per-service .env
- [ ] Old NestJS Docker image stopped in production
- [ ] Old NestJS deployment manifests removed from Kubernetes/Helm

#### ADR (Architecture Decision Records)

```
docs/adr/
├── adr-001-go-monorepo-structure.md    ❌ CHƯA
├── adr-002-shared-packages.md           ❌ CHƯA
├── adr-003-mongodb-primary-store.md    ❌ CHƯA
├── adr-004-strangler-fig-pattern.md    ❌ CHƯA
├── adr-005-chi-router-choice.md       ❌ CHƯA
├── adr-006-asynq-over-redis-streams.md ❌ CHƯA
└── adr-007-performance-benchmarks.md   ❌ CHƯA — sau khi chạy benchmark Phase 6
```

---

## ⚠️ CRITICAL NOTES CHO TẤT CẢ AGENTS

1. **Router**: LUÔN dùng `go-chi/chi/v5` cho tất cả HTTP routing — KHÔNG dùng Gin, Echo, Fiber
2. **Error wrapping**: `fmt.Errorf("ServiceName.Method: %w", err)` tại mọi boundary
3. **Context**: Tất cả operations phải có context với timeout
4. **Graceful shutdown**: Tất cả servers phải handle SIGINT/SIGTERM với timeout
5. **Structured logging**: zerolog với JSON output, always include: service, level, message, time, caller
6. **Functional options**: Constructor pattern cho tất cả services
7. **Interface segregation**: KHÔNG pass concrete types vào constructors, chỉ interfaces
8. **No hardcoded secrets**: Tất cả API keys, tokens từ config/env
9. **Build verification**: Sau khi xong, chạy `go build ./...` trong `D:\ERG\go-erg\` và fix tất cả lỗi
10. **Test**: Chạy `go test ./...` và fix test failures

---

## Thứ tự ưu tiên nếu chỉ có 1 agent

Nếu chỉ có 1 agent làm Phase 5 + 6:
1. Trending Service (Phase 5) — INTER-SERVICE CONTRACT quan trọng nhất (crawler-service PHỤ THUỘC vào trending feeds)
2. Trending handlers + feed_controller.go (INTER-SERVICE CONTRACT endpoint)
3. Integration docs
4. E2E tests
5. OpenAPI specs
6. ADRs
