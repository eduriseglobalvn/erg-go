# Startup Lifecycle

Not all work belongs on the critical boot path.

## Required Before Serving Traffic

- load configuration
- initialize logger
- initialize authentication validators
- connect mandatory infrastructure
- assemble routes and middleware
- start the HTTP listener

## Deferrable After Serving Traffic

- cache warmup
- initial refresh jobs
- noncritical schedulers
- best-effort background sync

## Operational Work

Keep these out of normal startup:

- historical backfills
- heavyweight migrations
- large seed jobs
- one-off maintenance tasks

## Current Migration Rule

When touching an existing module:

1. classify every startup side effect,
2. keep only traffic-critical work synchronous,
3. move one-off or heavy work into explicit commands,
4. document any exception that remains in startup.

