# API Reference — erg-go

> **HTTP Base URL**: `http://localhost:8080`
> **gRPC Target**: `localhost:8083` (Crawler), `localhost:8082` (Notification), `localhost:8081` (Bot)
> **Content-Type**: `application/json` (HTTP) | `application/grpc` (gRPC)
> **Auth**: JWT Bearer token (HTTP) | gRPC metadata `authorization` | HMAC (webhooks)

---

## HTTP REST API

### Health & Observability

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/healthz` | — | Basic liveness check |
| GET | `/api/bot/healthz` | — | Bot module health |
| GET | `/api/notifications/healthz` | — | Notifications module health |
| GET | `/api/crawler/healthz` | — | Crawler module health |
| GET | `/api/trending/healthz` | — | Trending module health |
| GET | `/metrics` | — | Prometheus metrics |

### Multi-Tenant Header

All tenant-scoped endpoints support:

```
X-Tenant-ID: acme           ← Header (highest priority)
Authorization: Bearer <JWT>  ← JWT with tenant_id claim (fallback)
Host: acme.erg.ninja        ← Subdomain (lowest priority)
```

### Authentication

**JWT (protected routes)**
```
Authorization: Bearer <jwt_token>
```

**HMAC Webhooks**
```
Discord:  X-Signature-Ed25519: <hex> + X-Signature-Timestamp: <unix>
Telegram: X-Telegram-Bot-Api-Secret-Token: <bot-token>
```

---

## Bot Module

### REST API

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/bot/conversations` | JWT | List active conversations |
| POST | `/api/bot/conversations/{id}/send` | JWT | Send message |
| GET | `/api/bot/link?code=` | — | Verify account link code |
| POST | `/api/bot/link` | JWT | Create account link code |

### Bot Commands

| Command | Description |
|---------|-------------|
| `/rss add <url>` | Subscribe to RSS feed |
| `/rss list` | List subscribed feeds |
| `/rss remove <id>` | Unsubscribe RSS feed |
| `/crawl start <url>` | Start crawl job |
| `/crawl status [job_id]` | Check crawl status |
| `/crawl stop <job_id>` | Stop crawl job |
| `/trending top [n]` | Top trending topics |
| `/trending keyword <topic>` | Topic detail |
| `/stats crawler` | Crawler statistics |

### Webhooks

```
POST /webhooks/discord
POST /webhooks/telegram
```

---

## Notification Module

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/notifications` | JWT | List notifications (paginated) |
| GET | `/api/notifications/:id` | JWT | Get notification by ID |
| POST | `/api/notifications/send` | JWT | Send single notification |
| POST | `/api/notifications/batch` | JWT | Batch send |
| POST | `/api/notifications/:id/cancel` | JWT | Cancel pending |
| GET | `/api/notifications/preferences?user_id=` | JWT | Get preferences |
| PUT | `/api/notifications/preferences` | JWT | Update preferences |
| GET | `/api/notifications/healthz` | — | Module health |
| POST | `/api/channels/discord/test` | JWT | Test Discord |
| POST | `/api/channels/telegram/test` | JWT | Test Telegram |
| GET | `/api/channels/status` | JWT | All channel status |

#### Request: Send Notification
```json
POST /api/notifications/send
{
  "channel": "discord",
  "recipient": "@username",
  "subject": "Crawl Complete",
  "body": "Your crawl finished successfully.",
  "metadata": {
    "url": "https://...",
    "thumbnail": "https://..."
  }
}
```

#### Request: Batch Send
```json
POST /api/notifications/batch
{
  "notifications": [
    { "channel": "discord", "recipient": "@user1", "body": "..." },
    { "channel": "telegram", "recipient": "123456", "body": "..." }
  ]
}
```

---

## Crawler Module

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/crawler/crawl` | JWT | Enqueue crawl URL |
| GET | `/api/crawler/crawl/{job_id}` | — | Get crawl status |
| GET | `/api/crawler/crawl/{job_id}/stream` | — | SSE progress stream |
| GET | `/api/crawler/history` | JWT | List crawl history |
| GET | `/api/crawler/history/{id}` | JWT | Get history entry |
| GET | `/api/crawler/stats` | — | Crawl statistics |
| GET | `/api/rss/feeds` | JWT | List RSS feeds |
| POST | `/api/rss/feeds` | JWT | Create RSS feed |
| GET | `/api/rss/feeds/{id}` | JWT | Get RSS feed |
| PUT | `/api/rss/feeds/{id}` | JWT | Update RSS feed |
| DELETE | `/api/rss/feeds/{id}` | JWT | Delete RSS feed |
| GET | `/api/blacklist` | JWT | List blacklist |
| POST | `/api/blacklist` | JWT | Add blacklist entry |
| DELETE | `/api/blacklist/{id}` | JWT | Remove blacklist entry |

#### Request: Enqueue Crawl
```json
POST /api/crawler/crawl
{
  "url": "https://example.com/article",
  "feed_id": "optional-feed-id",
  "priority": 3
}
```

#### SSE Progress Stream
```
GET /api/crawler/crawl/{job_id}/stream

data: {"type":"ping","job_id":"...","step":0}
data: {"type":"progress","job_id":"...","step":1,"message":"Checking blacklist"}
data: {"type":"progress","job_id":"...","step":5,"message":"Scoring content quality"}
data: {"type":"done","job_id":"...","step":12}
```

