# Runbook — erg-go

> **Binary**: `cmd/server` | **Library**: `go build ./lib/...` | **Full build**: `go build ./...`
> **Go version**: 1.21+ | **Proto**: libprotoc 34+

---

## Quick Start

```bash
# Development
go run ./cmd/server

# Production build (standalone binary)
go build -o erg-server ./cmd/server
./erg-server

# Build library only (no binary, just lib/)
go build ./lib/...

# Build with specific modules (plugin mode)
go build -tags "module_crawler,module_notification" -o erg-crawler ./cmd/server

# Docker
docker compose up -d
```

---

## Configuration

### config.yaml (standalone server)

```yaml
app:
  env: production
  host: "0.0.0.0"
  port: 8080
  read_timeout: 30s
  write_timeout: 60s
  shutdown_time: 30s

mongodb:
  uri: mongodb://localhost:27017
  database: erg_server

redis:
  host: localhost
  port: 6379

queue:
  redis_host: localhost
  redis_port: 6379
  concurrency: 20
  retry_backoff: true

http:
  cors:
    allowed_origins:
      - "https://yourdomain.com"
    allowed_methods: [GET, POST, PUT, DELETE, OPTIONS]
    allowed_headers: [Accept, Authorization, Content-Type, X-Request-ID, X-Tenant-ID]
  rate_limit:
    enabled: true
    requests_per_minute: 100
    burst: 20
```

### Environment variables (override config.yaml)

```bash
export MONGO_URI=mongodb://localhost:27017
export REDIS_ADDR=localhost:6379
export DISCORD_BOT_TOKEN=your_token
export TELEGRAM_BOT_TOKEN=your_token
export GEMINI_API_KEY=your_key
export SMTP_HOST=smtp.example.com
export SMTP_PORT=587
```

### config.yaml (multi-tenant)

```yaml
app:
  env: production
  host: "0.0.0.0"
  port: 8080

tenants:
  default: shared_defaults

  acme:
    enabled: true
    isolation: collection  # "collection" | "field" | "database"
    scraper:
      max_delay: 5s
      proxy_urls:
        - "http://acme-proxy:8080"
    queue:
      concurrency: 5
    notification:
      channels: [discord, email]

  startup_rocket:
    enabled: true
    trending:
      min_hot_score: 90
```

### config.yaml (service discovery)

```yaml
discovery:
  enabled: true
  backend: consul        # "consul" | "dns" | "static"
  consul:
    addr: "consul.internal.erg.ninja:8500"
    datacenter: "dc1"
    token: "${CONSUL_TOKEN}"
  dns:
    domain: "internal.erg.ninja"
  static:                 # development fallback
    services:
      crawler:
        - address: "localhost:8083"
      notification:
        - address: "localhost:8082"
      bot:
        - address: "localhost:8081"
```

### config.yaml (plugin mode)

```yaml
app:
  env: development
  host: "0.0.0.0"
  port: 8080

plugin:
  enabled: true
  dir: "./plugins"         # .so files location
  auto_discover: true

# Alternatively, use build tags to compile-time select modules:
# go build -tags "module_bot,module_notification" ./cmd/server
```

---

## Deployment

### Docker Compose (recommended)

```yaml
services:
  erg-server:
    image: erg-server:latest
    ports:
      - "8080:8080"
    environment:
      - MONGO_URI=mongodb://mongo:27017
      - REDIS_ADDR=redis:6379
    depends_on:
      - mongo
      - redis
    restart: unless-stopped

  mongo:
    image: mongo:7
    volumes:
      - mongo_data:/data/db

  redis:
    image: redis:7-alpine
    volumes:
      - redis_data:/data

volumes:
  mongo_data:
  redis_data:
```

```bash
# Deploy
docker compose up -d --build

# Rollback
docker compose pull
docker compose up -d

# View logs
docker compose logs -f erg-server

# Restart after config change
docker compose restart erg-server
```

### Systemd (bare metal)

