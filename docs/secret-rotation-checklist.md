# Secret Rotation Checklist

Use this checklist any time credentials appear outside the approved secret manager or when a teammate leaves the project.

## Rotate Immediately

- PostgreSQL: create a new database user/password, update deployment secrets, restart services, then revoke the old user.
- MongoDB: rotate the application user password and invalidate old connection strings.
- Redis/Valkey: rotate the default/application password for API and queue clients.
- SMTP: rotate mailbox/API password and verify transactional email delivery.
- R2: create a new access key pair, update secrets, verify uploads/downloads, then delete the old key.
- JWT: rotate access and refresh signing secrets with a short overlap window; force refresh-token reissue.
- Google/Auth bridge: rotate bridge/shared secrets and disable unused providers.

## Verify

- CI gitleaks scan passes.
- `go test ./...`, `go vet ./...`, and `gosec -severity medium -confidence medium ./...` pass.
- `/api/ready` is healthy after secret rollout.
- Failed login, token refresh, upload, email and queue flows are checked in staging.

## Never Commit

- `.env`
- `config/application.local.yaml`
- service account JSON
- cloud storage access keys
- database or SMTP passwords