---

## Trending Module

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/trending/topics` | — | Top trending topics (cached) |
| GET | `/api/trending/topics/{topic}` | — | Topic detail |
| GET | `/api/trending/news` | — | Supporting news articles |
| GET | `/api/trending/feeds` | JWT | URL discovery feed |
| GET | `/api/trending/history` | JWT | Historical snapshots |
| GET | `/api/trending/sources` | — | Source signals |
| POST | `/api/trending/refresh` | JWT | Trigger manual refresh |
| GET | `/api/trending/healthz` | — | Module health |

---

## Error Response Format

```json
{
  "code": "CRAWLER_RATE_LIMITED",
  "message": "Domain has exceeded crawl rate limit",
  "request_id": "uuid",
  "retry_after": 30,
  "trace_id": "abc123"
}
```

| HTTP | gRPC | Meaning |
|------|------|---------|
| 400 | `INVALID_ARGUMENT` | Bad request — invalid input |
| 401 | `UNAUTHENTICATED` | Missing/invalid auth |
| 403 | `PERMISSION_DENIED` | Insufficient permissions |
| 404 | `NOT_FOUND` | Resource not found |
| 409 | `ALREADY_EXISTS` | Duplicate resource |
| 429 | `RESOURCE_EXHAUSTED` | Rate limited |
| 500 | `INTERNAL` | Internal server error |
| 503 | `UNAVAILABLE` | Service unavailable |

---

## gRPC API Reference

### CrawlerService (`lib/crawler/v1/`)

```go
import "erg.ninja/lib/crawler/v1"

conn, _ := grpc.Dial("localhost:8083", grpc.WithInsecure())
client := crawlerv1.NewCrawlerServiceClient(conn)
```

| RPC | Request | Response |
|-----|---------|----------|
| `CrawlURL` | `CrawlURLRequest{url, tenant_id, priority}` | `CrawlURLResponse{job_id, status}` |
| `GetCrawlStatus` | `GetCrawlStatusRequest{job_id, tenant_id}` | `CrawlStatusResponse` |
| `ListFeeds` | `ListFeedsRequest{tenant_id, category, page_size}` | `ListFeedsResponse{feeds, next_page_token}` |
| `RefreshFeed` | `RefreshFeedRequest{feed_id, tenant_id, force}` | `RefreshFeedResponse{new_items}` |
| `GetStats` | `GetStatsRequest{tenant_id}` | `StatsResponse{total_crawls, success_crawls, ...}` |
| `StopCrawl` | `StopCrawlRequest{job_id, tenant_id}` | `StopCrawlResponse{stopped}` |
| `GetCrawlHistory` | `CrawlHistoryRequest{tenant_id, feed_id, status, page_size}` | `CrawlHistoryResponse{items, next_page_token}` |
| `Reindex` | `ReindexRequest{tenant_id, algorithm, batch_size}` | `ReindexResponse{job_id}` |

### BotService (`lib/bot/v1/`)

| RPC | Description |
|-----|-------------|
| `ListConversations` | Active conversations for a tenant |
| `SendMessage` | Send message to platform conversation |
| `GetWizardState` | Get current wizard state |
| `AdvanceWizard` | Advance wizard step |
| `ListWorkflows` | List workflow executions |
| `StartWorkflow` | Start new workflow |
| `CreateLinkCode` | Create account linking code |
| `ExecuteCommand` | Execute bot command |
| `HealthCheck` | Service health |

### NotificationService (`lib/notification/v1/`)

| RPC | Description |
|-----|-------------|
| `Send` | Send single notification |
| `Get` | Get notification by ID |
| `List` | Paginated notification list |
| `Cancel` | Cancel pending notification |
| `GetPreferences` | User notification preferences |
| `UpdatePreferences` | Update preferences |
| `SendBulk` | Batch send |

### TrendingService (`lib/trending/v1/`)

| RPC | Description |
|-----|-------------|
| `GetTopTopics` | Top trending topics |
| `GetTopic` | Single topic detail |
| `SearchTopics` | Search by keyword |
| `GetTopicNews` | News for a topic |
| `GetSnapshot` | Historical snapshot |
| `Refresh` | Trigger refresh |
| `GetKeywordTrend` | Volume timeline |
| `GetStats` | Aggregate statistics |

---

## Rate Limits

| Endpoint | Limit |
|----------|-------|
| Global HTTP | 100 req/min per IP |
| gRPC | 100 req/min per connection |
| Discord webhook | 200 req/min |
| SSE streams | 5 min idle timeout |
| Crawl jobs | Per-domain configurable |

---

## Tenancy

All endpoints are tenant-scoped. Tenant ID resolution order:

1. `X-Tenant-ID` header (explicit)
2. `tenant_id` claim in JWT token
3. Subdomain prefix (e.g. `acme.erg.ninja` → tenant `acme`)

If no tenant is specified, operations target the `default` tenant.

### Tenant Isolation

| Resource | Isolation Strategy |
|----------|-------------------|
| MongoDB collections | `{tenant_id}_{collection}` (configurable) |
| Redis keys | `tenant:{tenant_id}:{module}:{key}` |
| Asynq queues | `{base_queue}_{tenant_id}` |
| Config | Merged per-tenant overrides |
