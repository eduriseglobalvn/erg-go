# ERG-51 Capacity Report

Status: code-side readiness improved; final 100k certification still requires a distributed staging run.

Date: 2026-04-30

## Scope

ERG-51 covers the LMS exam path for 100,000 concurrent students:

- authentication
- student dashboard reads
- quiz package fetch
- attempt start
- answer save
- attempt sync
- attempt submit
- teacher live overview reads

## Implemented In This Batch

- `load/k6/lms_exam_flow.js` now exercises the full student and teacher hot paths.
- `make load-smoke` and `make load-100k` provide repeatable local and distributed test entry points.
- `make migrate/mongo-indexes` provisions LMS MongoDB indexes required by the hot paths.
- Attempt answer writes now update a single answer atomically instead of rewriting the entire answer map.
- Attempt submit is retry-safe: repeated submits return the existing submitted result instead of failing with a conflict.
- Attempt start is protected by a unique active-attempt index and re-reads the active attempt after duplicate insert races.

## Current Readiness Assessment

The code is ready for a staging load test, but the system is not yet certified for 100,000 concurrent students because no distributed staging result is attached.

Expected staging profile for the first certification run:

- API: at least 20 horizontally scaled replicas, then tune from measured CPU and p99 latency.
- MongoDB: production-tier cluster with indexes created by `make migrate/mongo-indexes`; watch connection saturation and slow queries.
- PostgreSQL: connection pool sized from actual auth/session/login load; pool saturation must stay under 80%.
- Redis: dedicated managed Redis/Valkey tier for rate limiting and queues; CPU below 70%.
- Load generators: 50-100 external generators, each proven stable before the full run.

## Done Gate

Move ERG-51 to Done only after a distributed staging report confirms:

- 100,000 concurrent students sustained for the agreed steady duration.
- HTTP error rate under 1%.
- p95/p99 latency gates from `docs/load-testing-100k.md` pass.
- DB, Redis, queue and pod metrics remain within the documented limits.
- No correctness issues are observed in attempt answers or submitted attempts.
