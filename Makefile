# Makefile — Build, test, lint, and deployment targets for erg monorepo.

# ── Variables ──────────────────────────────────────────────────────────────────
GOTEST      := go test
GOLINT      := golangci-lint run ./...
GOBUILD     := go build -ldflags="-s -w"
GOFMT       := gofmt -s -w
BIN_DIR     := bin
SERVICES    := bot-service notification-service crawler-service trending-service
GO_PACKAGES := $(shell go list ./...)

# Default Go environment.
export CGO_ENABLED ?= 0
export GOOS        ?= $(shell go env GOHOSTOS)
export GOARCH      ?= $(shell go env GOHOSTARCH)

.PHONY: all build test lint lint-fix clean docker-build docker-up docker-down \
        deploy migrate generate proto-install tidy fmt vet staticcheck \
        coverage ci help \
        plugin-build plugin-build/crawler-notif plugin-build/bot-notif \
        plugin-build/all plugin-list-tags

# ── Help ───────────────────────────────────────────────────────────────────────
help: ## Show this help message.
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# ── Core targets ──────────────────────────────────────────────────────────────
all: build ## Build all services (default).

build: ## Build the single erg-server binary into bin/.
	@mkdir -p $(BIN_DIR)
	go build -ldflags="-s -w" -o $(BIN_DIR)/erg-server ./cmd/server
	@echo "Built: $(BIN_DIR)/erg-server"

build/%: ## Build a specific service, e.g. make build/bot-service.
	$(GOBUILD) -o $(BIN_DIR)/$* ./cmd/$*

# ── Testing ────────────────────────────────────────────────────────────────────
test: ## Run all tests with race detector and verbose output.
	$(GOTEST) ./... -race -v -count=1 -timeout=10m

test/%: ## Run tests for a specific package, e.g. make test/pkg/config.
	$(GOTEST) ./$*/... -race -v -count=1

test-cover: ## Run tests with coverage report.
	$(GOTEST) ./... -race -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html

# ── Linting ────────────────────────────────────────────────────────────────────
lint: ## Run all linters (golangci-lint).
	$(GOLINT) ./...

lint-fix: ## Run linters with auto-fix where possible.
	golangci-lint run ./... --fix

# ── Formatting ─────────────────────────────────────────────────────────────────
fmt: ## Format all Go source files.
	$(GOFMT) .

tidy: ## Tidy go.mod and go.sum.
	go mod tidy -C .

vet: ## Run go vet on all packages.
	go vet ./...

staticcheck: ## Run staticcheck linter.
	staticcheck $(GO_PACKAGES)

# ── Proto generation (lib/ service clients) ─────────────────────────────────────
# Services managed by this Makefile.
SERVICES_PROTO := bot crawler notification trending

proto-install: ## Install protoc, buf CLI, and Go gRPC plugins.
	@echo "Installing protoc..."
	@which protoc || brew install protobuf 2>/dev/null || (echo "Install protoc: https://grpc.io/docs/protoc-installation/")
	@echo "Installing buf..."
	@which buf || go install github.com/bufbuild/buf/cmd/buf@latest
	@echo "Installing Go protoc plugins..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

proto-gen: proto-install ## Generate Go code from all proto files into lib/.
	@echo "Generating lib/ from proto/..."
	for svc in $(SERVICES_PROTO); do \
		echo "  → lib/$$svc/v1/"; \
		mkdir -p "lib/$$svc/v1"; \
		buf generate --template buf.gen.yaml "proto/lib/$$svc/v1/"; \
	done
	@echo "Proto generation complete."

proto-gen/%: proto-install ## Generate Go code for a specific service, e.g. make proto-gen/crawler.
	mkdir -p "lib/$*/v1"
	buf generate --template buf.gen.yaml "proto/lib/$*/v1/"

proto-lint: proto-install ## Lint all proto files with buf.
	buf lint proto/

proto-breaking: proto-install ## Check proto breaking changes against main branch.
	@echo "Checking breaking changes against main..."
	@if git rev-parse --verify main > /dev/null 2>&1; then \
		buf breaking --against '.git#branch=main' proto/; \
	else \
		echo "(main branch not found — skipping breaking change check)"; \
	fi

# ── Swagger ─────────────────────────────────────────────────────────────────────
SWAG_VERSION := v1.16.4

swag-init: ## Generate swagger docs (one-time, after adding @Security annotations).
	@which swag || go install github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)
	swag init -g cmd/server/server.go -o docs --parseDependency --parseInternal
	@echo "Swagger docs generated: docs/docs.go, docs/swagger.json, docs/swagger.yaml"

swag: swag-init ## Alias: run swag init + swagger docs updated.

swagger-ui: ## Download latest swagger-ui dist into docs/swagger-ui/.
	@mkdir -p docs/swagger-ui
	@echo "Swagger UI must be manually downloaded from https://github.com/swaggo/swagger-ui"
	@echo "Extract dist/ contents into docs/swagger-ui/"

generate: proto-gen ## Run all code generators (proto + any other).

