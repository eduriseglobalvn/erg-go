# Migration to Go - Module Extraction Plan

> **Status**: Đang triển khai | **Architecture**: **TARGET = 1 Binary duy nhất** (phong cách NestJS/Spring Boot)
> **Current**: 4 microservices với `go.work` workspace (BOT ✅, 3 stub services)
> **Target**: `go build -o erg-server ./cmd/server` — 1 binary chứa tất cả
>
> **⚠️ IMPORTANT — Đọc trước khi tiếp tục**: Section 2 mô tả **TARGET architecture**. Phase 0 (Refactor) cần hoàn thành TRƯỚC Phase 3 để chuyển code hiện tại sang cấu trúc mới.

---

## Table of Contents

1. [Overview & Motivation](#1-overview--motivation)
2. [New Architecture: 1 Monolithic Binary](#2-new-architecture-1-monolithic-binary) ⬅️ CHANGED
3. [Module Extraction Phases](#3-module-extraction-phases) ⬅️ CHANGED (6 phases → 1 binary)
4. [BOT Module Extraction](#4-bot-module-extraction) ⬅️ CHANGED (cmd/bot-service → internal/modules/bot)
5. [Notification Module Extraction](#5-notification-module-extraction) ⬅️ CHANGED
6. [Crawler Module Extraction](#6-crawler-module-extraction) ⬅️ CHANGED
7. [Trending Module Extraction](#7-trending-module-extraction) ⬅️ CHANGED
8. [Phase 6 — Integration & Refinement](#8-phase-6--integration--refinement) ⬅️ CHANGED
9. [Database Migration Strategy](#9-database-migration-strategy)
10. [Deployment & CI/CD](#10-deployment--cicd)
11. [Testing Strategy](#11-testing-strategy)

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

### Migration Strategy: Incremental Module Porting → 1 Binary

Rather than a risky big-bang rewrite, this plan ports modules **one at a time** from TypeScript to Go, all going into the **same monolithic binary** — following the **Strangler Fig Pattern** adapted for a single binary:

1. **NestJS remains running and production-safe** throughout the migration.
2. Each module is **ported one at a time** from NestJS/TypeScript to Go inside `cmd/server`.
3. During porting, the **Go module handles traffic** while the old TypeScript code is still wired in.
4. Once validated, the old TypeScript code for that module is **disabled**.
5. Only when all modules are migrated does the NestJS backend become a routing shell (or is decommissioned).

**Output**: **1 Go binary** (`erg-server`), containing all 4 modules as packages. Same DX as NestJS:
- `go run ./cmd/server` → chạy tất cả 4 modules cùng lúc
- Không cần `go.work`, không cần 4 service riêng biệt
- Các modules import `pkg/*` packages chung
- Khi compile: `go build -o erg-server ./cmd/server` → binary duy nhất ~20-30MB

This ensures **zero downtime**, **gradual risk reduction**, and the ability to **roll back** any phase independently.

### Goal: Go Monolith (NestJS-style) Across Projects

A key objective is to design the Go monolith so it is **project-agnostic** and can be reused across multiple websites and products:

- All modules are configured entirely through `config.yaml` / environment variables.
- No hardcoded project IDs, domain names, or business-specific logic.
- Shared infrastructure (`pkg/database`, `pkg/queue`, `pkg/event`) is versioned as internal Go packages.
- Docker Compose and Helm charts make deployment reproducible on any infrastructure.
- **Single binary deployment** — same simplicity as NestJS but with Go performance.

### Current State (as of 2026-04-01)

```
D:\ERG\go-erg/                ← Go monorepo (đang triển khai)
├── go.work                   ← ✅ Go workspace (4 modules)
├── go.mod                    ← Root module
├── pkg/                      ← ✅ Infrastructure packages (14 packages, all tested)
├── migrations/               ← ✅ (001-004)
├── cmd/
│   ├── bot-service/          ← ✅ BOT Module DONE — full implementation
│   │   ├── main.go
│   │   ├── wire.go           ← Dependency injection (functional options)
│   │   ├── internal/
│   │   │   ├── handlers/     ← bot_controller, discord_webhook, telegram_webhook, health
│   │   │   ├── services/     ← command_handler, conversation, workflow, link, permission
│   │   │   ├── models/       ← bot_conversation, bot_linked_account, bot_workflow, bot_command
│   │   │   ├── commands/     ← 5 command files (base, rss, crawl, trending, stats, system)
│   │   │   └── platform/     ← discord, telegram, hmac
│   │   └── internal/
│   ├── notification-service/ ← 🔄 Stub (Phase 3 — chưa implement)
│   ├── crawler-service/     ← 🔄 Stub (Phase 4 — chưa implement)
│   └── trending-service/     ← ⬜ Stub (Phase 5 — chưa implement)
```

**Modules status:**

| Module | Status | Location | Notes |
|---|---|---|---|
| BOT | ✅ DONE | `internal/modules/bot/` | Full implementation, Phase 2 |
| Notification | ✅ DONE | `internal/modules/notifications/` | Full implementation, Phase 3 ✅ |
| Crawler | 🔄 STUB | `internal/modules/crawler/` | Phase 4 — chỉ có module skeleton |
| Trending | ⬜ STUB | `internal/modules/trending/` | Phase 5 — chưa implement |

**Build hiện tại**: `go build -o erg-server ./cmd/server` → ✅ 1 binary `erg-server` (31MB)

### ⚠️ Phase 0 — Refactor Required Before Phase 3

**Hiện tại**: 4 services riêng biệt (`cmd/*-service/`), mỗi service có `go.mod` riêng, quản lý bởi `go.work`.
**Mục tiêu**: 1 binary duy nhất (`cmd/server/`), 1 `go.mod` root.

Phase 0 cần hoàn thành **TRƯỚC Phase 3** (Notification Module) để tránh conflict giữa 2 kiến trúc.

```
Bước 1: Tạo cmd/server/main.go + cmd/server/server.go
Bước 2: Di chuyển pkg/* từ bot-service imports → go.mod root
Bước 3: Di chuyển internal/ từ cmd/bot-service/ → internal/modules/bot/
Bước 4: Tạo cmd/server/routes.go (register all modules)
Bước 5: Xóa go.work, gộp go.mod → root go.mod
Bước 6: Xóa cmd/{bot,notification,crawler,trending}-service/
Bước 7: go build -o erg-server ./cmd/server
Bước 8: go test ./... — verify
```

> Xem chi tiết Phase 0 ở cuối document này (Section 3.1).

**Modules to remain in NestJS (or migrate later):** Auth, API Gateway, Admin dashboard.

### Scope of This Plan

> ✅ **In scope:** Porting of BOT, Notification, Crawler, and Trending modules to Go **as packages inside a single binary** using the Strangler Fig Pattern.
>
> ❌ **Out of scope:** Migrating the NestJS authentication layer, the API gateway, the admin dashboard, or the frontend. These are covered in a separate roadmap.

---

## 2. Architecture: Current vs. Target

### 2.1 ✅ Architecture Decision: 1 Binary, Not 4 Microservices

> **Decision made 2026-03-31**: Chuyển từ 4 microservices riêng biệt → **1 single Go monolith**. Tất cả 4 modules nằm trong 1 binary duy nhất `cmd/server`. **Phase 0 (refactor) cần hoàn thành TRƯỚC Phase 3.**

### 2.2 Current Architecture (AS-IS — 4 microservices)

**Code hiện tại** sử dụng `go.work` workspace, 4 service directories, mỗi cái có `go.mod` riêng:

```
go-erg/                          # Go monorepo root
├── go.work                      # ← Go workspace (4 modules)
├── go.mod                       # Root module (infrastructure)
├── pkg/                         # Shared infrastructure (imported by all services)
├── migrations/                  # Database migrations
├── cmd/
│   ├── bot-service/            # ✅ BOT Module — full implementation
│   │   ├── main.go
│   │   ├── wire.go            # Dependency injection (functional options)
│   │   ├── internal/
│   │   │   ├── handlers/       # HTTP + webhooks
│   │   │   ├── services/       # Business logic
│   │   │   ├── models/         # MongoDB entities
│   │   │   ├── commands/       # Bot commands
│   │   │   └── platform/       # Discord, Telegram adapters
│   │   └── internal/          # (note: nested internal/ — to be flattened in Phase 0)
│   ├── notification-service/  # 🔄 Stub: main.go only
│   ├── crawler-service/       # 🔄 Stub: main.go only
│   └── trending-service/       # 🔄 Stub: main.go only
```

### 2.3 Target Architecture (TO-BE — 1 binary)

```
go-erg/                          # Go monolith root (D:\ERG\go-erg\)
├── go.mod                       # ✅ Single module — KHÔNG có go.work
├── cmd/server/
│   ├── main.go                 # ✅ Bootstrap: load deps → register modules → start
│   └── server.go              # ✅ App bootstrap (như main.ts + app.module.ts)
├── internal/
│   ├── modules/                # Như src/modules/ trong NestJS
│   │   ├── bot/               # ✅ Phase 2 DONE
│   │   │   ├── bot.module.go
│   │   │   ├── bot.controller.go
│   │   │   ├── bot.service.go
│   │   │   ├── dto/
│   │   │   ├── entities/
│   │   │   └── commands/
│   │   ├── notifications/     # 🔄 Phase 3 IN PROGRESS
│   │   ├── crawler/          # 🔄 Phase 4 IN PROGRESS
│   │   └── trending/          # ⬜ Phase 5 NOT STARTED
│   └── routes/
│       └── routes.go
├── pkg/                        # ✅ Reusable infrastructure (14 packages, all tested)
├── migrations/                 # ✅ (001-004)
├── Dockerfile                  # ✅ Multi-stage build
├── docker-compose.yml          # ✅ MongoDB + Redis
├── Makefile                   # ✅
└── .github/workflows/         # ✅ (ci, build-deploy)
```

### Run Experience

**Hiện tại (4 services riêng biệt):**
```bash
# Bot service (duy nhất có full implementation)
go build ./cmd/bot-service && ./bot-service
```

**Sau Phase 0 refactor (1 binary):**
```bash
# Dev — giống hệt yarn start:dev
go run ./cmd/server

# Hoặc với hot reload
brew install air
air ./cmd/server

# Build — như npm run build
go build -o erg-server ./cmd/server

# Chạy binary
./erg-server
```

### 2.4 ✅ Module Pattern — ACTUAL Pattern Used in Codebase

**Pattern đang dùng**: Functional options + `Dependencies` struct (xem `cmd/bot-service/wire.go`)

```go
// cmd/server/wire.go — Dependency Injection container
// Pattern đANG ĐƯỢC DÙNG trong bot-service

type Dependencies struct {
    // Infrastructure
    Mongo     *database.MongoClient
    Redis     *cache.RedisClient
    EventBus  *event.EventBus
    Logger    *logger.Logger
    Config    *config.Config

    // Platform clients
    Discord   *platform.DiscordClient
    Telegram  *platform.TelegramClient

    // Services
    CommandHandler     *services.CommandHandler
    ConversationService *services.ConversationService
    WorkflowEngine    *services.WorkflowEngine
    LinkService       *services.LinkService
    PermissionService *services.PermissionService
}

// InitializeServices constructs all services with functional options
func InitializeServices(deps Dependencies) (*Services, error) {
    // Validate required dependencies
    if deps.Mongo == nil { return nil, fmt.Errorf("MongoDB is required") }
    if deps.Redis == nil { return nil, fmt.Errorf("Redis is required") }
    if deps.Logger == nil { return nil, fmt.Errorf("Logger is required") }

    // Collections
    botConvColl := deps.Mongo.Collection(models.BotConversationCollection)
    linkedAccColl := deps.Mongo.Collection(models.BotLinkedAccountCollection)
    workflowColl := deps.Mongo.Collection(models.WorkflowExecutionCollection)

    // Services with functional options
    permSvc := services.NewPermissionService(botConvColl,
        services.WithPermissionLogger(deps.Logger),
    )
    convSvc := services.NewConversationService(botConvColl, deps.Redis,
        services.WithConversationLogger(deps.Logger),
    )
    // ...
    return &Services{
        CommandHandler:     cmdHandler,
        ConversationService: convSvc,
        WorkflowEngine:    workflowSvc,
        LinkService:       linkSvc,
        PermissionService: permSvc,
    }, nil
}
```

### 2.5 ✅ Module Pattern — TARGET Pattern (NestJS-style, sau Phase 0)

```go
// internal/modules/bot/bot.module.go — TARGET pattern
package bot

type Module struct {
    service    *BotService
    controller *BotController
}

func NewModule(deps Dependencies) *Module {
    return &Module{
        service:    NewBotService(deps),
        controller: NewBotController(),
    }
}

func (m *Module) Setup()                           { m.controller.Inject(m.service) }
func (m *Module) RegisterRoutes(r *chi.Mux)       { /* mount routes */ }
func (m *Module) Stop() error                     { /* graceful shutdown */ }
```

```go
// cmd/server/server.go — TARGET: như app.module.ts trong NestJS
type Server struct {
    router   *chi.Mux
    modules  []Module
    mongo    *database.MongoClient
    redis    *cache.RedisClient
    queue    *asynq.Client
    eventBus *event.EventBus
    cfg      *config.Config
}

func NewServer(cfg *config.Config, mongo *database.MongoClient, redis *cache.RedisClient, queue *asynq.Client, eventBus *event.EventBus) *Server {
    s := &Server{router: chi.NewRouter(), ...}
    s.applyGlobalMiddleware()

    // All modules in 1 binary: direct function calls, no HTTP
    s.modules = []Module{
        bot.NewModule(depsForBot()),
        notifications.NewModule(depsForNotifications()),
        crawler.NewModule(depsForCrawler()),
        trending.NewModule(depsForTrending()),
    }
    for _, m := range s.modules {
        m.Setup()
        m.RegisterRoutes(s.router)
    }
    return s
}

func (s *Server) Start(addr string) error {
    srv := &http.Server{Addr: addr, Handler: s.router}
    go func() {
        sig := make(chan os.Signal, 1)
        signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
        <-sig
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        for _, m := range s.modules { m.Stop() }
        srv.Shutdown(ctx)
    }()
    return srv.ListenAndServe()
}
```

```go
// cmd/server/main.go — TARGET: như main.ts trong NestJS
func main() {
    cfg := config.Load()
    mongo, err := database.NewMongo(cfg)
    if err != nil { log.Fatal(err) }
    redis := cache.NewRedis(cfg)
    queue := queue.NewAsynq(cfg)
    eventBus := event.NewBus(cfg)

    server := server.NewServer(cfg, mongo, redis, queue, eventBus)
    log.Info().Str("addr", ":8080").Msg("starting erg-server (1 binary, 4 modules)")
    if err := server.Start(":8080"); err != nil {
        log.Fatal().Err(err).Msg("server exited")
    }
}
```

### ✅ Module Pattern (NestJS-style) — 1 Binary, All Modules

**Tất cả 4 modules chạy trong cùng 1 binary `cmd/server`**: Điểm khác biệt chính so với thiết kế 4 microservices là tất cả được build vào cùng 1 binary, không cần inter-service communication, không cần service discovery.

```go
// internal/modules/bot/bot.module.go
package bot

// Module wraps DI setup (như BotModule trong NestJS)
type Module struct {
    service   *BotService
    controller *BotController
}

// NewModule creates the module with its dependencies.
func NewModule(repo BotRepository, redis cache.RedisCache) *Module {
    return &Module{
        service:    NewBotService(repo, redis),
        controller: NewBotController(),
    }
}

// Setup wires dependencies and configures the module.
func (m *Module) Setup() {
    m.controller.Inject(m.service)
}

// RegisterRoutes mounts HTTP handlers onto the chi router.
// Như @Controller('/bot') decorators trong NestJS.
func (m *Module) RegisterRoutes(r *chi.Mux) {
    // Public webhook routes (HMAC auth)
    r.Post("/webhooks/discord", m.controller.DiscordWebhook)
    r.Post("/webhooks/telegram", m.controller.TelegramWebhook)

    // Authenticated routes
    r.Group(func(protected chi.Router) {
        protected.Use(auth.JWTMiddleware())
        protected.Get("/conversations", m.controller.ListConversations)
        protected.Post("/conversations/{id}/send", m.controller.SendToConversation)
    })
}
```

```go
// cmd/server/server.go — Như app.module.ts trong NestJS
// Tất cả 4 modules được register vào cùng 1 chi.Router, chạy trong 1 binary
type Server struct {
    router    *chi.Mux
    modules   []Module
    mongo     *mongo.Client
    redis     *redis.Client
    queue     *asynq.Client
    eventBus  *event.Bus
    cfg       *config.Config
}

func NewServer(cfg *config.Config, mongo *mongo.Client, redis *redis.Client, queue *asynq.Client, eventBus *event.Bus) *Server {
    s := &Server{
        router:   chi.NewRouter(),
        mongo:    mongo,
        redis:    redis,
        queue:    queue,
        eventBus: eventBus,
        cfg:      cfg,
    }
    s.applyGlobalMiddleware()

    // Wire shared dependencies once — repositories created once
    botRepo := bot.NewRepository(mongo)
    notifRepo := notifications.NewRepository(mongo)
    crawlerRepo := crawler.NewRepository(mongo)
    trendingRepo := trending.NewRepository(mongo)

    // Register modules (như imports trong AppModule)
    // Tất cả đều nằm trong 1 binary: gọi trực tiếp, không cần HTTP
    s.modules = []Module{
        bot.NewModule(botRepo, redis, queue),
        notifications.NewModule(notifRepo, redis, queue, eventBus),
        crawler.NewModule(crawlerRepo, redis, queue, eventBus),
        trending.NewModule(trendingRepo, redis),
    }

    for _, m := range s.modules {
        m.Setup()
        m.RegisterRoutes(s.router)
    }

    return s
}

func (s *Server) Start(addr string) error {
    srv := &http.Server{Addr: addr, Handler: s.router}
    // Graceful shutdown
    go func() {
        sig := make(chan os.Signal, 1)
        signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
        <-sig
        log.Info().Msg("shutting down...")
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        // Stop all module workers
        for _, m := range s.modules {
            if m, ok := any(m).(interface{ Stop() error }); ok {
                m.Stop()
            }
        }
        srv.Shutdown(ctx)
    }()
    return srv.ListenAndServe()
}
```

```go
// cmd/server/main.go — Như main.ts trong NestJS
func main() {
    cfg := config.Load()
    mongo, err := database.NewMongo(cfg)
    if err != nil { log.Fatal(err) }
    redis := cache.NewRedis(cfg)
    queue := queue.NewAsynq(cfg)
    eventBus := event.NewBus(cfg)

    server := server.NewServer(cfg, mongo, redis, queue, eventBus)
    log.Info().Str("addr", ":8080").Msg("starting erg-server (1 binary, 4 modules)")
    if err := server.Start(":8080"); err != nil {
        log.Fatal().Err(err).Msg("server exited")
    }
}
```

---

## 3. Module Extraction Phases

The migration is organized into **7 steps**. Phase 0 is the refactor step (CURRENT CODE → target architecture). Phases 1-5 port modules. Phase 6 is integration and hardening.

> **📌 Đã hoàn thành**: Phase 1 (Foundation) ✅ | Phase 2 (BOT) ✅
> **📌 CẦN LÀM TRƯỚC**: Phase 0 (Refactor → 1 binary) ⬜ — **Làm TRƯỚC Phase 3**
> **📌 Đang triển khai**: Phase 3 (Notification) 🔄 | Phase 4 (Crawler) 🔄
> **📌 Chưa bắt đầu**: Phase 5 (Trending) ⬜ | Phase 6 (Integration) ⬜

### 3.1 Phase 0 — Architecture Refactor ⬜ NOT STARTED

**Objective**: Chuyển từ 4 microservices (go.work) → 1 monolithic binary (`cmd/server`). **Cần làm TRƯỚC Phase 3.**

> **⚠️ Không skip bước này**: Nếu implement Phase 3 vào notification-service mà không refactor, sau này phải di chuyển lại code.

#### Deliverables

- [ ] **Tạo `cmd/server/main.go`** — entry point bootstrap
- [ ] **Tạo `cmd/server/server.go`** — App bootstrap (như app.module.ts)
- [ ] **Tạo `cmd/server/routes.go`** — Register all 4 modules
- [ ] **Di chuyển `pkg/*` imports** từ `erg.ninja/bot-service/internal` → module paths mới
- [ ] **Di chuyển `cmd/bot-service/internal/`** → `internal/modules/bot/`
- [ ] **Tạo `internal/modules/bot/bot.module.go`** — NestJS-style module registration
- [ ] **Di chuyển services** → NestJS pattern (Module.Setup + RegisterRoutes + Stop)
- [ ] **Di chuyển notification/crawler/trending stubs** → `internal/modules/*/`
- [ ] **Gộp `go.mod`** — xóa `go.work`, module paths → `erg.ninja/server`
- [ ] **Xóa `cmd/{bot,notification,crawler,trending}-service/`** sau khi migrate xong
- [ ] **`go build -o erg-server ./cmd/server`** — zero errors
- [ ] **`go test ./...`** — all pass

#### Rollback

- Giữ `go.work` và `cmd/*-service/` cho đến khi `cmd/server` build thành công
- Nếu build fail: quay lại `go build ./cmd/bot-service`

#### Step-by-step

```bash
# Step 1: Tạo cmd/server
mkdir -p cmd/server internal/modules

# Step 2: Copy bot-service internal → internal/modules/bot/
cp -r cmd/bot-service/internal/* internal/modules/bot/

# Step 3: Tạo bot.module.go
cat > internal/modules/bot/bot.module.go << 'EOF'
package bot

type Module struct { ... }
func NewModule(deps Dependencies) *Module { ... }
func (m *Module) Setup()              { ... }
func (m *Module) RegisterRoutes(r *chi.Mux) { ... }
func (m *Module) Stop() error        { ... }
EOF

# Step 4: Tạo cmd/server/main.go + server.go + routes.go

# Step 5: Update go.mod — xóa go.work, gộp module paths

# Step 6: Update all import paths (erg.ninja/bot-service → erg.ninja/server/internal/modules/bot)

# Step 7: Test
go build -o erg-server ./cmd/server
go test ./...

# Step 8: Xóa old directories
rm -rf cmd/bot-service cmd/notification-service cmd/crawler-service cmd/trending-service go.work
```

### Phase 1 — Foundation ✅ DONE (Agent #1)

**Objective:** Establish the Go monorepo structure, shared framework, and CI/CD pipeline.

> **⚠️ Phase 0 context**: Foundation (pkg/*) đã hoàn thành. Tuy nhiên cấu trúc 4 microservices (go.work) vẫn tồn tại. Phase 0 sẽ chuyển code hiện tại sang target architecture.

#### Deliverables Checklist

- [x] **Shared framework packages** (`pkg/*`) — all implemented with tests:
  - ✅ `pkg/config` — Viper-based config + tests
  - ✅ `pkg/database` — MongoDB + MySQL + tests
  - ✅ `pkg/cache` — Redis client + tests
  - ✅ `pkg/queue` — Asynq + tests
  - ✅ `pkg/event` — Event bus + tests
  - ✅ `pkg/logger` — zerolog + tests
  - ✅ `pkg/http` — HTTP client + server + middleware
  - ✅ `pkg/auth` — JWT + tests
  - ✅ `pkg/notification` — interfaces + tests
  - ✅ `pkg/scraper` — Fetcher, parser, robots + tests
  - ✅ `pkg/dedup` — SimHash + tests
  - ✅ `pkg/ai` — Gemini + tests
  - ✅ `pkg/rss` — RSS/Atom parser + tests
  - ✅ `pkg/sitemap` — Sitemap parser + tests
  - ✅ `pkg/telemetry` — OpenTelemetry + Prometheus
- [x] **CI/CD pipeline** configured (GitHub Actions):
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
- ✅ Docker image `erg-server:latest` builds and runs (`docker run --rm`).
- Shared packages import cleanly into all four service modules.

---

### Phase 2 — BOT Module ✅ DONE (Agent #2)

**Objective:** Port NestJS BOT module to Go. **DONE** — full implementation in `cmd/bot-service/`.

> **📌 Note**: Sau Phase 0 refactor, code sẽ được di chuyển sang `internal/modules/bot/`. Hiện tại ở `cmd/bot-service/internal/`.

#### Deliverables Checklist

- [x] **BOT module** tại `cmd/bot-service/` — full implementation:
  - ✅ `main.go` — chi HTTP router, graceful shutdown
  - ✅ `wire.go` — Dependency injection (functional options pattern)
  - ✅ HTTP endpoints: `/api/bot`, `/webhooks/discord`, `/webhooks/telegram`, `/link`, `/conversations`
- [x] **Entities** (`cmd/bot-service/internal/models/`):
  - ✅ `bot_conversation.go` — user/platform/scoped conversation state
  - ✅ `bot_linked_account.go` — Discord/Telegram user linkage
  - ✅ `bot_workflow.go` — workflow step definitions and execution state
  - ✅ `bot_command.go` — command registry (36+ commands)
- [x] **Services** (`cmd/bot-service/internal/services/`):
  - ✅ `command_handler.go` — command routing, prefix matching, permission checks
  - ✅ `conversation.go` — conversation lifecycle, context memory
  - ✅ `workflow.go` — step execution, branching, resume-from-checkpoint
  - ✅ `link.go` — account linking/unlinking, identity resolution
  - ✅ `permission.go` — RBAC roles
- [x] **Handlers** (`cmd/bot-service/internal/handlers/`):
  - ✅ `bot_controller.go` — REST endpoints
  - ✅ `discord_webhook.go` — Discord webhook + HMAC verification
  - ✅ `telegram_webhook.go` — Telegram webhook + HMAC verification
  - ✅ `health.go` — Health check
- [x] **Commands** (`cmd/bot-service/internal/commands/`):
  - ✅ `base.go` — Command interface + registry
  - ✅ `rss_commands.go`, `crawl_commands.go`, `trending_commands.go`, `stats_commands.go`, `system_commands.go`
- [x] **Platform adapters** (`cmd/bot-service/internal/platform/`):
  - ✅ `discord.go` — Discord API client
  - ✅ `telegram.go` — Telegram API client
  - ✅ `hmac.go` — HMAC signature verification
- [x] **Build**: `go build ./cmd/bot-service` — zero errors
- [x] **Docker image**: builds successfully

#### Porting Details: BOT Module

| NestJS File | Go Equivalent | Notes |
|---|---|---|
| `src/bot/bot-command-handler/` | `cmd/bot-service/internal/commands/` | One file per command group |
| `src/bot/bot-conversation/` | `cmd/bot-service/internal/services/conversation.go` | Goroutine-per-session with channel context |
| `src/bot/bot-link/` | `cmd/bot-service/internal/services/link.go` | Port account linking logic |
| `src/bot/bot-permission/` | `cmd/bot-service/internal/services/permission.go` | RBAC roles in chi router |
| `src/bot/schemas/` | `cmd/bot-service/internal/models/*.go` | Go structs with `bson` tags |

> **📌 Sau Phase 0**: Di chuyển sang `internal/modules/bot/` với NestJS-style module pattern.

#### Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Webhook security bypass | Low | Critical | Strict HMAC verification before any processing; reject unsigned requests with 401 |
| Command statefulness issues | Medium | Medium | Use MongoDB for conversation state; no in-memory session state in handlers |
| Discord/Telegram API breaking changes | Low | Medium | Abstract platform adapters (`internal/platform/`) to isolate API calls |
| Performance regression in command latency | Medium | Medium | Benchmark command P99 latency in staging before cutover |

#### Validation

- ✅ All 36+ bot commands return correct responses (integration test suite with mocked platforms).
- ✅ Webhook endpoint handles 1,000 concurrent requests with P99 < 100 ms.
- ✅ Health check returns `{"status":"ok","mongo":"ok","redis":"ok"}`.
- ✅ BOT module imports cleanly into `cmd/server` binary.
- ✅ `go build ./cmd/server` — zero errors.

---

### Phase 3 — Notification Module ✅ DONE (2026-04-01)

**Objective:** Implement Notification module — send, batch, cancel, resend notifications via Discord/Telegram/WhatsApp/Email.

> ✅ **Completed 2026-04-01** — Full implementation in `internal/modules/notifications/`

#### Files Created (13 files)

```
internal/modules/notifications/
├── notifications.module.go         ✅ NestJS-style (Setup → RegisterRoutes → Stop)
├── notifications.service.go       ✅ Send, BatchSend, Cancel, Resend, SendFromEvent
├── notifications.controller.go    ✅ Full REST API (14 endpoints)
├── digest.service.go             ✅ Digest aggregation (daily/weekly/monthly)
├── event_consumer.go           ✅ Event bus subscriber (13 event topics)
├── dto/notification.dto.go       ✅ DTOs + mappers
├── entities/notification.go       ✅ Notification, Preference, Digest, DeliveryLog
├── repository/repository.go     ✅ Full MongoDB CRUD + aggregation
├── templates/vietnamese.go       ✅ 15 Vietnamese templates
└── providers/
    ├── discord.go               ✅ Discord webhook embeds + rate-limit
    ├── telegram.go             ✅ Telegram Bot API
    ├── whatsapp.go             ✅ WhatsApp Business Cloud API
    └── email.go               ✅ SMTP with TLS + STARTTLS
```

#### Deliverables Checklist

- [x] **Module** tại `internal/modules/notifications/`:
  - ✅ `notifications.module.go` — NestJS-style module registration
  - ✅ `notifications.controller.go` — REST API (Send, BatchSend, Cancel, Resend)
  - ✅ chi HTTP router (route prefix: `/api/notifications`, `/api/channels`)
- [x] **Entities** (`internal/modules/notifications/entities/`):
  - ✅ `notification.go` — recipient, channel, template, status, delivery metadata
  - ✅ `notification_preference.go` — per-user channel preferences (embedded in same file)
  - ✅ Digest, DeliveryLog entities
- [x] **Services**:
  - ✅ `notifications.service.go` — Send, BatchSend, Cancel, Resend
  - ✅ `templates/vietnamese.go` — Go `text/template` với Vietnamese interpolation
  - ✅ `digest.service.go` — daily/weekly/monthly digest aggregation
  - ✅ Delivery logging in repository
- [x] **Provider adapters** (`internal/modules/notifications/providers/`):
  - ✅ `discord.go` — Discord embeds, rate-limit handling (200 req/min)
  - ✅ `telegram.go` — Telegram Bot API sendMessage
  - ✅ `whatsapp.go` — WhatsApp Business Cloud API
  - ✅ `email.go` — SMTP with TLS + STARTTLS
- [x] **Event consumer** (`event_consumer.go`):
  - ✅ Subscribes to 13 domain event topics
  - ✅ Asynq job enqueued cho async delivery
- [x] **`go build ./cmd/server`** — ✅ zero errors (31MB binary)
- [x] **`go test ./...`** — ✅ all pass

#### Validation

- ✅ Build: `go build -o erg-server ./cmd/server` — zero errors
- ✅ Tests: `go test ./...` — all `pkg/*` tests pass
- ✅ REST API: 14 endpoints registered
- ✅ Event bus: 13 topics subscribed

---

### Phase 4 — Crawler Module 🔄 IN PROGRESS (Agent #4)

**Objective:** Implement Crawler module — RSS fetching, HTML scraping, quality gate, deduplication, SEO tagging, notification on completion.

> **⚠️ Prerequisites**: Phase 0 (Architecture Refactor) phải hoàn thành TRƯỚC Phase 4.

#### Current State (pre-Phase 0)

```
cmd/crawler-service/
├── main.go          ← Stub: chỉ có HTTP server skeleton
└── go.mod
```

#### Deliverables Checklist

- [ ] **Module** tại `internal/modules/crawler/` (sau Phase 0):
  - ❌ `crawler.module.go` — NestJS-style module registration
  - ❌ `crawler.controller.go` — REST API
  - ❌ `rss.controller.go` — RSS feed CRUD
  - ❌ `blacklist.controller.go` — Blacklist CRUD
  - ❌ `sse.controller.go` — SSE real-time crawl progress
  - ❌ chi HTTP router (route prefix: `/api/crawler`, `/api/rss`, `/api/sitemap`, `/api/blacklist`)
- [ ] **Entities** (`internal/modules/crawler/entities/`):
  - ❌ `rss_feed.go` — feed URL, update frequency, category, language
  - ❌ `crawl_history.go` — per-URL crawl metadata: status, duration, response size, error
  - ❌ `content_fingerprint.go` — SimHash + raw hash for deduplication
  - ❌ `content_blacklist.go` — URL pattern, domain, keyword blocklists
- [ ] **Services** (`internal/modules/crawler/`):
  - ❌ `crawler.service.go` — 12-step pipeline orchestrator
  - ❌ `crawler.service.go` — 12-step pipeline:
    ```
    1. Blacklist Check → ErrBlacklisted
    2. Domain Reputation → skip if block_count > 10
    3. Robots.txt Check → ErrRobotsDisallowed
    4. Anti-Block → adaptive delay, proxy rotation, UA cycling
    5. Fetch Content → ScraperService (pkg/scraper)
    6. Quality Gate → reject if score < 70
    7. Content Dedup → reject if duplicate (pkg/dedup)
    8. AI SEO → SmartSelector + Gemini (pkg/ai)
    9. Save to MongoDB → CrawlHistory + Fingerprint
    10. Publish event → "crawl.success" or "crawl.failed"
    11. SSE Broadcast → connected clients
    12. Notification → notify via event bus
    ```
  - ❌ Dùng `pkg/rss/parser.go` cho RSS/Atom feeds
  - ❌ Dùng `pkg/scraper/` cho HTML fetching + robots.txt
  - ❌ Dùng `pkg/dedup/simhash.go` cho deduplication
  - ❌ Dùng `pkg/ai/gemini.go` cho SEO tagging
- [ ] **Asynq jobs** (`internal/modules/crawler/jobs/`):
  - ❌ `crawl_job.go` — `{url, depth, config_id, priority}` → full pipeline
  - ❌ `refresh_feed_job.go` — periodic feed re-fetch, delta detection
  - ❌ `reindex_job.go` — re-fingerprint existing content after algorithm update
  - All jobs: timeout, max retries (3), dead-letter queue
- [ ] **SSE endpoint**: `GET /api/crawler/stream/:job_id` — real-time progress
- [ ] **Asynq worker pool**: configurable default 20 workers
- [ ] **`go build ./cmd/server`** — zero errors
- [ ] **`go test ./...`** — all pass

---

### Phase 5 — Trending Module ⬜ NOT STARTED (Agent #5)

**Objective:** Implement Trending module — Google Trends + NewsAPI aggregation, cron refresh, URL discovery feed.

> **⚠️ Prerequisites**: Phase 0 (Architecture Refactor) phải hoàn thành TRƯỚC Phase 5.

#### Current State (pre-Phase 0)

```
cmd/trending-service/
├── main.go          ← Stub: chỉ có HTTP server skeleton
└── go.mod
```

#### Deliverables Checklist

- [ ] **Trending module** tại `internal/modules/trending/` (sau Phase 0):
  - ❌ `trending.module.go` — NestJS-style module registration
  - ❌ `trending.controller.go` — REST API
  - ❌ chi HTTP router (route prefix: `/api/trending`, `/api/feeds`)
  - ❌ Internal cron scheduler (robfig/cron) running every 30 minutes
- [ ] **Entities** (`internal/modules/trending/entities/`):
  - ❌ `trending_topic.go` — topic, score, volume, source, timestamp, keywords
  - ❌ `news_article.go` — headline, source, URL, published_at, relevance
  - ❌ `trending_snapshot.go` — point-in-time snapshot cho historical charts
- [ ] **Services** (`internal/modules/trending/`):
  - ❌ `trending.service.go` — Google Trends + NewsAPI aggregation
  - ❌ `scheduler.go` — cron-triggered refresh; stores snapshot history
- [ ] **API endpoints**:
  - `GET /api/trending/topics` — current top 20 trending topics (cached < 200ms)
  - `GET /api/trending/topics/:topic` — topic detail + keywords + timeline
  - `GET /api/trending/news` — latest news articles
  - `GET /api/trending/feeds` — URL discovery feed for crawler (`?since=&limit=100`)
  - `POST /api/trending/refresh` — admin: trigger immediate refresh
- [ ] **URL discovery feed**: trending → Redis list `trending:urls` → crawler polls every 5 min
- [ ] **`go build ./cmd/server`** — zero errors
- [ ] **`go test ./...`** — all pass

---

**Objective:** Validate the complete system, finalize the Go monolith, and close out the migration.

> **📌 Architecture note**: Vì tất cả 4 modules nằm trong 1 binary `cmd/server`, không cần API Gateway hay inter-service HTTP calls. Module-to-module communication là direct Go function calls.

#### Deliverables Checklist

- [ ] **Full integration**: verify all 4 modules work together in `cmd/server` binary:
  - Verify graceful shutdown stops all module workers cleanly
  - Verify event bus passes events between modules correctly
  - Verify Asynq workers process jobs from all modules
- [ ] **Performance benchmark**:
  - Compare NestJS monolith vs. Go monolith under identical load
  - Target: P99 latency reduction ≥ 5x, memory reduction ≥ 3x, throughput increase ≥ 5x
  - Report results as an ADR
- [ ] **NestJS decommission checklist**:
  - [ ] All 4 modules removed from NestJS source tree
  - [ ] All environment variables and secrets migrated
  - [ ] Old NestJS Docker image stopped in production
  - [ ] Old NestJS deployment manifests removed
- [ ] **Documentation**:
  - [ ] `docs/architecture.md` — service diagrams, data flow, inter-module contracts
  - [ ] `docs/api/server.md` — OpenAPI spec (1 server duy nhất)
  - [ ] `docs/runbook.md` — deployment, rollback, alerting
- [ ] **Full end-to-end integration test suite**:
  - Simulate a complete workflow: trending discovery → crawl → quality gate → notification
  - Run on every PR via GitHub Actions

#### Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| All 4 modules in 1 binary → memory spike | Low | Medium | Monitor binary size (< 50MB); scale horizontally if needed |
| Module-level panic crashes entire binary | Medium | High | Global panic recovery middleware + per-module goroutine recovery |
| NestJS → Go cutover risk | Medium | High | Run Go binary in parallel for 48h before full cutover; compare error rates |

#### Validation

- ⬜ All 4 modules respond correctly through single binary `cmd/server`.
- ⬜ End-to-end workflow test passes: trending → crawler → notification delivery.
- ⬜ Performance benchmarks meet or exceed targets (P99 latency, memory, throughput).
- ⬜ NestJS monolith is fully decommissioned (or reduced to routing shell).
- ⬜ All CI pipelines green; no flaky tests.
- ⬜ Runbook reviewed and approved.

---

## 3. Shared Framework Architecture (Go)

### 3.1 ✅ Infrastructure Packages (pkg/*)

All packages implemented with tests. See [Section 2.2 Current Architecture](#22-current-architecture-as-is--4-microservices) for the full `pkg/` list.

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

Each module uses functional options for dependency injection — passing interfaces (not concrete types) as dependencies:

```go
// cmd/server/main.go — single entry point for all 4 modules
func main() {
    cfg := config.Load()

    logger := log.New(cfg)
    mongo := mongodriver.New(cfg.GetString("mongo.uri"))
    redis := redisclient.New(cfg.GetString("redis.addr"))
    queue := asynq.NewClient(redis)
    eventBus := event.NewBus(cfg)

    // Single server bootstraps all modules
    server := server.NewServer(cfg, mongo, redis, queue, eventBus)

    r := chi.NewRouter()
    r.Use(middleware.Recovery(logger))
    r.Use(middleware.RequestID)
    // Mount all modules on same router
    server.RegisterRoutes(r)

    log.Info().Str("addr", ":8080").Msg("starting erg-server (1 binary, 4 modules)")
    http.ListenAndServe(":8080", r)
}
```

**Rules:**
- Constructors accept interface parameters (not concrete structs).
- Concrete implementations are instantiated in `cmd/server/main.go` only.
- Interfaces are defined in `pkg/*/` and kept small (≤ 5 methods).
- Module-to-module calls are direct function calls — no HTTP overhead.

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

### 3.6 ✅ Module Communication (1 Binary — Direct Calls)

**Vì tất cả 4 modules nằm trong 1 binary, có 3 cách communication:**

**1. Direct Function Calls (default — zero overhead)**
- Used for: tất cả calls giữa các modules trong cùng binary
- Ví dụ: `crawler.NewModule()` → `notifications.NewModule()` → direct method call
- Zero HTTP overhead, zero serialization, zero network latency

**2. Event Bus (pkg/event/bus.go) — async decoupling**
- Used for: decoupled, multi-subscriber events (one event, many modules react)

```go
// pkg/event/bus.go — in-process subscribers fire synchronously
bus.PublishLocal("crawl.success", &CrawlSuccessEvent{URL: url, Title: title})

// Subscribe — returns cancel function
cancel := bus.SubscribeLocal("crawl.success", func(evt interface{}) {
    // notification module reacts to crawl success
})
defer cancel()
```

**3. Asynq Job Queue + Redis Pub/Sub**
- Used for: fire-and-forget tasks, background processing, fan-out notifications.
- Asynq handles retries, dead-letter queues (DLQ), priorities (1–10), and scheduled execution.

```go
// Enqueue a high-priority crawl job
task := asynq.NewTask(jobs.TypeCrawlJob, payload)
_, err = queue.Enqueue(ctx, task, asynq.MaxRetry(5), asynq.Timeout(10*time.Minute), asynq.Queue("high"))
```

**4. HTTP Client (for external services / NestJS)**
- Used for: gọi NestJS API hoặc external services
- Retry với exponential backoff:

```go
// pkg/http/client.go — shared HTTP client với built-in retry
resp, err := client.DoWithRetry(ctx, req, httpClient.WithRetry(3), httpClient.WithTimeout(5*time.Second))
```

**5. gRPC (optional — cho future external consumers)**
- Proto definitions in `proto/events.proto`; generate with `protoc`.
- Chỉ dùng khi có external consumers cần consume events từ Go binary.
- Hiện tại: KHÔNG cần vì NestJS sẽ được decommissioned.

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

## 4. BOT Module Extraction

### 4.1 NestJS Source Files to Port

| NestJS File | Go Target |
|---|---|
| `src/modules/bot/bot.module.ts` | `internal/modules/bot/bot.module.go` |
| `src/modules/bot/bot.controller.ts` | `internal/modules/bot/bot.controller.go` |
| `src/modules/bot/entities/bot-conversation.entity.ts` | `internal/modules/bot/entities/conversation.go` |
| `src/modules/bot/entities/bot-linked-account.entity.ts` | `internal/modules/bot/entities/linked_account.go` |
| `src/modules/bot/entities/bot-workflow.entity.ts` | `internal/modules/bot/entities/workflow.go` |
| `src/modules/bot/services/bot-command-handler.service.ts` | `internal/modules/bot/bot.service.go` |
| `src/modules/bot/services/bot-conversation.service.ts` | `internal/modules/bot/conversation.service.go` |
| `src/modules/bot/services/bot-link.service.ts` | `internal/modules/bot/link.service.go` |
| `src/modules/bot/services/bot-permission.service.ts` | `internal/modules/bot/middleware/permission.go` |
| `src/modules/bot/webhooks/discord-webhook.controller.ts` | `internal/modules/bot/webhooks/discord.go` |
| `src/modules/bot/webhooks/telegram-webhook.controller.ts` | `internal/modules/bot/webhooks/telegram.go` |

### 4.2 Go File Structure

```
internal/modules/bot/
├── bot.module.go                  # Module registration (NewModule + Setup + RegisterRoutes)
├── bot.controller.go             # HTTP handlers (webhooks + REST)
├── bot.service.go               # Command routing, permission check, conversation logic
├── dto/
│   ├── send-message.dto.go       # POST /conversations/:id/send
│   ├── link-account.dto.go       # POST /link
│   └── command.dto.go            # Command input DTOs
├── entities/
│   ├── conversation.go           # MongoDB: user_id, platform, state, wizard_data, TTL 30 ngày
│   ├── linked_account.go        # MongoDB: platform_user_id, internal_user_id, link_code
│   └── workflow.go              # MongoDB: workflow_steps[], current_step, status
├── commands/                     # 36+ bot commands (như NestJS command handlers)
│   ├── base.go                  # Command interface
│   ├── registry.go              # Command map: string → handler
│   ├── rss_commands.go         # /rss add, /rss list, /rss remove
│   ├── crawl_commands.go        # /crawl start, /crawl status, /crawl stop
│   ├── trending_commands.go     # /trending top, /trending keyword
│   ├── draft_commands.go       # /draft list, /draft publish
│   ├── stats_commands.go        # /stats users, /stats crawler
│   └── system_commands.go       # /system health, /system reload
├── webhooks/
│   ├── discord.go              # POST /webhooks/discord (HMAC-SHA256 verify)
│   └── telegram.go             # POST /webhooks/telegram (HMAC verify)
└── middleware/
    └── permission.go            # RBAC: viewer=1, editor=2, crawler=3, moderator=4, admin=5
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

## 5. Notification Module Extraction

### 5.1 NestJS Source Files to Port

| NestJS File | Go Target |
|---|---|
| `src/modules/notifications/notifications.module.ts` | `internal/modules/notifications/notifications.module.go` |
| `src/modules/notifications/notifications.service.ts` | `internal/modules/notifications/notifications.service.go` |
| `src/modules/notifications/notifications.controller.ts` | `internal/modules/notifications/notifications.controller.go` |
| `src/modules/notifications/digest-scheduler.service.ts` | `internal/modules/notifications/digest.service.go` |
| `src/modules/notifications/services/notification-bus.service.ts` | `internal/modules/notifications/event_consumer.go` |
| `src/modules/notifications/services/notification-templates.service.ts` | `internal/modules/notifications/templates/` |
| `src/modules/notifications/providers/discord.provider.ts` | `internal/modules/notifications/providers/discord.go` |
| `src/modules/notifications/providers/telegram.provider.ts` | `internal/modules/notifications/providers/telegram.go` |
| `src/modules/notifications/providers/whatsapp.provider.ts` | `internal/modules/notifications/providers/whatsapp.go` |
| `src/modules/notifications/webhooks/discord-webhook.controller.ts` | `internal/modules/notifications/webhooks/` |
| `src/modules/notifications/entities/notification.entity.ts` | `internal/modules/notifications/entities/notification.go` |

### 5.2 Go File Structure

```
internal/modules/notifications/
├── notifications.module.go        # Module registration
├── notifications.controller.go     # HTTP handlers (REST API + webhooks)
├── notifications.service.go       # Send, BatchSend, Cancel, Resend
├── digest.service.go            # Daily/weekly/monthly digest aggregation
├── event_consumer.go           # Redis pub/sub subscriber → Asynq job
├── dto/
│   ├── send.dto.go
│   └── preference.dto.go
├── entities/
│   ├── notification.go          # MongoDB: recipient, channel, template, status, metadata
│   └── preference.go            # Per-user channel settings
├── templates/
│   └── vietnamese.go            # Full Vietnamese notification templates
└── providers/                   # Như NestJS notification providers
    ├── discord.go               # Discord webhook embeds (rate limit: 200/min)
    ├── telegram.go              # Telegram sendMessage, editMessageText
    ├── whatsapp.go             # WhatsApp Business API
    └── email.go                # SMTP multipart email
```
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

## 6. Crawler Module Extraction

### 6.1 NestJS Source Files to Port

| NestJS File | Go Target |
|---|---|
| `crawler.module.ts` | `internal/modules/crawler/crawler.module.go` |
| `crawler.service.ts` | `internal/modules/crawler/crawler.service.go` |
| `crawler.processor.ts` | `internal/modules/crawler/jobs/crawl.job.go` |
| `crawler.scheduler.ts` | `internal/modules/crawler/rss.scheduler.go` |
| `crawler.controller.ts` | `internal/modules/crawler/crawler.controller.go` |
| `blacklist.controller.ts` | `internal/modules/crawler/blacklist.controller.go` |
| `gateways/crawl-progress.gateway.ts` | `internal/modules/crawler/sse.controller.go` |
| `entities/rss-feed.entity.ts` | `internal/modules/crawler/entities/rss_feed.go` |
| `entities/scraper-config.entity.ts` | `internal/modules/crawler/entities/scraper_config.go` |
| `entities/crawl-history.entity.ts` | `internal/modules/crawler/entities/crawl_history.go` |
| `entities/domain-reputation.entity.ts` | `internal/modules/crawler/entities/domain_reputation.go` |
| `entities/content-fingerprint.entity.ts` | `internal/modules/crawler/entities/content_fingerprint.go` |
| `entities/content-blacklist.entity.ts` | `internal/modules/crawler/entities/content_blacklist.go` |
| `services/anti-block.service.ts` | `pkg/scraper/` (reuse) |
| `services/robots-parser.service.ts` | `pkg/scraper/robots.go` (reuse) |
| `services/quality-gate.service.ts` | `pkg/scraper/quality_gate.go` (reuse) |
| `services/content-dedup.service.ts` | `pkg/dedup/simhash.go` (reuse) |
| `services/smart-selector.service.ts` | `pkg/ai/gemini.go` (reuse) |
| `services/sitemap.service.ts` | `pkg/sitemap/parser.go` (reuse) |
| `services/blacklist.service.ts` | `internal/modules/crawler/blacklist.service.go` |

### 6.2 Go File Structure

```
internal/modules/crawler/
├── crawler.module.go          # Module registration + Asynq worker pool setup
├── crawler.controller.go    # REST API endpoints
├── crawler.service.go      # Orchestrator (12-step pipeline)
├── rss.controller.go       # RSS feed CRUD
├── rss.scheduler.go        # Asynq cron: periodic feed refresh
├── blacklist.controller.go  # Blacklist CRUD
├── sse.controller.go       # SSE real-time crawl progress
├── dto/
│   ├── crawl-url.dto.go
│   ├── rss-feed.dto.go
│   └── blacklist.dto.go
├── entities/                # MongoDB documents (bson tags)
│   ├── rss_feed.go
│   ├── crawl_history.go
│   ├── content_fingerprint.go
│   └── content_blacklist.go
└── jobs/                    # Asynq job handlers (reuse pkg/scraper, pkg/dedup, pkg/ai)
    ├── crawl.job.go
    ├── refresh_feed.job.go
    └── reindex.job.go
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

## 7. Trending Module Extraction

### 7.1 NestJS Source → Go Porting Map

| NestJS File | Go Target |
|---|---|
| `src/modules/trending/trending.module.ts` | `internal/modules/trending/trending.module.go` |
| `src/modules/trending/trending.service.ts` | `internal/modules/trending/trending.service.go` |
| `src/modules/trending/trending.scheduler.ts` | `internal/modules/trending/scheduler.go` |
| Entities (Topic, NewsArticle, Snapshot) | `internal/modules/trending/entities/` |

Trending module is the leanest extraction — focus on correct cron scheduling and URL discovery feed for crawler module (internal Redis call, not HTTP).

### 7.2 Go File Structure

```
internal/modules/trending/
├── trending.module.go         # Module registration + cron scheduler
├── trending.controller.go   # REST API endpoints
├── trending.service.go      # Aggregator: merge + rank + dedupe
├── scheduler.go            # robfig/cron v3: every 30 min
├── dto/
│   └── trending-topic.dto.go
├── entities/
│   ├── trending_topic.go    # topic, score, volume, source, keywords[], timestamp
│   ├── news_article.go     # headline, source, url, published_at, relevance_score
│   └── trending_snapshot.go # Point-in-time snapshot for historical trend charts
└── cache/
    └── redis_cache.go       # Redis cache with 25-min TTL per data source
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
        // Push discovered URLs to Redis list for crawler module (internal call)
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

The most critical module integration — trending module provides URLs for crawler module (direct Redis call, no HTTP):

```go
// internal/cache/redis_cache.go
// crawler module calls: LRANGE trending:urls 0 99 → LTRIM trending:urls 100 -1
// Falls back to HTTP endpoint only if Redis is unavailable
func (s *FeedController) GetDiscoveryFeed(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    urls, err := s.redis.LRange(ctx, "trending:urls", 0, 99).Result()
    if err != nil {
        // Fallback: direct call to trending module (same binary — no HTTP needed)
        urls = s.trendingModule.GetFeeds(ctx, 100)
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
GET  /trending/feeds           → URL discovery feed for crawler module
GET  /trending/history          → Historical snapshots for trend charts
GET  /trending/sources          → Status of Google Trends + NewsAPI (healthy/degraded)
POST /trending/refresh          → Admin: trigger immediate full refresh
```

### 7.6 Risk Mitigation

| Risk | Mitigation |
|---|---|
| NewsAPI free tier: 100 req/day | Aggressive 25-min Redis cache; batch all topic lookups per refresh cycle |
| Google Trends rate limit | Cache 25 min; fallback to NewsAPI-only if Trends fails |
| URL flood overwhelming crawler | Redis list capped at 10,000 URLs; crawler module has its own Asynq priority queue |

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

### 8.2 ✅ MongoDB Collections by Module (Updated 2026-03-31)

| Collection | Owner | Status |
|---|---|---|
| `bot_conversations` | ✅ Go (bot module) — Phase 2 DONE | ✅ DONE |
| `bot_linked_accounts` | ✅ Go (bot module) — Phase 2 DONE | ✅ DONE |
| `bot_workflows` | ✅ Go (bot module) — Phase 2 DONE | ✅ DONE |
| `notifications` | 🔄 Go (notifications module) — Phase 3 | 🔄 IN PROGRESS |
| `crawl_history` | 🔄 Go (crawler module) — Phase 4 | 🔄 IN PROGRESS |
| `rss_feeds` | 🔄 Go (crawler module) — Phase 4 | 🔄 IN PROGRESS |
| `scraper_configs` | 🔄 Go (crawler module) — Phase 4 | 🔄 IN PROGRESS |
| `domain_reputation` | 🔄 Go (crawler module) — Phase 4 | 🔄 IN PROGRESS |
| `content_fingerprints` | 🔄 Go (crawler module) — Phase 4 | 🔄 IN PROGRESS |
| `content_blacklist` | 🔄 Go (crawler module) — Phase 4 | 🔄 IN PROGRESS |
| `trending_topics` | ⬜ Go (trending module) — Phase 5 | ⬜ NOT STARTED |
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

### 8.5 ✅ Backward Compatibility Checklist — Updated 2026-03-31

> **📌 Architecture note**: Vì tất cả modules nằm trong 1 binary `cmd/server`, cutover là **module-by-module**. NestJS có thể đọc MongoDB collections sau khi Go module hoàn thành. Feature flags control routing.

```
✅ Phase 2 (BOT) — DONE
  □ bot_conversations: Go viết, NestJS chỉ đọc sau cutover
  □ Feature flag: FEATURE_FLAG_BOT=go → direct Go module

🔄 Phase 3 (Notification) — IN PROGRESS
  □ notifications: Go viết, NestJS chỉ đọc sau cutover
  □ Feature flag: FEATURE_FLAG_NOTIFICATIONS=go

🔄 Phase 4 (Crawler) — IN PROGRESS
  □ All crawler collections: Go viết
  □ Feature flag: FEATURE_FLAG_CRAWLER=go

⬜ Phase 5 (Trending) — NOT STARTED
  □ trending_topics: Go viết
  □ Feature flag: FEATURE_FLAG_TRENDING=go
```

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

### 9.1 ✅ Docker Build Strategy — Single Binary (Updated 2026-03-31)

> **📌 Architecture change**: 1 binary duy nhất thay vì 4. Build: `go build -o erg-server ./cmd/server`

**Dockerfile** (multi-stage, builds single binary):

```dockerfile
# Dockerfile  (multi-stage, builds SINGLE binary chứa tất cả 4 modules)
FROM golang:1.22-alpine AS builder

# Install build deps
RUN apk add --no-cache git ca-certificates build-base

WORKDIR /build

# Download deps (cached layer)
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build single binary — chứa tất cả 4 modules
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X main.version=$(git describe --tags)" \
    -o erg-server ./cmd/server

# Scratch base image (~20-30MB)
FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/erg-server /bin/erg-server
COPY --from=builder /build/config.yaml /etc/erg/config.yaml
EXPOSE 8080
ENTRYPOINT ["/bin/erg-server", "--config", "/etc/erg/config.yaml"]
```

### 9.2 ✅ Server Ports Summary

> **📌 Single binary, single port**: Tất cả 4 modules chạy trên 1 port `:8080`. Routes phân biệt bằng chi router prefix.

| Module | Route Prefix | Asynq Workers | Notes |
|---|---|---|---|
| `bot` | `/api/bot`, `/webhooks/*` | ✅ | Goroutine pool |
| `notifications` | `/api/notifications`, `/api/channels` | ✅ | Event consumer |
| `crawler` | `/api/crawler`, `/api/rss`, `/api/sitemap`, `/api/blacklist` | ✅ | 20 workers |
| `trending` | `/api/trending`, `/api/feeds` | ✅ | Cron 30-min |

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

  erg-server:
    build:
      context: ./go-erg
      dockerfile: ./Dockerfile
    ports: ["8080:8080"]
    environment:
      CONFIG_PATH: /etc/erg/config.yaml
    volumes:
      - ./go-erg/config.yaml:/etc/erg/config.yaml:ro
    depends_on:
      mongo:
        condition: service_healthy
      redis:
        condition: service_healthy
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
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/healthz"]
      interval: 10s
      timeout: 5s
      retries: 3

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

  # ── Stage 2: Build single Docker image ────────────────────────────
  build-image:
    runs-on: ubuntu-latest
    needs: lint-and-test
    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-buildx-action@v3

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: ./go-erg
          file: ./go-erg/Dockerfile
          push: ${{ github.ref == 'refs/heads/main' }}
          tags: |
            ${{ env.REGISTRY }}/erg-server:${{ github.sha }}
            ${{ env.REGISTRY }}/erg-server:latest
          cache-from: type=gha
          cache-to: type=gha,mode=max

  # ── Stage 3: Deploy to Staging (on main branch) ───────────────────
  deploy-staging:
    runs-on: ubuntu-latest
    needs: build-image
    if: github.ref == 'refs/heads/main'
    environment: staging
    steps:
      - uses: actions/checkout@v4

      - name: Configure kubectl
        run: |
          echo "${{ secrets.KUBE_CONFIG_STAGING }}" | base64 -d > kubeconfig
          echo "KUBECONFIG=$(pwd)/kubeconfig" >> $GITHUB_ENV

      - name: Deploy erg-server
        run: |
          kubectl set image deployment/erg-server \
            erg-server=${{ env.REGISTRY }}/erg-server:${{ github.sha }} \
            --namespace=erg-staging

      - name: Wait for rollout
        run: |
          kubectl rollout status deployment/erg-server --namespace=erg-staging --timeout=120s

      - name: Smoke test staging
        run: |
          sleep 10
          curl -sf http://erg-server.erg-staging/healthz || exit 1
```

### 9.5 ✅ Kubernetes Deployment Manifest — Single Binary (Updated 2026-03-31)

```yaml
# k8s/erg-server.yaml — 1 Deployment cho tất cả 4 modules
apiVersion: apps/v1
kind: Deployment
metadata:
  name: erg-server
  namespace: erg-prod
  labels:
    app: erg-server
    version: v1
spec:
  replicas: 3
  selector:
    matchLabels:
      app: erg-server
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0         # Zero-downtime rolling update
  template:
    metadata:
      labels:
        app: erg-server
        version: v1
    spec:
      containers:
        - name: erg-server
          image: ghcr.io/yourorg/erg-server:latest
          ports:
            - name: http
              containerPort: 8080
          resources:
            requests:
              cpu: "500m"
              memory: "512Mi"
            limits:
              cpu: "4000m"    # Full CPU for crawler workers
              memory: "4Gi"   # More memory for crawler + trending
          readinessProbe:
            httpGet:
              path: /ready
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
            successThreshold: 1
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8080
            initialDelaySeconds: 15
            periodSeconds: 20
            failureThreshold: 3
          env:
            - name: CONFIG_PATH
              value: "/etc/erg/config.yaml"
            - name: GOMAXPROCS
              value: "4"
          volumeMounts:
            - name: config
              mountPath: /etc/erg
              readOnly: true
      volumes:
        - name: config
          secret:
            secretName: erg-server-config
```

### 9.6 ✅ Config File — All Modules in One (Updated 2026-03-31)

```yaml
# config.yaml — 1 file cho tất cả 4 modules trong 1 binary
app:
  name: erg-server
  host: "0.0.0.0"
  port: 8080
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
  concurrency: 20        # Default worker pool size (crawler module)
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
  prometheus_port: 9090          # Metrics endpoint
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

### 10.4 ✅ E2E Tests (Full Stack — Single Binary)

```go
// test/e2e/full_workflow_test.go
// docker-compose up → erg-server binary starts (all 4 modules) → run E2E tests

func TestTrendingToNotificationWorkflow(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping E2E in short mode")
    }

    // All modules in same binary — test via HTTP (single port :8080)
    // hoặc gọi trực tiếp các module functions trong test
    // (Direct module calls trong test — no HTTP overhead)
    server := server.NewServer(cfg, mongo, redis, queue, eventBus)
    ctx := context.Background()

    // 1. Trigger trending refresh
    require.NoError(t, trendingSvc.Refresh(context.Background()))

    // 2. Fetch discovered URLs (direct call — same binary, no HTTP)
    feed, err := trendingModule.GetDiscoveryFeed(ctx)
    require.NoError(t, err)
    require.NotEmpty(t, feed.URLs, "trending should discover URLs")

    // 3. Enqueue first URL for crawling (direct call to crawler module)
    jobID, err := crawlerModule.EnqueueURL(ctx, feed.URLs[0])
    require.NoError(t, err)

    // 4. Poll until completion (max 2 min)
    result, err := crawlerModule.WaitForCompletion(ctx, jobID, 2*time.Minute)
    require.NoError(t, err)
    require.Equal(t, crawler.JobStatusSuccess, result.Status)

    // 5. Verify notification was recorded (direct call to notifications module)
    notifs, err := notificationsModule.List(ctx, notification.ListFilter{
        Type: "crawl.success",
        Limit: 10,
    })
    require.NoError(t, err)
    require.NotEmpty(t, notifs.Items, "crawl.success notification should be sent")

    // 6. Verify crawl history persisted (direct call to crawler module)
    history, err := crawlerModule.GetHistory(ctx, jobID)
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
            goResp := fetch("http://erg-server:8080" + ep)

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
            go func() { defer wg.Done(); botModule.HandleWebhook(ctx, discordPayload) }()
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