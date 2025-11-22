.PHONY: help build db-up db-down db-reset db-migrate oidc-dev-keys keycloak-up keycloak-down keycloak-logs keycloak-reset test test-unit test-unit-db test-contract test-integration test-integration-mode1 test-integration-mode2 test-integration-all test-all ci-test test-integration-setup test-integration-teardown test-clean clean

help: ## Display available targets
	@echo "Grid Terraform State Management - Makefile"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

GRIDAPI_SRCS := $(shell find cmd/gridapi -name '*.go' -o -name '*.sql' -o -name 'model.conf')
bin/gridapi: $(GRIDAPI_SRCS)
	@echo "Building gridapi..."
	@cd cmd/gridapi && go build -o ../../bin/gridapi .

GRIDCTL_SRCS := $(shell find cmd/gridctl -name '*.go' -o -name '*.tmpl')
bin/gridctl: $(GRIDCTL_SRCS)
	@echo "Building gridctl..."
	@cd cmd/gridctl && go build -o ../../bin/gridctl .

build: ## Build gridapi and gridctl to bin/ directory
	@$(MAKE) bin/gridapi
	@$(MAKE) bin/gridctl

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

db-migrate: build ## Run database migrations
	@echo "Initializing migration tables..."
	@bin/gridapi db init
	@echo "Running migrations..."
	@bin/gridapi db migrate

oidc-dev-keys: ## Generate OIDC signing keys for local development (FR-110)
	@echo "Generating OIDC development signing keys..."
	@mkdir -p cmd/gridapi/internal/auth/keys
	@./scripts/dev/generate-oidc-keys.sh
	@echo "✓ Keys generated in cmd/gridapi/internal/auth/keys/"
	@echo "  Note: These are for local development only. Production must use secure key vault."

keycloak-up: ## Start Keycloak via docker compose (FR-111)
	@echo "Starting Keycloak..."
	@docker compose up -d keycloak
	@echo "Waiting for Keycloak to be healthy..."
	@sleep 5
	@docker compose ps keycloak
	@echo "✓ Keycloak available at http://localhost:8443"
	@echo "  Admin credentials: admin/admin"

keycloak-down: ## Stop Keycloak via docker compose (FR-111)
	@echo "Stopping Keycloak..."
	@docker compose stop keycloak

keycloak-logs: ## Show Keycloak logs (FR-111)
	@docker compose logs -f keycloak

keycloak-reset: ## Reset Keycloak environment (stop, prune volumes, restart) (FR-112)
	@echo "Resetting Keycloak environment..."
	@./scripts/dev/keycloak-reset.sh

test: test-all ## Run all tests (alias for test-all)

test-unit: ## Run unit tests (no external dependencies)
	@echo "Running unit tests..."
	@go test -v -short -race \
		./cmd/gridapi/internal/config/... \
		./cmd/gridapi/internal/server/... \
		./cmd/gridapi/internal/services/state/... \
		./cmd/gridapi/internal/services/tfstate/... \
		./cmd/gridapi/internal/services/graph/... \
		./cmd/gridapi/internal/services/dependency/... \
		./cmd/gridapi/internal/services/iam/... \
		./pkg/sdk/... \
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

test-integration: ## Run integration tests (no OIDC - automated setup via TestMain) - excludes Mode 1/Mode 2
	@echo "Running integration tests with automated setup..."
	@echo "Ensuring database is running..."
	@docker compose up -d postgres
	@sleep 2
	@cd tests/integration && go test -v -race -timeout 5m -skip "TestMode1|TestMode2"

