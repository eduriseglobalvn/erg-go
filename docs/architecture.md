# Architecture — erg-go (Shared Microservice Library)

> **Status**: Transformed from single binary monolith → config-driven, multi-tenant, service-discoverable shared library
> **Build**: `go build ./...` | `go build ./cmd/server` (standalone) | `go build ./lib/...` (library only)
> **Router**: `go-chi/chi/v5` for HTTP | `google.golang.org/grpc` for RPC

---

## System Overview

```
┌──────────────────────────────────────────────────────────────────────┐
│                        erg-go Architecture                           │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │                    Consumer Services                          │    │
│  │  erg-backend (NestJS)  │  Future Go microservices  │  ...   │    │
│  └──────────────────────────┬───────────────────────────────┬─────┘    │
│                             │                               │           │
│                   ┌─────────▼──────────┐         ┌─────────▼───────┐  │
│                   │  lib/crawler/v1   │         │ lib/bot/v1      │  │
│                   │  lib/notification/v1│        │ lib/trending/v1 │  │
│                   │  (gRPC clients)     │         │ (gRPC clients)  │  │
│                   └─────────┬──────────┘         └─────────┬───────┘  │
│                             │                               │           │
│         ┌───────────────────┴───────────────────────────────┘           │
│         │                    lib/ (v1 stable API surface)                │
│         │         Client factories, service stubs, transport             │
│         └─────────────────────────┬──────────────────────────────────┘ │
│                                   │                                        │
│  ┌────────────────────────────────▼──────────────────────────────────┐  │
│  │                    erg-server (standalone binary)                   │  │
│  │                    cmd/server + lib/* + internal/*                │  │
│  │                                                                      │  │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐         │  │
│  │  │  BOT     │  │NOTIFICA- │  │ CRAWLER  │  │TRENDING │         │  │
│  │  │  Module  │  │TIONS Mod │  │  Module  │  │  Module  │         │  │
│  │  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘         │  │
│  │       │             │             │             │                 │  │
│  │       └─────────────┴──────┬──────┴─────────────┘                 │  │
│  │                            │                                       │  │
│  │              ┌─────────────▼───────────┐                          │  │
│  │              │     Event Bus           │                          │  │
│  │              │ (in-process + Redis)   │                          │  │
│  │              └─────────────┬───────────┘                          │  │
│  │   ┌────────────────────────┼────────────────────────────┐         │  │
│  │   │                        │                            │         │  │
│  │   ▼                        ▼                            ▼         │  │
│  │ ┌──────────┐  ┌──────────┐  │  ┌──────────┐  ┌────────┐       │  │
│  │ │  MongoDB  │  │  Redis   │  │  │  Asynq   │  │ pkg/* │       │  │
│  │ │(documents)│  │(cache+bus)│  │  │ (jobs)   │  │(shared)│       │  │
│  │ └──────────┘  └──────────┘     └──────────┘  └────────┘       │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                    pkg/ (PUBLIC API — importable)               │  │
│  │                                                                      │  │
│  │  tenant/          — Tenant context, middleware, isolation layers  │  │
│  │  discovery/       — Service registry: Consul, DNS, Static catalog  │  │
│  │  plugin/         — Module plugin loader (build tags + runtime)  │  │
│  │  compose/        — Service manifest loader, dependency resolver    │  │
│  │  errors/         — 40+ error codes, gRPC status + HTTP mapper  │  │
│  │  config/         — Viper YAML/ENV config                         │  │
│  │  database/       — MongoDB + MySQL (pgx v5)                   │  │
│  │  cache/         — Redis client                                 │  │
│  │  queue/          — Asynq client + server                        │  │
│  │  event/         — In-process + Redis pub/sub event bus         │  │
│  │  logger/        — zerolog structured logging                   │  │
│  │  http/           — HTTP client, server, interceptors            │  │
│  │  auth/          — JWT validation                               │  │
│  │  scraper/       — Fetcher + robots.txt parser                  │  │
│  │  dedup/         — SimHash deduplication                        │  │
│  │  ai/             — Gemini AI integration                       │  │
│  │  rss/            — RSS/Atom parser                             │  │
│  │  sitemap/        — Sitemap parser                               │  │
│  │  telemetry/       — OpenTelemetry + Prometheus                  │  │
│  └──────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────┘
```

