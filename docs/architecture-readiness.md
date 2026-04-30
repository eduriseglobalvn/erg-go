# Architecture Readiness

## Runtime Boundaries

`cmd/server` is the production API entrypoint. `cmd/plugin-server` remains a plugin host. `cmd/db-migrate` is the explicit schema/backfill command and must be run by CI/CD or an operator before deployment when schema changes are shipped.

The HTTP server no longer performs PostgreSQL schema mutation or MongoDB backfill by default. Local development can temporarily enable `database.auto_migrate` or `database.run_backfills`, but production config validation rejects both.

## Configuration

Configuration is layered from:

- `config/application.yaml`
- `config/application.{profile}.yaml`
- `config/application.local.yaml`
- legacy `config.yaml`
- `.env`
- process environment variables

Secrets should be supplied through `SECRET_` environment variables or a platform secret manager, not committed files.

## Microservice Extraction Path

Modules should stay behind explicit module dependencies and route registration boundaries. The first candidates for extraction are:

- LMS attempt/session traffic, because it has peak exam concurrency.
- Document processing, because it has large upload and watermark workloads.
- Notification/queue workers, because they are async and independently scalable.

Before extracting a module, add an API contract test, a module-specific load test, and an idempotent migration path.

## Operational Gates

Production deployment requires:

- explicit migration step using `cmd/db-migrate`
- secret scan in CI
- gosec high/medium findings at zero
- readiness and metrics routes exposed
- trusted proxy CIDRs configured when behind a load balancer
