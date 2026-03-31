#!/usr/bin/env bash
# db-migrate.sh — Run MongoDB index migrations for all services.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

# Default: run all migrations.
SERVICE="${SERVICE:-all}"
DRY_RUN="${DRY_RUN:-false}"

# MongoDB connection.
MONGO_URI="${MONGODB_URI:-mongodb://localhost:27017}"
MONGO_DB="${MONGO_DB:-erg}"

echo "=== erg monorepo: database migrations ==="
echo "  MongoDB: $MONGO_URI"
echo "  Service: $SERVICE"
echo "  Dry run: $DRY_RUN"
echo ""

# Determine which migrations to run.
case "$SERVICE" in
  bot)
    MIGRATIONS=("001_bot_indexes.go")
    DATABASES=("$MONGO_DB")
    ;;
  notification)
    MIGRATIONS=("002_notification_indexes.go")
    DATABASES=("${MONGO_DB}_notifications")
    ;;
  crawler)
    MIGRATIONS=("003_crawler_indexes.go")
    DATABASES=("${MONGO_DB}_crawler")
    ;;
  trending)
    MIGRATIONS=("004_trending_indexes.go")
    DATABASES=("${MONGO_DB}_trending")
    ;;
  all)
    MIGRATIONS=(
      "001_bot_indexes.go"
      "002_notification_indexes.go"
      "003_crawler_indexes.go"
      "004_trending_indexes.go"
    )
    DATABASES=(
      "${MONGO_DB}"
      "${MONGO_DB}_notifications"
      "${MONGO_DB}_crawler"
      "${MONGO_DB}_trending"
    )
    ;;
  *)
    echo "Unknown service: $SERVICE" >&2
    echo "Valid options: bot, notification, crawler, trending, all" >&2
    exit 1
    ;;
esac

# Run migrations using the Go migration runner.
for i in "${!MIGRATIONS[@]}"; do
  MIGRATION="${MIGRATIONS[$i]}"
  DATABASE="${DATABASES[$i]}"
  echo "Running migration: $MIGRATION (database: $DATABASE)"
  if [ "$DRY_RUN" = "true" ]; then
    echo "  [DRY RUN] Would run: go run scripts/run_migrations.go --migration=$MIGRATION --db=$DATABASE"
  else
    go run "$ROOT_DIR/scripts/run_migrations.go" \
      --migration="$MIGRATION" \
      --db="$DATABASE" \
      --mongo-uri="$MONGO_URI" \
      || { echo "Migration $MIGRATION failed" >&2; exit 1; }
  fi
done

echo ""
echo "=== All migrations complete ==="
