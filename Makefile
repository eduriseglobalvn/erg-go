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
export GOOS        ?= linux

.PHONY: all build test lint lint-fix clean docker-build docker-up docker-down \
        deploy migrate generate proto-install tidy fmt vet staticcheck \
        coverage ci help

# ── Help ───────────────────────────────────────────────────────────────────────
help: ## Show this help message.
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# ── Core targets ──────────────────────────────────────────────────────────────
all: build ## Build all services (default).

build: ## Build all service binaries into bin/.
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -o $(BIN_DIR)/bot-service          ./cmd/bot-service
	$(GOBUILD) -o $(BIN_DIR)/notification-service ./cmd/notification-service
	$(GOBUILD) -o $(BIN_DIR)/crawler-service      ./cmd/crawler-service
	$(GOBUILD) -o $(BIN_DIR)/trending-service      ./cmd/trending-service
	@echo "Built: $(SERVICES)"

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

tidy: ## Tidy go.mod and go.sum for all modules.
	go mod tidy -C .
	@for svc in $(SERVICES); do go mod tidy -C ./cmd/$$svc; done

vet: ## Run go vet on all packages.
	go vet ./...

staticcheck: ## Run staticcheck linter.
	staticcheck $(GO_PACKAGES)

# ── Code generation ────────────────────────────────────────────────────────────
generate: ## Run code generators (protobuf, Wire DI).
	cd proto && protoc --go_out=../pkg --go_opt=paths=source_relative \
		--go-grpc_out=../pkg --go-grpc_opt=paths=source_relative \
		events.proto
	@echo "Regenerate wire dependencies: wire gen ./..."

proto-install: ## Install protobuf compiler and Go plugins.
	which protoc || (echo "Installing protoc..." && go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
		go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest)
	which wire || go install github.com/google/wire/cmd/wire@latest

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
run/%: ## Run a service locally (requires local infrastructure).
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

# ── Cleanup ────────────────────────────────────────────────────────────────────
clean: ## Remove build artifacts, test caches, and coverage reports.
	rm -rf $(BIN_DIR)
	rm -f coverage.out coverage.html
	go clean -testcache

dist-clean: clean ## Remove all generated files and downloaded modules.
	rm -rf vendor
	go clean -modcache
