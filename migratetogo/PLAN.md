# Migration Plan — erg-backend → erg-go
**Date**: 2026-04-10
**Author**: Senior Engineer + Senior Architect (30y combined exp)
**Status**: PHASE 0 — FOUNDATION (F0) TBD

> Migrate ALL remaining NestJS modules from `erg-backend/` to Go modules in `erg-go/internal/modules/`.

---

## Executive Summary

| Aspect | Detail |
|---|---|
| **Source** | `erg-backend/` (NestJS, TypeScript, 25 modules) |
| **Target** | `erg-go/internal/modules/` (Go, chi/v5, 4 modules exist) |
| **Router** | `go-chi/chi/v5` (NOT Gin — current erg-go uses chi) |
| **ORM** | `go.mongodb.org/mongo-driver/v2` + `gorm.io/driver/mongodb` for MySQL |
| **Auth** | `pkg/auth/jwt.go` exists (validate only) — need JWT **issuance** + session |
| **Queues** | `erg.ninja/pkg/queue` (Asynq) already exists |
| **Pattern** | NestJS AppModule → chi Module: `NewModule()` → `Setup()` → `RegisterRoutes(r)` → `Stop()` |

---

## Gap Analysis: erg-backend vs erg-go

| Module | erg-backend | erg-go | Status |
|---|---|---|---|
| **bot** | ✅ | ✅ | DONE |
| **crawler** | ✅ 5-stage pipeline | ✅ 12-step pipeline | DONE |
| **notifications** | ✅ (proxy only) | ✅ Full | DONE |
| **trending** | ✅ (proxy only) | ✅ Full | DONE |
| **access-control** | ✅ RBAC, 7 roles | ❌ | MISSING |
| **auth** | ✅ JWT+refresh+PIN+OAuth | 🔶 JWT validate only | PARTIAL |
| **sessions** | ✅ Redis+MySQL | ❌ | MISSING |
| **users** | ✅ Full CRUD | ❌ | MISSING |
| **posts** | ✅ R2+soft-delete | ❌ | MISSING |
| **ai-content** | ✅ 5 AI providers | ❌ | MISSING |
| **seo** | ✅ 22+ services | ❌ | MISSING |
| **analytics** | ✅ MongoDB tracking | ❌ | MISSING |
| **audit** | ✅ MongoDB audit | ❌ | MISSING |
| **notifications-in-app** | ✅ SSE+grouping | ❌ | MISSING |
| **courses** | ✅ MySQL | ❌ | MISSING |
| **documents** | ✅ R2+watermark | ❌ | MISSING |
| **recruitment** | ✅ MySQL+CV | ❌ | MISSING |
| **reviews** | ✅ MySQL | ❌ | MISSING |
| **elearning** | ✅ MongoDB | ❌ | MISSING |
| **menus** | ✅ Per-domain | ❌ | MISSING |
| **pages** | ✅ MySQL | ❌ | MISSING |
| **operations** | ✅ Health+abuse | ❌ | MISSING |
| **sitemap** | ✅ XML | ❌ | MISSING |

---

## Migration Phases

```
F0  Foundation           — GORM, UberFX, chi middleware, R2, Auth full, config
P1  CMS & Taxonomy       — pages, menus, documents, courses, reviews, sessions
P2  Core Business        — posts, users, auth full, recruitment, elearning, analytics
P3  Enterprise          — access-control, seo, ai-content, operations, audit
P4  Infrastructure      — sitemap, integrations
P5  Cutover             — router, nginx, docker, docs, cutover
```

---

## Phase F0 — Foundation (est. 10 days)

| ID | Task | Agent | Dependencies |
|---|---|---|---|
| F0-01 | GORM Database Setup + MySQL client | Agent 1 | — |
| F0-02 | UberFX App Bootstrap with chi | Agent 1 | F0-01 |
| F0-03 | chi Router + Standard Middleware | Agent 1 | F0-02 |
| F0-04 | Standard Response + Validation | Agent 2 | F0-01 |
| F0-05 | R2 Cloudflare Storage (full) | Agent 2 | F0-01 |
| F0-06 | Auth Service (JWT issuance + refresh + PIN) | Agent 3 | F0-01 |
| F0-07 | Config.yaml + Environment Variables | Agent 3 | F0-01 |
| F0-08 | Module Skeleton Generator (scaffold) | Agent 4 | F0-02, F0-03 |
| F0-09 | Database Audit Middleware | Agent 4 | F0-01 |
| F0-10 | API Contract Snapshot (OpenAPI) | Agent 5 | F0-03 |
| F0-11 | Hot Reload (Air) + Dev Script | Agent 5 | F0-02 |

