# erg — Go Monorepo

> High-performance Go microservices for the ERG platform: BOT, Notification, Crawler, and Trending services.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Go Monorepo (erg.ninja)                  │
│                                                              │
│  cmd/                                                         │
│  ├── bot-service/        ← Discord/Telegram command processor│
│  ├── notification-service/ ← Multi-channel notifications     │
│  ├── crawler-service/     ← Web scraping, RSS, Sitemap       │
│  └── trending-service/    ← Google Trends, News API aggregator│
│                                                              │
│  pkg/ (shared infrastructure)                                │
│  ├── config/     ← Viper YAML/env/flags config               │
│  ├── database/   ← MongoDB (go.mongodb-driver v2)            │
│  ├── cache/      ← Redis (go-redis/v9)                       │
│  ├── queue/      ← Asynq background job processing           │
│  ├── event/      ← In-process event bus + Redis pub/sub      │
│  ├── logger/     ← zerolog structured logging                │
│  ├── http/       ← chi router + middleware stack             │
│  ├── auth/       ← JWT validation (HS256/RS256)              │
│  ├── notification/ ← Notification interfaces                  │
│  ├── scraper/    ← HTTP fetcher, robots.txt, HTML parser    │
│  ├── dedup/      ← SimHash near-duplicate detection          │
│  ├── ai/         ← Gemini AI client for CSS selectors         │
│  ├── rss/        ← RSS/Atom/JSON Feed parser                 │
│  ├── sitemap/    ← XML sitemap discovery + parsing            │
│  └── telemetry/  ← OpenTelemetry + Prometheus metrics        │
└─────────────────────────────────────────────────────────────┘
```

## Quick Start

```bash
# Start local infrastructure
make docker-up

# Build all services
make build

# Run all tests
make test

# Lint
make lint

# Run a service locally
make run/bot-service
```

## Services

### bot-service (port 8081)
Handles Discord and Telegram bot commands, conversation management, and workflow automation.

**Key features:**
- Command routing with prefix and slash command support
- Conversation state machine with MongoDB persistence
- Multi-platform support (Discord, Telegram)
- Workflow trigger system
- 30-day TTL on conversations for privacy

### notification-service (port 8082)
Multi-channel notification fan-out with retry, digest scheduling, and delivery tracking.

**Key features:**
- Discord, Telegram, WhatsApp, Email, Slack channels
- Exponential backoff retry with dead-letter queue
- Digest mode for batched notifications
- Asynq-powered async processing
- Per-channel rate limit enforcement

### crawler-service (port 8083)
Web crawling with anti-blocking, SimHash deduplication, and AI-assisted content scoring.

**Key features:**
- RSS/Atom/JSON feed polling
- XML sitemap discovery and recursive parsing
- robots.txt compliance with crawl-delay
- Proxy rotation and User-Agent cycling
- SimHash near-duplicate detection (Hamming ≤ 6)
- Gemini AI CSS selector suggestions
- Asynq worker pool for parallel crawling

### trending-service (port 8084)
Scheduled aggregation of trending topics from Google Trends, News API, and custom sources.

**Key features:**
- Cron-based aggregation (every 30 minutes)
- Regional trend filtering
- Time-windowed scoring (1h, 24h, 7d, 30d)
- MongoDB compound indexes for fast top-k queries

## Shared Packages

### `pkg/config`
Viper-based configuration with YAML/env/flags support and secret injection.

```go
cfg := config.NewDefault()
loader := config.NewLoader(config.WithConfigPaths("."))
if err := loader.Load(cfg); err != nil { ... }
```

### `pkg/database`
MongoDB via mongo-go-driver v2 with connection pooling, retry, and health checks.

```go
mongo, err := database.NewMongoClient(ctx, cfg.MongoDB)
```

### `pkg/cache`
Redis client with GET/SET, Pub/Sub, and distributed locking.

```go
lock, err := redis.AcquireLock(ctx, "my-lock", 30*time.Second)
```

### `pkg/queue`
Asynq client/server for background jobs with priority queues and DLQ.

```go
client.Enqueue(ctx, "crawl:page", payload, queue.WithQueue("high"))
```

### `pkg/event`
In-process event bus + Redis pub/sub for cross-service events.

```go
bus.Publish(ctx, "user.created", eventPayload)
// Cross-service via Redis:
bus := event.NewEventBus("bot-service", event.WithRedisBackend(redis))
```

### `pkg/http`
chi router with standard middleware: Recovery → RequestID → RealIP → Logger → CORS → RateLimit.

```go
server := http.NewServer(cfg.HTTP, log)
server.MountHealthRoutes()
server.Get("/api/v1/items", handler)
```

### `pkg/scraper`
Production web fetcher with robots.txt, proxy rotation, UA cycling, and adaptive delays.

```go
fetcher := scraper.NewFetcher(cfg.Scraper)
result := fetcher.Fetch(ctx, "https://example.com")
```

### `pkg/dedup`
SimHash FNV-1a + SHA-256 deduplication with Hamming distance comparison.

```go
deduper := dedup.NewDeduper(store)
isDup, reason, _ := deduper.IsDuplicate(ctx, "article content")
```

## Configuration

All configuration is via `config.yaml` or environment variables (with `__` separator):

```yaml
app:
  name: bot-service
  port: 8080
  env: development

mongodb:
  uri: mongodb://localhost:27017
  database: erg

redis:
  host: localhost
  port: 6379

scraper:
  min_delay: 3s
  respect_robots_txt: true
  user_agents:
    - "Mozilla/5.0 ..."

queue:
  concurrency: 10
  redis_host: localhost
```

Environment variable override:
```bash
export MONGODB__DATABASE=production_db
export REDIS__HOST=redis.prod.internal
export SECRET_DATABASE__PASSWORD=super-secret
```

## Testing

```bash
# All tests with race detector
make test

# Coverage report
make test-cover

# Specific package
go test ./pkg/scraper/... -v
```

## Deployment

```bash
# Build Docker images
make docker-build

# Deploy to staging
make deploy

# Start local stack
make docker-up
```

## Migration Status

| Phase | Description | Status |
|---|---|---|
| Foundation | Shared packages, infrastructure | ✅ **Active** |
| BOT Service | Discord/Telegram command processor | 🔄 Phase 2 |
| Notification Service | Multi-channel notifications | 🔄 Phase 3 |
| Crawler Service | Web scraping, RSS, dedup | 🔄 Phase 4 |
| Trending Service | Google Trends aggregation | 🔄 Phase 5 |