---

## Directory Structure

```
erg-go/
├── cmd/server/          # Standalone server entry point (wires all modules)
├── cmd/plugin-server/  # Runtime plugin server (loads .so modules dynamically)
├── internal/
│   ├── routes/          # HTTP route registration
│   └── modules/        # NestJS-style modules (deprecated, migrating to lib/)
│       ├── bot/
│       ├── notifications/
│       ├── crawler/
│       └── trending/
├── lib/                ← PUBLIC: gRPC service libraries (v1, semver)
│   ├── crawler/v1/     # CrawlerService gRPC client + server stub
│   ├── bot/v1/         # BotService gRPC client + server stub
│   ├── notification/v1/ # NotificationService gRPC client + stub
│   └── trending/v1/   # TrendingService gRPC client + server stub
├── proto/              ← SOURCE: proto definitions
│   ├── lib/crawler/v1/
│   ├── lib/bot/v1/
│   ├── lib/notification/v1/
│   ├── lib/trending/v1/
│   └── events.proto
└── pkg/               ← PUBLIC: shared infrastructure packages
    ├── tenant/         # Multi-tenant isolation
    ├── discovery/      # Service discovery
    ├── plugin/         # Plugin architecture
    ├── compose/        # Config-driven composition
    ├── errors/         # Structured error codes
    ├── config/
    ├── database/
    ├── cache/
    ├── queue/
    ├── event/
    ├── logger/
    ├── http/
    ├── auth/
    ├── scraper/
    ├── dedup/
    ├── ai/
    ├── rss/
    ├── sitemap/
    └── telemetry/
```

---

## Module Communication

### 1. gRPC (primary — lib/ service layer)

External consumers use generated gRPC clients from `lib/`:

```go
import "erg.ninja/lib/crawler/v1"

client, err := crawlerv1.NewClient("crawler.internal:8083")
resp, err := client.CrawlURL(ctx, &crawlerv1.CrawlURLRequest{
    Url:       "https://example.com",
    TenantId:  "acme",
    Priority:  crawlerv1.PRIORITY_NORMAL,
    FetchContent: true,
})
```

### 2. HTTP REST (fallback)

Each `lib/` client has an HTTP fallback when gRPC is unavailable:

```go
client, err := crawlerv1.NewClient("http://crawler.internal:8083/api")
```

### 3. Direct Function Calls (same-process)

Internal modules call each other directly (zero overhead):

```go
// internal/modules/bot/services/command_handler.go
crawlerAdapter := bot.NewCrawlerAdapter(crawlerService)
trendAdapter  := bot.NewTrendingAdapter(trendingService)
```

### 4. Event Bus (`pkg/event/bus.go`)

Decoupled async events with Redis pub/sub support:

```go
// Crawler publishes:
bus.Publish(ctx, "crawl.success", map[string]string{"url": url, "title": title})

// Notifications subscribes:
bus.Subscribe("crawl.success", func(payload []byte) { ... })
```

14 event topics: `crawl.success`, `crawl.failed`, `trending.hot_topic`, `rss.added`, `system.alert`, `queue.overload`, etc.

### 5. Asynq Job Queue (Redis-backed)

```go
// Enqueue a crawl job
asynqClient.Enqueue(ctx, "crawl:url", payload, asynq.MaxRetry(3))

// Tenant-isolated queues
tenantClient := NewTenantAsynqClient(asynqClient, "acme")
tenantClient.Enqueue(ctx, "crawl:url", payload) // → queue "crawl_acme"
```

---

## gRPC Services (lib/)

### CrawlerService (`lib/crawler/v1/`)

```protobuf
service CrawlerService {
  rpc CrawlURL(CrawlURLRequest) returns (CrawlURLResponse);
  rpc GetCrawlStatus(GetCrawlStatusRequest) returns (CrawlStatusResponse);
  rpc ListFeeds(ListFeedsRequest) returns (ListFeedsResponse);
  rpc RefreshFeed(RefreshFeedRequest) returns (RefreshFeedResponse);
  rpc GetStats(GetStatsRequest) returns (StatsResponse);
  rpc StopCrawl(StopCrawlRequest) returns (StopCrawlResponse);
  rpc GetCrawlHistory(CrawlHistoryRequest) returns (CrawlHistoryResponse);
  rpc Reindex(ReindexRequest) returns (ReindexResponse);
}
```

