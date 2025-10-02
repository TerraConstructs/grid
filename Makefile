.PHONY: help build db-up db-down db-reset test test-unit test-unit-db test-contract test-integration test-all ci-test test-integration-setup test-integration-teardown test-clean clean

help: ## Display available targets
	@echo "Grid Terraform State Management - Makefile"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build gridapi and gridctl to bin/ directory
	@echo "Building gridapi..."
	@cd cmd/gridapi && go build -o ../../bin/gridapi .
	@echo "Building gridctl..."
	@cd cmd/gridctl && go build -o ../../bin/gridctl .
	@echo "Build complete: bin/gridapi, bin/gridctl"

db-up: ## Start PostgreSQL via docker compose up -d
	@echo "Starting PostgreSQL..."
	@docker compose up -d postgres
	@echo "Waiting for PostgreSQL to be healthy..."
	@docker compose ps postgres

db-down: ## Stop PostgreSQL via docker compose down
	@echo "Stopping PostgreSQL..."
	@docker compose down

db-reset: ## Fresh database (docker compose down -v && docker compose up -d)
	@echo "Resetting database (removing volumes)..."
	@docker compose down -v
	@echo "Starting fresh PostgreSQL..."
	@docker compose up -d postgres
	@echo "Waiting for PostgreSQL to be healthy..."
	@sleep 2
	@docker compose ps postgres

test: test-all ## Run all tests (alias for test-all)

test-unit: ## Run unit tests (no external dependencies)
	@echo "Running unit tests..."
	@go test -v -short -race \
		./cmd/gridapi/internal/config/... \
		./cmd/gridapi/internal/server/... \
		./cmd/gridapi/internal/state/... \
		./cmd/gridctl/...

test-unit-db: ## Run repository unit tests (requires database)
	@echo "Running repository unit tests..."
	@echo "Ensuring database is running..."
	@docker compose up -d postgres
	@sleep 2
	@go test -v -race ./cmd/gridapi/internal/repository/...

test-contract: ## Run contract tests (requires server) - WIP: Tests are placeholder TODOs
	@echo "Contract tests are currently TODO placeholders (all skipped)"
	@echo "To implement: Add gridapi service to docker-compose.yml or use TestMain pattern"
	@echo "Skipping contract tests..."
	@# TODO: Uncomment when contract tests are implemented
	@# docker compose up -d
	@# sleep 3
	@# ./scripts/wait-for-health.sh
	@# go test -v -race ./tests/contract/...

test-integration: ## Run integration tests (automated setup via TestMain)
	@echo "Running integration tests with automated setup..."
	@echo "Ensuring database is running..."
	@docker compose up -d postgres
	@sleep 2
	@cd tests/integration && go test -v -race -timeout 5m

test-all: ## Run all test suites
	@echo "Running all test suites..."
	$(MAKE) test-unit
	$(MAKE) test-unit-db
	$(MAKE) test-contract
	$(MAKE) test-integration
	@echo "✓ All test suites passed"

ci-test: ## Run CI test pipeline
	@echo "Running CI test pipeline..."
	@docker compose up -d
	@sleep 3
	@./scripts/wait-for-health.sh
	$(MAKE) test-unit
	$(MAKE) test-unit-db
	$(MAKE) test-contract
	$(MAKE) test-integration
	@docker compose down
	@echo "✓ CI pipeline completed"

test-integration-setup: ## Start gridapi server for manual testing
	@echo "Starting gridapi server for manual testing..."
	@docker compose up -d
	@sleep 2
	@./bin/gridapi serve --server-addr :8080 --db-url "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable" &
	@echo "Server starting... waiting for health check"
	@./scripts/wait-for-health.sh
	@echo "Server ready at http://localhost:8080"

test-integration-teardown: ## Stop gridapi server
	@echo "Stopping gridapi server..."
	@pkill -TERM gridapi || true
	@sleep 1

test-clean: ## Clean test artifacts and test data
	@echo "Cleaning test artifacts..."
	@rm -rf tests/integration/tmp_*
	@rm -f tests/**/*.test
	@./scripts/test-cleanup.sh

clean: ## Remove bin/ directory and test artifacts
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@echo "Cleaning test artifacts..."
	@go clean -testcache
	@echo "Clean complete"