test-integration-mode1: build ## Run Mode 1 (External IdP) integration tests with Keycloak
	@echo "Running Mode 1 (External IdP) integration tests..."
	@echo "Ensuring database and Keycloak are running..."
	@docker compose up -d postgres keycloak
	@sleep 5
	@echo "Extracting client credentials from realm-export.json..."
	$(eval GRIDAPI_SECRET := $(shell jq -r '.clients[] | select(.clientId=="grid-api") | .secret' tests/fixtures/realm-export.json))
	$(eval INTEGRATION_TESTS_SECRET := $(shell jq -r '.clients[] | select(.clientId=="integration-tests") | .secret' tests/fixtures/realm-export.json))
	@echo "✓ Credentials extracted: grid-api (gridapi server), integration-tests (test client)"
	@echo "Running tests with Mode 1 environment..."
	@cd tests/integration && \
		EXTERNAL_IDP_ISSUER="http://localhost:8443/realms/grid" \
		EXTERNAL_IDP_CLIENT_ID="grid-api" \
		EXTERNAL_IDP_CLIENT_SECRET="$(GRIDAPI_SECRET)" \
		EXTERNAL_IDP_CLI_CLIENT_ID="gridctl" \
		EXTERNAL_IDP_REDIRECT_URI="http://localhost:8080/auth/sso/callback" \
		MODE1_TEST_CLIENT_ID="integration-tests" \
		MODE1_TEST_CLIENT_SECRET="$(INTEGRATION_TESTS_SECRET)" \
		go test -v -race -timeout 10m -run "TestMode1"

test-integration-mode2: build ## Run Mode 2 (Internal IdP) integration tests
	@echo "Running Mode 2 (Internal IdP) integration tests..."
	@echo "Ensuring database is running..."
	@docker compose up -d postgres
	@sleep 2
	@echo "Running tests with Mode 2 environment..."
	@cd tests/integration && \
		OIDC_ISSUER="http://localhost:8080" \
		OIDC_CLIENT_ID="gridapi" \
		OIDC_SIGNING_KEY_PATH="tmp/keys/signing-key.pem" \
		go test -v -race -timeout 5m -run "TestMode2"

test-integration-all: build ## Run full integration suite (Mode 1 + Mode 2 with db resets)
	@echo "=========================================="
	@echo "Full Integration Test Suite"
	@echo "=========================================="
	@echo ""
	@echo "Phase 1: Mode 1 (External IdP with Keycloak)"
	@echo "------------------------------------------"
	@$(MAKE) db-reset
	@$(MAKE) db-migrate
	@docker compose up -d postgres keycloak
	@sleep 5
	@echo "Extracting client credentials from realm-export.json..."
	$(eval GRIDAPI_SECRET := $(shell jq -r '.clients[] | select(.clientId=="grid-api") | .secret' tests/fixtures/realm-export.json))
	$(eval INTEGRATION_TESTS_SECRET := $(shell jq -r '.clients[] | select(.clientId=="integration-tests") | .secret' tests/fixtures/realm-export.json))
	@echo "✓ Credentials extracted: grid-api (gridapi server), integration-tests (test client)"
	@cd tests/integration && \
		EXTERNAL_IDP_ISSUER="http://localhost:8443/realms/grid" \
		EXTERNAL_IDP_CLIENT_ID="grid-api" \
		EXTERNAL_IDP_CLIENT_SECRET="$(GRIDAPI_SECRET)" \
		EXTERNAL_IDP_REDIRECT_URI="http://localhost:8080/auth/sso/callback" \
		MODE1_TEST_CLIENT_ID="integration-tests" \
		MODE1_TEST_CLIENT_SECRET="$(INTEGRATION_TESTS_SECRET)" \
		go test -v -race -timeout 10m -run "TestMode1" || { echo "❌ Mode 1 tests failed"; exit 1; }
	@echo "✓ Mode 1 tests passed"
	@echo ""
	@echo "Phase 2: Mode 2 (Internal IdP)"
	@echo "------------------------------------------"
	@$(MAKE) db-reset
	@docker compose up -d postgres
	@sleep 2
	@cd tests/integration && \
		OIDC_ISSUER="http://localhost:8080" \
		OIDC_CLIENT_ID="gridapi" \
		OIDC_SIGNING_KEY_PATH="tmp/keys/signing-key.pem" \
		go test -v -race -timeout 5m -run "TestMode2" || { echo "❌ Mode 2 tests failed"; exit 1; }
	@echo "✓ Mode 2 tests passed"
	@echo ""
	@echo "=========================================="
	@echo "✓ Full integration suite completed!"
	@echo "=========================================="

test-all: ## Run all test suites
	@echo "Running all test suites..."
	$(MAKE) test-unit
	$(MAKE) test-unit-db
	$(MAKE) test-contract
	$(MAKE) test-integration-all
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