### BotService (`lib/bot/v1/`)

```protobuf
service BotService {
  rpc ListConversations(ListConversationsRequest) returns (ListConversationsResponse);
  rpc SendMessage(SendMessageRequest) returns (SendMessageResponse);
  rpc GetWizardState(GetWizardStateRequest) returns (WizardState);
  rpc AdvanceWizard(AdvanceWizardRequest) returns (AdvanceWizardResponse);
  rpc ListWorkflows(ListWorkflowsRequest) returns (ListWorkflowsResponse);
  rpc StartWorkflow(StartWorkflowRequest) returns (StartWorkflowResponse);
  rpc CreateLinkCode(CreateLinkCodeRequest) returns (CreateLinkCodeResponse);
  rpc ExecuteCommand(ExecuteCommandRequest) returns (ExecuteCommandResponse);
  rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);
}
```

### NotificationService (`lib/notification/v1/`)

```protobuf
service NotificationService {
  rpc Send(SendNotificationRequest) returns (SendNotificationResponse);
  rpc Get(GetNotificationRequest) returns (GetNotificationResponse);
  rpc List(ListNotificationsRequest) returns (ListNotificationsResponse);
  rpc Cancel(CancelNotificationRequest) returns (CancelNotificationResponse);
  rpc GetPreferences(GetPreferencesRequest) returns (GetPreferencesResponse);
  rpc UpdatePreferences(UpdatePreferencesRequest) returns (UpdatePreferencesResponse);
  rpc SendBulk(SendBulkRequest) returns (SendBulkResponse);
}
```

### TrendingService (`lib/trending/v1/`)

```protobuf
service TrendingService {
  rpc GetTopTopics(GetTopTopicsRequest) returns (GetTopTopicsResponse);
  rpc GetTopic(GetTopicRequest) returns (GetTopicResponse);
  rpc SearchTopics(SearchTopicsRequest) returns (SearchTopicsResponse);
  rpc GetTopicNews(GetTopicNewsRequest) returns (GetTopicNewsResponse);
  rpc GetSnapshot(GetSnapshotRequest) returns (GetSnapshotResponse);
  rpc Refresh(RefreshTrendingRequest) returns (RefreshTrendingResponse);
  rpc GetKeywordTrend(GetKeywordTrendRequest) returns (GetKeywordTrendResponse);
}
```

---

## Multi-Tenant Architecture

All shared infrastructure supports tenant isolation:

```
┌─────────────────────────────────────────────────┐
│              Tenant Context Propagation           │
│                                                  │
│  1. X-Tenant-ID header  → 2. JWT claim  → 3. Subdomain │
│           ↓                      ↓              ↓           │
│  tenant.WithTenant(ctx, "acme")                    │
└─────────────────────────────────────────────────┘
         ↓
┌─────────────────────────────────────────────────┐
│  MongoDB: "acme_crawl_histories" collection    │
│  Redis:   "tenant:acme:crawler:jobs:abc123"    │
│  Asynq:  "crawl_acme" queue → "dlq_acme" DLQ │
│  Config:  per-tenant override merged on top     │
└─────────────────────────────────────────────────┘
```

---

## Service Discovery

Three discovery backends (configurable):

| Backend | Use Case | Dependency |
|---|---|---|
| **Static** | Development | None |
| **DNS SRV** | Kubernetes | CoreDNS |
| **Consul** | Production | Consul cluster |

```go
catalog := discovery.NewStaticCatalog(map[string][]string{
    "crawler":      {"localhost:8083"},
    "notification": {"localhost:8082"},
})
resolver := discovery.NewResolver(catalog, "crawler")
conn, _ := grpc.Dial("crawler://", grpc.WithResolvers(resolver))
```

---

## Crawl Pipeline (12 steps)

