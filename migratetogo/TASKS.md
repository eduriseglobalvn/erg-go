# Migration Tasks — Detailed Implementation Guide
**Date**: 2026-04-10
**Author**: Senior Engineer + Senior Architect (30y combined exp)

> Each task MUST follow clean architecture. Verify `go build ./...` and `go vet ./...` pass before marking complete.

---

## F0-01 — GORM Database Setup + MySQL Client

**Path**: `pkg/database/mysql.go`, `pkg/database/postgres.go`

**What to do**:
1. Add `GORMMySQLClient` to `pkg/database/mysql.go`:
   - Wrap `*sql.DB` with GORM `gorm.Config`
   - Support config: `host`, `port`, `user`, `password`, `database`, `max_open_conns`, `max_idle_conns`, `conn_max_lifetime`
   - Provide `func (*GORMMySQLClient) DB() *gorm.DB` and `func (*GORMMySQLClient) Close() error`
2. Add `GORMPostgresClient` to `pkg/database/postgres.go`:
   - Use `gorm.io/driver/postgres` OpenBSD-compatible driver
   - Same interface pattern as GORMMySQLClient
3. Update `pkg/config/config.go` — add `MySQLConfig{}` and `PostgresConfig{}` sections
4. Update `config.yaml` — add MySQL and PostgreSQL connection strings
5. Wire into `cmd/server/server.go` via UberFX providers

**Evidence**: `go build ./...` + `go vet ./...` pass

---

## F0-02 — UberFX App Bootstrap with chi

**Path**: `cmd/server/server.go`

**What to do**:
1. Refactor `cmd/server/server.go` to use `uber-go/fx`:
   - `fx.New()` → `fx.Provide(...)` for all deps (Mongo, Redis, MySQL, Bus, Logger, Config, Queue, JWT, Tenant, Discovery)
   - `fx.Invoke(registerHTTPServer)` to start the HTTP server
   - Keep existing graceful shutdown (SIGINT/SIGTERM with 30s timeout)
2. All existing functionality must remain identical — only restructure the DI

**Evidence**: `go build ./cmd/server` + existing routes still work

---

## F0-03 — chi Router + Standard Middleware

**Path**: `cmd/server/server.go`, `internal/middleware/`

**What to do**:
1. Ensure chi router has all standard middleware in order:
   ```go
   r.Use(recoverMiddleware)           // panic recovery
   r.Use(middleware.RequestID)        // X-Request-ID
   r.Use(middleware.Logger)           // zerolog request log
   r.Use(middleware.CORS)            // CORS
   r.Use(middleware.RateLimit)        // global rate limit
   r.Use(middleware.Compress)         // gzip
   r.Use(tenant.TenantMiddleware(...)) // multi-tenancy
   r.Use(interceptors.ErrorInterceptor(...)) // error handling
   ```
2. Add `middleware/` package if any are missing from `internal/middleware/`
3. Document middleware order in code comments

**Evidence**: Server starts, all 4 modules routes accessible, no duplicate middleware

---

## F0-04 — Standard Response + Validation

**Paths**: `internal/dto/response/`, `internal/validator/`

**What to do**:
1. Create `internal/dto/response/response.go`:
   ```go
   type APIResponse struct {
       Success bool        `json:"success"`
       Data    interface{} `json:"data,omitempty"`
       Error   *APIError   `json:"error,omitempty"`
       Meta    *Meta       `json:"meta,omitempty"` // pagination
       Message string      `json:"message,omitempty"`
   }
   type APIError struct { Code string `json:"code"`; Message string `json:"message"` }
   type Meta struct { Page int `json:"page"`; Limit int `json:"limit"`; Total int64 `json:"total"` }
   func OK(w http.ResponseWriter, data interface{})
   func OKMeta(w http.ResponseWriter, data interface{}, meta *Meta)
   func Created(w http.ResponseWriter, data interface{})
   func Error(w http.ResponseWriter, status int, code, message string)
   ```
2. Create `internal/validator/validator.go`:
   - Use `go-playground/validator/v10`
   - Reusable validation helper for request DTOs
   - Centralized error message formatting
3. Every controller MUST use `response.OK()`, `response.Created()`, `response.Error()`

**Evidence**: All 4 existing modules use the new response helpers, no direct `json.NewEncoder` in controllers

---

## F0-05 — R2 Cloudflare Storage (full)

**Path**: `pkg/storage/r2.go` (already exists — extend it)

**What to do**:
1. Check existing `pkg/storage/r2.go` — extend it to match NestJS `StorageService`:
   ```go
   // Image: resize 1920px → WebP @85% → upload to R2
   ProcessAndUpload(ctx context.Context, buf []byte, folder, filename string) (string, error)
   // Raw file (PDF, Word) — no processing
   UploadRawFile(ctx context.Context, buf []byte, folder, filename, mimetype string) (string, error)
   // Delete (idempotent)
   DeleteFile(ctx context.Context, fileURL string) error
   // Read back
   GetFileBuffer(ctx context.Context, fileURL string) ([]byte, error)
   ```