```ini
# /etc/systemd/system/erg-server.service
[Unit]
Description=erg-go server (multi-module Go service)
After=network.target mongod.service redis.service

[Service]
Type=simple
User=erg
Group=erg
WorkingDirectory=/opt/erg-server
ExecStart=/opt/erg-server/erg-server
Restart=always
RestartSec=5
Environment=MONGO_URI=mongodb://localhost:27017
Environment=REDIS_ADDR=localhost:6379

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable erg-server
sudo systemctl start erg-server
sudo systemctl status erg-server
sudo journalctl -u erg-server -f
```

---

## Health Checks

```bash
# Basic liveness
curl http://localhost:8080/healthz

# Prometheus metrics
curl http://localhost:8080/metrics

# Module-specific
curl http://localhost:8080/api/bot/healthz
curl http://localhost:8080/api/notifications/healthz
curl http://localhost:8080/api/crawler/healthz
curl http://localhost:8080/api/trending/healthz
```

```json
// Healthy response:
{"status":"ok","components":{"mongo":"ok","redis":"ok","asynq":"ok"}}
```

---

## Multi-Tenant Operations

```bash
# Set tenant via header
curl -H "X-Tenant-ID: acme" http://localhost:8080/api/crawler/stats

# Set tenant via JWT claim (tenant_id in JWT payload)
curl -H "Authorization: Bearer <jwt_with_tenant_id>" http://localhost:8080/api/crawler/stats

# Set tenant via subdomain
curl http://acme.erg.ninja:8080/api/crawler/stats
```

### Tenant-scoped Redis keys

```bash
# List tenant-scoped keys
redis-cli KEYS "tenant:acme:*"

# Inspect crawl queue for a tenant
redis-cli LLEN queue:crawl_acme
redis-cli LRANGE queue:dlq_acme 0 -1
```

---

## Service Discovery Operations

```bash
# If using Consul, check service registration
curl http://localhost:8500/v1/catalog/service/crawler

# DNS-based discovery (Kubernetes headless service)
dig srv _crawler._tcp.internal.erg.ninja
```

---

## Common Operations

### Active SSE connections
```bash
redis-cli KEYS "sse:*"
```

### Flush trending cache
```bash
redis-cli DEL trending:topics trending:news
```

### Trigger feed refresh
```bash
curl -X POST http://localhost:8080/api/rss/feeds/{id}/refresh
```

### Enqueue crawl job manually
```bash
curl -X POST http://localhost:8080/api/crawler/crawl \
  -H "Authorization: Bearer $JWT" \
  -H "X-Tenant-ID: acme" \
  -d '{"url":"https://example.com","priority":3}'
```

---

## gRPC Client Usage (lib/)

```go
package main

import (
    "context"
    "fmt"

    "erg.ninja/lib/crawler/v1"
)

func main() {
    // Create gRPC client
    client, err := crawlerv1.NewClient("localhost:8083",
        crawlerv1.WithConnectTimeout(5 * time.Second),
    )
    if err != nil {
        panic(err)
    }
    defer client.GRPCClient().Close()

    // Call CrawlURL
    resp, err := client.CrawlURL(context.Background(), &crawlerv1.CrawlURLRequest{
        Url:       "https://example.com",
        TenantId:  "acme",
        Priority:  crawlerv1.PRIORITY_NORMAL,
    })
    if err != nil {
        panic(err)
    }
    fmt.Printf("Job ID: %s, Status: %s\n", resp.JobId, resp.Status)
}
```

---

## Build Tags (Plugin Architecture)

```bash
# Full binary (all modules)
go build -o erg-full ./cmd/server

# Selective modules
go build -tags "module_crawler,module_notification" -o erg-crawler-notif ./cmd/server
go build -tags "module_bot" -o erg-bot ./cmd/server

# Library mode (embed in another binary)
go build -tags library ./cmd/server_library
```

Available tags: `module_bot`, `module_crawler`, `module_notification`, `module_trending`, `all_modules`, `library`

---

## Proto Generation

```bash
# Install plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate for all services
for svc in crawler bot notification trending; do
  protoc \
    --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/lib/$svc/v1/$svc.proto
done
```
