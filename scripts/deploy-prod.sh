#!/usr/bin/env sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
ENV_FILE="$ROOT_DIR/.env.production"
ENV_EXAMPLE="$ROOT_DIR/.env.production.example"
COMPOSE_FILE="$ROOT_DIR/docker-compose.prod.yml"

if [ ! -f "$ENV_FILE" ]; then
  if [ ! -f "$ENV_EXAMPLE" ]; then
    echo "Missing $ENV_EXAMPLE" >&2
    exit 1
  fi
  cp "$ENV_EXAMPLE" "$ENV_FILE"
  echo "Created .env.production from .env.production.example"
fi

if grep -q "replace-with-" "$ENV_FILE"; then
  echo ".env.production still contains replace-with-* placeholders." >&2
  echo "Edit .env.production and replace all secrets/domains before deploying." >&2
  exit 1
fi

docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d --build