2. Add image processing using `github.com/disintegration/imaging` (resize → WebP)
3. Add MIME type validation (allow: image/jpeg, image/png, image/gif, image/webp, application/pdf, application/msword, application/vnd.openxmlformats-officedocument.wordprocessingml.document)
4. Max file size config: 5MB for images, 10MB for documents
5. Update `pkg/config/config.go` with `R2Config{}`

**Evidence**: Unit tests for ProcessAndUpload, UploadRawFile, DeleteFile pass

---

## F0-06 — Auth Service (JWT issuance + refresh + PIN)

**Path**: `internal/modules/auth/`

**What to do**:
1. Create `internal/modules/auth/auth.module.go` + all sub-packages
2. Entity `User` (MySQL):
   ```go
   type User struct {
       ID           primitive.ObjectID `bson:"_id,omitempty" gorm:"primaryKey"`
       Email        string             `bson:"email" gorm:"uniqueIndex;not null"`
       PasswordHash string             `bson:"password_hash" gorm:"not null"`
       FullName     string             `bson:"full_name"`
       AvatarURL    string             `bson:"avatar_url"`
       Status       UserStatus         `bson:"status"` // ACTIVE, PENDING, BANNED, BLOCKED
       Provider     string             `bson:"provider"` // local, google, facebook, apple
       ProviderID   string             `bson:"provider_id"`
       Roles        []string           `bson:"roles"`
       CreatedAt    time.Time          `bson:"created_at"`
       UpdatedAt    time.Time          `bson:"updated_at"`
   }
   ```
3. Entity `UserSession` (MySQL):
   ```go
   type UserSession struct {
       ID           primitive.ObjectID `bson:"_id,omitempty" gorm:"primaryKey"`
       UserID       primitive.ObjectID `bson:"user_id;index"`
       SessionID    string             `bson:"session_id;uniqueIndex"`
       IPAddress    string             `bson:"ip_address"`
       UserAgent    string             `bson:"user_agent"`
       RefreshToken string             `bson:"refresh_token_hash"`
       ExpiresAt    time.Time          `bson:"expires_at"`
       RevokedAt    *time.Time         `bson:"revoked_at,omitempty"`
       CreatedAt    time.Time          `bson:"created_at"`
   }
   ```
4. Entity `PinCode` (MongoDB, TTL index):
   ```go
   type PinCode struct {
       ID        primitive.ObjectID `bson:"_id,omitempty"`
       Email     string             `bson:"email;index"`
       Code      string             `bson:"code"` // 6-digit
       Purpose   string             `bson:"purpose"` // register, forgot_password, verify
       ExpiresAt time.Time          `bson:"expires_at"`
       UsedAt    *time.Time         `bson:"used_at,omitempty"`
       CreatedAt time.Time          `bson:"created_at"`
   }
   ```
5. Services:
   - `AuthService.Register(ctx, email, password, fullName) (user, error)` — hash with argon2, send PIN email
   - `AuthService.Login(ctx, email, password, ip) (accessToken, refreshToken, session, error)` — argon2 verify, check abuse, issue tokens
   - `AuthService.RefreshToken(ctx, refreshToken) (accessToken, newRefreshToken, error)` — detect token reuse → revoke ALL sessions
   - `AuthService.VerifyPIN(ctx, email, code, purpose) (user, error)` — activate or reset
   - `AuthService.ForgotPassword(ctx, email)` — send reset PIN
   - `AuthService.ResetPassword(ctx, email, code, newPassword)` — reset + revoke all sessions
   - `AuthService.Logout(ctx, sessionID)` — revoke session + clear Redis cache
   - `AuthService.GetProfile(ctx, userID)` — get user from JWT
6. HTTP Endpoints:
   ```
   POST /auth/register
   POST /auth/login
   POST /auth/logout
   POST /auth/refresh
   POST /auth/verify-pin
   POST /auth/forgot-password
   POST /auth/reset-password
   GET  /auth/profile
   ```
7. Use existing `pkg/auth/jwt.go` for JWT issuance (extend `AuthServiceProvider` or create `TokenService`)
8. Integrate `AbuseDetectionService` for login attempt tracking (5 failures → block IP)

**Evidence**: `go test ./internal/modules/auth/...` pass, manual curl test: register → verify-pin → login → refresh → logout

---

## F0-07 — Config.yaml + Environment Variables

**Path**: `pkg/config/config.go`, `config.yaml`