```
1. URL → Blacklist Check       → blocked? → reject
2.   → Domain Reputation       → score < 10? → skip
3.   → Fetch (robots.txt)      → disallowed? → reject
4.   → Content Quality Gate   → score < 70? → reject
5.   → Content Dedup (SimHash) → duplicate? → reject
6.   → Metadata Extraction    → title, description
7.   → Tag + Language          → tags, language
8.   → Compute SHA-256 hash   → content fingerprint
9.   → Save to MongoDB        → CrawlHistory + Fingerprint
10.  → Publish crawl.success  → Event Bus
11.  → SSE Broadcast          → connected clients
12.  → Trigger Notifications  → via event bus
```

---

## Database Collections

| Collection | Module | Notes |
|---|---|---|
| `bot_conversations` | bot | User conversation state, wizard data |
| `bot_linked_accounts` | bot | Discord/Telegram → internal user linkage |
| `workflow_executions` | bot | Multi-step workflow state machine |
| `notifications` | notifications | Full delivery log |
| `notification_preferences` | notifications | Per-user channel preferences |
| `notification_digests` | notifications | Daily/weekly/monthly digests |
| `rss_feeds` | crawler | Feed metadata + fetch state |
| `crawl_histories` | crawler | Per-URL crawl results |
| `content_fingerprints` | crawler | SimHash + SHA-256 for dedup |
| `content_blacklists` | crawler | URL/domain/keyword blocks |
| `domain_reputations` | crawler | Domain reputation scores |
| `trending_topics` | trending | Aggregated topic scores |
| `news_articles` | trending | Supporting news articles |
| `trending_snapshots` | trending | Historical data for charts |

---

## Middleware Stack

```
chi.Mux chain (outermost → innermost):
1. chiMiddleware.Recoverer           — Panic recovery
2. RequestIDMiddleware              — X-Request-ID header
3. RealIPMiddleware                 — Trust X-Forwarded-For
4. LoggerMiddleware                 — zerolog request logging
5. TenantMiddleware                 — X-Tenant-ID / JWT / subdomain
6. cors.Handler                    — CORS (configured origins)
7. RateLimitMiddleware             — Per-IP token bucket
8. AuthMiddleware                  — JWT on protected routes
9. Module handlers                 — Route-specific logic
```

---

## Graceful Shutdown

```
SIGINT / SIGTERM received
  → Stop accepting connections (http.Server.Shutdown)
  → Discovery deregistration (Consul heartbeat stop)
  → Call Module.Stop(ctx) in reverse order:
    1. Bot module        → stop SSE hub, close connections
    2. Notifications     → stop digest + event consumer
    3. Trending          → stop scheduler
    4. Crawler           → drain SSE hub + wait 500ms
  → Close Asynq workers (drain pending jobs)
  → Close MongoDB connection (10s timeout)
  → Close Redis connection
  → Exit 0
```

---

## Error Codes (`pkg/errors/`)

40+ structured error codes organized by domain:

| Domain | Errors |
|---|---|
| **Crawler** | `CRAWLER_BLACKLISTED`, `CRAWLER_ROBOTS_DISALLOWED`, `CRAWLER_QUALITY_TOO_LOW`, `CRAWLER_DUPLICATE`, `CRAWLER_TIMEOUT`, `CRAWLER_RATE_LIMITED`, `CRAWLER_INVALID_URL` |
| **Bot** | `BOT_UNAUTHORIZED`, `BOT_COMMAND_NOT_FOUND`, `BOT_CONVERSATION_LIMIT` |
| **Notification** | `NOTIFICATION_INVALID_PAYLOAD`, `NOTIFICATION_DELIVERY_FAILED`, `NOTIFICATION_QUEUE_FULL`, `NOTIFICATION_CHANNEL_DISABLED` |
| **Trending** | `TRENDING_NO_DATA`, `TRENDING_STALE_CACHE`, `TRENDING_RATE_LIMITED` |
| **Auth** | `UNAUTHENTICATED`, `TOKEN_EXPIRED`, `TOKEN_INVALID`, `FORBIDDEN` |
| **Tenant** | `TENANT_MISSING`, `TENANT_NOT_FOUND` |

All errors map to gRPC status codes and HTTP status codes automatically.
