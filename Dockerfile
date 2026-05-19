FROM golang:1.26-alpine AS builder

RUN apk add --no-cache ca-certificates git tzdata

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=local
ARG BUILD_DATE=unknown
ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" \
    -o /out/erg-server ./cmd/server && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/db-migrate ./cmd/db-migrate && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/hoclieu-seed ./cmd/hoclieu-seed && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/lms-seed ./cmd/lms-seed

FROM alpine:3.22 AS runtime

RUN apk add --no-cache ca-certificates tzdata wget && \
    addgroup -S erg && \
    adduser -S -D -H -h /app -s /sbin/nologin -G erg erg

WORKDIR /app

COPY --from=builder /out/erg-server /usr/local/bin/erg-server
COPY --from=builder /out/db-migrate /usr/local/bin/db-migrate
COPY --from=builder /out/hoclieu-seed /usr/local/bin/hoclieu-seed
COPY --from=builder /out/lms-seed /usr/local/bin/lms-seed
COPY --from=builder /src/config/application.yaml /app/config/application.yaml
COPY --from=builder /src/docs /app/docs

ENV APP__HOST=0.0.0.0
ENV APP__PORT=8080
ENV APP__ENV=production
ENV GIN_MODE=release
ENV TZ=Asia/Ho_Chi_Minh
ENV GODEBUG=http2server=0

EXPOSE 8080

USER erg

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1

ENTRYPOINT ["erg-server"]