**What to do**:
1. Review ALL existing config fields in `pkg/config/config.go`
2. Add missing sections:
   - `AuthConfig{}`: `jwt_secret`, `jwt_public_key`, `jwt_issuer`, `jwt_algorithms`, `access_token_ttl`, `refresh_token_ttl`, `argon2_memory`, `argon2_iterations`
   - `SMTPConfig{}`: `host`, `port`, `user`, `password`, `from_address`, `from_name`
   - `R2Config{}`: `account_id`, `access_key_id`, `secret_access_key`, `bucket`, `public_url`, `public_url_base`
   - `MySQLConfig{}`: `host`, `port`, `user`, `password`, `database`, `max_open_conns`, `max_idle_conns`, `ssl_mode`
   - `PostgresConfig{}`: same fields
   - `AbuseConfig{}`: `max_failed_login`, `block_duration`, `max_404_per_minute`, `high_frequency_threshold`
3. Update `config.yaml` with all new sections + realistic defaults
4. Validate required fields at startup — fail fast if missing

**Evidence**: `go test ./pkg/config/...` passes, missing required fields cause clear error message

---

## F0-08 — Module Skeleton Generator

**Path**: `scripts/generate_module.go` (new file)

**What to do**:
1. Create `scripts/generate_module.go` as a Go program (not a script):
   ```bash
   go run scripts/generate_module.go --name users
   ```
2. It creates the full clean architecture structure:
   ```
   internal/modules/{name}/
   ├── {name}.module.go        (Module with NewModule, Setup, RegisterRoutes, Stop)
   ├── {name}.service.go       (interface + implementation stub)
   ├── {name}.controller.go    (chi routes + response helpers)
   ├── dto/request/{name}.go  (Create/Update/List DTOs with validation tags)
   ├── dto/response/{name}.go (Response DTOs)
   ├── entities/{entity}.go   (MongoDB/MySQL models)
   └── repository/{name}.go   (CRUD operations)
   ```
3. Pre-fill with proper imports, package structure, TODOs for each method
4. Each module registers via `routes.Register()` or build tags

**Evidence**: `go run scripts/generate_module.go --name pages` creates valid, compilable structure

---

## F0-09 — Database Audit Middleware

**Path**: `internal/middleware/audit.go`

**What to do**:
1. Create `@Auditable` middleware pattern:
   ```go
   type Auditable struct {
       Action       string // "create", "update", "delete", "restore"
       ResourceType string // "post", "user", "page", "job"
   }
   // Decorator-style: apply to handler functions via context
   func WithAudit(ctx context.Context, auditable *Auditable, userID string, resourceID string)
   ```
2. Create `internal/modules/audit/audit.module.go`:
   - Entity `AuditLog` (MongoDB):
     ```go
     type AuditLog struct {
         ID           primitive.ObjectID `bson:"_id,omitempty"`
         TenantID     string             `bson:"tenant_id"`
         UserID       string             `bson:"user_id"`
         UserEmail    string             `bson:"user_email"`
         Action       string             `bson:"action"`
         ResourceType string             `bson:"resource_type"`
         ResourceID   string             `bson:"resource_id"`
         Changes      map[string]Change  `bson:"changes"` // before/after diff
         IPAddress    string             `bson:"ip_address"`
         UserAgent    string             `bson:"user_agent"`
         Timestamp    time.Time          `bson:"timestamp"`
     }
     ```
   - Repository with paginated query
   - HTTP endpoint: `GET /audit/logs` (admin only, JWT required)
3. Create audit helper for controllers: `AuditLog(ctx, userID, action, resourceType, resourceID, changes)`
4. Wire into all controllers via `AuditMiddleware` or inline calls

**Evidence**: `AuditMiddleware` used in at least 3 existing controllers (posts, users, crawler)

---

## F0-10 — API Contract Snapshot (OpenAPI)

**Path**: `docs/openapi.yaml`

**What to do**:
1. Generate OpenAPI 3.0 spec documenting ALL existing erg-go endpoints:
   - Use `github.com/swaggo/swag` annotations OR manual YAML
   - Add to each controller method: `// @Summary`, `// @Description`, `// @Tags`, `// @Accept json`, `// @Produce json`, `// @Param`, `// @Success`, `// @Failure`, `// @Security BearerAuth`
2. Output: `docs/openapi.yaml` (manually written based on existing controllers)
3. Cover: bot, crawler, notifications, trending — all 50+ endpoints
4. Mark deprecated endpoints with `deprecated: true`

**Evidence**: `docs/openapi.yaml` exists and covers all existing routes

---

## F0-11 — Hot Reload (Air) + Dev Script

**Path**: `.air.toml`, `Makefile`

**What to do**:
1. Create `.air.toml` for development hot reload:
   ```toml
   [build]
   cmd = "go build -o tmp/erg-server ./cmd/server"
   bin = "tmp/erg-server"
   watch_dir = ["."]
   exclude_dir = ["tmp", "vendor", "bin", ".git"]
   exclude_regex = ["_test\\.go", "_gen\\.go"]

   [run]
   env = { AIR_ENV = "development" }
   ```
2. Update `Makefile`:
   ```make
   dev:
       air
   .PHONY: dev
   ```
