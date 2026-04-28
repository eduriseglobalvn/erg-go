# erg-go — Shared Microservice Library for EduRise Global

> Transform erg-go from a single binary monolith → **config-driven, multi-tenant, service-discoverable shared library** consumable by multiple Go services and NestJS backends.

**Build**: `go build ./...` | **gRPC**: 4 services (Crawler, Bot, Notification, Trending) | **Go**: 1.21+

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│              Consumer Services                               │
│  erg-backend (NestJS)  │  Future Go microservices  │  ...   │
└──────────────────────────┬───────────────────────┬────────────┘
                          │                       │
               ┌──────────▼──────┐   ┌────────▼───────────┐
               │ lib/crawler/v1 │   │ lib/bot/v1         │
               │ lib/notification│   │ lib/trending/v1    │
               │ (gRPC clients) │   │ (gRPC clients)     │
               └──────────┬──────┘   └────────┬───────────┘
                          │                       │
         ┌─────────────────┴───────────────────┘
         │            lib/ (v1 stable API surface)
         └─────────────────────┬──────────────────────────┘
                               │
         ┌─────────────────────▼───────────────────────────┐
         │   erg-server (standalone binary + lib/ consumers) │
         │   cmd/server + lib/* + internal/* + pkg/*         │
         │                                                   │
         │  BOT  │  NOTIFICATIONS  │  CRAWLER  │  TRENDING │
         │       │                │           │            │
         │       └──────┬─────────┘           │            │
         │              │   Event Bus          │            │
         │    MongoDB │ Redis │ Asynq │ pkg/*  │            │
         └──────────────────────────────────────────────┘
                               │
         ┌─────────────────────▼───────────────────────────┐
         │     pkg/ (PUBLIC — importable by any service)    │
         │                                                   │
         │  tenant/     Multi-tenant context + isolation       │
         │  discovery/  Consul / DNS / Static catalog         │
         │  plugin/     Build tags + runtime .so loader       │
         │  compose/    Service manifest + dependency resolver  │
         │  errors/     40+ error codes, gRPC/HTTP mapping   │
         │  config/     Viper YAML/env/flags                  │
         │  database/   MongoDB + MySQL (pgx v5)              │
         │  cache/      Redis GET/SET + Pub/Sub              │
         │  queue/      Asynq client + server                 │
         │  event/      In-process + Redis pub/sub           │
         │  logger/     zerolog structured logging            │
         │  http/       chi router + middleware + interceptors │
         │  auth/       JWT validation                        │
         │  scraper/    Fetcher + robots.txt parser          │
         │  dedup/      SimHash deduplication                │
         │  ai/         Gemini AI integration                │
         │  rss/         RSS/Atom parser                    │
         │  sitemap/     Sitemap parser                      │
         │  telemetry/   OpenTelemetry + Prometheus           │
         └─────────────────────────────────────────────────┘
```

---

## Quick Start

```bash
# Standalone server
go build -o erg-server ./cmd/server
./erg-server

# Library (use in other Go services)
go build ./lib/...

# Full build
go build ./...

# Docker
docker compose up -d
```

---

## Library Usage (gRPC Clients)

```go
package main

import (
    "context"
    "erg.ninja/lib/crawler/v1"
)

func main() {
    // Create gRPC client
    client, err := crawlerv1.NewClient("localhost:8083",
        crawlerv1.WithConnectTimeout(5*time.Second),
    )
    if err != nil {
        panic(err)
    }

    // Call service
    resp, err := client.CrawlURL(context.Background(), &crawlerv1.CrawlURLRequest{
        Url:       "https://example.com",
        TenantId:  "acme",
        Priority:  crawlerv1.PRIORITY_NORMAL,
    })
    println("Job ID:", resp.JobId)
}
```

---

## Services

### Crawler (port 8083)
Web crawling with anti-blocking, SimHash deduplication, and AI-assisted content scoring.

**Key features:**
- RSS/Atom/JSON feed polling with SSE progress streaming
- XML sitemap discovery + recursive parsing
- robots.txt compliance with crawl-delay support
- Proxy rotation + User-Agent cycling
- SimHash near-duplicate detection (Hamming ≤ 6)
- 8-rule quality gate (threshold ≥ 5/8)
- Asynq worker pool for parallel crawling
- Domain reputation tracking

**gRPC methods:** `CrawlURL`, `GetCrawlStatus`, `ListFeeds`, `RefreshFeed`, `GetStats`, `StopCrawl`, `GetCrawlHistory`, `Reindex`

### Bot (port 8081)
Handles Discord/Telegram webhooks, conversation management, and workflow automation.

**Key features:**
- Discord slash commands + HMAC-SHA256 webhook verification
- Telegram webhooks + Ed25519 bot API verification
- Multi-step wizard engine (RSS add, account linking)
- Automation workflow engine (pause/resume/cancel)
- RBAC 5-level permission system (owner > admin > moderator > trusted > user)
- 6-character alphanumeric link codes (Redis TTL)

**gRPC methods:** `ListConversations`, `SendMessage`, `GetWizardState`, `AdvanceWizard`, `ListWorkflows`, `StartWorkflow`, `CreateLinkCode`, `ExecuteCommand`, `HealthCheck`

### Notification (port 8082)
Multi-channel notification fan-out with retry, digest scheduling, and delivery tracking.

**Key features:**
- Discord, Telegram, WhatsApp, Email channels
- Exponential backoff retry + dead-letter queue
- Digest mode (daily/weekly/monthly batching)
- 14 event topics from event bus
- Per-user channel preferences
- Per-channel rate limit enforcement

**gRPC methods:** `Send`, `Get`, `List`, `Cancel`, `GetPreferences`, `UpdatePreferences`, `SendBulk`

### Trending (port 8084)
Scheduled aggregation of trending topics from crawled content + external sources.

**Key features:**
- Cron-based aggregation (every 15 minutes)
- Time-windowed scoring (1h, 24h, 7d, 30d)
- Redis-cached results
- MongoDB compound indexes for fast top-k queries
- Point-in-time snapshots for historical charts
- Hot-topic alerts

**gRPC methods:** `GetTopTopics`, `GetTopic`, `SearchTopics`, `GetTopicNews`, `GetSnapshot`, `Refresh`, `GetKeywordTrend`, `GetStats`

---

## Shared Packages

| Package | Description |
|---------|-------------|
| `pkg/tenant` | Tenant context propagation, middleware, MongoDB/Redis/Asynq isolation |
| `pkg/discovery` | Service registry: Consul, DNS SRV, Static catalog |
| `pkg/plugin` | Build tags + runtime `.so` module loader |
| `pkg/compose` | Service manifest loader, topological dependency sort |
| `pkg/errors` | 40+ structured error codes, gRPC ↔ HTTP mapper |
| `pkg/config` | Viper YAML/env/flags, secret injection |
| `pkg/database` | MongoDB (mongo-driver v2) + MySQL (pgx v5) |
| `pkg/cache` | Redis GET/SET, distributed locks, Pub/Sub |
| `pkg/queue` | Asynq client + server, priority queues, DLQ |
| `pkg/event` | In-process + Redis pub/sub event bus |
| `pkg/http` | chi router, middleware stack, interceptors |
| `pkg/scraper` | HTTP fetcher, robots.txt, proxy rotation |
| `pkg/dedup` | SimHash + SHA-256 content deduplication |
| `pkg/ai` | Gemini AI client, dual-tier L1/L2 cache |
| `pkg/rss` | RSS 2.0 / Atom 1.0 / JSON Feed parser |
| `pkg/sitemap` | XML sitemap discovery + recursive parsing |
| `pkg/telemetry` | OpenTelemetry tracing + Prometheus metrics |
| `pkg/auth` | JWT validation (HS256/RS256) |
| `pkg/logger` | zerolog structured logging |
| `pkg/monitoring` | Component health checks |

---

## Configuration

```yaml
app:
  env: production
  host: "0.0.0.0"
  port: 8080

mongodb:
  uri: mongodb://localhost:27017
  database: erg_server

redis:
  host: localhost
  port: 6379

queue:
  concurrency: 20
  retry_backoff: true

# Multi-tenant
tenants:
  default: shared_defaults
  acme:
    scraper.max_delay: 5s
    queue.concurrency: 5
  startup_rocket:
    trending.min_hot_score: 90

# Service discovery
discovery:
  enabled: true
  backend: consul
  consul:
    addr: "consul.internal:8500"
```

---

## Testing

```bash
go test ./... -v -race
go test ./pkg/discovery/... -v
go test ./pkg/tenant/... -v
```

---

## Deployment

```bash
# Docker Compose
docker compose up -d

# Build binary
go build -o erg-server ./cmd/server

# Build with specific modules
go build -tags "module_crawler,module_notification" -o erg-crawler-notif ./cmd/server
```

---

## Proto Generation

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

for svc in crawler bot notification trending; do
  protoc --go_out=. --go_opt=paths=source_relative \
         --go-grpc_out=. --go-grpc_opt=paths=source_relative \
         proto/lib/$svc/v1/$svc.proto
done
```
