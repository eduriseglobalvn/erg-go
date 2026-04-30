# Configuration

`erg-server` uses layered YAML configuration, similar in spirit to Spring Boot
`application.yaml` profiles, plus environment/secret-manager overrides for
sensitive values.

## File Layers

Config files are loaded from the repository root and `config/` directory.
Later layers override earlier layers.

1. `application.yaml`
2. `config.yaml` for legacy compatibility
3. `application.<profile>.yaml`
4. `config.<profile>.yaml` for legacy compatibility
5. `application.local.yaml`
6. `config.local.yaml`
7. `application.<profile>.local.yaml`
8. `config.<profile>.local.yaml`
9. `.env`
10. process environment variables

The tracked default file is:

```text
config/application.yaml
```

Local secret overrides should use:

```text
config/application.local.yaml
```

That file is intentionally ignored by Git.

## Profiles

The active profile is resolved in this order:

1. Explicit loader option in code
2. `ERG_PROFILE`
3. `APP_PROFILE`
4. `APP__ENV`
5. `APP_ENV`
6. `NODE_ENV`
7. `app.env` from the base YAML file

Examples:

```powershell
$env:APP_PROFILE = "development"
make dev
```

```powershell
$env:APP_PROFILE = "production"
.\bin\erg-server
```

## Secrets

Do not commit real secrets into any tracked YAML file.

Use environment variables or your deployment secret manager:

```text
SECRET_AUTH__JWT_SECRET
SECRET_AUTH__JWT_REFRESH_SECRET
SECRET_DB__PASSWORD
SECRET_MONGODB__URI
SECRET_REDIS__PASSWORD
SECRET_QUEUE__REDIS_PASSWORD
SECRET_SMTP__PASSWORD
SECRET_R2__ACCESS_KEY_ID
SECRET_R2__SECRET_KEY
SECRET_AI__GEMINI_API_KEY
```

For local development only, copy:

```text
config/application.local.yaml.example
```

to:

```text
config/application.local.yaml
```

and fill local-only values. Never commit that file.

## Migration From `.env`

Use YAML for non-secret structured config such as ports, CORS origins, timeouts,
pool sizes, telemetry, logging, and feature toggles.

Keep `.env` only for temporary local overrides and secrets while the team moves
to a real secret manager.

## Runtime Safety Switches

PostgreSQL schema changes and legacy backfills are disabled in the server by
default:

```yaml
database:
  auto_migrate: false
  run_backfills: false
```

Run them explicitly:

```powershell
go run ./cmd/db-migrate
go run ./cmd/db-migrate -backfill
```

Production config validation rejects `auto_migrate: true` and
`run_backfills: true`.

## Trusted Proxies

If the API is behind a load balancer or ingress, configure the proxy CIDRs that
are allowed to supply `X-Forwarded-For` or `X-Real-IP`:

```yaml
http:
  trusted_proxy_cidrs:
    - 10.0.0.0/8
```

When this list is empty, the backend ignores spoofable forwarding headers and
uses the direct peer IP.