3. Create `scripts/dev.sh`:
   ```bash
   #!/bin/bash
   # Start dependencies: MongoDB, Redis
   docker compose -f docker-compose.dev.yml up -d
   # Run with hot reload
   air
   ```

**Evidence**: `make dev` starts server with hot reload, file changes trigger rebuild within 2s

---

## P1-01 — pages Module

**Path**: `internal/modules/pages/`

**Reference**: `erg-backend/src/modules/pages/`

**Entities**: `Page` (MySQL) — slug, domain, title, content, meta_title, meta_description, faq_json, status, published_at

**Endpoints**:
```
GET  /pages/:slug         — cached 5 min, includes FAQ, meta_title, meta_description
```

**Features**:
- Slug-based lookup with domain filter
- In-memory or Redis cache (5 min TTL)
- FAQ JSON rendering

---

## P1-02 — menus Module

**Path**: `internal/modules/menus/`

**Reference**: `erg-backend/src/modules/menus/`

**No DB** — hardcoded per-domain menus in Go code or YAML config.

**Endpoints**:
```
GET /menus/structure?domain=ai.erg.edu.vn
```

**Domains**: `erg.edu.vn`, `ai.erg.edu.vn`, `tinhocquocte.erg.edu.vn`

---

## P1-03 — documents Module

**Path**: `internal/modules/documents/`

**Reference**: `erg-backend/src/modules/documents/`

**Entities**: `Document` (MySQL) — filename, original_name, mime_type, size, r2_url, watermark_config, status, uploaded_by, tenant_id

**Endpoints**:
```
POST /documents              — Upload PDF + apply watermark + upload to R2
GET  /documents             — List all
GET  /documents/:id         — Get metadata
GET  /documents/:id/file    — Stream PDF (security headers)
PATCH /documents/:id        — Update metadata/watermark config
DELETE /documents/:id       — Delete + remove from R2
```

**Features**:
- PDF watermark: text, position, opacity, color, fontSize, per-page
- R2 upload with structured path: `documents/{tenant_id}/{uuid}/{filename}`
- Security headers on streaming: `X-Frame-Options=DENY`, `Content-Security-Policy`, `X-Content-Type-Options`, `Cache-Control=no-store`

---

## P1-04 — courses Module

**Path**: `internal/modules/courses/`

**Reference**: `erg-backend/src/modules/courses/`

**Entities** (MySQL):
- `Course` — title, slug, subdomain, description, thumbnail_url, instructor_id, status, enrollment_count, rating_avg, schema_type, schema_data_json
- `CourseSyllabus` — course_id, order, title, content
- `CourseInstructor` — name, bio, avatar_url, title
- `CourseLesson` — course_id, order, title, content, duration_minutes, video_url
- `CourseEnrollment` — course_id, user_id, enrolled_at, completed_at, progress_percent

**Endpoints**:
```
GET  /courses                    — List all
GET  /courses/subdomain/:sub    — Find by subdomain
GET  /courses/:id               — Get course + schema markup
POST /courses                   — Create (admin)
PATCH /courses/:id              — Update
PATCH /courses/:id/theme         — Update theme config
POST /courses/:id/lessons/reorder — Reorder lessons
DELETE /courses/:id             — Delete
```

**Features**:
- Schema.org `Course` markup via `SchemaMarkupService`
- Subdomain routing

---

## P1-05 — reviews Module

**Path**: `internal/modules/reviews/`

**Reference**: `erg-backend/src/modules/reviews/`

**Entities**: `Review` (MySQL) — target_id, target_type (POST|COURSE|COURSE_LESSON), rating (1-5), comment, user_name, status (PENDING|APPROVED|REJECTED), is_featured, reply_content, reply_by, reply_at, admin_note, reviewed_by, reviewed_at, created_at

**Endpoints**:
```
POST /reviews                     — Submit review (rate-limit: 5/hour per IP, duplicate check)
GET  /reviews?targetId=...      — List approved reviews (paginated)
GET  /reviews/stats?targetId=..  — Average rating + count
GET  /reviews/admin              — All reviews (filter by status/type) [admin]
PATCH /reviews/:id/status       — Approve/Reject + admin note [admin]
POST /reviews/batch/status       — Bulk status update [admin]
POST /reviews/:id/reply         — Admin reply [admin]
PATCH /reviews/:id/feature      — Toggle featured [admin]
```

---

## P1-06 — sessions Module

**Path**: `internal/modules/sessions/`

**Reference**: `erg-backend/src/modules/sessions/`

**Entities**: `UserSession` (MySQL) — already defined in F0-06

**Endpoints**:
```
GET /sessions/current             — Full session context (user + roles + permissions + session info)
```

**Response**:
```json
{
  "user": { "id", "email", "fullName", "avatarUrl", "status" },
  "accessControl": { "roles": [], "permissions": [] },
  "session": { "id", "ipAddress", "lastActiveAt", "expiresAt" },
  "system": { "serverTime", "version" }
}
```

