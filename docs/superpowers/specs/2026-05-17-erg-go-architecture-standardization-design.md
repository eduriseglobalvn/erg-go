# ERG Go Architecture Standardization Design

## Goal

Refactor `erg-go` into a large-scale modular monolith that keeps domain boundaries explicit, applies one consistent API/security/error-handling model, and can later be split into microservices without reworking the whole codebase.

## Architectural Direction

Use a domain-first modular monolith with lightweight Clean Architecture boundaries inside each module:

```text
internal/
├── platform/
│   ├── context/
│   ├── exception/
│   ├── logging/
│   ├── middleware/
│   ├── response/
│   ├── security/
│   └── validation/
├── modules/
│   └── <module>/
│       ├── api/
│       │   ├── controller/
│       │   ├── form/
│       │   ├── request/
│       │   └── response/
│       ├── application/
│       │   ├── service/
│       │   └── usecase/
│       ├── domain/
│       │   ├── entity/
│       │   ├── exception/
│       │   ├── policy/
│       │   └── valueobject/
│       ├── infrastructure/
│       │   ├── client/
│       │   ├── persistence/
│       │   └── repository/
│       ├── mapper/
│       ├── module.go
│       └── routes.go
└── shared/
    ├── contracts/
    ├── events/
    └── kernel/
```

This preserves one deployable server today while making each domain portable later.

## Design Principles

1. Organize by bounded context first, not by global technical layer.
2. Keep HTTP concerns in `api`, orchestration in `application`, business rules in `domain`, and IO in `infrastructure`.
3. Expose module capabilities only through explicit contracts, application services, or events.
4. Keep API DTOs separate from domain entities and persistence records.
5. Treat migrations, backfills, and heavy seed jobs as operational commands, not default startup work.
6. Standardize request validation, response envelopes, exception mapping, and audit-friendly logging once in `internal/platform`.

## Cross-Cutting Platform

### Response

All HTTP responses use one envelope:

```json
{
  "success": true,
  "code": "OK",
  "message": "Success",
  "data": {},
  "meta": {
    "request_id": "..."
  }
}
```

Errors use:

```json
{
  "success": false,
  "code": "AUTH_INVALID_CREDENTIALS",
  "message": "Invalid email or password",
  "errors": [
    {
      "field": "email",
      "message": "email must be valid"
    }
  ],
  "meta": {
    "request_id": "..."
  }
}
```

### Exception Handling

Use one shared `AppError` contract with domain-specific codes:

```go
type AppError struct {
    Code       string
    Message    string
    HTTPStatus int
    Details    any
    Cause      error
}
```

Every module owns its own exception codes while the platform maps them into consistent HTTP responses.

### Validation and Form Safety

All inbound payloads must use explicit request DTOs or form structs with:

- strict type binding
- field length limits
- enum allowlists
- maximum pagination limits
- normalized identifiers
- upload filename sanitization
- MIME and size validation for files
- no direct entity binding from user input
- module-specific validators for business rules

### Security

Security controls become reusable platform features:

- JWT authentication
- route permission guards
- portal / tenant restrictions
- request ID propagation
- rate limiting
- trusted proxy handling
- CORS policy
- CSRF/session controls where cookies are used
- safe upload helpers
- SSRF-safe dialers and remote fetch policies

### Logging

Keep infrastructure logger implementation in `pkg/logger`, but standardize application usage in `internal/platform/logging`:

- controller: request boundary events
- application service: business events
- repository/client: IO failures and slow operations
- exception mapper: normalized error emission
- every request log includes request ID, tenant ID, user ID when available

## Module Variants

### Full Domain Module

Use for business-heavy modules such as `auth`, `users`, `lms`, `notifications`, `documents`, `crawler`, `posts`.

### Lightweight Module

Use for simple modules that do not justify every subfolder yet, such as `centers` or `sitemap`. They still follow the same dependency direction and response/validation rules, but may omit empty folders until needed.

## Microservice Extraction Rules

1. A module owns its own data model and persistence boundary.
2. Other modules cannot import its private repository or entities directly.
3. Cross-module sync calls go through contracts or application interfaces.
4. Cross-module async communication uses events.
5. Shared types live in `internal/shared/contracts` only when they are truly cross-domain.
6. Public gRPC/HTTP contracts remain independent from internal persistence structures.

## Startup Model

### Must run before serving traffic

- config load
- logger setup
- primary connection setup
- auth validator setup
- router assembly
- HTTP listener startup

### May run after serving traffic

- cache warmup
- trending refresh
- noncritical periodic jobs

### Must move out of default startup

- historical backfills
- heavy migration work
- large seed jobs
- one-off maintenance tasks

## Migration Strategy

1. Introduce platform primitives first: response, exception, validation, logging, context.
2. Create a canonical module template and refactor 2-3 representative modules end-to-end.
3. Migrate remaining modules in waves based on coupling and business risk.
4. Move expensive boot-time side effects out of the request-serving path.
5. Tighten security/validation rules while touching each module.

## First-Wave Modules

- `auth`: security-sensitive, already layered, good canonical example
- `users`: common CRUD + uploads + shared identity concerns
- `lms`: complex domain, proves the architecture under load

## Non-Goals

- Do not split the system into separately deployed microservices in this project.
- Do not change external product behavior except where required for safer validation, consistent errors, or startup lifecycle improvements.
- Do not create empty folders merely for visual symmetry in modules that do not yet need them.

## Success Criteria

1. New modules have one obvious place for every concern.
2. API responses and validation failures are consistent across migrated modules.
3. Security-sensitive inputs are validated through explicit DTO/form contracts.
4. Direct cross-module coupling is reduced and documented.
5. Startup work is categorized and nonessential heavy tasks are removed from the critical boot path.
6. A future microservice extraction of a migrated module requires minimal internal reshaping.
