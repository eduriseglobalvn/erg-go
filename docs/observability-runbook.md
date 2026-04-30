# Observability Runbook

## SLOs

- API availability: 99.9% monthly.
- API latency: p95 under 500 ms and p99 under 1.5 s for LMS critical routes.
- LMS attempt save error rate: under 1%.
- Auth failure spike detection: alert on 5x baseline failed logins over 5 minutes.
- Queue processing: p95 job latency under 60 seconds for notification/digest jobs.

## Dashboards

- API: request rate, error rate, p50/p95/p99 latency, status codes, route labels.
- Database: PostgreSQL pool usage, wait count, slow query count, MongoDB pool usage, operation latency.
- Redis/Queue: Redis latency, command errors, queue depth, retries, dead-letter count.
- LMS: attempt start/save/sync/submit rate, save latency, submit failures, active attempt count.
- Documents: upload/download rate, rejected files, watermark failures, R2 errors.
- Auth/Security: login failures, blocked users/IPs, permission denials, firewall blocks, rate limit blocks.

## Alerts

- p95 API latency over 1 second for 10 minutes.
- 5xx rate over 2% for 5 minutes.
- LMS answer save error rate over 1% for 5 minutes.
- PostgreSQL pool usage over 80% for 10 minutes.
- Redis latency over 50 ms p95 for 10 minutes.
- Queue dead-letter count increases.
- Document watermark failure count greater than zero for protected documents.

## Redaction Rules

Logs must not include:

- `Authorization`, cookies or bearer tokens
- passwords, PINs or one-time codes
- JWT access/refresh tokens
- R2/SMTP/DB/Redis credentials
- raw service account JSON

Security events should keep actor ID, tenant ID, route, decision, reason code and request ID.