**Features**:
- Redis cache: `session_ctx:{userId}:{sessionId}` with 15-min TTL
- Cache invalidation on: profile update, password change, remote logout
- User status enforcement (PENDING/BANNED/BLOCKED → reject)

---

## P1-07 — elearning Module

**Path**: `internal/modules/elearning/`

**Reference**: `erg-backend/src/modules/elearning/`

**Entities** (MongoDB):
- `ElearningCategory` — name, slug, description, parent_id, order, is_active, levels[]
- `ElearningLevel` — category_id, name, slug, description, order, units[]
- `ElearningUnit` — level_id, name, slug, description, content, order

**Endpoints**:
```
GET /elearning/categories              — List active (nested)
GET /elearning/categories/:slug         — Get category tree
GET /elearning/levels/:slug             — Get level with units
GET/POST/PATCH/DELETE /admin/elearning/categories [admin]
POST/PATCH/DELETE /admin/elearning/levels [admin]
POST/PATCH/DELETE /admin/elearning/units [admin]
```

---

## P2-01 — posts Module

**Path**: `internal/modules/posts/`

**Reference**: `erg-backend/src/modules/posts/`

**Entities** (MySQL):
- `Post` — title, slug, content, excerpt, thumbnail_url, view_count, seo_score, schema_type, schema_data_json, robots_index, robots_follow, status (DRAFT|PUBLISHED|SCHEDULED|HIDDEN), deleted_at (soft delete), author_id, category_id, tags, published_at, scheduled_at, created_at, updated_at
- `PostCategory` — name, slug, description, parent_id, order, seo_title, seo_description, schema_type

**Public Endpoints**:
```
GET /posts              — List (paginated, filter by category/status/tag)
GET /posts/hidden       — Hidden posts [admin]
GET /posts/trash        — Soft-deleted [admin]
GET /posts/slug/:slug   — By slug (cached 5 min)
GET /posts/:id          — By ID
POST /posts/preview/:id — Save/get preview draft
GET /posts/preview/:id  — Get preview draft
```

**Admin Endpoints**:
```
POST   /posts                    — Create
PUT    /posts/:id                — Update
DELETE /posts/:id               — Soft delete → trash
PUT    /posts/:id/restore        — Restore
DELETE /posts/:id/permanent      — Hard delete
POST   /posts/:id/promote        — Promote HIDDEN → PUBLIC
POST   /posts/images/upload      — Upload image to R2 (5MB, jpg/png/gif/webp)
DELETE /posts/images             — Delete image from R2
```

**Features**:
- SSE real-time updates via `SseService`
- Image upload to R2 with resize → WebP processing
- Soft delete (trash) with 30-day retention
- Preview drafts
- Audit logging via `AuditMiddleware`

---

## P2-02 — users Module

**Path**: `internal/modules/users/`

**Reference**: `erg-backend/src/modules/users/`

**Entities**: `User` — already defined in F0-06

**My Profile Endpoints**:
```
GET  /users/me                    — Get profile
PATCH /users/me                  — Update profile
POST /users/onboarding           — Update profile + upload avatar
PUT  /users/me/password          — Change password (invalidates all sessions)
GET  /users/me/sessions          — List active sessions
DELETE /users/me/sessions/:id    — Revoke specific session (remote logout)
```

**Admin Endpoints**:
```
GET  /users                      — Paginated user list (search, filter by status/role/provider)
GET  /users/:id                  — Full user detail (sessions, social accounts)
GET  /users/:id/activity         — Activity log (paginated)
PUT  /users/:id/status           — Block/Ban/Active
POST /users/:id/roles            — Assign roles
POST /users/:id/permissions      — Direct permission overrides (GRANT/DENY)
DELETE /users/:id                — Hard delete
```

---

## P2-03 — auth Full (Social OAuth)

**Path**: `internal/modules/auth/` (extend F0-06)

**Reference**: `erg-backend/src/modules/auth/` social login

**What to add**:
1. Google OAuth2: `GET /auth/oauth/google`, `GET /auth/oauth/google/callback`
2. Facebook OAuth2: `GET /auth/oauth/facebook`, `GET /auth/oauth/facebook/callback`
3. Apple Sign In: `POST /auth/oauth/apple` (ID token verification)

**Entities**: `SocialAccount` (MongoDB) — user_id, provider (google|facebook|apple), provider_id, access_token, refresh_token, expires_at

**Flow**:
1. Redirect to provider → get authorization code
2. Exchange code for tokens → get user profile
3. Find or create User by email or social provider_id
4. Link social account to user
5. Issue JWT access + refresh tokens
6. Redirect to frontend with tokens

---

## P2-04 — recruitment Module

**Path**: `internal/modules/recruitment/`

**Reference**: `erg-backend/src/modules/recruitment/`

