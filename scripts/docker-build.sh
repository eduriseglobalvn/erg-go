#!/usr/bin/env bash
# docker-build.sh — Build all Docker images for the erg monorepo.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

# Configuration.
REGISTRY="${REGISTRY:-ghcr.io/erg}"
TAG="${TAG:-$(git rev-parse --short HEAD 2>/dev/null || echo 'latest')}"
PLATFORMS="${PLATFORMS:-linux/amd64,linux/arm64}"
PUSH="${PUSH:-false}"

# Enable Docker buildx.
if ! docker buildx version >/dev/null 2>&1; then
  echo "Docker buildx not available. Falling back to standard build."
  PLATFORMS="linux/amd64"
fi

echo "=== erg monorepo: Docker build ==="
echo "  Registry: $REGISTRY"
echo "  Tag: $TAG"
echo "  Platforms: $PLATFORMS"
echo "  Push: $PUSH"
echo ""

# Create a temporary Dockerfile for multi-service build.
MULTI_DOCKERFILE="${ROOT_DIR}/Dockerfile.multi"
cat > "$MULTI_DOCKERFILE" << 'MULTIEOF'
# Multi-service Dockerfile for erg monorepo.
# Built by docker-build.sh
ARG TARGETservice
FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS builder
ARG TARGETservice
RUN apk add --no-cache git ca-certificates
WORKDIR /build
COPY go.mod go.sum* ./
RUN if [ -f go.sum ]; then go mod download; fi
COPY . .
ARG CGO_ENABLED=0
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=${CGO_ENABLED} GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
  -ldflags="-s -w" \
  -o /bin/${TARGETservice} ./cmd/${TARGETservice}

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /bin/${TARGETservice} /bin/${TARGETservice}
ENTRYPOINT ["/bin/${TARGETservice}"]
MULTIEOF

services=("bot-service" "notification-service" "crawler-service" "trending-service")

for service in "${services[@]}"; do
  image="${REGISTRY}/${service}:${TAG}"
  echo "Building $image ..."

  if docker buildx version >/dev/null 2>&1; then
    docker buildx build \
      --platform "$PLATFORMS" \
      --build-arg TARGETservice="$service" \
      --tag "$image" \
      --tag "${REGISTRY}/${service}:latest" \
      --file "$MULTI_DOCKERFILE" \
      --push="$PUSH" \
      "$ROOT_DIR"
  else
    docker build \
      --tag "$image" \
      --file "$MULTI_DOCKERFILE" \
      --build-arg TARGETservice="$service" \
      "$ROOT_DIR"
  fi

  echo "  ✓ $image"
done

# Cleanup.
rm -f "$MULTI_DOCKERFILE"

echo ""
echo "=== Docker build complete ==="
if [ "$PUSH" = "true" ]; then
  echo "Images pushed to $REGISTRY"
fi
