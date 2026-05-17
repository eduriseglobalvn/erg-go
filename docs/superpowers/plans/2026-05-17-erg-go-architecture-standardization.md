# ERG Go Architecture Standardization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor `erg-go` into a domain-first modular monolith with consistent platform primitives, safer request handling, and clearer future microservice boundaries.

**Architecture:** Introduce `internal/platform` for cross-cutting concerns, preserve `pkg/` for reusable infrastructure, and migrate modules toward `api / application / domain / infrastructure` boundaries in waves. Start with platform primitives and canonical modules before touching the rest of the repo.

**Tech Stack:** Go, Gin, Uber FX, Viper, MongoDB, PostgreSQL/GORM, Redis, Asynq, OpenTelemetry, Prometheus.

---

## File Map

- Create `internal/platform/context/*` for request-scoped metadata.
- Create `internal/platform/exception/*` for app errors and mapping.
- Create `internal/platform/response/*` for standardized API envelopes.
- Create `internal/platform/validation/*` for request validation helpers.
- Create `docs/architecture/module-standard.md` for contributor guidance.
- Create `docs/architecture/startup-lifecycle.md` for startup categories.
- Gradually migrate module internals under `internal/modules/<module>/{api,application,domain,infrastructure}`.

### Task 1: Build request context platform primitives

**Files:**
- Create: `internal/platform/context/request.go`
- Test: `internal/platform/context/request_test.go`

- [ ] **Step 1: Write failing tests for request metadata round-tripping**
- [ ] **Step 2: Run `go test ./internal/platform/context` and confirm failure**
- [ ] **Step 3: Implement typed context helpers for request ID, tenant ID, user ID**
- [ ] **Step 4: Re-run tests and confirm pass**

### Task 2: Build shared exception primitives

**Files:**
- Create: `internal/platform/exception/app_error.go`
- Create: `internal/platform/exception/app_error_test.go`

- [ ] **Step 1: Write failing tests for `AppError`, wrapping, and status defaults**
- [ ] **Step 2: Run `go test ./internal/platform/exception` and confirm failure**
- [ ] **Step 3: Implement minimal shared exception type**
- [ ] **Step 4: Re-run tests and confirm pass**

### Task 3: Build standardized response envelope

**Files:**
- Create: `internal/platform/response/response.go`
- Create: `internal/platform/response/response_test.go`

- [ ] **Step 1: Write failing Gin tests for success/error envelope shape**
- [ ] **Step 2: Run `go test ./internal/platform/response` and confirm failure**
- [ ] **Step 3: Implement envelope writer and metadata helpers**
- [ ] **Step 4: Re-run tests and confirm pass**

### Task 4: Build validation helper layer

**Files:**
- Create: `internal/platform/validation/validator.go`
- Create: `internal/platform/validation/validator_test.go`

- [ ] **Step 1: Write failing tests for field-error normalization and pagination limits**
- [ ] **Step 2: Run `go test ./internal/platform/validation` and confirm failure**
- [ ] **Step 3: Implement reusable validator helpers**
- [ ] **Step 4: Re-run tests and confirm pass**

### Task 5: Document the new module standard

**Files:**
- Create: `docs/architecture/module-standard.md`
- Create: `docs/architecture/startup-lifecycle.md`

- [ ] **Step 1: Document full and lightweight module layouts**
- [ ] **Step 2: Document dependency direction and future microservice extraction rules**
- [ ] **Step 3: Document startup responsibilities by category**

### Task 6: Create the first migration wave

**Files:**
- Modify later: `internal/modules/auth/**`
- Modify later: `internal/modules/users/**`
- Modify later: `internal/modules/lms/**`

- [ ] **Step 1: Inventory current file ownership and coupling**
- [ ] **Step 2: Add migration checklist per module**
- [ ] **Step 3: Migrate one module at a time behind existing tests**

### Task 7: Migrate the rest of the repo in waves

**Files:**
- Modify later: remaining `internal/modules/**`

- [ ] **Step 1: Group remaining modules by risk and coupling**
- [ ] **Step 2: Migrate low-risk modules first**
- [ ] **Step 3: Migrate mixed modules**
- [ ] **Step 4: Migrate complex modules**
- [ ] **Step 5: Run full regression suite after each wave**

## Verification

- Run focused platform tests:
  - `go test ./internal/platform/...`
- Run regression suites for migrated modules:
  - `go test ./internal/modules/auth/... ./internal/modules/users/... ./internal/modules/lms/...`
- Run broader verification before completion:
  - `go test ./...`