**Entities** (MySQL):
- `Job` — title, slug, summary, description, requirements, benefits, salary, salary_currency, work_type, location, country, is_active, is_hot, is_new, is_urgent, view_count, deadline, status (NORMAL|HOT|URGENT), created_at, updated_at
- `Candidate` — full_name, email, phone, cv_url (R2), cover_letter, apply_type (ONLINE|ZALO), status (PENDING|REVIEWED|INTERVIEW|OFFER|REJECTED), job_id, applied_at, updated_at, admin_note

**Public Endpoints**:
```
GET  /recruitment/jobs               — List active jobs (search, filter, sort)
GET  /recruitment/jobs/:slug        — Job detail + schema markup
POST /recruitment/apply             — Apply with CV upload (PDF/Word, 2MB max)
GET  /recruitment/tracking/:code    — Track application status
```

**Admin Endpoints**:
```
POST /recruitment/apply/zalo         — Create candidate from Zalo (no CV)
POST /recruitment/jobs              — Create job
PUT  /recruitment/jobs/:id          — Update
DELETE /recruitment/jobs/:id        — Soft deactivate
PATCH /recruitment/jobs/:id/toggle-hot
PATCH /recruitment/jobs/:id/toggle-urgent
PATCH /recruitment/jobs/:id/status
GET  /recruitment/admin/candidates  — List all candidates (filter by job)
PATCH /recruitment/admin/candidates/:id/status — Update candidate status + public note
```

**Features**:
- Auto-set `isNew` (≤7 days), `isUrgent` (≤5 days to deadline), `isHot` (>20 views + active deadline)
- Email confirmation with tracking link
- CV upload to R2 `recruitment/cv/{slugified_name}/{uuid}.pdf`

---

## P2-05 — analytics Module

**Path**: `internal/modules/analytics/`

**Reference**: `erg-backend/src/modules/analytics/`

**Entities** (MongoDB):
- `Visit` — tenant_id, session_id, user_id, ip_address, country, city, device_type (desktop|mobile|tablet), os, browser, referrer, entry_url, exit_url, duration_seconds, created_at
- `AnalyticsEvent` — tenant_id, visit_id, user_id, event_type, event_data_json, created_at

**Endpoints**:
```
POST /insight/session/begin        — Track page visit (IP geo, device type, OS, browser)
POST /insight/behavior            — Track user behavior events
PUT  /insight/session/:id/finish  — Update visit duration
GET  /insight/stats?range=7d|30d|90d — Visitor stats (desktop/mobile)
GET  /insight/overview            — Full dashboard
GET  /insight/posts/summary       — Post analytics by month/category
GET  /insight/export              — CSV export
```

**Features**:
- geoip-lite for IP → country/city
- ua-parser-js equivalent: `github.com/mssola/useragent` for OS/browser
- Deduplicate unique visitors by IP per day
- Compute bounce rate, avg session duration
- Traffic source classification (Direct / Google / Facebook / Other)
- Top pages, top posts by view count
- Monthly publication stats

---

## P2-06 — trending Module v2

**Path**: `internal/modules/trending/` (REBUILD from scratch)

**Reference**: `erg-backend/src/modules/trending-proxy/`

**What to fix/extend in existing trending module**:
1. Add MongoDB indexes on `trending_topics` (score, slug, updated_at)
2. Add `GetSources()` — health check of all signal sources
3. Add historical retention: delete snapshots older than 90 days
4. Better algorithm: combine crawler + RSS + search + social signals
5. Add topic categories (education, tech, news, career)
6. Add `POST /api/trending/feeds/:id/disable` — admin control

---

## P3-01 — access-control Module (RBAC)

**Path**: `internal/modules/access-control/`

**Reference**: `erg-backend/src/modules/access-control/`

**Entities** (MySQL):
- `Role` — name, slug, description, permissions (many-to-many), is_system, created_at
- `Permission` — name, slug, resource, action, description, group_slug, created_at
- `PermissionGroup` — name, slug, description, order
- `UserPermission` — user_id, permission_id, action (GRANT|DENY), assigned_by, created_at

**Permission Groups** (seeded on boot):
- `content_management` → `posts.*`, `courses.*`, `crawler.*`, `reviews.*`
- `seo_management` → `seo.*`
- `user_management` → `users.*`, `roles.*`
- `system_settings` → `system.*`, `api-keys.*`, `audit.*`, `menus.*`, `pages.*`
- `recruitment` → `recruitment.*`
- `analytics` → `analytics.*`

**Roles** (seeded): `user`, `editor`, `content_manager`, `seo_specialist`, `hr_manager`, `viewer`, `admin`

**Services**:
- `GetUserPermissionsAndFeatures(ctx, userID)` → effective permission set + UI feature map
- `AssignDirectPermissions(ctx, userID, permissions, action)` — with delegation validation
- `GetRolePermissions(ctx, roleSlug)` → list permissions for a role
- `SeedDefaultRoles()` — on module Setup()

