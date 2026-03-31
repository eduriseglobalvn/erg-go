# Migration to Go - Module Extraction Plan

> **Status**: Draft | **Target**: Production Go monorepo | **Migration Strategy**: Strangler Fig Pattern

---

## Table of Contents

1. [Overview & Motivation](#1-overview--motivation)
2. [Extraction Phases](#2-extraction-phases)
3. [Shared Framework Architecture (Go)](#3-shared-framework-architecture-go)
4. [BOT Service Extraction](#4-bot-service-extraction)
5. [Notification Service Extraction](#5-notification-service-extraction)
6. [Crawler Service Extraction](#6-crawler-service-extraction)
7. [Trending Service Extraction](#7-trending-service-extraction)
8. [Database Migration Strategy](#8-database-migration-strategy)
9. [Deployment & CI/CD](#9-deployment--cicd)
10. [Testing Strategy](#10-testing-strategy)

---

## 1. Overview & Motivation

### Why Migrate from NestJS/TypeScript to Go?

The current backend at `D:\ERG\erg-backend` is a **monolithic NestJS/TypeScript application** that handles multiple high-load domains simultaneously: bot command processing, real-time notifications, web crawling, and trending data aggregation. As the system scales, several structural limitations of the NestJS monolith have become blockers:

- **CPU-bound performance bottlenecks**: TypeScript's runtime overhead makes it poorly suited for I/O-heavy workloads with large concurrency demands (e.g., crawler workers processing hundreds of feeds in parallel).
- **Memory footprint**: Node.js/NestJS consumes significantly more RAM than Go under equivalent load, increasing infrastructure costs.
- **Concurrency model**: JavaScript's event-loop concurrency works well for I/O, but CPU-intensive crawling/scraping tasks require true goroutine parallelism with minimal overhead.
- **Operational complexity**: The monolith bundles all modules into a single deployable unit — any single module's failure affects the entire system, and scaling requires scaling everything.

### Benefits of the Go Migration

| Dimension | NestJS/TypeScript | Go (Post-Migration) |
|---|---|---|
| HTTP throughput | Baseline | **~5–10x lower latency**, higher RPS per instance |
| Memory usage | ~150–300 MB base | **~10–30 MB base** per service |
| Concurrency | Async/await (event loop) | **Goroutines** — millions of lightweight threads |
| Crawler workers | 1 process, blocked I/O | **Parallel goroutines**, channel-based job dispatch |
| Deployment | Single monolith JAR | **Independent service images**, independent scaling |
| Type safety | TypeScript (compile-time only) | **Go static binaries**, nil-safety, struct embedding |
| Cold start | ~2–5 s (NestJS bootstrap) | **< 50 ms** (compiled binary, no runtime bootstrap) |
| Ecosystem | npm/yarn, Node.js | **Go modules**, single static binary |

Go is purpose-built for exactly the workloads in this stack:
- **BOT service** — high-frequency webhook ingestion from Discord/Telegram with low-latency command dispatch.
- **Notification service** — fan-out to multiple providers (Discord, Telegram, WhatsApp) with retry logic and digest scheduling.
- **Crawler service** — CPU/I/O-intensive page fetching, parsing, anti-blocking, SimHash deduplication, and scoring.
- **Trending service** — scheduled data aggregation from external APIs (Google Trends, News API).

### Strangler Fig Pattern Strategy

Rather than a risky big-bang rewrite, this plan follows the **Strangler Fig Pattern** — a incremental migration approach:

1. **NestJS remains running and production-safe** throughout the migration.
2. Each module is **extracted one at a time** into a standalone Go service.
3. During extraction, a **thin API proxy layer** in NestJS routes traffic to the new Go service while the old code is still wired in.
4. Once validation passes, the old NestJS module is **disabled** and the Go service takes over.
5. Only when all modules are migrated does the NestJS monolith become a routing shell (or is decommissioned).

This ensures **zero downtime**, **gradual risk reduction**, and the ability to **roll back** any phase independently.

### Goal: Reusable Go Services Across Projects

A key objective is to design each extracted Go service so it is **project-agnostic** and can be reused across multiple websites and products:

- Services are configured entirely through `config.yaml` / environment variables.
- No hardcoded project IDs, domain names, or business-specific logic.
- Shared infrastructure (`pkg/database`, `pkg/queue`, `pkg/event`) is versioned as internal Go modules.
- Docker Compose and Helm charts make deployment reproducible on any infrastructure.

### Current State

```
D:\ERG\erg-backend/           ← NestJS monolith (TypeScript)
├── src/
│   ├── bot/                  ← BOT module (36+ commands, conversations, workflows)
│   ├── notifications/        ← Notification module (multi-provider, digest)
│   ├── crawler/              ← Crawler module (feeds, scraping, anti-block, scoring)
│   ├── trending/             ← Trending module (Google Trends, News API)
│   └── shared/               ← Shared NestJS utilities (DB, auth, config)
└── erg-backend.ts            ← Application bootstrap
```

**Modules in scope for migration:**

| Module | Language (current) | Target Service | Database | Workers |
|---|---|---|---|---|
| BOT | TypeScript | `bot-service` | MongoDB | Goroutine pool |
| Notification | TypeScript | `notification-service` | MongoDB | Cron + async queue |
| Crawler | TypeScript | `crawler-service` | MongoDB | Asynq job queue |
| Trending | TypeScript | `trending-service` | MongoDB | Cron (30-min) |

**Modules to remain in NestJS (or migrate later):** Auth, API Gateway, Admin dashboard.

### Scope of This Plan

> ✅ **In scope:** Extraction and migration of BOT, Notification, Crawler, and Trending modules to Go as standalone services using the Strangler Fig Pattern.
>
> ❌ **Out of scope:** Migrating the NestJS authentication layer, the API gateway, the admin dashboard, or the frontend. These are covered in a separate roadmap.

---

## 2. Extraction Phases

The migration is organized into **6 sequential phases** over **14 weeks**. Each phase is self-contained with its own deliverables, validation criteria, and rollback strategy. Phases 1–5 each target a single service; Phase 6 is integration and hardening.

### Phase 1 — Foundation (Week 1–2)

**Objective:** Establish the Go monorepo structure, shared framework, and CI/CD pipeline before any service extraction begins.

#### Deliverables Checklist

- [ ] **Monorepo scaffold** at `D:\ERG/go-erg/`:
  ```
  go-erg/
  ├── cmd/
  │   ├── bot-service/
  │   ├── notification-service/
  │   ├── crawler-service/
  │   └── trending-service/
  ├── pkg/
  │   ├── config/          ← viper-based config loading from YAML/env
  │   ├── database/        ← MongoDB client (mongo-go-driver), connection pooling
  │   ├── queue/           ← Asynq client wrappers (Redis-backed job queue)
  │   ├── event/           ← Event bus (in-process channels + Redis pub/sub)
  │   ├── logger/          ← structured logging (zerolog/slog)
  │   ├── http/            ← chi router, middleware (auth, rate-limit, recovery)
  │   ├── grpc/            ← gRPC server helpers (for inter-service calls)
  │   └── telemetry/       ← OpenTelemetry tracing + Prometheus metrics
  ├── scripts/
  │   ├── docker-build.sh
  │   └── migrate-db.sh
  ├── Dockerfile.base      ← multi-stage base image (Go 1.22+, ca-certificates, ca-certs)
  └── go.mod / go.work     ← Go workspace with module replace directives
  ```
- [ ] **Shared framework packages** (`pkg/*`) implemented with:
  - MongoDB connection pool with automatic retry and read preference support
  - Viper config with env variable overrides and secrets injection from Vault/env files
  - Zerolog structured logger with JSON output and correlation ID injection
  - Asynq client with dead-letter queue and exponential backoff configuration
  - Standard chi-router middleware stack (CORS, auth, request ID, rate limit, panic recovery)
- [ ] **Docker base image** built and pushed to registry (`erg-go-base:latest`).
- [ ] **CI/CD pipeline** configured (GitHub Actions or GitLab CI):
  - On push: `go vet`, `staticcheck`, `golangci-lint`, `go test ./... -race`
  - On merge to `main`: build all 4 service binaries, push Docker images, deploy to staging
- [ ] **Migrate shared entities** (User, Role, Permission) to Go:
  - Port MongoDB schemas from NestJS `src/shared/entities/`
  - Create `pkg/auth` package with JWT validation helpers used by all services
  - Write integration tests against a real MongoDB instance (testcontainers)

#### Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Team unfamiliar with Go idioms | Medium | Medium | Pair programming, Go code review checklist, enforce `golangci-lint` |
| Monorepo structure disagreements | Medium | High | Finalize structure in Week 1, document ADR (Architecture Decision Record) |
| CI/CD pipeline complexity | Low | Medium | Use reusable GitHub Actions workflows; do NOT over-engineer in Week 1 |
| Config secrets leakage | Low | Critical | Use `docker secret` or Vault; never commit `.env` files |

#### Validation

- `go build ./...` succeeds for all packages with zero errors.
- `go test ./...` passes with `-race` flag (detect data races in shared framework).
- Docker image for `bot-service` base skeleton builds and runs (`docker run --rm`).
- Shared packages import cleanly into all four service modules.

---

### Phase 2 — BOT Service (Week 3–4)

**Objective:** Extract the NestJS BOT module into a standalone `bot-service`.

#### Deliverables Checklist

- [ ] **BOT service binary** at `cmd/bot-service/main.go`:
  - chi HTTP router, graceful shutdown (`os/signal` + `context.WithCancel`)
  - Health check endpoint (`GET /healthz` → MongoDB ping + Redis ping)
  - Readiness endpoint (`GET /ready` → all dependencies connected)
- [ ] **Ported entities** (MongoDB, `internal/models/`):
  - `BotConversation` — user/platform/scoped conversation state
  - `BotLinkedAccount` — Discord/Telegram user linkage to internal User ID
  - `BotWorkflow` — workflow step definitions and execution state
  - `BotCommand` — command registry (36+ commands ported from `bot-command-handler`)
- [ ] **Ported services** (`internal/services/`):
  - `BotCommandHandler` — command routing, prefix matching, permission checks
  - `ConversationService` — conversation lifecycle, context memory
  - `WorkflowEngine` — step execution, branching, resume-from-checkpoint
  - `LinkService` — account linking/unlinking, identity resolution
- [ ] **Webhook handlers** (`internal/handlers/`):
  - `DiscordWebhookHandler` — Discord interaction endpoint (`POST /webhooks/discord`)
  - `TelegramWebhookHandler` — Telegram webhook endpoint (`POST /webhooks/telegram`)
  - HMAC signature verification for both platforms
- [ ] **BOT module removed from NestJS**, replaced with **API proxy**:
  ```nginx
  # NestJS routes /api/bot/* → bot-service:8081
  location /api/bot/ {
    proxy_pass http://bot-service:8081/;
  }
  ```
- [ ] **Unit tests** for all command handlers (mock DB, mock external calls).
- [ ] **Docker image** built and deployed to staging as `erg-bot-service:latest`.

#### Porting Details: BOT Module

| NestJS File | Go Equivalent | Notes |
|---|---|---|
| `src/bot/bot-command-handler/` | `internal/commands/` | One file per command; switch to `switch` statement map |
| `src/bot/bot-conversation/` | `internal/services/conversation.go` | Goroutine-per-session with channel context |
| `src/bot/bot-link/` | `internal/services/link.go` | Port account linking logic |
| `src/bot/bot-permission/` | `internal/middleware/permission.go` | Middleware on chi router |
| `src/bot/schemas/` | `internal/models/*.go` | Go structs with `bson` tags |

#### Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Webhook security bypass | Low | Critical | Strict HMAC verification before any processing; reject unsigned requests with 401 |
| Command statefulness issues | Medium | Medium | Use MongoDB for conversation state; no in-memory session state in handlers |
| Discord/Telegram API breaking changes | Low | Medium | Abstract platform adapters (`internal/platform/`) to isolate API calls |
| Performance regression in command latency | Medium | Medium | Benchmark command P99 latency in staging before cutover |

#### Validation

- All 36+ bot commands return correct responses (integration test suite with mocked platforms).
- Webhook endpoint handles 1,000 concurrent requests with P99 < 100 ms.
- Health check returns `{"status":"ok","mongo":"ok","redis":"ok"}`.
- NestJS API proxy returns same response as direct bot-service call (response comparison test).

---

### Phase 3 — Notification Service (Week 5–6)

**Objective:** Extract the Notification module into a standalone `notification-service`.

#### Deliverables Checklist

- [ ] **Notification service binary** at `cmd/notification-service/main.go`.
- [ ] **Ported entities** (`internal/models/`):
  - `Notification` — recipient, channel, template, status, delivery metadata
  - `NotificationPreference` — per-user channel preferences (mute, digest settings)
  - `NotificationTemplate` — multilingual template definitions (Vietnamese first-class)
- [ ] **Ported services** (`internal/services/`):
  - `NotificationService` — send, batch-send, cancel, resend operations
  - `TemplateRenderer` — Go `text/template` with Vietnamese interpolation support
  - `DigestScheduler` — daily/weekly/monthly digest aggregation (cron-based)
  - `DeliveryTracker` — retry logic, exponential backoff, delivery receipts
- [ ] **Provider adapters** (`internal/providers/`):
  - `DiscordWebhookProvider` — Discord embeds, rate-limit handling (200 req/min)
  - `TelegramBotProvider` — Telegram Bot API sendMessage/editMessage
  - `WhatsAppProvider` — WhatsApp Business API (interface for future expansion)
  - All providers implement `Notifier` interface:
    ```go
    type Notifier interface {
        Send(ctx context.Context, msg *Notification) error
        Supports(channel ChannelType) bool
    }
    ```
- [ ] **NotificationBus** refactored as an **event-driven gRPC/HTTP consumer**:
  - Subscribes to Redis pub/sub channels for domain events (`user.created`, `content.published`, etc.)
  - Event envelope: `{event_type, source_service, payload, timestamp}`
  - Asynq job enqueued for async delivery with priority queue support
- [ ] **NestJS proxy** updated: `location /api/notifications/` → `notification-service:8082`.
- [ ] **Unit + integration tests** for all providers (mock HTTP responses).
- [ ] **Docker image** deployed to staging as `erg-notification-service:latest`.

#### Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Vietnamese template rendering bugs | Medium | Medium | Visual regression tests with golden files; cover all interpolation cases |
| Provider rate limit cascades | Medium | High | Implement token-bucket rate limiter per provider; dead-letter queue for exceeded requests |
| Event bus message loss on restart | Low | High | Asynq persistence; at-least-once delivery with idempotency keys on consumers |
| WhatsApp API credential rotation | Low | High | Vault integration for provider API keys; auto-rotate credentials |

#### Validation

- Sending to all three providers (Discord, Telegram, WhatsApp) produces correct payloads.
- Digest scheduler correctly batches and sends at configured times.
- Event consumer processes 10,000 events/second without dropped messages (load test).
- NestJS proxy response matches direct notification-service response.

---

### Phase 4 — Crawler Service (Week 7–10)

**Objective:** Extract the Crawler module — the largest and most complex module — into a standalone `crawler-service`. This is the highest-risk phase and gets **4 dedicated weeks**.

#### Deliverables Checklist

- [ ] **Crawler service binary** at `cmd/crawler-service/main.go`:
  - HTTP API server on `:8083`
  - Asynq worker pool (configurable: default 20 workers, scales with CPU cores)
  - SSE (Server-Sent Events) gateway on `/crawl/stream/:job_id` for real-time crawl progress
  - Graceful shutdown: drain in-flight crawl jobs before shutdown signal completes
- [ ] **Ported entities** (`internal/models/`):
  - `RssFeed` — feed URL, update frequency, category, language (Vietnamese)
  - `ScraperConfig` — CSS selectors, headers, cookie jar, JavaScript rendering settings
  - `CrawlHistory` — per-URL crawl metadata: status, duration, response size, error
  - `DomainReputation` — domain trust score, last seen, block history
  - `ContentFingerprint` — SimHash + raw hash for deduplication
  - `ContentBlacklist` — URL pattern, domain, keyword blocklists
- [ ] **Ported services** (`internal/services/`):
  - `FeedFetcher` — concurrent RSS/Atom/JSON feed polling with ETag/Last-Modified support
  - `ScraperService` — HTML fetching, robots.txt respect, anti-block (proxy rotation, user-agent, delay)
  - `RobotsParser` — Go port of robots.txt parsing (or use `github.com/GPedersen/robots_parser`)
  - `QualityGate` — 8-rule content scoring (length, originality, freshness, keyword density, readability, media presence, structure, spam signals); score ≥ 70 = publishable
  - `ContentDedup` — SimHash algorithm for near-duplicate detection; raw SHA-256 for exact duplicates
  - `SmartSelector` — Gemini AI–powered CSS selector suggestion for unstructured pages
  - `SitemapParser` — XML sitemap discovery and recursive crawling
  - `BlacklistChecker` — URL/domain/keyword blacklist matching (Aho-Corasick for keyword speed)
- [ ] **Asynq job types** (`internal/jobs/`):
  - `CrawlJob` — `{url, depth, config_id, priority}` → fetch → parse → score → dedup → store
  - `RefreshFeedJob` — periodic feed re-fetch, delta detection
  - `ReindexJob` — re-fingerprint existing content after algorithm update
  - All jobs include: timeout, max retries (3), dead-letter queue path
- [ ] **Crawl progress SSE endpoint**:
  - Client connects: `GET /crawl/stream/:job_id`
  - Server pushes: `{url, status, items_discovered, items_scraped, errors[], progress_pct}`
  - Goroutine-safe channel broadcast to all connected clients watching the same job
- [ ] **NestJS proxy** updated: `location /api/crawler/` → `crawler-service:8083`.
- [ ] **Load test**: 500 concurrent crawl jobs, verify P99 < 5 s per job enqueue.
- [ ] **Docker image** deployed to staging as `erg-crawler-service:latest`.

#### Architecture Diagram: Crawler Service

```
┌─────────────────────────────────────────────────────────────────┐
│                    crawler-service (:8083)                      │
│                                                                 │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────┐  │
│  │  chi HTTP    │    │  Asynq       │    │  SSE Gateway     │  │
│  │  API Server  │    │  Worker Pool │    │  (job progress)  │  │
│  └──────┬───────┘    │  (20 workers)│    └────────┬─────────┘  │
│         │           └──────┬───────┘             │            │
│         │                  │                     │            │
│         ▼                  ▼                     │            │
│  ┌─────────────────────────────────────────┐     │            │
│  │           internal/services/             │     │            │
│  │  FeedFetcher │ Scraper │ QualityGate    │◄────┘            │
│  │  Dedup │ SmartSelector │ BlacklistCheck │                   │
│  └─────────────────────────────────────────┘                   │
│                          │                                      │
│                          ▼                                      │
│                   ┌──────────────┐                              │
│                   │   MongoDB    │                              │
│                   │ (feeds, hist,│                              │
│                   │  fingerprints)│                              │
│                   └──────────────┘                              │
│                          │                                      │
│                          ▼                                      │
│                   ┌──────────────┐                              │
│                   │    Redis     │                              │
│                   │  (Asynq job  │                              │
│                   │   queue +    │                              │
│                   │  pub/sub)    │                              │
│                   └──────────────┘                              │
└─────────────────────────────────────────────────────────────────┘
```

#### Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Crawler gets blocked by target domains | High | High | Implement proxy pool rotation, adaptive delay (minimum 3s between requests), user-agent cycling; respect `X-Robots-Tag` |
| SimHash collision causing false positives | Medium | Medium | Two-stage dedup (exact SHA-256 first, then SimHash); store Hamming distance threshold in config |
| Gemini API cost at scale | Medium | High | Cache selector suggestions per domain; batch selector requests; set monthly API budget alert |
| Memory explosion from large pages | Medium | Medium | Max response size cap (10 MB); streaming HTML parser (`goquery`/`colly`) over full-load |
| Asynq job backlog during peak | Medium | Medium | Auto-scale worker count based on Redis queue depth; separate high-priority vs. low-priority queues |
| SSE connection leaks on worker crash | Low | Medium | Channel close on job completion; client heartbeat every 30s; max 10,000 concurrent SSE connections |

#### Validation

- Feed fetcher correctly processes RSS, Atom, and JSON Feed formats.
- Quality gate rejects pages with score < 70 and accepts pages with score ≥ 70 (unit test with sample HTML).
- SimHash correctly identifies near-duplicate articles (score < 3 Hamming distance).
- SSE endpoint streams real-time progress to connected clients.
- Asynq workers complete 1,000 crawl jobs/hour in staging load test with no OOM.
- NestJS proxy response matches direct crawler-service response.

---

### Phase 5 — Trending Service (Week 11–12)

**Objective:** Extract the Trending module into a standalone `trending-service`.

#### Deliverables Checklist

- [ ] **Trending service binary** at `cmd/trending-service/main.go`:
  - HTTP API server on `:8084`
  - Internal cron scheduler (Go's `robfig/cron`) running every 30 minutes
  - Read-only data API (trending feeds consumed by crawler-service and frontend)
- [ ] **Ported services** (`internal/services/`):
  - `GoogleTrendsService` — Google Trends API (or scrape fallback via `serpapi/google-trends-api`)
  - `NewsApiService` — NewsAPI.org integration for top headlines by keyword/country
  - `TrendingAggregator` — merge + rank + dedupe results from all sources
  - `TrendingScheduler` — cron-triggered refresh; stores snapshot history for trend charts
- [ ] **Ported entities** (`internal/models/`):
  - `TrendingTopic` — topic, score, volume, source, timestamp, related keywords
  - `NewsArticle` — headline, source, URL, published_at, relevance score
  - `TrendingSnapshot` — point-in-time snapshot for historical trend charting
- [ ] **API endpoints** (`internal/handlers/`):
  - `GET /trending/topics` — current top 20 trending topics
  - `GET /trending/topics/:topic` — topic detail with related keywords and timeline
  - `GET /trending/news` — latest news articles related to trending keywords
  - `GET /trending/feeds` — **URL discovery feed** for crawler-service (`?since=<timestamp>&limit=100`)
  - `POST /trending/refresh` — trigger immediate refresh (admin only)
- [ ] **URL discovery feed** consumed by `crawler-service`:
  - trending-service writes discovered URLs to a Redis list
  - crawler-service polls every 5 minutes: `LRANGE trending:urls 0 99` then `LTRIM`
  - Fallback: HTTP endpoint `/trending/feeds` polled by crawler Asynq job
- [ ] **NestJS proxy** updated: `location /api/trending/` → `trending-service:8084`.
- [ ] **Docker image** deployed to staging as `erg-trending-service:latest`.

#### Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Google Trends API rate limits | Medium | Medium | Cache responses for 25 minutes; fallback to NewsAPI-only if Trends is unavailable |
| NewsAPI free tier → 100 req/day limit | High | Medium | Cache aggressively; batch topic lookups; consider SerpAPI or RapidAPI as paid fallback |
| URL discovery flood (too many URLs at once) | Medium | Medium | Redis list with max size (10,000); crawler-service throttles via its own priority queue |
| Trending data staleness | Low | Medium | Staleness header on all responses; `/healthz` reports last-successful-refresh timestamp |

#### Validation

- Cron job runs every 30 minutes and produces non-empty trending topic lists.
- URL discovery feed returns valid, non-duplicate URLs.
- `GET /trending/topics` responds in < 200 ms (cached data).
- External API failures are logged and degraded gracefully (service remains up with stale data).
- NestJS proxy response matches direct trending-service response.

---

### Phase 6 — Integration & Refinement (Week 13–14)

**Objective:** Validate the complete system, finalize the API gateway, and close out the migration.

#### Deliverables Checklist

- [ ] **API Gateway setup** (nginx or dedicated Go gateway):
  - Route all `/api/bot/*` → `bot-service:8081`
  - Route all `/api/notifications/*` → `notification-service:8082`
  - Route all `/api/crawler/*` → `crawler-service:8083`
  - Route all `/api/trending/*` → `trending-service:8084`
  - Centralized rate limiting, request logging, TLS termination
  - (Optional) gRPC inter-service calls for internal communication
- [ ] **Service mesh evaluation** (Istio or Linkerd):
  - mTLS between services
  - Distributed tracing across all 4 services (OpenTelemetry → Jaeger)
  - Traffic splitting for gradual rollouts (canary deployments)
  - **Decision**: Implement if operational overhead is acceptable; otherwise defer to post-migration
- [ ] **Full end-to-end integration test suite**:
  - Simulate a complete workflow: trending discovery → crawl → quality gate → notification
  - Run on every PR via GitHub Actions
- [ ] **Performance benchmarking**:
  - Compare NestJS monolith vs. 4-service Go deployment under identical load
  - Target: P99 latency reduction ≥ 5x, memory reduction ≥ 3x, throughput increase ≥ 5x
  - Report results as an ADR
- [ ] **NestJS monolith decommission checklist**:
  - [ ] All 4 modules removed from NestJS source tree
  - [ ] NestJS only runs auth gateway + admin dashboard (if applicable)
  - [ ] All environment variables and secrets migrated to Vault or `.env` files per service
  - [ ] Old NestJS Docker image stopped in production
  - [ ] Old NestJS deployment manifests removed from Kubernetes/Helm
- [ ] **Documentation**:
  - `go-erg/README.md` — monorepo overview, building, running
  - `go-erg/docs/architecture.md` — service diagrams, data flow, inter-service contracts
  - `go-erg/docs/api/*.md` — OpenAPI specs per service
  - `go-erg/docs/runbook.md` — operational runbook (deployment, rollback, alerting)
  - Health check and metric endpoints documented for all 4 services

#### Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| API Gateway routing bugs after full cutover | Medium | High | Run canary (10% traffic) for 48 hours before full cutover; compare error rates |
| Inter-service contract drift | Medium | High | Enforce API contracts with OpenAPI schema CI checks; version all APIs from Day 1 |
| Operational overhead of 4 services > 1 monolith | Medium | Medium | Kubernetes/Helm reduces operational burden; Docker Compose for local dev |
| Data migration bugs (MongoDB schema changes) | Low | High | Run MongoDB migration scripts in staging with production-sized dataset before applying in prod |

#### Validation

- All 4 Go services respond correctly through the API Gateway.
- End-to-end workflow test passes: trending → crawler → notification delivery.
- Performance benchmarks meet or exceed targets.
- NestJS monolith is fully decommissioned (or reduced to routing shell).
- All CI pipelines green; no flaky tests.
- Runbook reviewed and approved by at least 2 team members.

---

## 3. Shared Framework Architecture (Go)

### 3.1 Go Monorepo Structure

```
go-erg/                          # Go monorepo root
├── cmd/                         # Service entry points (one dir per service)
│   ├── bot-service/              # BOT service
│   │   └── main.go
│   ├── notification-service/     # Notification service
│   │   └── main.go
│   ├── crawler-service/         # Crawler service
│   │   └── main.go
│   └── trending-service/        # Trending service
│       └── main.go
├── internal/                    # Private application code (not importable)
│   ├── models/                  # Domain models (shared across services)
│   ├── handlers/                # HTTP handlers
│   ├── services/                # Business logic
│   └── jobs/                    # Queue job types
├── pkg/                         # Public shared packages (can be imported)
│   ├── config/                  # Viper-based config: YAML/env/flags
│   ├── database/
│   │   ├── mysql.go             # MySQL via sqlx + connection pool
│   │   └── mongo.go             # MongoDB via mongo-go-driver
│   ├── cache/
│   │   └── redis.go             # Redis client (go-redis/redis v9), pub/sub, distributed lock
│   ├── queue/
│   │   └── asynq.go             # Asynq client/server (BullMQ equivalent in Go)
│   ├── event/
│   │   └── bus.go               # In-process event bus + Redis pub/sub bridge
│   ├── logger/
│   │   └── log.go               # zerolog structured logger with correlation ID
│   ├── http/
│   │   ├── client.go            # HTTP client: retry, timeout, circuit breaker
│   │   ├── server.go            # chi router + middleware stack
│   │   └── middleware/          # Auth, rate-limit, request-ID, recovery, CORS
│   ├── auth/
│   │   └── jwt.go               # JWT validation middleware
│   ├── notification/
│   │   ├── interfaces.go        # NotifierProvider interface
│   │   └── providers/           # Discord, Telegram, WhatsApp adapters
│   ├── scraper/
│   │   ├── fetcher.go           # HTTP fetcher with anti-block
│   │   ├── parser.go            # goquery HTML parser
│   │   ├── playwright.go         # chromedp for JS-rendered pages
│   │   └── robots.go            # robots.txt parser
│   ├── dedup/
│   │   └── simhash.go           # SimHash (FNV-1a) + Levenshtein for dedup
│   ├── ai/
│   │   └── gemini.go            # Gemini AI client
│   ├── rss/
│   │   └── parser.go            # RSS/Atom/JSON feed parser
│   ├── sitemap/
│   │   └── parser.go            # XML sitemap discovery + parsing
│   └── telemetry/
│       └── otel.go              # OpenTelemetry + Prometheus metrics
├── proto/                       # gRPC proto definitions
│   └── events.proto             # Inter-service event contracts
├── scripts/
│   ├── docker-build.sh
│   └── db-migrate.sh
├── Dockerfile.base               # Multi-stage base image
├── go.work                      # Go workspace (modules)
├── go.mod                       # Main module
└── Makefile
```

### 3.2 Key Interface Definitions

```go
// pkg/config/config.go
type Config interface {
    GetString(key string) string
    GetInt(key string) int
    GetDuration(key string) time.Duration
    GetBool(key string) bool
    Unmarshal(v interface{}) error
}

// pkg/database/mongo.go
type MongoDB interface {
    Collection(name string) *mongo.Collection
    Ping(ctx context.Context) error
    Close(ctx context.Context)
}

// pkg/cache/redis.go
type RedisCache interface {
    Get(ctx context.Context, key string) (string, error)
    Set(ctx context.Context, key string, val interface{}, exp time.Duration) error
    Del(ctx context.Context, keys ...string) error
    PubSub(ctx context.Context, channel string) *redis.PubSub
    Lock(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

// pkg/queue/asynq.go
type TaskQueue interface {
    Enqueue(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
    RegisterHandler(typeName string, h asynq.HandlerFunc)
    Start() error
    Stop() error
}

// pkg/event/bus.go
type EventBus interface {
    Publish(ctx context.Context, topic string, payload []byte) error
    Subscribe(topic string, handler func([]byte)) error
    PublishLocal(topic string, event interface{}) error
    SubscribeLocal(topic string, handler func(interface{})) func()
}

// pkg/notification/interfaces.go
type NotifierProvider interface {
    Send(ctx context.Context, msg *Notification) error
    Supports(channel ChannelType) bool
    Name() string
}

// pkg/scraper/fetcher.go
type Fetcher interface {
    Fetch(ctx context.Context, url string) (*FetchResult, error)
    FetchWithConfig(ctx context.Context, url string, cfg *ScraperConfig) (*FetchResult, error)
}

// pkg/dedup/simhash.go
type DedupService interface {
    ComputeFingerprint(content string) (uint64, error)
    IsDuplicate(fingerprint uint64, threshold int) (bool, error)
    StoreFingerprint(ctx context.Context, fingerprint uint64, url string) error
}
```

### 3.3 Technology Choices & Rationale

| Package | Technology | Rationale |
|---|---|---|
| HTTP Router | `go-chi/chi` v5 | Lightweight, idiomatic Go, no reflection |
| HTTP Client | `net/http` + stdlib retry | Minimal deps, full control |
| MySQL | `jackc/pgx/v5` + `jackc/pgx/v5/stdlib` | Best performing Postgres/MySQL driver |
| MongoDB | `mongodb/mongo-go-driver/v2` | Official driver, connection pooling |
| Redis | `redis/go-redis/v9` | Feature-rich, easy Redis Streams/PubSub |
| Job Queue | `hibiken/asynq` | Best BullMQ equivalent; Redis-backed, retries, DLQ |
| Logging | `rs/zerolog` | Structured JSON, zero allocation, fastest |
| Config | `spf13/viper` | YAML/JSON/env/flags, works great with Go |
| HTML Parsing | `goquery` | jQuery-style DOM, lightweight |
| JS Rendering | `go-rod/rod` or `chromedp/chromedp` | Headless Chrome for SPA pages |
| AI | `google/generative-ai-go` | Official Gemini SDK |
| gRPC | `google.golang.org/grpc` | Fast binary RPC for inter-service |
| Telemetry | `go.opentelemetry.io/otel` + `prometheus/client_golang` | Standard observability |
| Testing | `stretchr/testify` | Assertions + mocking |

### 3.4 Dependency Injection Pattern

Go avoids heavy DI frameworks (no Wire, no fx) to keep binaries lean and startup fast. Instead, use the **functional options pattern** throughout:

```go
// Option is a functional option for service constructors
type Option func(*CrawlerService) *CrawlerService

// WithRedis applies a custom Redis client
func WithRedis(r RedisCache) Option {
    return func(s *CrawlerService) *CrawlerService {
        s.redis = r
        return s
    }
}

// WithMongo applies a custom MongoDB client
func WithMongo(m MongoDB) Option {
    return func(s *CrawlerService) *CrawlerService {
        s.mongo = m
        return s
    }
}

// NewCrawlerService constructs the service, applying all options
func NewCrawlerService(cfg Config, opts ...Option) *CrawlerService {
    svc := &CrawlerService{cfg: cfg}
    for _, opt := range opts {
        svc = opt(svc)
    }
    return svc
}
```

Each `main.go` manually builds its own service container — passing interfaces (not concrete types) as dependencies:

```go
// cmd/crawler-service/main.go
func main() {
    cfg := config.Load()

    logger := log.New(cfg)
    mongo := mongodriver.New(cfg.GetString("mongo.uri"))
    redis := redisclient.New(cfg.GetString("redis.addr"))
    queue := asynq.NewClient(redis)

    crawler := services.NewCrawlerService(cfg,
        services.WithMongo(mongo),
        services.WithRedis(redis),
        services.WithQueue(queue),
    )

    r := chi.NewRouter()
    r.Use(middleware.Recovery(logger))
    r.Use(middleware.RequestID)
    r.Handle("/crawl", crawlerHandler(crawler))

    http.ListenAndServe(":8080", r)
}
```

**Rules:**
- Constructors accept interface parameters (not concrete structs).
- Concrete implementations are instantiated in `main.go` only.
- Interfaces are defined in `pkg/*/interfaces.go` and kept small (≤ 5 methods).

### 3.5 Shared Middleware Stack (chi router)

```go
// Standard middleware chain, outermost → innermost:
// Recovery → RequestID → RealIP → Logger → CORS → RateLimit → Auth → Handler

r := chi.NewRouter()

// Global panic recovery (must be first)
r.Use(middleware.Recovery)

// Request context enrichment
r.Use(middleware.RequestID)
r.Use(middleware.RealIP)

// Structured request logging (zerolog)
r.Use(middleware.RequestLogger(&zerolog.Logger{}))

// CORS — allow all origins in dev; restrict in prod via env
r.Use(cors.Handler(cors.Options{
    AllowedOrigins:   []string{"*"}, // tighten in production
    AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
    AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
    AllowCredentials: true,
    MaxAge:           300,
}))

// Rate limiting per IP (in-memory) or Redis-backed for distributed deployments
r.Use(ratelimit.New(
    ratelimit.NewStore(redis),
    &ratelimit.Options{
        Max:      100,
        Duration: time.Minute,
    },
))

// JWT authentication (only on protected routes)
r.Group(func(protected chi.Router) {
    protected.Use(auth.JWTMiddleware(jwtSecret))
    protected.HandleFunc("/admin", adminHandler)
})

// Health checks are unauthenticated
r.GetFunc("/healthz", healthz)
r.GetFunc("/ready", ready)

// Mount service routers
r.Mount("/crawl", crawlerRouter)
r.Mount("/notify", notificationRouter)
r.Mount("/trending", trendingRouter)
r.Mount("/bot", botRouter)
```

### 3.6 Inter-Service Communication

Three communication patterns, chosen by latency and coupling requirements:

**1. Synchronous — REST over HTTP (chi router)**
- Used for: request/response calls where the caller waits.
- All HTTP calls include a 5s timeout and 3 retries with exponential backoff (jittered):

```go
// pkg/http/client.go — shared HTTP client with built-in retry
func (c *Client) DoWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
    var lastErr error
    for attempt := 0; attempt <= 3; attempt++ {
        if attempt > 0 {
            backoff := time.Duration(math.Pow(2, float64(attempt)))*time.Second + time.Duration(rand.Intn(1000))*time.Millisecond
            select {
            case <-ctx.Done():
                return nil, ctx.Err()
            case <-time.After(backoff):
            }
        }
        resp, err := c.Do(req)
        if err == nil && resp.StatusCode < 500 {
            return resp, nil
        }
        lastErr = err
    }
    return nil, fmt.Errorf("http request failed after 3 retries: %w", lastErr)
}
```

**2. Asynchronous — Asynq job queue + Redis Pub/Sub**
- Used for: fire-and-forget tasks, background processing, fan-out notifications.
- Asynq handles retries, dead-letter queues (DLQ), priorities (1–10), and scheduled execution.

```go
// Enqueue a high-priority crawl job
task := asynq.NewTask(jobs.TypeCrawlJob, payload)
_, err = queue.Enqueue(ctx, task, asynq.MaxRetry(5), asynq.Timeout(10*time.Minute), asynq.Queue("high"))
```

**3. Event Fan-Out — Redis Pub/Sub + local event bus**
- Used for: decoupled, multi-subscriber notifications (one event, many handlers).

```go
// pkg/event/bus.go
// Publish a local event (in-process subscribers fire synchronously)
bus.PublishLocal("crawl.success", &CrawlSuccessEvent{URL: url, Title: title})

// Publish a cross-service event (Redis pub/sub)
bus.Publish(ctx, "events:crawl.success", jsonPayload)

// Subscribe — returns cancel function
cancel := bus.SubscribeLocal("crawl.success", func(evt interface{}) {
    // handle event
})
defer cancel()
```

**4. gRPC (optional)**
- Used for: high-frequency internal calls between tightly coupled services (e.g., crawler ↔ notification for real-time alerts).
- Proto definitions in `proto/events.proto`; generate with `protoc`.
- Only deploy gRPC when the overhead of JSON/HTTP becomes a bottleneck.

### 3.7 Error Handling Standard

Go errors are values. The codebase follows a strict error-wrapping convention using `pkg/errors` (based on panicking `fmt.Errorf` with `%w` — no runtime overhead):

```go
// Domain-specific error types (not generic errors.New)
var (
    ErrFeedNotFound     = errors.New("feed not found")
    ErrDuplicateURL     = errors.New("duplicate URL")
    ErrQualityTooLow    = errors.New("content quality below threshold")
    ErrRateLimited      = errors.New("provider rate limit exceeded")
    ErrBlockDetected    = errors.New("anti-bot block detected")
)

// Wrapping with stack traces at service boundaries
func (s *CrawlerService) Fetch(ctx context.Context, url string) (*Result, error) {
    result, err := s.fetcher.Fetch(ctx, url)
    if err != nil {
        // Wrap at every boundary; stack trace preserved
        return nil, fmt.Errorf("CrawlerService.Fetch(%s): %w", url, err)
    }
    return result, nil
}
```

**Structured error response** (all API errors return this JSON shape):

```go
// pkg/http/server.go
type APIError struct {
    Code      string      `json:"code"`
    Message   string      `json:"message"`
    RequestID string      `json:"request_id"`
    Details   interface{} `json:"details,omitempty"`
}

func WriteError(w http.ResponseWriter, r *http.Request, status int, err error) {
    reqID := middleware.GetReqID(r.Context())
    var details interface{}
    var code string

    switch {
    case errors.Is(err, ErrFeedNotFound):
        code = "FEED_NOT_FOUND"
        details = map[string]string{"feed_url": extractURL(err)}
    case errors.Is(err, ErrRateLimited):
        code = "RATE_LIMITED"
        details = map[string]any{"retry_after_seconds": 60}
    default:
        code = "INTERNAL_ERROR"
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(APIError{
        Code:      code,
        Message:   err.Error(),
        RequestID: reqID,
        Details:   details,
    })
}
```

---

## 4. BOT Service Extraction

### 4.1 NestJS Source Files to Port

| NestJS File | Go Target |
|---|---|
| `src/modules/bot/bot.module.ts` | `cmd/bot-service/main.go` |
| `src/modules/bot/bot.controller.ts` | `internal/handlers/bot_controller.go` |
| `src/modules/bot/entities/bot-conversation.entity.ts` | `internal/models/bot_conversation.go` |
| `src/modules/bot/entities/bot-linked-account.entity.ts` | `internal/models/bot_linked_account.go` |
| `src/modules/bot/entities/bot-workflow.entity.ts` | `internal/models/bot_workflow.go` |
| `src/modules/bot/services/bot-command-handler.service.ts` | `internal/services/command_handler.go` |
| `src/modules/bot/services/bot-conversation.service.ts` | `internal/services/conversation.go` |
| `src/modules/bot/services/bot-link.service.ts` | `internal/services/link.go` |
| `src/modules/bot/services/bot-permission.service.ts` | `internal/middleware/permission.go` |
| `src/modules/bot/webhooks/discord-webhook.controller.ts` | `internal/handlers/discord_webhook.go` |
| `src/modules/bot/webhooks/telegram-webhook.controller.ts` | `internal/handlers/telegram_webhook.go` |

### 4.2 Go File Structure

```
cmd/bot-service/
├── main.go                       # Entry: chi router, graceful shutdown, options-based DI
├── wire.go                       # Service container constructor
internal/
├── models/
│   ├── bot_conversation.go       # MongoDB: user_id, platform, state, context (BSON)
│   ├── bot_linked_account.go     # MongoDB: platform_user_id, internal_user_id, link_code
│   ├── bot_workflow.go           # MongoDB: workflow_steps, current_step, status
│   └── bot_command.go            # In-memory: command registry map
├── services/
│   ├── command_handler.go        # Route commands to handlers, permission check
│   ├── conversation.go           # Multi-step wizard: state machine, context memory
│   ├── workflow.go               # Step execution, resume, branching
│   └── link.go                    # 6-char code generation, expiry, verification
├── commands/
│   ├── base.go                    # Command interface
│   ├── rss_commands.go            # /rss add, /rss list, /rss remove
│   ├── crawl_commands.go          # /crawl start, /crawl status, /crawl stop
│   ├── trending_commands.go       # /trending top, /trending keyword
│   ├── draft_commands.go          # /draft list, /draft publish
│   ├── stats_commands.go          # /stats users, /stats crawler
│   └── system_commands.go         # /system health, /system reload
├── handlers/
│   ├── discord_webhook.go         # POST /webhooks/discord (HMAC-SHA256 verify)
│   ├── telegram_webhook.go        # POST /webhooks/telegram (HMAC verify)
│   └── bot_controller.go          # REST API: GET /conversations, POST /link
├── middleware/
│   └── permission.go              # RBAC: viewer/editor/crawler/moderator/admin
└── platform/
    ├── discord.go                 # Discord API client
    └── telegram.go               # Telegram Bot API client
```

### 4.3 Porting Notes for Key Logic

**BotCommandHandler (36+ commands)**
NestJS class methods map → Go function map:

```go
// commands/registry.go
type CommandHandler func(ctx context.Context, update *PlatformUpdate) string

// Command registry: string → handler function
var commandRegistry = map[string]CommandHandler{
    "rss add":      HandleRSSAdd,
    "rss list":     HandleRSSList,
    "rss remove":   HandleRSSRemove,
    "crawl start":  HandleCrawlStart,
    "crawl status": HandleCrawlStatus,
    "crawl stop":   HandleCrawlStop,
    "trending top": HandleTrendingTop,
    "system health": HandleSystemHealth,
    // ... all 36+ commands
}

// Dispatcher
func (s *CommandService) Handle(ctx context.Context, update *PlatformUpdate) string {
    handler, ok := s.registry[update.Command]
    if !ok {
        return "Unknown command. Type /help for available commands."
    }
    // Permission check before execution
    if err := s.permission.Check(ctx, update.UserID, update.Command); err != nil {
        return fmt.Sprintf("⛔ %s", err.Error())
    }
    return handler(ctx, update)
}
```

**Conversation wizard state machine**

```go
// services/conversation.go — thread-safe conversation wizard
type ConversationService struct {
    mongo   MongoDB
    redis   RedisCache
    mu      sync.RWMutex // guards in-memory wizard state
    wizards map[string]*WizardState // platform_conversation_id → state
}

type WizardState struct {
    Step       string
    Data       map[string]string
    StartedAt  time.Time
    ExpiresAt  time.Time
}

// AdvanceStep validates input and transitions to the next step
func (s *ConversationService) AdvanceStep(ctx context.Context, convID string, input string) (string, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    wizard, ok := s.wizards[convID]
    if !ok {
        return "", ErrNoActiveWizard
    }

    // Persist to MongoDB for durability across restarts
    wizard.Data[wizard.Step] = input
    wizard.Step = nextStep(wizard.Step)
    wizard.ExpiresAt = time.Now().Add(5 * time.Minute)

    if err := s.persistWizard(ctx, convID, wizard); err != nil {
        return "", fmt.Errorf("persist wizard: %w", err)
    }

    return s.renderPrompt(wizard.Step), nil
}
```

**Permission levels**

```go
// middleware/permission.go — RBAC roles
const (
    PermissionViewer    = "viewer"    // 1 — read-only
    PermissionEditor    = "editor"    // 2 — can manage drafts
    PermissionCrawler   = "crawler"    // 3 — can trigger crawls
    PermissionModerator = "moderator"  // 4 — can blacklist
    PermissionAdmin     = "admin"      // 5 — full access
)

// Command → minimum required permission
var commandPermissions = map[string]int{
    "rss add":       PermissionCrawler,
    "crawl start":   PermissionCrawler,
    "system reload": PermissionAdmin,
    // ...
}

func (p *PermissionService) Check(ctx context.Context, userID, command string) error {
    userPerm := p.getUserPermission(ctx, userID) // from MongoDB user record
    required := commandPermissions[command]
    if userPerm < required {
        return fmt.Errorf("%w: requires level %d, you have %d", ErrForbidden, required, userPerm)
    }
    return nil
}
```

**Account linking (Redis 6-char code)**

```go
// services/link.go — 6-char alphanumeric code, 5-min TTL
func (s *LinkService) CreateLinkCode(ctx context.Context, userID string) (string, error) {
    code := generateCode(6) // e.g. "A3K9XM"

    // Store in Redis: SETEX bot:link:{code} {user_id} 300
    key := fmt.Sprintf("bot:link:%s", code)
    if err := s.redis.Set(ctx, key, userID, 5*time.Minute); err != nil {
        return "", fmt.Errorf("set link code: %w", err)
    }
    return code, nil
}

func generateCode(n int) string {
    const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // omit confusing chars: 0/O, 1/I
    b := make([]byte, n)
    for i := range b {
        b[i] = charset[rand.Intn(len(charset))]
    }
    return string(b)
}
```

**Discord HMAC verification**

```go
// handlers/discord_webhook.go
// Discord signs requests with Ed25519 (X-Signature-Ed25519) or HMAC-SHA256 (X-Hub-Signature-256)
func verifyDiscordRequest(body []byte, signature, timestamp string, secret string) bool {
    // Ed25519 (preferred by Discord)
    if signature != "" {
        msg := []byte(timestamp) + body
        return ed25519.Verify(pubKey, msg, decodedSignature)
    }

    // HMAC-SHA256 fallback (X-Hub-Signature-256 = "sha256=<hex>")
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write([]byte(timestamp))
    mac.Write(body)
    expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(expected), []byte(signature))
}
```

**Telegram HMAC verification**

```go
// handlers/telegram_webhook.go
// Telegram verifies by computing HMAC-SHA256 of data-check-string using bot token as secret
func verifyTelegramRequest(dataCheckString, secret, hash string) bool {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write([]byte(dataCheckString))
    return hmac.Equal([]byte(mac.String()), []byte(hash))
}
```

### 4.4 API Endpoints

```
GET  /healthz                      → Health check
POST /webhooks/discord             → Discord incoming events (DM, message, interaction)
POST /webhooks/telegram             → Telegram webhook (message, callback_query)
POST /link                         → Create account link (6-char code)
GET  /link/:code                   → Verify account link
GET  /conversations                → List active conversations (admin)
POST /conversations/:id/send       → Send message to conversation
```

### 4.5 Health Check

```go
// handlers/health.go
type HealthResponse struct {
    Status  string `json:"status"`
    Mongo   string `json:"mongo"`
    Redis   string `json:"redis"`
    Uptime  string `json:"uptime"`
}

func healthz(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
    defer cancel()

    status := "ok"
    var parts []string

    if err := s.mongo.Ping(ctx); err != nil {
        status = "degraded"
        parts = append(parts, fmt.Sprintf("mongo:%v", err))
    } else {
        parts = append(parts, "mongo:ok")
    }

    if _, err := s.redis.Ping(ctx); err != nil {
        status = "degraded"
        parts = append(parts, fmt.Sprintf("redis:%v", err))
    } else {
        parts = append(parts, "redis:ok")
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(HealthResponse{
        Status: status,
        Mongo:  "ok",
        Redis:  "ok",
        Uptime: time.Since(startedAt).Round(time.Second).String(),
    })
}
```

---

## 5. Notification Service Extraction

### 5.1 NestJS Source Files to Port

| NestJS File | Go Target |
|---|---|
| `src/modules/notifications/notifications.module.ts` | `cmd/notification-service/main.go` |
| `src/modules/notifications/notifications.service.ts` | `internal/services/notification.go` |
| `src/modules/notifications/notifications.controller.ts` | `internal/handlers/notification_controller.go` |
| `src/modules/notifications/digest-scheduler.service.ts` | `internal/services/digest.go` |
| `src/modules/notifications/services/notification-bus.service.ts` | `internal/event/consumer.go` |
| `src/modules/notifications/services/notification-templates.service.ts` | `internal/templates/` |
| `src/modules/notifications/providers/discord.provider.ts` | `internal/providers/discord.go` |
| `src/modules/notifications/providers/telegram.provider.ts` | `internal/providers/telegram.go` |
| `src/modules/notifications/providers/whatsapp.provider.ts` | `internal/providers/whatsapp.go` |
| `src/modules/notifications/webhooks/discord-webhook.controller.ts` | `internal/handlers/webhook_controller.go` |
| `src/modules/notifications/webhooks/telegram-webhook.controller.ts` | `internal/handlers/webhook_controller.go` |
| `src/modules/notifications/webhooks/whatsapp-webhook.controller.ts` | `internal/handlers/webhook_controller.go` |
| `src/modules/notifications/entities/notification.entity.ts` | `internal/models/notification.go` |

### 5.2 Go File Structure

```
cmd/notification-service/
├── main.go
├── wire.go
internal/
├── models/
│   ├── notification.go             # MongoDB: recipient, channel, template, status, metadata
│   ├── notification_preference.go  # Per-user channel settings
│   └── notification_template.go    # Vietnamese template definitions
├── services/
│   ├── notification.go            # Send, BatchSend, Cancel, Resend
│   ├── template_renderer.go         # Go text/template with Vietnamese interpolation
│   ├── digest.go                   # Daily/weekly/monthly digest aggregation
│   └── delivery_tracker.go         # Retry logic, exponential backoff, receipt
├── providers/
│   ├── discord.go                  # Discord webhook embeds (rate limit: 200/min)
│   ├── telegram.go                 # Telegram sendMessage, editMessageText
│   └── whatsapp.go                # WhatsApp Business API
├── event/
│   └── consumer.go                 # Redis pub/sub subscriber, Asynq job enqueuer
└── handlers/
    ├── notification_controller.go
    ├── channel_controller.go       # Connect/disconnect channels
    └── webhook_controller.go
```

### 5.3 Provider Interface (Key)

```go
// providers/interfaces.go
type ChannelType string

const (
    ChannelDiscord  ChannelType = "discord"
    ChannelTelegram ChannelType = "telegram"
    ChannelWhatsApp ChannelType = "whatsapp"
    ChannelEmail    ChannelType = "email"
)

type Notifier interface {
    Send(ctx context.Context, msg *Notification) error
    Supports(channel ChannelType) bool
    Name() string
    RateLimit() (requestsPerMinute int, retryAfter time.Duration)
}

// NotificationService routes to the correct provider
type NotificationService struct {
    providers       []Notifier
    templateRenderer *TemplateRenderer
    deliveryTracker *DeliveryTracker
    logger          *log.Logger
}

func (s *NotificationService) Send(ctx context.Context, msg *Notification) error {
    // Render template first
    body, err := s.templateRenderer.Render(msg.Template, msg.Data)
    if err != nil {
        return fmt.Errorf("render template %s: %w", msg.Template, err)
    }
    msg.Body = body

    // Route to appropriate provider
    for _, p := range s.providers {
        if p.Supports(msg.Channel) {
            if err := p.Send(ctx, msg); err != nil {
                s.logger.Error().Err(err).Str("provider", p.Name()).Str("channel", string(msg.Channel)).Send()
                return fmt.Errorf("provider %s: %w", p.Name(), err)
            }
            return nil
        }
    }
    return ErrNoProviderForChannel
}

// BatchSend fans out to all user-preferred channels concurrently
func (s *NotificationService) BatchSend(ctx context.Context, msgs []*Notification) []error {
    var wg sync.WaitGroup
    errs := make([]error, len(msgs))

    for i, msg := range msgs {
        i, msg := i, msg
        wg.Add(1)
        go func() {
            defer wg.Done()
            errs[i] = s.Send(ctx, msg)
        }()
    }
    wg.Wait()
    return errs
}
```

### 5.4 Vietnamese Notification Templates

Port ALL templates from TypeScript. Key examples:

```go
// templates/vietnamese.go
package templates

import "strings"

var (
    formatCrawlSuccess = `🎉 Crawl thành công!
📰 {{.Title}}
🌐 {{.URL}}
⏱ Thời gian: {{.Duration}}`

    formatCrawlFailed = `❌ Crawl thất bại!
🌐 {{.URL}}
⚠️ Lý do: {{.Error}}
🔄 Thử lại: {{.RetryAt}}`

    formatHotTopicAlert = `🔥 Topic hot: {{.Topic}}
📊 Volume: {{.Volume}}
🔗 {{.URL}}`

    formatDailyDigest = `📬 Daily Digest — {{.Date}}
Bài viết nổi bật trong ngày:
{{range .Items}}• {{.}}
{{end}}
Xem thêm: {{.DashboardURL}}`

    formatSystemAlert = `⚠️ {{.AlertType}}
{{.Message}}
🕐 {{.Timestamp}}`

    formatQueueStatus = `📊 Queue Status
⏳ Depth: {{.Depth}}
⚡ Processing: {{.ProcessingRate}}/min
❌ Errors: {{.ErrorRate}}/min`

    formatRssAdded = `✅ RSS đã thêm!
📡 {{.FeedName}}
🔗 {{.FeedURL}}`
)

// TemplateRenderer renders Go templates with Vietnamese interpolation
type TemplateRenderer struct {
    funcs map[string]func(string) string
}

func (r *TemplateRenderer) Render(template string, data map[string]string) (string, error) {
    tmpl, err := template.New("").Parse(template)
    if err != nil {
        return "", err
    }
    var buf strings.Builder
    if err := tmpl.Execute(&buf, data); err != nil {
        return "", err
    }
    return buf.String(), nil
}
```

### 5.5 Event Consumer

```go
// event/consumer.go — Redis pub/sub + Asynq bridge
type EventConsumer struct {
    bus        *event.Bus
    queue      *asynq.Client
    dispatcher *NotificationDispatcher
}

func NewEventConsumer(bus *event.Bus, queue *asynq.Client) *EventConsumer {
    c := &EventConsumer{bus: bus, queue: queue}

    // Subscribe to all event topics
    topics := []string{
        "events:crawl.success",
        "events:crawl.failed",
        "events:trending.alert",
        "events:system.warning",
        "events:queue.status",
        "events:rss.added",
    }

    for _, topic := range topics {
        if err := bus.Subscribe(topic, c.handleEvent); err != nil {
            panic(fmt.Sprintf("subscribe to %s: %v", topic, err))
        }
    }

    return c
}

func (c *EventConsumer) handleEvent(payload []byte) {
    var event Event
    if err := json.Unmarshal(payload, &event); err != nil {
        return
    }

    // Enqueue Asynq job "send-notification" with event priority
    task := asynq.NewTask(jobs.TypeSendNotification, payload)
    priority := asynq.PriorityHigh
    if _, err := c.queue.Enqueue(context.Background(), task, asynq.Queue(string(priority))); err != nil {
        c.dispatcher.logger.Error().Err(err).Str("topic", event.Topic).Send()
    }
}
```

### 5.6 API Endpoints

```
GET  /healthz
GET  /notifications                 → List notifications (paginated)
GET  /notifications/:id             → Get notification detail
POST /notifications/send            → Send a notification
POST /notifications/batch           → Batch send
GET  /notifications/preferences    → Get user preferences
PUT  /notifications/preferences    → Update preferences
POST /channels/discord/test        → Test Discord webhook
POST /channels/telegram/test       → Test Telegram bot
POST /channels/whatsapp/test        → Test WhatsApp
GET  /channels/status               → All channel connection status
```

---

## 6. Crawler Service Extraction

### 6.1 NestJS Source Files to Port

| NestJS File | Go Target |
|---|---|
| `crawler.module.ts` | `cmd/crawler-service/main.go` |
| `crawler.service.ts` | `internal/services/crawler_orchestrator.go` |
| `crawler.processor.ts` | `internal/jobs/crawl_processor.go` |
| `crawler.scheduler.ts` | `internal/services/feed_scheduler.go` |
| `crawler.controller.ts` | `internal/handlers/crawler_controller.go` |
| `blacklist.controller.ts` | `internal/handlers/blacklist_controller.go` |
| `gateways/crawl-progress.gateway.ts` | `internal/handlers/sse_gateway.go` |
| `entities/rss-feed.entity.ts` | `internal/models/rss_feed.go` |
| `entities/scraper-config.entity.ts` | `internal/models/scraper_config.go` |
| `entities/crawl-history.entity.ts` | `internal/models/crawl_history.go` |
| `entities/domain-reputation.entity.ts` | `internal/models/domain_reputation.go` |
| `entities/content-fingerprint.entity.ts` | `internal/models/content_fingerprint.go` |
| `entities/content-blacklist.entity.ts` | `internal/models/content_blacklist.go` |
| `services/anti-block.service.ts` | `internal/services/anti_block.go` |
| `services/robots-parser.service.ts` | `internal/services/robots_parser.go` |
| `services/quality-gate.service.ts` | `internal/services/quality_gate.go` |
| `services/content-dedup.service.ts` | `internal/services/content_dedup.go` |
| `services/smart-selector.service.ts` | `internal/services/smart_selector.go` |
| `services/sitemap.service.ts` | `internal/services/sitemap.go` |
| `services/blacklist.service.ts` | `internal/services/blacklist.go` |

### 6.2 Go File Structure

```
cmd/crawler-service/
├── main.go                      # Entry point: chi server + Asynq workers + SSE
├── wire.go
internal/
├── models/                      # MongoDB documents with bson tags
│   ├── rss_feed.go              # URL, category, language, frequency, last_fetch, status
│   ├── scraper_config.go        # domain → CSS selectors mapping
│   ├── crawl_history.go         # url, status, score, duration, errors, items_discovered
│   ├── domain_reputation.go     # domain, success_rate, block_count, last_seen
│   ├── content_fingerprint.go   # simhash uint64, sha256, url, created_at
│   └── content_blacklist.go     # type(url/domain/keyword), pattern, reason, active
├── services/
│   ├── orchestrator.go          # Main crawl pipeline: discover → fetch → score → dedup → store → notify
│   ├── feed_fetcher.go          # Concurrent RSS/Atom/JSON fetch with ETag/Last-Modified
│   ├── scraper.go               # HTML fetch: robots.txt → proxy → UA → delay → fetch
│   ├── anti_block.go            # Proxy pool, UA rotation, adaptive delay, backoff
│   ├── robots_parser.go         # robots.txt respect list
│   ├── quality_gate.go          # 8-rule scoring (0-100): ≥70 = publishable
│   ├── content_dedup.go         # SimHash (FNV-1a) + exact SHA-256 dedup
│   ├── smart_selector.go        # Gemini AI → CSS selector suggestions per domain
│   ├── sitemap.go               # XML sitemap discovery + recursive crawl
│   └── blacklist.go             # URL/domain/keyword blocklist (Aho-Corasick)
├── jobs/
│   ├── crawl_job.go             # Asynq job: {url, depth, config_id, priority}
│   ├── refresh_feed_job.go      # Periodic feed refresh
│   └── reindex_job.go           # Algorithm update re-fingerprinting
├── handlers/
│   ├── crawler_controller.go    # REST API
│   ├── blacklist_controller.go
│   ├── sse_gateway.go           # Server-Sent Events for real-time crawl progress
│   └── feed_controller.go       # RSS feed CRUD
└── sse/
    └── broadcaster.go          # Thread-safe channel map for SSE connections
```

### 6.3 Quality Gate — 8 Rules (Port from TypeScript)

```go
// services/quality_gate.go — 8-rule scoring, each rule contributes up to 12.5 pts → total 0–100
type QualityScore struct {
    Total       float64
    Length      float64  // ≥500 words = 12.5, <200 = 0, linear between
    Originality float64  // No keyword stuffing; AI-detect score >0.7 = 0
    Freshness   float64  // Published ≤30 days = 12.5, >90 days = 0
    Readability float64  // Flesch reading ease ≥60 = 12.5
    Media       float64  // Has image = 6, has alt text = 6.5
    Structure   float64  // Has H1-H6 headings = 7.5, has lists = 5
    SpamSignals float64  // Spam keyword hit = −12.5, ads-heavy = −12.5

    // Threshold: ≥70 = publishable, <70 = reject
}

const PublishThreshold = 70.0

func (q *QualityGate) Score(ctx context.Context, html string) (*QualityScore, error) {
    doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
    if err != nil {
        return nil, fmt.Errorf("parse HTML: %w", err)
    }

    text := doc.Text()
    s := &QualityScore{}

    s.Length = q.scoreLength(text)
    s.Originality = q.scoreOriginality(text)
    s.Freshness = q.scoreFreshness(doc)
    s.Readability = q.scoreReadability(text)
    s.Media = q.scoreMedia(doc)
    s.Structure = q.scoreStructure(doc)
    s.SpamSignals = q.scoreSpamSignals(text, doc)

    s.Total = s.Length + s.Originality + s.Freshness + s.Readability +
              s.Media + s.Structure + s.SpamSignals

    return s, nil
}

func (q *QualityGate) ShouldPublish(score *QualityScore) bool {
    return score.Total >= PublishThreshold
}

// Rule implementations
func (q *QualityGate) scoreLength(text string) float64 {
    words := len(strings.Fields(text))
    switch {
    case words >= 500:
        return 12.5
    case words < 200:
        return 0
    default:
        return float64(words-200) / 300 * 12.5
    }
}

func (q *QualityGate) scoreFreshness(doc *goquery.Document) float64 {
    dateStr := doc.Find(`meta[property="article:published_time"]`).AttrOr("content", "")
    if dateStr == "" {
        return 6.25 // neutral — no date found
    }
    pubDate, err := time.Parse(time.RFC3339, dateStr)
    if err != nil {
        return 6.25
    }
    daysOld := time.Since(pubDate).Hours() / 24
    switch {
    case daysOld <= 30:
        return 12.5
    case daysOld <= 90:
        return 6.25
    default:
        return 0
    }
}
```

### 6.4 SimHash Content Deduplication (Port from TypeScript)

```go
// services/content_dedup.go
// Ported from TypeScript SimHash/FNV-1a + fastest-levenshtein
// Go version using the same algorithm: Hamming distance ≤ threshold (default 6) = duplicate

import (
    "crypto/sha256"
    "hash/fnv"
    "math/bits"
    "strings"
    "unicode"
)

type DedupService struct {
    mongo     MongoDB
    redis     RedisCache
    threshold uint64
}

// ComputeSimHash produces a 64-bit SimHash fingerprint using FNV-1a trigram hashing
func (s *DedupService) ComputeSimHash(content string) (uint64, error) {
    tokens := tokenize(content)
    trigrams := makeTrigrams(tokens)

    var v0, v1 uint64 = 0, 0 // 128-bit accumulator (two uint64s)

    for i, tg := range trigrams {
        h := fnv1aHash(tg)

        // Accumulate into 128 bits
        if i%2 == 0 {
            v0 += h
        } else {
            v1 += h
        }
    }

    // Threshold each bit: ≥50% of positions → 1, else 0
    fingerprint := (v0 & 0xFFFFFFFF) ^ (v1 & 0xFFFFFFFF00000000)
    return fingerprint, nil
}

func tokenize(text string) []string {
    return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
        return !unicode.IsLetter(r) && !unicode.IsNumber(r)
    })
}

func makeTrigrams(tokens []string) []string {
    var result []string
    for i := 0; i+2 < len(tokens); i++ {
        result = append(result, strings.Join(tokens[i:i+3], " "))
    }
    return result
}

func fnv1aHash(s string) uint64 {
    h := fnv.New64a()
    h.Write([]byte(s))
    return h.Sum64()
}

// IsDuplicate checks both exact SHA-256 match and SimHash Hamming distance
func (s *DedupService) IsDuplicate(ctx context.Context, content string) (bool, error) {
    // Step 1: Exact SHA-256 match → immediate reject
    shaHash := sha256.Sum256([]byte(content))
    if s.exactMatch(ctx, shaHash[:]) {
        return true, nil
    }

    // Step 2: SimHash Hamming distance
    fingerprint, err := s.ComputeSimHash(content)
    if err != nil {
        return false, err
    }

    // Fetch stored fingerprints (optimized: bucket by top 16 bits)
    bucket := fingerprint >> 48
    candidates, err := s.mongoFetchBucket(ctx, bucket)
    if err != nil {
        return false, err
    }

    for _, stored := range candidates {
        dist := bits.OnesCount64(fingerprint ^ stored)
        if dist <= s.threshold {
            return true, nil
        }
    }
    return false, nil
}

func (s *DedupService) exactMatch(ctx context.Context, shaHash []byte) bool {
    col := s.mongo.Collection("content_fingerprints")
    count, _ := col.CountDocuments(ctx, bson.M{"sha256": bson.E{Key: "$in", Value: shaHash}})
    return count > 0
}

func (s *DedupService) mongoFetchBucket(ctx context.Context, bucket uint64) ([]uint64, error) {
    col := s.mongo.Collection("content_fingerprints")
    cursor, err := col.Find(ctx, bson.M{"bucket": bucket})
    if err != nil {
        return nil, err
    }
    defer cursor.Close(ctx)

    var fingerprints []uint64
    for cursor.Next(ctx) {
        var doc struct {
            Simhash uint64 `bson:"simhash"`
        }
        if err := cursor.Decode(&doc); err != nil {
            continue
        }
        fingerprints = append(fingerprints, doc.Simhash)
    }
    return fingerprints, nil
}
```

### 6.5 Crawl Pipeline (V2 — Port)

```
1. URL Discovery         → FeedFetcher (RSS), SitemapParser, TrendingService feed
2. Blacklist Check       → BlacklistChecker (URL/domain/keyword Aho-Corasick)
3. Domain Reputation     → DomainReputationService (skip blocked domains)
4. Robots.txt Check      → RobotsParser (respect crawl-delay, allow paths)
5. Anti-Block            → AntiBlockService (proxy, UA rotation, delay)
6. Fetch Content         → ScraperService (goquery → chromedp fallback)
7. Quality Gate          → QualityGate (8-rule score ≥70)
8. Content Dedup         → DedupService (SimHash, Hamming ≤6 = dup)
9. AI SEO (parallel)     → SmartSelector + Gemini AI (titles, meta, alt-texts)
10. Save Result           → MongoDB (CrawlHistory + Fingerprint)
11. Notify                → Publish event "crawl.success" or "crawl.failed" → NotificationBus
12. SSE Broadcast         → CrawlProgressGateway → connected web clients
```

### 6.6 API Endpoints

```
GET  /healthz, /ready
GET  /crawl/stats              → Queue depth, crawl rate, success rate
GET  /crawl/ai-quota           → Gemini API usage
GET  /crawl/quality-stats      → Average quality score, pass/fail ratio
GET  /crawl/dedup-stats        → Dedup hit rate, avg processing time
GET  /crawl/history            → Paginated crawl history
POST /crawl/url                → Enqueue single URL for immediate crawl
POST /crawl/batch              → Enqueue multiple URLs
GET  /crawl/stream/:job_id     → SSE real-time crawl progress
GET  /rss/feeds                → List RSS feeds
POST /rss/feeds                → Add RSS feed
PUT  /rss/feeds/:id            → Update feed
DELETE /rss/feeds/:id          → Remove feed
POST /rss/sync                 → Trigger manual feed sync
GET  /rss/preview/:url         → Preview feed without saving
GET  /configs                  → List scraper configs
POST /configs                  → Add scraper config (domain → CSS selectors)
PUT  /configs/:id              → Update config
POST /configs/test             → Test CSS selector against URL
GET  /sitemap/discover         → Discover sitemaps for a domain
GET  /sitemap/parse            → Parse sitemap and return URLs
GET  /blacklist                → List blacklist entries
POST /blacklist                → Add entry
DELETE /blacklist/:id          → Remove entry
```

### 6.7 Asynq Job Types

```go
// internal/jobs/types.go
const (
    TypeCrawlJob       = "crawl:run"
    TypeRefreshFeedJob = "crawl:refresh_feed"
    TypeReindexJob     = "crawl:reindex"
)

// CrawlJobPayload — the payload for a crawl:run Asynq task
type CrawlJobPayload struct {
    URL      string `json:"url"`
    Depth    int    `json:"depth"`
    ConfigID string `json:"config_id"`
    Priority int    `json:"priority"` // 1 = high, 5 = low
    Source   string `json:"source"`   // "rss", "sitemap", "manual", "trending"
}

// Asynq job handler registration
func RegisterHandlers(sc *asynq.Server) {
    sc.Handle(TypeCrawlJob, HandleCrawlJob)
    sc.Handle(TypeRefreshFeedJob, HandleRefreshFeedJob)
    sc.Handle(TypeReindexJob, HandleReindexJob)
}

func HandleCrawlJob(ctx context.Context, t *asynq.Task) error {
    var payload CrawlJobPayload
    if err := json.Unmarshal(t.Payload(), &payload); err != nil {
        return fmt.Errorf("unmarshal crawl payload: %w", err)
    }
    return crawlOrchestrator.Run(ctx, &payload)
}
```

---

---

## 7. Trending Service Extraction

### 7.1 NestJS Source → Go Porting Map

| NestJS File | Go Equivalent | Porting Notes |
|---|---|---|
| `src/modules/trending/trending.module.ts` | `cmd/trending-service/main.go` | chi server, robfig/cron |
| `src/modules/trending/trending.service.ts` | `internal/services/aggregator.go` | Merge + rank + dedupe all sources |
| `src/modules/trending/trending.scheduler.ts` | `internal/services/scheduler.go` | Every 30 min via `robfig/cron` |
| Entities (Topic, NewsArticle, Snapshot) | `internal/models/` | MongoDB BSON documents |

Trending module is the leanest extraction — focus on correct cron scheduling and URL discovery feed for crawler-service.

### 7.2 Go File Structure

```
cmd/trending-service/
├── main.go                      # Entry: chi server, cron scheduler, graceful shutdown
internal/
├── models/
│   ├── trending_topic.go        # topic, score, volume, source, keywords[], timestamp
│   ├── news_article.go          # headline, source, url, published_at, relevance_score
│   └── trending_snapshot.go     # Point-in-time snapshot for historical trend charts
├── services/
│   ├── google_trends.go        # serpapi/google-search-results-go OR chromedp scrape fallback
│   ├── news_api.go             # NewsAPI.org via net/http + API key
│   ├── aggregator.go           # Merge + rank + dedupe trending results
│   └── scheduler.go            # robfig/cron v3 — every 30 min
├── handlers/
│   ├── trending_controller.go  # REST API
│   └── feed_controller.go     # URL discovery feed (for crawler-service polling)
└── cache/
    └── redis_cache.go          # Redis cache with 25-min TTL per data source
```

### 7.3 Key Implementation Details

```go
// internal/services/scheduler.go
func (s *Scheduler) Start() {
    // Every 30 minutes: fetch Google Trends + NewsAPI → aggregate → store → push URLs to Redis
    s.cron.AddFunc("*/30 * * * *", func() {
        ctx := context.Background()
        topics, err := s.aggregator.Refresh(ctx)
        if err != nil {
            slog.Error("trending refresh failed", "err", err)
            return
        }
        // Push discovered URLs to Redis list for crawler-service
        for _, url := range extractURLs(topics) {
            if err := s.redis.LPush(ctx, "trending:urls", url).Err(); err != nil {
                slog.Warn("failed to push trending URL", "url", url, "err", err)
            }
        }
        s.redis.LTrim(ctx, "trending:urls", 0, 9999) // Max 10,000 URLs
    })
}
```

```go
// internal/services/aggregator.go
// Merges + ranks + deduplicates results from Google Trends + NewsAPI
type TrendingAggregator struct {
    trendsAPI *GoogleTrendsService
    newsAPI   *NewsApiService
    mongo     *mongo.Database
    redis     *redis.Client
}

func (a *TrendingAggregator) Refresh(ctx context.Context) ([]*TrendingTopic, error) {
    var wg sync.WaitGroup
    var mu sync.Mutex
    allTopics := make([]*TrendingTopic, 0)

    // Fetch both sources concurrently
    wg.Add(2)
    go func() { defer wg.Done(); topics, _ := a.trendsAPI.Fetch(ctx); mu.Lock(); allTopics = append(allTopics, topics...); mu.Unlock() }()
    go func() { defer wg.Done(); articles, _ := a.newsAPI.Fetch(ctx); mu.Lock(); allTopics = append(allTopics, articlesToTopics(articles)...); mu.Unlock() }()
    wg.Wait()

    // Deduplicate by URL, rank by score, take top 20
    ranked := a.deduplicateAndRank(allTopics)
    return ranked[:min(20, len(ranked))], nil
}
```

### 7.4 URL Discovery Feed (Crawler Integration)

The most critical inter-service contract — trending-service provides URLs for crawler-service:

```go
// internal/cache/redis_cache.go
// crawler-service calls: LRANGE trending:urls 0 99 → LTRIM trending:urls 100 -1
// Falls back to HTTP endpoint if Redis is unavailable
func (s *FeedController) GetDiscoveryFeed(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    urls, err := s.redis.LRange(ctx, "trending:urls", 0, 99).Result()
    if err != nil {
        // Fallback: call trending-service HTTP endpoint
        resp, _ := s.httpClient.Get("http://trending-service:8084/trending/feeds?limit=100")
        defer resp.Body.Close()
        json.NewDecoder(resp.Body).Decode(&urls)
    }
    json.NewEncoder(w).Encode(map[string]interface{}{"urls": urls, "count": len(urls)})
}
```

### 7.5 API Endpoints

```
GET  /healthz                   → {"status":"ok","last_refresh":"2026-03-31T07:00:00Z"}
GET  /ready                     → MongoDB + Redis connected
GET  /trending/topics           → Top 20 trending topics (cached)
GET  /trending/topics/:topic    → Topic detail with keywords + timeline
GET  /trending/news             → Latest news articles
GET  /trending/feeds           → URL discovery feed for crawler-service
GET  /trending/history          → Historical snapshots for trend charts
GET  /trending/sources          → Status of Google Trends + NewsAPI (healthy/degraded)
POST /trending/refresh          → Admin: trigger immediate full refresh
```

### 7.6 Risk Mitigation

| Risk | Mitigation |
|---|---|
| NewsAPI free tier: 100 req/day | Aggressive 25-min Redis cache; batch all topic lookups per refresh cycle |
| Google Trends rate limit | Cache 25 min; fallback to NewsAPI-only if Trends fails |
| URL flood overwhelming crawler | Redis list capped at 10,000 URLs; crawler-service has its own priority queue |

---

## 8. Database Migration Strategy

### 8.1 Architecture Decision: Keep MongoDB as Primary Store

All four extracted modules (BOT, Notification, Crawler, Trending) use **MongoDB only** — no MySQL migration needed for these phases. This eliminates the highest-risk migration vector.

**Dual-write strategy during Strangler Fig transition:**
```
NestJS ──writes──► MongoDB ──reads──► Go Service
   │                                      ▲
   └──────── reads (backward compat) ─────┘
```

### 8.2 MongoDB Collections by Service

| Collection | Owner | Strangler Fig Strategy |
|---|---|---|
| `bot_conversations` | Go (bot-service) from Week 3 | NestJS reads only after cutover |
| `bot_linked_accounts` | Go (bot-service) from Week 3 | Same |
| `bot_workflows` | Go (bot-service) from Week 3 | Same |
| `notifications` | Go (notification-service) from Week 5 | Same |
| `crawl_history` | Go (crawler-service) from Week 7 | Same |
| `rss_feeds` | Go (crawler-service) from Week 7 | Same |
| `scraper_configs` | Go (crawler-service) from Week 7 | Same |
| `domain_reputation` | Go (crawler-service) from Week 7 | Same |
| `content_fingerprints` | Go (crawler-service) from Week 7 | Same |
| `content_blacklist` | Go (crawler-service) from Week 7 | Same |
| `trending_topics` | Go (trending-service) from Week 11 | Same |
| `auth_activity_logs` | NestJS only | Not migrated |

### 8.3 MongoDB Connection (Production-Grade)

```go
// pkg/database/mongo.go
type MongoDB struct {
    client   *mongo.Client
    database *mongo.Database
}

func NewMongoDB(cfg Config) (*MongoDB, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    opts := options.Client().
        ApplyURI(cfg.GetString("mongo.uri")).
        SetMaxPoolSize(uint64(cfg.GetInt("mongo.pool_size"))).   // default: 100
        SetMinPoolSize(uint64(cfg.GetInt("mongo.min_pool"))).   // default: 10
        SetServerSelectionTimeout(5 * time.Second).
        SetConnectTimeout(10 * time.Second).
        SetKeepAlive(true).
        SetRetryWrites(true).                                     // automatic retry on transient errors
        SetRetryReads(true)

    client, err := mongo.Connect(ctx, opts)
    if err != nil {
        return nil, fmt.Errorf("mongo connect: %w", err)
    }
    if err := client.Ping(ctx, nil); err != nil {
        return nil, fmt.Errorf("mongo ping: %w", err)
    }
    return &MongoDB{client: client, database: client.Database(cfg.GetString("mongo.db"))}, nil
}

func (m *MongoDB) Collection(name string) *mongo.Collection {
    return m.database.Collection(name)
}

func (m *MongoDB) Close(ctx context.Context) { m.client.Disconnect(ctx) }
```

### 8.4 Index Creation (Migration Scripts)

```go
// scripts/migrate/001_bot_indexes.go
func Up(ctx context.Context, db *mongo.Database) error {
    _, err := db.Collection("bot_conversations").Indexes().CreateMany(ctx, []mongo.IndexModel{
        {Keys: bson.D{{Key: "user_id", Value: 1}}},
        {Keys: bson.D{{Key: "platform", Value: 1}}},
        {Keys: bson.D{{Key: "status", Value: 1}}},
        {Keys: bson.D{{Key: "updated_at", Value: -1}}},
        // TTL: auto-delete stale conversations after 30 days
        {Keys: bson.D{{Key: "updated_at", Value: 1}},
         Options: options.Index().SetExpireAfterSeconds(30*24*3600)},
    }); if err != nil {
        return fmt.Errorf("bot_conversations indexes: %w", err)
    }

    _, err = db.Collection("crawl_history").Indexes().CreateMany(ctx, []mongo.IndexModel{
        {Keys: bson.D{{Key: "url", Value: 1}}, Options: options.Index().SetUnique(true)},
        {Keys: bson.D{{Key: "status", Value: 1}}},
        {Keys: bson.D{{Key: "score", Value: -1}}},
        {Keys: bson.D{{Key: "created_at", Value: -1}}},
        // Partial index: only completed crawls
        {Keys: bson.D{{Key: "score", Value: -1}}},
         Options: options.Index().SetPartialFilterExpression(bson.D{
             {Key: "status", Value: "completed"},
         })},
    }); if err != nil {
        return fmt.Errorf("crawl_history indexes: %w", err)
    }

    _, err = db.Collection("content_fingerprints").Indexes().CreateMany(ctx, []mongo.IndexModel{
        // Index first 16 bits of SimHash for fast Hamming distance queries
        {Keys: bson.D{{Key: "simhash_bucket", Value: 1}}},
        {Keys: bson.D{{Key: "sha256", Value: 1}}, Options: options.Index().SetUnique(true)},
        {Keys: bson.D{{Key: "url", Value: 1}}},
    }); if err != nil {
        return fmt.Errorf("content_fingerprints indexes: %w", err)
    }

    return nil
}
```

### 8.5 Backward Compatibility Checklist

```
Phase 2 (Week 3-4): BOT cutover
  □ bot_conversations: Go writes, NestJS reads only
  □ Feature flag: FEATURE_FLAG_BOT=go (NestJS checks flag → proxies to bot-service:8081)

Phase 3 (Week 5-6): Notification cutover
  □ notifications: Go writes, NestJS reads only
  □ Feature flag: FEATURE_FLAG_NOTIFICATIONS=go

Phase 4 (Week 7-10): Crawler cutover
  □ All crawler collections: Go owns writes
  □ Feature flag: FEATURE_FLAG_CRAWLER=go

Phase 5 (Week 11-12): Trending cutover
  □ trending_topics: Go owns writes
  □ Feature flag: FEATURE_FLAG_TRENDING=go

Phase 6 (Week 13-14): NestJS decommission
  □ Remove all feature flags
  □ Disable old NestJS module imports
  □ Remove old NestJS module code from src/modules/
```

### 8.6 MongoDB Migration Validation

- Run migration scripts in **staging** with production-sized dataset before applying in production
- Verify all indexes exist: `db.<collection>.getIndexes()`
- Monitor `db.currentOp()` for slow queries (>500ms) during first week
- Set up MongoDB Atlas alerting on: connection pool exhaustion, query time >2s, replication lag >10s

---

## 9. Deployment & CI/CD

### 9.1 Docker Build Strategy

**Shared base image** — one `Dockerfile` builds all 4 binaries, each service uses a thin `Dockerfile`:

```dockerfile
# Dockerfile.base  (multi-stage, builds ALL services)
FROM golang:1.22-alpine AS builder

# Install build deps
RUN apk add --no-cache git ca-certificates build-base

WORKDIR /build

# Download deps (cached layer)
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build all 4 services
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X main.version=$(git describe --tags)" \
    -o bot-service ./cmd/bot-service

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o notification-service ./cmd/notification-service

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o crawler-service ./cmd/crawler-service

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o trending-service ./cmd/trending-service

# Scratch base image (smallest possible)
FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/bot-service            /bin/bot-service
COPY --from=builder /build/notification-service   /bin/notification-service
COPY --from=builder /build/crawler-service        /bin/crawler-service
COPY --from=builder /build/trending-service        /bin/trending-service
COPY --from=builder /build/configs/               /etc/erg/
```

```dockerfile
# cmd/bot-service/Dockerfile  (thin, uses base)
FROM erg-go-base:latest
COPY configs/bot-service.yaml /etc/erg/bot-service.yaml
EXPOSE 8081
ENTRYPOINT ["/bin/bot-service", "--config", "/etc/erg/bot-service.yaml"]
```

### 9.2 Service Ports Summary

| Service | HTTP Port | Asynq Port | Config File |
|---|---|---|---|
| `bot-service` | 8081 | — | `bot-service.yaml` |
| `notification-service` | 8082 | 9092 | `notification-service.yaml` |
| `crawler-service` | 8083 | 9093 | `crawler-service.yaml` |
| `trending-service` | 8084 | — | `trending-service.yaml` |

### 9.3 docker-compose.yml (Full Dev Stack)

```yaml
version: '3.9'

services:
  mongo:
    image: mongo:7
    ports: ["27017:27017"]
    volumes:
      - mongo-data:/data/db
      - ./scripts/migrate:/docker-entrypoint-initdb.d  # Auto-run migrations
    environment:
      MONGO_INITDB_DATABASE: erg_analytics
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]
    command: >
      redis-server
      --requirepass "Dev.erg.edu.vn"
      --maxmemory 256mb
      --maxmemory-policy allkeys-lru
    volumes:
      - redis-data:/data
    restart: unless-stopped

  bot-service:
    build:
      context: ./go-erg
      dockerfile: ./cmd/bot-service/Dockerfile
    ports: ["8081:8081"]
    environment:
      CONFIG_PATH: /etc/erg/bot-service.yaml
    volumes:
      - ./go-erg/configs/bot-service.yaml:/etc/erg/bot-service.yaml:ro
    depends_on:
      mongo:
        condition: service_healthy
      redis:
        condition: service_healthy
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8081/healthz"]
      interval: 10s
      timeout: 5s
      retries: 3

  notification-service:
    build:
      context: ./go-erg
      dockerfile: ./cmd/notification-service/Dockerfile
    ports: ["8082:8082", "9092:9092"]
    environment:
      CONFIG_PATH: /etc/erg/notification-service.yaml
    volumes:
      - ./go-erg/configs/notification-service.yaml:/etc/erg/notification-service.yaml:ro
    depends_on: [mongo, redis]
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8082/healthz"]
      interval: 10s; timeout: 5s; retries: 3

  crawler-service:
    build:
      context: ./go-erg
      dockerfile: ./cmd/crawler-service/Dockerfile
    ports: ["8083:8083", "9093:9093"]
    environment:
      CONFIG_PATH: /etc/erg/crawler-service.yaml
      GOMAXPROCS: "4"          # Limit CPU cores for crawler workers
    volumes:
      - ./go-erg/configs/crawler-service.yaml:/etc/erg/crawler-service.yaml:ro
    depends_on: [mongo, redis]
    restart: unless-stopped
    deploy:
      resources:
        limits:
          cpus: '4'
          memory: 2G
        reservations:
          cpus: '1'
          memory: 512M
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8083/healthz"]
      interval: 10s; timeout: 5s; retries: 3

  trending-service:
    build:
      context: ./go-erg
      dockerfile: ./cmd/trending-service/Dockerfile
    ports: ["8084:8084"]
    environment:
      CONFIG_PATH: /etc/erg/trending-service.yaml
    volumes:
      - ./go-erg/configs/trending-service.yaml:/etc/erg/trending-service.yaml:ro
    depends_on: [mongo, redis]
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8084/healthz"]
      interval: 10s; timeout: 5s; retries: 3

volumes:
  mongo-data:
  redis-data:
```

### 9.4 GitHub Actions CI/CD Pipeline

```yaml
# .github/workflows/go-services.yml
name: Go Services CI/CD

on:
  push:
    branches: [main, 'feature/**']
  pull_request:
    branches: [main]

env:
  GO_VERSION: '1.22'
  REGISTRY: ghcr.io/${{ github.repository_owner }}

jobs:
  # ── Stage 1: Lint & Test ──────────────────────────────────────────
  lint-and-test:
    runs-on: ubuntu-latest
    services:
      mongo:
        image: mongo:7
        ports: ["27017:27017"]
      redis:
        image: redis:7-alpine
        ports: ["6379:6379"]
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true

      - name: Download deps
        run: cd go-erg && go mod download

      - name: go vet
        run: cd go-erg && go vet ./...

      - name: golangci-lint
        run: |
          cd go-erg
          go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
          golangci-lint run ./... --timeout 5m

      - name: Unit tests (with race detector)
        run: |
          cd go-erg
          go test ./... -race -coverprofile=coverage.out -covermode=atomic
        env:
          MONGO_URI: mongodb://localhost:27017
          REDIS_HOST: localhost
          REDIS_PORT: 6379

      - uses: codecov/codecov-action@v4
        with:
          file: go-erg/coverage.out
          fail_ci_if_error: true

  # ── Stage 2: Build all 4 Docker images ────────────────────────────
  build-images:
    runs-on: ubuntu-latest
    needs: lint-and-test
    strategy:
      matrix:
        service: [bot-service, notification-service, crawler-service, trending-service]
    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-buildx-action@v3

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: ./go-erg
          file: ./go-erg/cmd/${{ matrix.service }}/Dockerfile
          push: ${{ github.ref == 'refs/heads/main' }}
          tags: |
            ${{ env.REGISTRY }}/${{ matrix.service }}:${{ github.sha }}
            ${{ env.REGISTRY }}/${{ matrix.service }}:latest
          cache-from: type=gha
          cache-to: type=gha,mode=max
          outputs: type=docker,dest=/tmp/${{ matrix.service }}.tar

      - name: Upload image artifact
        uses: actions/upload-artifact@v4
        with:
          name: ${{ matrix.service }}-image
          path: /tmp/${{ matrix.service }}.tar

  # ── Stage 3: Deploy to Staging (on main branch) ───────────────────
  deploy-staging:
    runs-on: ubuntu-latest
    needs: build-images
    if: github.ref == 'refs/heads/main'
    environment: staging
    steps:
      - uses: actions/checkout@v4

      - name: Configure kubectl
        run: |
          echo "${{ secrets.KUBE_CONFIG_STAGING }}" | base64 -d > kubeconfig
          echo "KUBECONFIG=$(pwd)/kubeconfig" >> $GITHUB_ENV

      - name: Deploy all services
        run: |
          kubectl set image deployment/bot-service \
            bot-service=${{ env.REGISTRY }}/bot-service:${{ github.sha }} \
            --namespace=erg-staging
          kubectl set image deployment/notification-service \
            notification-service=${{ env.REGISTRY }}/notification-service:${{ github.sha }} \
            --namespace=erg-staging
          kubectl set image deployment/crawler-service \
            crawler-service=${{ env.REGISTRY }}/crawler-service:${{ github.sha }} \
            --namespace=erg-staging
          kubectl set image deployment/trending-service \
            trending-service=${{ env.REGISTRY }}/trending-service:${{ github.sha }} \
            --namespace=erg-staging

      - name: Wait for rollout
        run: |
          kubectl rollout status deployment/bot-service --namespace=erg-staging --timeout=120s
          kubectl rollout status deployment/notification-service --namespace=erg-staging --timeout=120s
          kubectl rollout status deployment/crawler-service --namespace=erg-staging --timeout=120s
          kubectl rollout status deployment/trending-service --namespace=erg-staging --timeout=120s

      - name: Smoke test staging
        run: |
          sleep 10
          curl -sf http://bot-service.erg-staging/healthz || exit 1
          curl -sf http://notification-service.erg-staging/healthz || exit 1
          curl -sf http://crawler-service.erg-staging/healthz || exit 1
          curl -sf http://trending-service.erg-staging/healthz || exit 1
```

### 9.5 Kubernetes Deployment Manifests

```yaml
# k8s/crawler-service.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: crawler-service
  namespace: erg-prod
  labels:
    app: crawler-service
    version: v1
spec:
  replicas: 3
  selector:
    matchLabels:
      app: crawler-service
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0         # Zero-downtime rolling update
  template:
    metadata:
      labels:
        app: crawler-service
        version: v1
    spec:
      containers:
        - name: crawler-service
          image: ghcr.io/yourorg/crawler-service:latest
          ports:
            - name: http
              containerPort: 8083
            - name: asynq
              containerPort: 9093
          resources:
            requests:
              cpu: "500m"
              memory: "512Mi"
            limits:
              cpu: "2000m"
              memory: "2Gi"
          readinessProbe:
            httpGet:
              path: /ready
              port: 8083
            initialDelaySeconds: 5
            periodSeconds: 10
            successThreshold: 1
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8083
            initialDelaySeconds: 15
            periodSeconds: 20
            failureThreshold: 3
          env:
            - name: CONFIG_PATH
              value: "/etc/erg/crawler-service.yaml"
            - name: GOMAXPROCS
              value: "4"
          volumeMounts:
            - name: config
              mountPath: /etc/erg
              readOnly: true
      volumes:
        - name: config
          secret:
            secretName: crawler-service-config
```

### 9.6 Per-Service Config Files

```yaml
# configs/crawler-service.yaml
app:
  name: crawler-service
  host: "0.0.0.0"
  port: 8083
  env: "${APP_ENV}"

mongo:
  uri: "${MONGO_URI}"
  db: "erg_analytics"
  pool_size: 100
  min_pool: 10

redis:
  host: "${REDIS_HOST}"
  port: 6379
  password: "${REDIS_PASSWORD}"
  db: 0

asynq:
  host: "${REDIS_HOST}"
  port: 6379
  password: "${REDIS_PASSWORD}"
  concurrency: 20        # Default worker pool size
  retry_max: 3
  retry_delay: 10s
  dead_letter_ttl: 168h  # 7 days

crawler:
  max_content_size: 10485760    # 10 MB
  request_timeout: 30s
  min_fetch_interval: 3s         # Respectful crawling
  quality_threshold: 70          # Publishable score ≥ 70
  dedup_hamming_threshold: 6   # Hamming distance ≤ 6 = duplicate

ai:
  gemini_api_key: "${GEMINI_API_KEY}"
  gemini_model: "gemini-1.5-flash"
  timeout: 60s
  max_retries: 2

logging:
  level: "${LOG_LEVEL:-info}"
  format: "json"

telemetry:
  prometheus_port: 9094          # Metrics endpoint
  otel_endpoint: "${OTEL_ENDPOINT}"
```

---

## 10. Testing Strategy

### 10.1 Test Pyramid

```
         ▲
        /E2E\        ~15 tests: full business workflows
       /──────\
      /Integra\    ~60 tests: service + real infra (testcontainers)
     /──────────\
    /  Unit      \  ~250 tests: each function, edge cases, mocks
   /──────────────\
  [Testcontainers: MongoDB + Redis]
```

### 10.2 Unit Tests

```go
// internal/services/quality_gate_test.go
func TestQualityGate_Score(t *testing.T) {
    qg := NewQualityGate()

    tests := []struct {
        name         string
        html         string
        wantPass     bool
        wantMinScore float64
    }{
        {
            name:         "high quality article",
            html:         makeHTML(600, true, true, true),
            wantPass:     true,
            wantMinScore: 70,
        },
        {
            name:         "thin content below threshold",
            html:         makeHTML(50, false, false, false),
            wantPass:     false,
            wantMinScore: 0,
        },
        {
            name:         "medium quality article",
            html:         makeHTML(400, true, false, true),
            wantPass:     false, // below 70
            wantMinScore: 50,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            score := qg.Score(tt.html)
            if got := score.Total; got < tt.wantMinScore {
                t.Errorf("Score.Total = %v, want ≥ %v", got, tt.wantMinScore)
            }
            if qg.ShouldPublish(score) != tt.wantPass {
                t.Errorf("ShouldPublish() = %v, want %v", qg.ShouldPublish(score), tt.wantPass)
            }
        })
    }
}

// internal/services/simhash_test.go
func TestSimHash_IsDuplicate(t *testing.T) {
    svc := NewDedupService(mockMongo, mockRedis, 6)

    // Same article → duplicate
    fp1, _ := svc.ComputeSimHash("This is a sample article content.")
    svc.Store(context.Background(), fp1, "https://example.com/article1")

    dup, _ := svc.IsDuplicate(context.Background(), fp1)
    require.True(t, dup, "identical content should be duplicate")

    // Completely different article → not duplicate
    fp2, _ := svc.ComputeSimHash("Completely different topic about cooking recipes and food.")
    dup2, _ := svc.IsDuplicate(context.Background(), fp2)
    require.False(t, dup2, "different content should not be duplicate")

    // Near-duplicate (Hamming ≤ 6) → duplicate
    fp3, _ := svc.ComputeSimHash("This is a sample article content with slight changes.")
    dup3, _ := svc.IsDuplicate(context.Background(), fp3)
    require.True(t, dup3, "near-duplicate should be flagged as duplicate")
}
```

### 10.3 Integration Tests (Testcontainers)

```go
// test/integration/crawler_integration_test.go
// Uses testcontainers for MongoDB + Redis — no external deps needed

func TestCrawlJob_EndToEnd(t *testing.T) {
    ctx := context.Background()

    // Start MongoDB container
    mongo, err := mongodb.RunContainer(ctx, mongodb.WithImage("mongo:7"))
    require.NoError(t, err)
    defer mongo.Terminate(ctx)

    // Start Redis container
    redisC, err := redis.RunContainer(ctx, redis.WithImage("redis:7-alpine"))
    require.NoError(t, err)
    defer redisC.Terminate(ctx)

    mongoURI, _ := mongo.ConnectionString(ctx)
    redisAddr, _ := redisC.ConnectionString(ctx)

    // Build crawler with real containers
    db, _ := mongo.NewMongoDB(mongoConfig(mongoURI))
    rdb, _ := redis.NewRedisClient(redisAddr)
    crawler := NewCrawlerOrchestrator(db, rdb)

    // Enqueue a real crawl job
    jobID, err := crawler.Enqueue(ctx, &CrawlJobPayload{
        URL:      "https://httpbin.org/html",
        Depth:    1,
        Priority: 1,
        Source:   "manual",
    })
    require.NoError(t, err)

    // Wait for result (timeout 60s)
    result, err := crawler.WaitForResult(ctx, jobID, 60*time.Second)
    require.NoError(t, err)
    require.Equal(t, JobStatusSuccess, result.Status)
    require.GreaterOrEqual(t, result.Score, float64(0))

    // Verify stored in MongoDB
    history, _ := crawler.GetHistory(ctx, jobID)
    require.NotNil(t, history)
    require.Equal(t, "completed", history.Status)
}

func TestDigestScheduler_DailyDigest(t *testing.T) {
    // ... test digest aggregation logic with mocked time ...
    svc := NewDigestScheduler(mockNotifier, mockMongo)
    svc.t = fakeClock{t: time.Date(2026, 3, 31, 8, 0, 0, 0, time.UTC)}
    err := svc.TriggerDailyDigest(context.Background())
    require.NoError(t, err)
    // Assert notification was sent
    require.Equal(t, 1, len(mockNotifier.sentNotifications))
    require.Contains(t, mockNotifier.sentNotifications[0].Body, "Daily Digest")
}
```

### 10.4 E2E Tests (Full Stack)

```go
// test/e2e/full_workflow_test.go
// docker-compose up all 4 services → run E2E tests against them

func TestTrendingToNotificationWorkflow(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping E2E in short mode")
    }

    crawlerSvc := crawler.NewClient("http://localhost:8083")
    trendingSvc := trending.NewClient("http://localhost:8084")
    notifSvc := notification.NewClient("http://localhost:8082")

    // 1. Trigger trending refresh
    require.NoError(t, trendingSvc.Refresh(context.Background()))

    // 2. Fetch discovered URLs
    feed, err := trendingSvc.GetDiscoveryFeed(context.Background())
    require.NoError(t, err)
    require.NotEmpty(t, feed.URLs, "trending should discover URLs")

    // 3. Enqueue first URL for crawling
    jobID, err := crawlerSvc.EnqueueURL(context.Background(), feed.URLs[0])
    require.NoError(t, err)

    // 4. Poll until completion (max 2 min)
    result, err := crawlerSvc.WaitForCompletion(context.Background(), jobID, 2*time.Minute)
    require.NoError(t, err)
    require.Equal(t, crawler.JobStatusSuccess, result.Status)

    // 5. Verify notification was recorded
    notifs, err := notifSvc.List(context.Background(), notification.ListFilter{
        Type: "crawl.success",
        Limit: 10,
    })
    require.NoError(t, err)
    require.NotEmpty(t, notifs.Items, "crawl.success notification should be sent")

    // 6. Verify crawl history persisted
    history, err := crawlerSvc.GetHistory(context.Background(), jobID)
    require.NoError(t, err)
    require.NotZero(t, history.ID)
}
```

### 10.5 API Contract Tests (Strangler Fig Validation)

Critical during migration — ensure Go service responses match NestJS:

```go
// test/contract/api_contract_test.go
func TestBotAPI_ContractParity(t *testing.T) {
    // Test all BOT API endpoints produce identical responses
    endpoints := []string{
        "GET /bot/conversations",
        "POST /bot/link",
        "GET /bot/link/test123",
    }

    for _, ep := range endpoints {
        t.Run(ep, func(t *testing.T) {
            nestJSResp := fetch("http://nestjs:3003" + ep)
            goResp := fetch("http://bot-service:8081" + ep)

            if nestJSResp.Status != goResp.Status {
                t.Errorf("Status mismatch for %s: NestJS=%d, Go=%d", ep, nestJSResp.Status, goResp.Status)
            }
            if !jsonEq(nestJSResp.Body, goResp.Body) {
                t.Errorf("Body mismatch for %s:\nNestJS: %s\nGo: %s", ep, nestJSResp.Body, goResp.Body)
            }
        })
    }
}
```

### 10.6 Performance Benchmark

```go
// test/benchmark/simhash_benchmark_test.go
func BenchmarkSimHash_Compute(b *testing.B) {
    content := strings.Repeat("Vietnamese news article about technology. ", 200)

    b.ResetTimer()
    b.ReportAllocs()
    for i := 0; i < b.N; i++ {
        ComputeSimHash(content)
    }
}
// Target: < 500 µs per article on standard hardware
// Go advantage: no GC pressure from string allocations (string → []rune → uint64)

// test/benchmark/http_benchmark_test.go
func BenchmarkHTTPBotWebhook(b *testing.B) {
    // Simulate Discord webhook burst: 1000 concurrent requests
    // Target: P99 < 50ms, zero errors
    b.Run("concurrent_1000", func(b *testing.B) {
        var wg sync.WaitGroup
        for i := 0; i < 1000; i++ {
            wg.Add(1)
            go func() { defer wg.Done(); http.Post(botServiceURL, "application/json", bytes.NewReader(discordPayload)) }()
        }
        wg.Wait()
    })
}
```

### 10.7 Testing Tools Summary

| Tool | Purpose |
|---|---|
| `testing` (stdlib) | All test types |
| `stretchr/testify` | Assertions, `require`, `mock` |
| `testcontainers/testcontainers-go` | Ephemeral MongoDB + Redis in Docker |
| `vektra/mockery` | Generate mocks from Go interfaces |
| `jstemmer/go-junit-report` | JUnit XML for CI integration |
| `go-jmh/jmh` | Micro-benchmarking if needed |
| `masterzen/winrm` | Remote E2E (Windows target) if needed |

### 10.8 CI Gate (Required to Merge)

```
✅ go vet ./...
✅ golangci-lint run ./... (no errors, no warnings)
✅ go test ./... -race -coverprofile=coverage.out
   — Coverage must not drop by >5% vs main branch
✅ docker build ./cmd/<service> (all 4 services)
✅ API contract tests pass (NestJS vs Go response comparison)
✅ Integration tests pass (testcontainers)
```

---

_(End of Plan)_