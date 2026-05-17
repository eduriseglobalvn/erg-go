# Module Standard

`erg-go` uses a domain-first modular monolith. Organize code by bounded context first, then by responsibility inside each module.

## Full Domain Module

Use the full layout for modules with meaningful business rules, persistence, or security-sensitive workflows:

```text
internal/modules/<module>/
├── api/
│   ├── controller/
│   ├── form/
│   ├── request/
│   └── response/
├── application/
│   ├── service/
│   └── usecase/
├── domain/
│   ├── entity/
│   ├── exception/
│   ├── policy/
│   └── valueobject/
├── infrastructure/
│   ├── client/
│   ├── persistence/
│   └── repository/
├── mapper/
├── module.go
└── routes.go
```

## Lightweight Module

Small modules may omit folders they do not need yet. They must still preserve the same dependency direction and use shared platform primitives.

## Responsibility Rules

- `api`: HTTP-only concerns such as request binding, response formatting, route registration, and transport DTOs.
- `application`: orchestration and use cases that coordinate domain behavior and infrastructure.
- `domain`: entities, policies, invariants, value objects, and domain exceptions.
- `infrastructure`: database repositories, external clients, persistence adapters.
- `mapper`: transforms between API DTOs, domain objects, and persistence records.

## Dependency Direction

- `api` may depend on `application`.
- `application` may depend on `domain` abstractions and interfaces.
- `domain` must not depend on transport or infrastructure.
- `infrastructure` may implement interfaces required by `application` or `domain`.
- Other modules must not import private repositories or entities directly.

## API Contracts

- Never bind user input directly into persistence models.
- Use explicit request DTOs and form structs.
- Use explicit response DTOs instead of returning entities directly.
- Normalize errors through the shared exception and response platform.

## Microservice Extraction Guidance

- A module should own its persistence boundary.
- Cross-module sync communication should use contracts or application interfaces.
- Cross-module async communication should use events.
- Shared types belong in `internal/shared/contracts` only when they are genuinely cross-domain.