**HTTP Endpoints**:
```
GET  /access/roles                     — List all roles [admin]
POST /access/roles                     — Create role [admin]
GET  /access/permissions                — List all permissions [admin]
POST /access/permissions                — Create permission [admin]
GET  /access/users/:id/permissions     — Get user's effective permissions [admin]
POST /access/users/:id/permissions     — Assign direct permissions [admin]
GET  /access/features                  — Get feature flags for current user [auth]
```

**Middleware**: `PermissionsGuard(permissions []string)` — reads JWT claims → checks permission set

---

## P3-02 — seo Module

**Path**: `internal/modules/seo/`

**Reference**: `erg-backend/src/modules/seo/`

**Entities** (MySQL + MongoDB):
- `SeoHistory` (MySQL) — post_id, score, readability, keyword_density, meta_score, structure_score, created_at
- `SchemaTemplate` (MySQL) — type, template_json, name, description
- `SeoKeyword` (MySQL) — keyword, post_id, position, search_volume, created_at
- `SeoRedirect` (MySQL) — from_url, to_url, status_code (301|302), created_at
- `Seo404Log` (MySQL) — path, referrer, ip_address, user_agent, created_at
- `GoogleSearchConsole` (MySQL) — site_url, access_token, refresh_token, expires_at
- `SeoConfig` (MySQL) — key, value
- `SeoScoreHistory` (MongoDB) — post_id, score, created_at
- `SearchEngineSubmissionLog` (MongoDB) — post_id, engine, status, submitted_at

**Controllers**:
```
# Analysis
GET  /seo/analyze/:postId           — Comprehensive SEO analysis
GET  /seo/schema/:postId            — Schema.org JSON-LD markup (cached 1h)
POST /seo/schema/:postId/validate   — Validate via Google Rich Results Test
POST /seo/schema/:postId            — Save custom schema data

# History & Trends
GET  /seo/history/:postId           — SEO score history (30 days)
GET  /seo/trends/:postId            — Trend data

# Google Search Console
GET  /seo/gsc/:postId               — GSC data
GET  /seo/gsc/auth/url              — OAuth2 authorization URL
POST /seo/gsc/auth/callback         — Handle OAuth2 callback
POST /seo/gsc/sync                  — Sync GSC data
GET  /seo/gsc/top-posts             — Top performing posts
GET  /seo/performance               — Performance report
GET  /seo/performance/queries       — Top search queries

# AI Suggestions
POST /seo/suggest-titles            — AI-generated titles
POST /seo/suggest-meta             — AI-generated meta descriptions
POST /seo/generate-alt-texts        — AI-generated image alt texts

# Keywords
GET/POST/DELETE /seo/keywords/:id  — Manage tracked keywords
PUT  /seo/apply-autolinks/:postId  — Apply internal auto-links

# Redirects & Monitoring
GET/POST/PUT/DELETE /seo/redirects — Manage 301/302 redirects
GET/POST /seo/404-logs             — 404 error log

# Config
GET/PUT /seo/config/:key            — SEO configuration KV store
```

**Services**:
- `SeoAnalyzerService` — comprehensive post analysis (score, readability, structure, keywords)
- `SchemaMarkupService` — Article, Course, JobPosting, FAQPage, BreadcrumbList schemas
- `AutoLinkingService` — internal link automation
- `KeywordResearchService` — seed keyword → related keywords

---

## P3-03 — ai-content Module

**Path**: `internal/modules/ai-content/`

**Reference**: `erg-backend/src/modules/ai-content/`

**Entities** (MySQL):
- `ApiKey` — provider, key_hash (AES-256 encrypted), user_id, label, is_active, last_used_at, created_at
- `User` (lean) — user_id, email, plan, is_active

**Entities** (MongoDB):
- `AiJob` — user_id, job_id, provider, model, type (generate|refine|image|batch), status, input_json, output_json, error, created_at, completed_at

**Providers** (Factory pattern):
1. OpenAI — `https://api.openai.com/v1/chat/completions`
2. Anthropic — `https://api.anthropic.com/v1/messages`
3. Google AI — `https://generativelanguage.googleapis.com/v1beta/models`
4. Ollama — `http://localhost:11434/api/generate`
5. Azure OpenAI — `https://{resource}.openai.azure.com/openai/deployments/{deployment}/chat/completions`

**Controllers**:
```
POST /ai-content/generate          — Queue AI post generation (throttled: 3 req/min/user)
POST /ai-content/refine           — Synchronous text refinement
GET  /ai-content/status/:jobId    — Poll BullMQ job state
GET  /ai-content/provider-health  — Health status of all AI providers
GET/POST /ai-content/api-keys/*   — API key CRUD + rotation
POST /ai-content/batch            — Batch generation
```