---

## Phase P1 — CMS & Taxonomy (est. 8 days)

| ID | Task | Agent | Dependencies |
|---|---|---|---|
| P1-01 | pages module (MySQL, slug-cache) | Agent 6 | F0-* |
| P1-02 | menus module (per-domain, no DB) | Agent 6 | F0-* |
| P1-03 | documents module (R2, PDF watermark) | Agent 6 | F0-05 |
| P1-04 | courses module (MySQL, schema markup) | Agent 7 | F0-* |
| P1-05 | reviews module (MySQL, moderation) | Agent 7 | F0-* |
| P1-06 | sessions module (Redis + MySQL) | Agent 7 | F0-01, F0-06 |
| P1-07 | elearning module (MongoDB) | Agent 8 | F0-* |

---

## Phase P2 — Core Business (est. 10 days)

| ID | Task | Agent | Dependencies |
|---|---|---|---|
| P2-01 | posts module (MySQL, R2, soft-delete) | Agent 6 | F0-05, P1-03 |
| P2-02 | users module (MySQL, avatar) | Agent 7 | F0-06, P1-06 |
| P2-03 | auth full (social OAuth: Google, Facebook, Apple) | Agent 8 | F0-06 |
| P2-04 | recruitment module (MySQL, CV R2) | Agent 6 | F0-05 |
| P2-05 | analytics module (MongoDB, geo-IP) | Agent 7 | F0-* |
| P2-06 | trending module v2 (rebuild from scratch) | Agent 8 | P1-05 |

---

## Phase P3 — Enterprise (est. 10 days)

| ID | Task | Agent | Dependencies |
|---|---|---|---|
| P3-01 | access-control module (RBAC, MySQL) | Agent 6 | P2-02 |
| P3-02 | seo module (MySQL, GSC OAuth2) | Agent 7 | P2-01 |
| P3-03 | ai-content module (5 providers) | Agent 8 | F0-05 |
| P3-04 | operations module (health, abuse) | Agent 6 | P2-02 |
| P3-05 | audit module (MongoDB) | Agent 7 | P3-01 |
| P3-06 | notifications-in-app (SSE, grouping) | Agent 8 | F0-* |

---

## Phase P4 — Infrastructure (est. 5 days)

| ID | Task | Agent | Dependencies |
|---|---|---|---|
| P4-01 | sitemap module (XML) | Agent 6 | P2-01 |
| P4-02 | Integration test suite | Agent 7 | P3-* |
| P4-03 | E2E test suite | Agent 8 | P4-01 |

---

## Phase P5 — Cutover (est. 5 days)

| ID | Task | Agent | Dependencies |
|---|---|---|---|
| P5-01 | Central router + nginx config | Agent 6 | P4-* |
| P5-02 | Docker Compose cutover | Agent 7 | P5-01 |
| P5-03 | API documentation | Agent 8 | P5-01 |
| P5-04 | Production cutover checklist | Agent 6 | P5-02 |

---

## Architecture Rules (Senior Engineer Standards)

1. **Router**: `go-chi/chi/v5` — NOT Gin, NOT Echo
2. **Module pattern**: `NewModule()` → `Setup() error` → `RegisterRoutes(*chi.Mux)` → `Stop(context.Context) error`
3. **Error wrapping**: `fmt.Errorf("module.service.method: %w", err)` at every boundary
4. **Context**: All operations with timeout context (5s DB, 10s external)
5. **Graceful shutdown**: SIGINT/SIGTERM với 30s timeout
6. **No hardcoded secrets**: All API keys from config/env
7. **CI gate**: Block `erg.ninja/internal` imports to `pkg/`
8. **Database**: MongoDB via `go.mongodb.org/mongo-driver/v2`; MySQL via `gorm.io/driver/mongodb`
9. **Multi-tenancy**: Use existing `pkg/tenant/` wrappers
10. **gRPC clients**: Use `lib/shared/factory.go` for cross-service calls

---

## Clean Architecture Per Module

```
internal/modules/{name}/
├── {name}.module.go        ← Module entry (NewModule, Setup, RegisterRoutes, Stop)
├── {name}.service.go       ← Business logic (interface-based)
├── {name}.controller.go    ← HTTP handlers (chi routes)
├── dto/
│   ├── request/           ← Request DTOs + validation
│   └── response/          ← Response DTOs
├── entities/
│   └── {entity}.go       ← Domain models (no ORM annotations)
├── repository/
│   └── repository.go     ← Data access layer (MongoDB/MySQL)
├── jobs/                  ← Asynq job handlers (if async)
└── {feature}/             ← Sub-features
    └── {feature}.go
```
