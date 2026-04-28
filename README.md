# ERG Platform Backend

Backend Go dùng chung cho hệ sinh thái EduRise Global. Repo này không chỉ phục vụ website ERG, mà còn là nền tảng API chung cho eLearning/LMS, admin console, nội dung, tuyển dụng, thông báo, crawler/trending và các tác vụ vận hành nội bộ.

Điểm chạy chính hiện tại là một HTTP server:

```bash
go run ./cmd/server
```

Server mặc định chạy ở `http://localhost:8080`, dùng Gin + Uber FX, MongoDB, Redis/Asynq, PostgreSQL cho một số module lõi, và có Swagger/Scalar API reference tích hợp.

## Sản Phẩm Đang Dùng Backend Này

| Sản phẩm | Backend cung cấp |
| --- | --- |
| ERG website/admin | Auth, users, pages, posts, menus, documents, SEO, sitemap, reviews, recruitment, public disclosure, notifications, crawler, trending |
| eLearning/LMS | Course catalog, course detail/schema, category, level, unit, user/profile/session, role/permission, notifications |
| Internal operations | Access control, audit logs, analytics, system health, background jobs, storage integrations |
| Future services | Shared packages trong `pkg/`, proto/gRPC clients trong `lib/`, compose/plugin wiring |

## Kiến Trúc Ngắn Gọn

```text
Frontend / Admin / eLearning
        |
        v
cmd/server
        |
        +-- internal/modules/*   Product modules and REST APIs
        +-- pkg/*                Shared infrastructure packages
        +-- lib/*                Public gRPC/proto API surface
        |
        +-- MongoDB              Main document database
        +-- Redis + Asynq        Cache, pub/sub, background jobs
        +-- PostgreSQL           Relational/core modules when configured
        +-- R2 / Google Drive    Optional file storage
```

## Module Chính

### Shared Platform

- `auth`: register, login, Google bridge login, refresh token, profile.
- `users`, `profiles`, `sessions`: tài khoản, hồ sơ, phiên đăng nhập.
- `access_control`: roles, permissions, RBAC cho admin.
- `notifications`: gửi thông báo, preferences, channel status, retry/batch.
- `documents`: quản lý file/tài liệu qua R2 hoặc Google Drive.
- `audit`, `analytics`, `operations`: log, thống kê, health/ops endpoints.

### ERG Website/Core

- `pages`, `posts`, `menus`: CMS cho website.
- `seo`, `sitemap`, `reviews`: SEO metadata, sitemap, đánh giá.
- `recruitment`: job listing, candidate application, admin candidate flow.
- `public_disclosure`: công bố thông tin/tài liệu công khai.
- `crawler`, `trending`, `ai_content`: crawl nội dung, nguồn tin, trending topics, AI hỗ trợ xử lý nội dung.

### eLearning/LMS

- `courses`: danh sách khóa học, course detail, subdomain, schema, theme, lesson reorder.
- `elearning`: category, level, unit public/admin APIs.
- Dùng chung auth, user/profile, RBAC, notifications, documents và tenant context với ERG.

## API Cơ Bản

| Endpoint | Mục đích |
| --- | --- |
| `GET /api/health` | Liveness check |
| `GET /api/ready` | Readiness check, kiểm tra MongoDB/Redis |
| `GET /metrics` | Prometheus metrics |
| `GET /swagger/` | Swagger UI |
| `GET /reference` | Scalar API reference |
| `POST /api/auth/login` | Đăng nhập |
| `GET /api/courses` | Public course list |
| `GET /api/elearning/categories` | Public eLearning categories |
| `GET /api/trending/topics` | Trending topics |

Các API chi tiết nằm trong Swagger generated docs tại `docs/` và có thể xem trực tiếp khi server chạy.

## Yêu Cầu Local

- Go theo `go.mod` (`go 1.25.0`)
- Docker hoặc Docker Desktop để chạy MongoDB/Redis local
- `make` nếu muốn dùng các shortcut trong `Makefile`
- `air` nếu muốn hot reload bằng `make dev`

Kiểm tra nhanh:

```bash
go version
docker --version
docker compose version
```

## Chạy Local

### 1. Bật hạ tầng local

```bash
docker compose up -d mongodb redis
```

MongoDB sẽ chạy ở `localhost:27017`, Redis ở `localhost:6379`.

### 2. Tạo file môi trường

```bash
cp .env.example .env
```

Điền các secret cần thiết nếu module bạn test cần dùng, ví dụ JWT, R2, SMTP, Gemini, bot token. Với luồng cơ bản, MongoDB/Redis local là đủ để server khởi động.

### 3. Chạy server

```bash
go run ./cmd/server
```

Kiểm tra:

```bash
curl http://localhost:8080/api/health
curl http://localhost:8080/api/ready
```

### 4. Build binary

```bash
make build
./bin/erg-server
```

Hoặc build trực tiếp:

```bash
go build -o bin/erg-server ./cmd/server
```

## Lệnh Hay Dùng

