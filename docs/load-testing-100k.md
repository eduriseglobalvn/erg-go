# 100k Concurrent Student Load Test

This repository contains the k6 scenario used to prove the LMS exam path:

- login, when `CREDENTIALS_FILE` is provided
- student dashboard reads
- quiz package fetch
- attempt start
- answer save
- attempt sync
- attempt submit
- teacher quiz progress and assignment progress reads

## Required Staging Data

Create a production-like staging dataset before running the 100k test:

- at least 100,000 active student accounts
- one published quiz with realistic slides/questions/assets
- one assignment targeting the test students
- one teacher/admin token for overview traffic
- MongoDB and PostgreSQL indexes created with `make migrate/mongo-indexes`

Required environment values:

```bash
export LOAD_BASE_URL=https://api-staging.erg.edu.vn
export TENANT_ID=default
export AUTH_TOKEN=student-or-shared-test-token
export TEACHER_TOKEN=teacher-or-admin-token
export ASSIGNMENT_ID=<assignment-id>
export QUIZ_ID=<quiz-id>
export CLASS_ID=<class-id>
```

For per-user login load, provide a CSV file:

```csv
# email,password
student001@example.test,Password123!
student002@example.test,Password123!
```

Then set:

```bash
export CREDENTIALS_FILE=load/fixtures/staging-students.csv
export LOGIN_EACH_ITERATION=true
```

## Local Smoke

Use a small run to verify the flow and data before any large test:

```bash
make load-smoke \
  LOAD_BASE_URL="$LOAD_BASE_URL" \
  AUTH_TOKEN="$AUTH_TOKEN" \
  TEACHER_TOKEN="$TEACHER_TOKEN" \
  TENANT_ID="$TENANT_ID" \
  ASSIGNMENT_ID="$ASSIGNMENT_ID" \
  QUIZ_ID="$QUIZ_ID" \
  CLASS_ID="$CLASS_ID" \
  LOAD_RAMP_VUS=5 \
  LOAD_STEADY_VUS=5 \
  LOAD_STEADY_DURATION=2m
```

## Distributed 100k Run

Run k6 from multiple generators. Do not run 100,000 VUs from one developer laptop.

Recommended split:

- 100 generators x 1,000 steady VUs
- or 50 generators x 2,000 steady VUs, only after each generator is proven stable

Example per-generator command:

```bash
make load-100k \
  LOAD_BASE_URL="$LOAD_BASE_URL" \
  AUTH_TOKEN="$AUTH_TOKEN" \
  TEACHER_TOKEN="$TEACHER_TOKEN" \
  TENANT_ID="$TENANT_ID" \
  ASSIGNMENT_ID="$ASSIGNMENT_ID" \
  QUIZ_ID="$QUIZ_ID" \
  CLASS_ID="$CLASS_ID" \
  LOAD_RAMP_VUS=1000 \
  LOAD_STEADY_VUS=1000 \
  LOAD_RAMP_DURATION=15m \
  LOAD_STEADY_DURATION=30m \
  LOAD_RAMP_DOWN_DURATION=10m
```

## Pass Gates

ERG-51 can be marked Done only when the distributed staging run produces evidence for these gates:

- HTTP error rate under 1%.
- p95 request latency under 500 ms.
- p99 request latency under 1.5 s.
- p95 answer save latency under 300 ms.
- p99 answer save latency under 1 s.
- p95 submit latency under 1 s.
- p99 submit latency under 3 s.
- MongoDB and PostgreSQL connection pools stay below 80% saturation.
- Redis CPU stays below 70% during rate limiting and queue traffic.
- Queue depth remains bounded and drains after the test.
- No pod restarts, OOM kills, or readiness probe failures.

## Evidence To Attach

Attach these artifacts to ERG-51 before moving it to Done:

- k6 summary JSON from every generator
- aggregated p95/p99 latency and error-rate report
- API pod CPU/memory/restart graph
- MongoDB connection pool, CPU, lock and slow query metrics
- PostgreSQL connection pool, CPU and slow query metrics
- Redis latency, CPU and command-rate metrics
- queue depth and consumer throughput metrics
- final conclusion stating whether 100,000 concurrent students is supported and the exact infrastructure profile used