# ── Docker ────────────────────────────────────────────────────────────────────
docker-build: ## Build all Docker images via docker compose.
	docker compose build

docker-up: ## Start all services in detached mode.
	docker compose up -d
	@echo "Services starting..."
	@docker compose ps

docker-down: ## Stop all services.
	docker compose down

docker-logs: ## Follow logs from all services.
	docker compose logs -f

docker-logs/%: ## Follow logs for a specific service.
	docker compose logs -f $*

docker-clean: ## Remove all containers, volumes, and images.
	docker compose down -v --remove-orphans

# ── Database migrations ────────────────────────────────────────────────────────
migrate: ## Run database migrations for all services.
	go run scripts/run_migrations.go

migrate/%: ## Run migrations for a specific service.
	go run scripts/run_migrations.go --service=$*

# ── Development helpers ─────────────────────────────────────────────────────────
run/%: ## Run a service locally, e.g. make run/server (runs ./cmd/server).
	go run ./cmd/$*

dev: ## Run all services with hot reload (requires air or fresh).
	air -c .air.toml

watch: ## Run tests on file changes (requires gotest).
	gotestsum -- -race -count=1 ./...

# ── Deployment ────────────────────────────────────────────────────────────────
deploy: ## Deploy all services to staging (requires kubectl context).
	@echo "Deploying to staging..."
	kubectl apply -f k8s/
	@echo "Deployment complete."

deploy/%: ## Deploy a specific service.
	@kubectl apply -f k8s/$*/

rollback/%: ## Rollback a specific service to the previous version.
	@kubectl rollout undo deployment/erg-$*

# ── CI ────────────────────────────────────────────────────────────────────────
ci: fmt tidy vet test lint ## Run full CI pipeline locally.

# ── Plugin / module composition (Phase 4, task3.md) ──────────────────────────────
#
# Compile-time module selection via Go build tags.
# Valid module names: bot, crawler, notification, trending
#
# Examples:
#   make plugin-build/all         # builds erg-full  (all 4 modules)
#   make plugin-build/crawler-notif  # builds erg-crawler-notif
#   make plugin-list-tags         # show available build tags

MODULE_NAMES := bot crawler notification trending

plugin-list-tags: ## List all available build tags for module selection.
	@echo "Available module build tags (Phase 4):"
	@echo "  module_bot          — Telegram/Discord bot module"
	@echo "  module_crawler      — RSS/crawler module"
	@echo "  module_notification — Notification module (Discord/Telegram/WhatsApp/Email)"
	@echo "  module_trending     — Trending topics module"
	@echo ""
	@echo "Compound tags:"
	@echo "  all_modules         — enables all modules (same as default build)"
	@echo ""
	@echo "Usage:"
	@echo "  go build -tags 'module_crawler,module_notification' -o bin/erg-crawler-notif ./cmd/server"

plugin-build/all: ## Build full server (all 4 modules, same as default 'build').
	@mkdir -p $(BIN_DIR)
	go build -tags 'all_modules' -ldflags="-s -w" -o $(BIN_DIR)/erg-full ./cmd/server
	@echo "Built: $(BIN_DIR)/erg-full (all_modules)"

plugin-build/crawler-notif: ## Build with crawler + notification modules only.
	@mkdir -p $(BIN_DIR)
	go build -tags 'module_crawler,module_notification' -ldflags="-s -w" -o $(BIN_DIR)/erg-crawler-notif ./cmd/server
	@echo "Built: $(BIN_DIR)/erg-crawler-notif (module_crawler + module_notification)"

plugin-build/bot-notif: ## Build with bot + notification modules only.
	@mkdir -p $(BIN_DIR)
	go build -tags 'module_bot,module_notification' -ldflags="-s -w" -o $(BIN_DIR)/erg-bot-notif ./cmd/server
	@echo "Built: $(BIN_DIR)/erg-bot-notif (module_bot + module_notification)"

# Generic target: make plugin-build/MODULE1+MODULE2 e.g. plugin-build/crawler+trending
plugin-build/%: ## Build with custom module combination, e.g. make plugin-build/crawler+trending.
	@{ \
	mods=$$(echo '$(@:plugin-build/%=%)' | tr '+' ' ' | tr '[:lower:]' '[:upper:]'); \
	tags=$$(echo $$mods | tr ' ' '\n' | grep . | sed 's/^/module_/' | tr '\n' ' '); \
	echo "Building with tags: $$tags"; \
	mkdir -p $(BIN_DIR) && \
	go build -tags "$$tags" -ldflags="-s -w" -o $(BIN_DIR)/erg-$(subst +,-,$(@:plugin-build/%=%)) ./cmd/server && \
	echo "Built: $(BIN_DIR)/erg-$(subst +,-,$(@:plugin-build/%=%))"; \
	}

# ── Cleanup ────────────────────────────────────────────────────────────────────
clean: ## Remove build artifacts, test caches, and coverage reports.
	rm -rf $(BIN_DIR)
	rm -f coverage.out coverage.html
	go clean -testcache

dist-clean: clean ## Remove all generated files and downloaded modules.
	rm -rf vendor
	go clean -modcache
