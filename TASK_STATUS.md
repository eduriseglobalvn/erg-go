# TASK_STATUS.md — Migration to Go

> **Last updated**: 2026-04-09
> **Build**: ✅ `go build ./cmd/server` — zero errors
> **Tests**: ✅ All `go test ./...` — 17/17 packages pass

---

## Architecture

✅ **1 Binary monolith** (`cmd/server`) — NestJS AppModule style
✅ **Router**: chi/v5 xuyên suốt
✅ **DI**: UberFX
✅ **Build**: `go build ./cmd/server` — zero errors

---

## Current State Summary

| Area | Status | Notes |
|---|---|---|
| **cmd/server + main.go** | ✅ DONE | UberFX DI, chi router |
| **internal/routes/routes.go** | ✅ DONE | chi-based, plugin registry |
| **pkg/ packages** | ✅ DONE | 20 packages |
| **lib/ gRPC clients** | ✅ DONE | bot, crawler, notification, trending |
| **proto/ definitions** | ✅ DONE | 4 proto files |
| **internal/modules/ (all 4)** | ✅ DONE | bot, crawler, notifications, trending |
| **Makefile** | ✅ FIXED | Single binary, plugin-build targets |

---

## Phase 2 — Multi-Tenant Isolation ✅ (2026-04-09)

**Package `pkg/tenant/` — ALL FILES ALREADY EXISTED:**
- `context.go` ✅ — WithTenant / FromContext / MustFromContext / IsValid / Normalize
- `middleware.go` ✅ — TenantMiddleware (chi HTTP middleware)
- `mongo.go` ✅ — TenantMongoClient (per-tenant collection prefix)
- `redis.go` ✅ — TenantRedis (key prefix: `tenant:{id}:...`)
- `asynq.go` ✅ — TenantAsynqClient (per-tenant queue routing)
- `config.go` ✅ — TenantConfig + TenantDef + IsolationMode

**Files updated today:**

| File | Change |
|---|---|
| `pkg/config/config.go` | Added `Tenant TenantConfig` field + `TenantConfig` + `TenantDef` types |
| `config.yaml` | Added `tenant:` section (enabled: false by default) |
| `internal/routes/routes.go` | Wired `TenantMiddleware` + `TenantMongoClient` creation |
| `internal/routes/plugin_wiring.go` | Added `TenantMongoClient` to all module wiring |
| `internal/modules/bot/bot.module.go` | Added `TenantMongoClient` to Deps |
| `internal/modules/crawler/crawler.module.go` | Added `TenantMongoClient` to Deps |
| `internal/modules/notifications/notifications.module.go` | Added `TenantMongoClient` to Deps |
| `internal/modules/trending/trending.module.go` | Added `TenantMongoClient` to Deps |

**How it works:**
1. `routes.RegisterWithConfig` creates `TenantMongoClient` if `tenant.enabled: true`
2. Chi `TenantMiddleware` reads `X-Tenant-ID` header → JWT `tenant_id` claim → subdomain → default
3. All modules receive `TenantMongoClient` via their `Deps`
4. When `tenant.enabled: false` (default), falls back to `"default"` tenant — **zero breaking change**

**To enable multi-tenancy:** set `tenant.enabled: true` in config.yaml

---

## Build/Test Commands

```bash
cd /Users/vuong/ERG.Workspace/erg-go

go build ./cmd/server          # Single binary → ./erg-server
go test ./... -count=1        # All 17 packages
make build   # → bin/erg-server
```