```bash
make help          # Xem toàn bộ lệnh hỗ trợ
make build         # Build ./cmd/server ra bin/erg-server
make dev           # Hot reload bằng air
make test          # Chạy test với race detector
make fmt           # Format Go code
make tidy          # go mod tidy
make swag          # Generate Swagger docs
make clean         # Xóa build/test artifacts
```

Chạy test nhanh hơn theo package:

```bash
go test ./pkg/config ./pkg/auth ./pkg/cache ./pkg/queue ./cmd/server
```

## Cấu Hình Và Secret

Repo này hỗ trợ cấu hình qua `config.yaml` và environment variables. Với local/dev, ưu tiên dùng `.env` dựa trên `.env.example`.

Các file sau là local-only và không được commit:

- `.env`
- `config.yaml`
- `.go_cache/`
- `.cache/`
- `.omc/`
- `bin/`
- binary build như `server`, `erg-server`, `plugin-server`, `*.test`

Quy ước secret:

```bash
SECRET_AUTH__JWT_SECRET=...
SECRET_R2__SECRET_KEY=...
SECRET_SMTP__PASSWORD=...
SECRET_AI__GEMINI_API_KEY=...
```

Không commit password database, token bot, API key, private config hoặc file binary build. Nếu GitHub chặn push vì secret scanning, hãy xóa secret khỏi commit rồi amend/rewrite commit trước khi push lại.

## Multi-Tenant

Backend có tenant middleware và các helper trong `pkg/tenant`. Khi bật tenant trong config, request có thể truyền tenant bằng header:

```http
X-Tenant-ID: default
```

Nếu không bật tenant, server chạy single-tenant mode và dùng tenant mặc định.

## Cấu Trúc Thư Mục

```text
cmd/server/              Entry point chính của backend
cmd/plugin-server/       Runtime plugin server
internal/routes/         Nơi wire toàn bộ module vào Gin router
internal/modules/        Product modules: ERG, eLearning, shared platform
internal/middleware/     Auth, RBAC, CORS, logging, recovery, rate limit
internal/persistence/    Migration/backfill cho Postgres core
pkg/                     Shared infrastructure packages
lib/                     Public gRPC/proto generated clients
proto/                   Proto source definitions
docs/                    Swagger/OpenAPI generated docs
migrations/              Migration helpers
scripts/                 Utility scripts
```

## Shared Packages Trong `pkg/`

| Package | Vai trò |
| --- | --- |
| `pkg/config` | Load YAML/env/secret config |
| `pkg/database` | MongoDB, PostgreSQL/GORM, MySQL helpers |
| `pkg/cache` | Redis client |
| `pkg/queue` | Asynq client/server |
| `pkg/event` | In-process + Redis pub/sub event bus |
| `pkg/auth` | JWT validation |
| `pkg/tenant` | Tenant context and isolation helpers |
| `pkg/storage` | R2 and Google Drive clients |
| `pkg/logger` | zerolog structured logging |
| `pkg/http` | HTTP client/server helpers and interceptors |
| `pkg/plugin`, `pkg/compose`, `pkg/discovery` | Module composition, plugin and discovery support |
| `pkg/rss`, `pkg/sitemap`, `pkg/scraper`, `pkg/dedup`, `pkg/ai` | Content/crawler/trending support |

## Swagger/OpenAPI

Generate lại docs sau khi thay đổi route annotation:

```bash
make swag
```

Khi server đang chạy:

```text
http://localhost:8080/swagger/
http://localhost:8080/reference
```

## Docker Compose

Hiện tại nên dùng Docker Compose chủ yếu để bật hạ tầng local:

```bash
docker compose up -d mongodb redis
docker compose ps
docker compose logs -f mongodb redis
docker compose down
```

Một số service cũ trong `docker-compose.yml` là di sản từ giai đoạn microservice. Runtime chính của repo hiện tại vẫn là `cmd/server`.

## Quy Trình Dev Khuyến Nghị

1. Tạo branch riêng cho feature/fix.
2. Chạy `docker compose up -d mongodb redis`.
3. Chạy `go run ./cmd/server` hoặc `make dev`.
4. Test API qua Swagger/Scalar hoặc frontend tương ứng.
5. Chạy `make fmt` và test package liên quan.
6. Kiểm tra `git status` trước khi commit để không dính secret/cache/binary.

## Deploy

Build artifact chuẩn:

```bash
make build
```

Khi deploy, cấu hình môi trường nên được inject qua env/secret manager của hạ tầng, không commit `config.yaml` production vào Git. Các service phụ trợ tối thiểu cần có:

- MongoDB
- Redis
- PostgreSQL nếu dùng các module yêu cầu GORM/Postgres
- R2 hoặc Google Drive nếu bật upload/document storage
- SMTP nếu bật email transactional

## Ghi Chú Cho Frontend

- ERG và eLearning dùng chung auth/user/session, nên token và user profile phải thống nhất.
- Các endpoint admin thường cần `Authorization: Bearer <token>` và role `admin`.
- Các endpoint public như course/category/page/post/recruitment public có thể gọi không cần token tùy route.
- Khi làm multi-tenant, frontend nên gửi `X-Tenant-ID` nếu môi trường có nhiều tenant.