**Services**:
- `AiContentService` — generate, refine, batch generate
- `AiImageService` — AI image generation
- `ProviderHealthService` — health-check all AI providers
- `ApiKeyService` — AES-256 encrypted key storage + rotation
- `AiRateLimiterService` — per-user rate limiting (3 req/min)
- SEO AI: `SeoTitleService`, `SeoMetaService`, `SeoImageAltService`, `KeywordSuggestionService`

**Queue** (Asynq): `ai-content-queue` with retry (3 attempts, 60s backoff)

---

## P3-04 — operations Module

**Path**: `internal/modules/operations/`

**Reference**: `erg-backend/src/modules/operations/`

**Entities** (MySQL):
- `SystemConfig` — key, value, description, updated_at

**Controllers**:
```
GET /health                       — Unified health: MySQL + MongoDB + Redis + erg-go probes
GET/PUT /operations/config/:key  — System config KV store
```

**Services**:
- `HealthProbesService` — calls erg-go `/healthz`, `/ready`, `/metrics` for other services
- `AbuseDetectionService` — tracks failed logins (5/hour → block), 404 hits (50/min → block), high frequency (100/10s → block)
- `IpProtectionService` — IP blocklist with TTL-based unblocking
- `SystemConfigService` — KV store backed by MySQL

---

## P3-05 — audit Module

**Path**: `internal/modules/audit/`

**Reference**: `erg-backend/src/modules/audit/`

**Entities** (MongoDB): `AdminAuditLog` — already in F0-09

**Endpoints**:
```
GET /audit/logs                   — Paginated audit logs [admin, JWT + PermissionsGuard('audit.read')]
```

**Query filters**: user_id, action, resource_type, resource_id, date_range, tenant_id

---

## P3-06 — notifications-in-app Module

**Path**: `internal/modules/notifications-in-app/` (extend existing notifications)

**Reference**: `erg-backend/src/modules/notifications/`

**Entities**: `Notification` (MongoDB):
```go
type Notification struct {
    ID         primitive.ObjectID `bson:"_id,omitempty"`
    TenantID   string             `bson:"tenant_id"`
    UserID     string             `bson:"user_id"`
    Type       string             `bson:"type"`
    Title      string             `bson:"title"`
    Message    string             `bson:"message"`
    Priority   string             `bson:"priority"` // LOW|NORMAL|HIGH|URGENT
    Channel    string             `bson:"channel"`  // IN_APP|EMAIL|BOTH
    Status     string             `bson:"status"`   // UNREAD|READ
    GroupKey   string             `bson:"group_key,omitempty"`
    Source     string             `bson:"source,omitempty"`
    ActorID    string             `bson:"actor_id,omitempty"`
    ActorName  string             `bson:"actor_name,omitempty"`
    Actions    []Action           `bson:"actions,omitempty"`
    Metadata   map[string]any     `bson:"metadata,omitempty"`
    ExpiresAt  *time.Time         `bson:"expires_at,omitempty"`
    ReadAt     *time.Time         `bson:"read_at,omitempty"`
    CreatedAt  time.Time          `bson:"created_at"`
}
```

**Endpoints**:
```
GET  /notifications                     — List user notifications
GET  /notifications/unread-count        — Unread count
PATCH /notifications/:id/read           — Mark as read
PATCH /notifications/read-all           — Mark all as read
GET  /notifications/by-type/:type      — Filter by type
GET  /notifications/by-source/:src     — Filter by source
GET  /notifications/group/:key          — Grouped notifications
DELETE /notifications/read-all          — Delete all read
DELETE /notifications/:id               — Delete single
```

**Features**:
- SSE real-time push: `sseService.emitToUser(userID, 'notification', data)`
- Admin broadcast: `sseService.emitToAdmins()`
- Cron: cleanup read notifications older than 30 days
- `createForAdmins()` — fan-out notification to all users with `admin` role

---

## P4-01 — sitemap Module

**Path**: `internal/modules/sitemap/`

**Reference**: `erg-backend/src/modules/sitemap/`

**Entities**: None (proxies PostsModule and CoursesModule)

**Controllers**:
```
GET /sitemap.xml    — Dynamic XML sitemap (all published posts + courses, grouped by section)
GET /robots.txt     — robots.txt (configurable allow/disallow)
```

**Features**:
- Cache sitemap for 30 min
- Group by: pages, posts, courses, recruitment
- Include lastmod, changefreq, priority per section

---

## Verification Checklist (per module)

For EVERY module, before marking complete, verify:

1. `go build ./...` — zero errors
2. `go vet ./...` — zero warnings
3. `go test ./internal/modules/{name}/...` — all tests pass (or skip if no tests yet)
4. All HTTP endpoints registered and accessible
5. All DB entities have MongoDB/MySQL indexes defined
6. All async jobs (Asynq) have proper error handling and retry
7. All external calls have timeout context
8. All errors use `pkg/errors/` codes
9. Multi-tenancy: tenant_id filtered in all queries
10. Clean Architecture: no direct DB access in controllers

---
