# Grid

[![PR Tests](https://github.com/TerraConstructs/grid/actions/workflows/pr-tests.yml/badge.svg)](https://github.com/TerraConstructs/grid/actions/workflows/pr-tests.yml)
[![Release](https://github.com/TerraConstructs/grid/actions/workflows/release-please.yml/badge.svg)](https://github.com/TerraConstructs/grid/actions/workflows/release-please.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/TerraConstructs/grid/cmd/gridapi)](https://goreportcard.com/report/github.com/TerraConstructs/grid/cmd/gridapi)

Grid is a remote Terraform/OpenTofu state service paired with a friendly CLI that automates backend configuration, dependency wiring, and collaborative workflows.

## Highlights
- **Remote State API** (`gridapi`) implementing the Terraform HTTP backend protocol with PostgreSQL persistence
- **State Labels** with policy-based validation and HashiCorp bexpr filtering for organizing and querying states
- **OIDC Authentication** supporting both external IdP (Keycloak) and internal IdP modes with JWT/refresh tokens
- **RBAC Authorization** with group-based roles and label expression matching using Casbin policies
- **JSON Schema Validation** with automatic inference and validation status surfaced in the web UI
- **Directory-aware CLI** (`gridctl`) for creating states, managing dependencies, and generating backend configs
- **Web Dashboard** (`webapp`) with auth flows, label filtering, dependency graphs, and schema validation UI
- **Go and TypeScript SDKs** for programmatic access and automation

## Demo
![Grid CLI demo showing dependency sync](demo/demo.gif)

The animated demo captures the full workflow: creating two Terraform states (`network` and `cluster`), wiring a dependency, syncing managed HCL, and listing states/dependencies. Regenerate it any time with:

```bash
./record.sh # from repo root
```

> [!IMPORTANT]
> Requires `vhs` installed (see demo/README.md).

## Quickstart

```bash
# Build CLI and API binaries
make build

# Start dependencies (PostgreSQL via Docker Compose)
make db-up

# Run database migrations
make db-migrate

# Run the API server without AuthN/AuthZ (defaults: :8080, DATABASE_URL env var)
./bin/gridapi serve --db-url "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable" &

# Use gridctl against the running server
./bin/gridctl state list --server http://localhost:8080

# Dashboard (webapp)
cd webapp
pnpm install
pnpm dev
```

Grid CLI commands read `.grid/` context in your working directory so repeated calls automatically target the same state. Override with explicit flags whenever needed.

## Features

### State Labels
Organize and filter states with user-defined labels (string, number, boolean types):
- **Policy-based validation**: Enforce regex patterns, allowed values, and size limits
- **HashiCorp bexpr filtering**: Query states with complex expressions like `env == "prod" and region in ["us-west", "us-east"]`
- **CLI support**: `gridctl state set <id> --label env=prod --label active=true --label -deprecated`
- **SDK helpers**: `BuildBexprFilter()`, `ConvertLabelsToProto()` for programmatic label management

### Authentication & Authorization
Flexible OIDC-based authentication with fine-grained RBAC:

#### OIDC Authentication (Mode 1 & Mode 2)
- **Mode 1 - External IdP**: Grid as Resource Server validating tokens from Keycloak or other OIDC providers
- **Mode 2 - Internal IdP**: Grid as complete OIDC provider with embedded authorization server
- **Token support**: JWT access tokens (RS256), refresh tokens, httpOnly cookies for web sessions
- **Flows**: OAuth2 authorization code with PKCE, client credentials (service accounts), device authorization
- **Session management**: Database-backed sessions with configurable expiration

#### RBAC with Label Expressions
- **Group-based roles**: Assign users to groups mapped to roles with specific permissions
- **Label expression matching**: Policies can match states by label expressions (e.g., `env == "dev"`)
- **Casbin enforcement**: Policy-based authorization with flexible rule definitions
- **JIT provisioning**: Automatic user creation on first login with group mapping
- **Performance optimized**: Lock-free cache reads with atomic.Value (67% faster, 70% fewer DB queries)

### JSON Schema Validation
Validate Terraform outputs against JSON schemas with automatic inference:
- **Schema declaration**: Set/get schemas per output via RPC or CLI
- **Automatic validation**: Validates outputs on Terraform state upload (synchronous, non-blocking)
- **Schema inference**: Automatically generates schemas from actual output values (asynchronous)
- **Validation metadata**: Status (`valid`, `invalid`, `not_validated`, `error`) exposed in all RPC responses
- **Structured errors**: JSON path-specific validation error messages
- **Source tracking**: Distinguish between `manual` and `inferred` schemas
- **CLI commands**:
  - `gridctl state get-output-schema <state> <output>`
  - `gridctl state set-output-schema <state> <output> --file schema.json`
- **Web UI**: Validation status with color-coded indicators, expandable schema viewer, error messages

### State Dependencies
Wire dependencies between states for hierarchical infrastructure with automatic drift and validation tracking:

#### Core Features
- **Automatic tracking**: Dependencies inferred from Terraform remote state data sources
- **Directed acyclic graph (DAG)**: Detect and prevent circular dependencies
- **Mock values**: Define dependencies ahead-of-time with mock values before producer outputs exist
- **CLI management**: `gridctl state add-dependency`, `gridctl state list-dependencies`
- **Graph visualization**: Interactive dependency graph in web dashboard with status indicators

#### Edge Status (Composite Model)
Each dependency edge tracks two orthogonal dimensions: **drift** (producer vs consumer synchronization) and **validation** (schema compliance).

**Status Values:**
- **`pending`**: Initial state, consumer hasn't observed producer output yet
- **`clean`**: Consumer is up-to-date (`in_digest == out_digest`) AND output passes schema validation
- **`clean-invalid`**: Consumer is up-to-date (`in_digest == out_digest`) BUT output fails schema validation
- **`dirty`**: Consumer is stale (`in_digest != out_digest`) AND output passes schema validation
- **`dirty-invalid`**: Consumer is stale (`in_digest != out_digest`) AND output fails schema validation
- **`mock`**: Using mock value; producer output doesn't exist yet
- **`missing-output`**: Producer output key was removed
- **`potentially-stale`**: Transitive upstream drift (computed via BFS propagation)

**Digest Tracking:**
- **`in_digest`**: Fingerprint of producer's current output value
- **`out_digest`**: Fingerprint of value last observed by consumer
- **Drift detection**: Automatic comparison on every state upload

**State Status (Rollup):**
Grid computes an overall state status based on incoming dependency edges:
- **`clean`**: All incoming edges are `clean`
- **`stale`**: Has at least one incoming `dirty` or `pending` edge (direct dependency drift)
- **`potentially-stale`**: No direct dirty edges, but has transitive upstream drift (propagated via BFS)

This status is automatically computed and available in `GetStateInfo` RPC responses and the web dashboard.

### Web Dashboard
React-based web interface with full feature coverage:
- **Authentication UI**: OAuth2/OIDC login flows with session persistence
- **State management**: Create, list, view, and filter states
- **Label filtering**: Interactive label filter builder with expression preview
- **Schema validation UI**: Color-coded validation status, expandable schemas, error messages with JSON paths
- **Dependency graph**: Interactive React Flow graph with zoom/pan controls
- **Responsive design**: Tailwind CSS with mobile-friendly layout

## Installation

### Pre-built Binaries

Download the latest release for your platform from the [Releases page](https://github.com/TerraConstructs/grid/releases):

- **gridapi**: `linux/amd64`, `linux/arm64`
- **gridctl**: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`
- **webapp**: Single `.tar.gz` bundle

### npm Package

Install the JavaScript SDK:

```bash
npm install @tcons/grid
# or
pnpm add @tcons/grid
```

### Build from Source

```bash
# Build CLI and API binaries
make build

# Binaries will be in ./bin/
./bin/gridapi --help
./bin/gridctl --help
```

## Repository Layout

```
.
├── api/                        # Generated protobuf/Connect code (Go)
│   └── state/v1/              # State service definitions
├── proto/                      # Protobuf source definitions
│   └── state/v1/              # State service .proto files
├── cmd/
│   ├── gridapi/               # API server
│   │   ├── cmd/              # Cobra commands (serve, db)
│   │   └── internal/         # Server implementation
│   │       ├── auth/         # OIDC provider implementation
│   │       ├── config/       # Configuration loading
│   │       ├── db/           # Database models and provider
│   │       ├── migrations/   # Schema migrations
│   │       ├── middleware/   # Auth/authz interceptors
│   │       ├── repository/   # Data access layer
│   │       ├── server/       # HTTP handlers (Connect + Terraform)
│   │       └── services/     # Business logic
│   │           ├── dependency/
│   │           ├── graph/
│   │           ├── iam/      # IAM and authentication services
│   │           ├── inference/ # Schema inference
│   │           ├── state/
│   │           ├── tfstate/
│   │           └── validation/ # Schema validation
│   └── gridctl/               # CLI
│       ├── cmd/              # Cobra commands
│       └── internal/         # CLI internals
├── pkg/sdk/                   # Go SDK for Grid API
├── js/sdk/                    # TypeScript SDK
│   ├── gen/                  # Generated from protobuf
│   └── src/                  # SDK helpers and utilities
├── webapp/                    # React web application
│   ├── src/
│   │   ├── components/      # UI components (auth, labels, outputs, graph)
│   │   ├── context/         # React context providers
│   │   ├── hooks/           # Custom React hooks
│   │   ├── services/        # API service layer
│   │   └── types/           # TypeScript type definitions
│   └── test/                # Frontend tests
├── tests/
│   ├── contract/            # API contract tests
│   ├── e2e/                 # Playwright end-to-end tests
│   │   ├── helpers/        # Test helpers (auth, state, keycloak)
│   │   └── setup/          # Test setup scripts
│   ├── fixtures/            # Test data (Keycloak realm, schemas, states)
│   └── integration/         # Go integration tests
├── examples/                 # Terraform sample projects
│   └── terraform/
├── demo/                     # VHS demo scripts and assets
├── specs/                    # Feature specifications
│   ├── 005-add-state-dimensions/   # Labels feature
│   ├── 006-authz-authn-rbac/       # Auth/authz
│   ├── 007-webapp-auth/            # WebApp authentication
│   ├── 008-cicd-workflows/
│   └── 010-output-schema-support/  # Schema validation
├── scripts/                  # Development and CI scripts
└── initdb/                   # Database initialization scripts
```

## Testing

Grid has comprehensive test coverage across unit, integration, and end-to-end tests. The most critical tests for feature validation are the **integration tests** and **Playwright e2e tests**, both running in CI/CD on every PR.

### Integration Tests

Integration tests use a real PostgreSQL database and automated server setup via `TestMain`. Located in `tests/integration/`.

#### Test Modes

**No Auth Mode** (Default - runs in CI/CD):
```bash
make test-integration
```
Tests core functionality without authentication:
- State CRUD operations (`quickstart_test.go`)
- Labels lifecycle, filtering, policy validation (`labels_test.go`)
- JSON Schema validation, inference, error handling (`output_validation_test.go`, `output_schema_test.go`)
- State dependencies and DAG enforcement (`dependency_test.go`)
- Locking and conflict handling (`lock_conflict_test.go`)
- Edge status tracking (`edge_status_composite_test.go`)

**Mode 1 - External IdP** (Keycloak-based auth):
```bash
make test-integration-mode1
```
Requires external Keycloak instance. Tests:
- Token validation and infrastructure setup (`auth_mode1_test.go`)
- RBAC with group-based permissions (`auth_mode1_rbac_helpers.go`)
- Device authorization flow
- JIT user provisioning

**Mode 2 - Internal IdP** (runs in CI/CD):
```bash
make test-integration-mode2
```
Grid as complete OIDC provider. Tests:
- OAuth2/OIDC flows (authorization code, client credentials)
- Token issuance (access tokens, refresh tokens)
- Session management (`auth_mode2_test.go`)

#### Key Test Coverage

- **Labels**: 238 lines covering lifecycle, policy validation, bexpr filtering
- **Schema Validation**: 966 lines covering pass/fail scenarios, error messages, validation metadata
- **Schema Operations**: 496 lines covering CRUD, pre-declaration, source tracking
- **Dependencies**: DAG enforcement, circular detection, edge status
- **Locking**: Pessimistic locking, conflict scenarios, lock metadata

### End-to-End Tests (Playwright)

Browser-based tests using Playwright. Located in `tests/e2e/`.

#### E2E Test Suites

**Default Flow** (No Auth):
```bash
pnpm test:e2e
# or
npx playwright test --config=playwright.config.ts
```
Tests basic web workflows without authentication.

**Auth Flow** (Keycloak SSO):
```bash
pnpm test:e2e:auth
# or
npx playwright test --config=playwright.config.auth.ts
```
Tests OAuth2/OIDC authentication flows with Keycloak:
- Login/logout flows (`auth-flow.spec.ts`)
- Session persistence across page reloads
- Group-based RBAC enforcement
- JIT user provisioning

**Test Helpers**:
- `helpers/auth.helpers.ts` - Login, logout, session verification
- `helpers/keycloak.helpers.ts` - Keycloak user management
- `helpers/state.helpers.ts` - State creation, verification

### CI/CD Test Coverage

Every PR runs the following tests via `.github/workflows/pr-tests.yml`:

1. **unit-tests**: Go unit tests (no external dependencies)
2. **integration-tests**: No-auth integration tests with PostgreSQL
3. **integration-tests-mode2**: Mode 2 (internal IdP) integration tests
4. **frontend-tests**: Webapp build and tests
5. **js-sdk-tests**: TypeScript SDK tests
6. **go-lint**: golangci-lint
7. **buf-lint**: Protobuf linting
8. **buf-breaking**: Breaking change detection

**Note**: Mode 1 (Keycloak) integration tests and auth e2e tests require external infrastructure and do not run in CI/CD automatically.

### Running Tests Locally

```bash
# Unit tests (no dependencies)
make test-unit

# Integration tests (requires PostgreSQL)
make db-up                    # Start PostgreSQL via Docker Compose
make test-integration         # No-auth mode
make test-integration-mode2   # Mode 2 (internal IdP)

# E2E tests (requires gridapi + webapp running)
cd tests/e2e
pnpm install
pnpm test:e2e                 # No-auth
pnpm test:e2e:auth            # With Keycloak (requires setup)

# All tests
make test-all                 # Unit + integration (no auth)
```

## Technology Stack

### Backend
- **Language**: Go 1.24+ (workspace-based monorepo)
- **Database**: PostgreSQL with Bun ORM
- **RPC Framework**: Connect RPC (protobuf/gRPC-compatible)
- **Authentication**: Zitadel OIDC library (`github.com/zitadel/oidc`)
- **Authorization**: Casbin policy engine
- **Schema Validation**: `github.com/santhosh-tekuri/jsonschema/v6`
- **Label Filtering**: HashiCorp bexpr (boolean expression evaluator)
- **Migrations**: bun/migrate with embedded SQL
- **CLI Framework**: Cobra + Viper

### Frontend (WebApp)
- **Framework**: React 18 with TypeScript 5.x
- **RPC Client**: @connectrpc/connect-web
- **Build Tool**: Vite
- **Styling**: Tailwind CSS
- **Icons**: Lucide React
- **Graph Visualization**: React Flow
- **State Management**: React Context + localStorage/sessionStorage

### Testing
- **Go Testing**: testify/assert, testify/require
- **E2E Testing**: Playwright with TypeScript
- **Test Containers**: Docker Compose for PostgreSQL
- **Keycloak Integration**: External IdP for auth testing

### Protocols & Standards
- **Terraform HTTP Backend**: Official HashiCorp remote state protocol
- **OpenID Connect 1.0**: OAuth2 + OIDC flows (authorization code, client credentials, device flow)
- **JSON Schema Draft 2020-12**: Output validation schemas
- **Connect RPC**: Modern gRPC-compatible HTTP/1.1 and HTTP/2 protocol

### Infrastructure
- **Container Runtime**: Docker + Docker Compose
- **CI/CD**: GitHub Actions
- **Release Management**: release-please
- **Code Generation**: buf (protobuf), go generate

## Beads Usage

Beads is a issue tracking tool for Humans and AI that integrates with git. See [steveyegge/beads](https://github.com/steveyegge/beads).

### Git Hooks

Git Hooks for beads handle SQLite to JSONL flushing before commits to avoid dirty working trees. See [git-hooks/README.md](git-hooks/README.md) for details.
