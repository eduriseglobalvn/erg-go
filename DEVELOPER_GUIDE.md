# erg.ninja — Developer Guide

> **Author**: Senior Engineer + Senior Architect (30y combined exp)
> **Project**: erg-go (`erg.ninja`)
> **Version**: v0.1.0 (pre-release)

`erg.ninja` là một Go module cung cấp 4 microservices có thể import riêng lẻ hoặc chạy như một monolith binary. Được viết bằng Go 1.22+, chi/v5 router, MongoDB + Redis, và gRPC cho inter-service communication.

---

## Table of Contents

1. [Quick Start](#1-quick-start)
2. [Installation](#2-installation)
3. [Architecture Overview](#3-architecture-overview)
4. [Using as a Library](#4-using-as-a-library)
5. [Service Discovery](#5-service-discovery)
6. [Multi-Tenancy](#6-multi-tenancy)
7. [Running the Full Server](#7-running-the-full-server)
8. [Building Binaries](#8-building-binaries)
9. [Configuration](#9-configuration)
10. [Contributing](#10-contributing)

---

## 1. Quick Start

```bash
# Clone
git clone https://github.com/your-org/erg-go.git
cd erg-go

# Build
make build

# Run
./bin/erg-server

# Test
make test
```

Server khởi động tại `http://localhost:8080`. Health check tại `http://localhost:8080/healthz`.

---

## 2. Installation

```bash
go get erg.ninja@latest
```

Hoặc import trong `go.mod`:

```go
require erg.ninja v0.1.0
```

---

## 3. Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                   erg-server (cmd/server)               │
│                                                          │
│  ┌──────────┐ ┌──────────┐ ┌────────────┐ ┌──────────┐ │
│  │    bot   │ │  crawler │ │notification│ │ trending │ │
│  └────┬─────┘ └────┬─────┘ └─────┬──────┘ └────┬─────┘ │
│       │            │            │            │        │
│  ┌────┴────────────┴────────────┴────────────┴────┐  │
│  │              chi/v5 HTTP Router                 │  │
│  └──────────────────────────┬──────────────────────┘  │
│                              │                        │
│  ┌──────────────────────────┴──────────────────────┐  │
│  │  MongoDB  │  Redis  │  Asynq Queue  │  gRPC  │  │
│  └──────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

**4 Modules:**

| Module | Port | Description |
|--------|------|-------------|
| `bot` | 8086 | Telegram + Discord bot |
| `crawler` | 8083 | RSS/Atom feed crawler |
| `notification` | 8084 | Discord/Telegram/WhatsApp/Email |
| `trending` | 8085 | Trending topics aggregator |

---

## 4. Using as a Library

### 4.1 Importing Individual Services

```go
package main

import (
    "context"
    crawlerv1 "erg.ninja/lib/crawler/v1"
)

func main() {
    client, err := crawlerv1.NewClient("localhost:8083")
    if err != nil {
        panic(err)
    }
    defer client.Close()

    resp, err := client.CrawlURL(context.Background(), &crawlerv1.CrawlURLRequest{
        Url:      "https://example.com/feed.xml",
        TenantId: "default",
    })
    println("Job ID:", resp.JobId)
}
```

### 4.2 Importing Multiple Services

```go
import (
    crawlerv1 "erg.ninja/lib/crawler/v1"
    notifv1 "erg.ninja/lib/notification/v1"
    trendv1 "erg.ninja/lib/trending/v1"
)

// Create clients for each service
crawler, _ := crawlerv1.NewClient("localhost:8083")
defer crawler.Close()

notif, _ := notifv1.NewClient("localhost:8084")
defer notif.Close()

trend, _ := trendv1.NewClient("localhost:8085")
defer trend.Close()
```

---

## 5. Service Discovery

### 5.1 Direct Mode (default)

Services use static `host:port` addresses from `config.yaml`:

```yaml
discovery:
  enabled: false
  static:
    services:
      crawler:
        - address: "localhost:8083"
```

### 5.2 Dynamic Discovery (Consul / DNS / Static)

Enable service discovery for automatic load balancing:

```yaml
discovery:
  enabled: true
  backend: static  # or "consul", "dns"
```

```go
import (
    "erg.ninja/lib/shared"
    "erg.ninja/pkg/discovery"
)

// Create a static catalog (or ConsulCatalog / DNSCatalog)
catalog := discovery.NewStaticCatalog()
factory := shared.NewFactory(catalog)

// All clients automatically resolve via discovery
client, _ := crawlerv1.NewClient("crawler",
    crawlerv1.WithDiscovery(factory, "crawler"),
)
defer client.Close()
```

---

## 6. Multi-Tenancy

`erg.ninja` hỗ trợ multi-tenant isolation qua 2 strategies:

### Collection Isolation (recommended)

Mỗi tenant có collection riêng:

```
# Tenant "acme"
acme_crawl_histories
acme_feeds

# Tenant "corp"
corp_crawl_histories
corp_feeds
```

### Field Isolation

Một collection chia sẻ, query có `tenant_id` filter:

```
crawl_histories.tenant_id = "acme"
```

### Usage

```go
import "erg.ninja/pkg/tenant"

// Inject tenant ID from HTTP header
ctx := r.Context()
ctx = tenant.WithTenant(ctx, "acme") // or reads X-Tenant-ID header

// All DB operations automatically scope to tenant
```

---

## 7. Running the Full Server

### 7.1 Development

```bash
make dev
```

### 7.2 Production

```bash
# Single binary
./bin/erg-server

# Or with environment variables
JWT_SECRET=your-secret ./bin/erg-server
```

### 7.3 Docker

```bash
# Build
docker build -t erg-server .

# Run
docker run -p 8080:8080 erg-server
```

---

## 8. Building Binaries

### 8.1 Default Build (all 4 modules)

```bash
make build
# Output: bin/erg-server
```

### 8.2 Selective Module Build

```bash
# Bot + Notification only
make plugin-build/bot-notif
# Output: bin/erg-bot-notif

# Crawler + Notification only
make plugin-build/crawler-notif
# Output: bin/erg-crawler-notif
```

### 8.3 Build Tags

```bash
go build -tags 'module_crawler,module_notification' ./cmd/server
```

Available tags: `module_bot`, `module_crawler`, `module_notification`, `module_trending`, `all_modules`

---

## 9. Configuration

### 9.1 config.yaml

Copy `config.yaml.example` (if exists) or use defaults:

```yaml
app:
  port: 8080

auth:
  jwt_secret: "${JWT_SECRET}"

mongodb:
  uri: "mongodb://localhost:27017"

redis:
  host: "localhost"
  port: 6379

discovery:
  enabled: false
  backend: static

tenant:
  enabled: false
  isolation: collection
```

### 9.2 Environment Variables

```bash
export JWT_SECRET="your-secret"
export MONGO_URI="mongodb://localhost:27017"
```

---

## 10. Contributing

```bash
# Fork → Clone → Branch
git checkout -b feature/my-feature

# Develop
make test

# Lint
make lint

# Build
make build

# Open PR
```

**CI Pipeline:**
- `make ci` — full local CI (fmt + tidy + vet + test + lint)
- GitHub Actions: proto-breaking check, public API boundary, multi-arch Docker build

---

## Module API Reference

### bot (lib/bot/v1)

```go
client.ListConversations(ctx, req)
client.SendMessage(ctx, req)
client.ExecuteCommand(ctx, req)
client.StartWorkflow(ctx, req)
```

### crawler (lib/crawler/v1)

```go
client.CrawlURL(ctx, req)        // Submit URL for crawling
client.GetCrawlStatus(ctx, req) // Check job status
client.ListFeeds(ctx, req)      // List RSS feeds
client.RefreshFeed(ctx, req)     // Force-refresh feed
```

### notification (lib/notification/v1)

```go
client.Send(ctx, req)                 // Single notification
client.SendBulk(ctx, req)            // Batch
client.GetPreferences(ctx, req)      // User preferences
client.UpdatePreferences(ctx, req)   // Update preferences
```

### trending (lib/trending/v1)

```go
client.GetTopTopics(ctx, req)       // Trending topics
client.GetTopic(ctx, req)          // Single topic
client.SearchTopics(ctx, req)      // Search
client.GetLatestSnapshot(ctx, req)  // Current snapshot
```