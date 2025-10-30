# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

**Note**: This project uses [bd (beads)](https://github.com/steveyegge/beads) for issue tracking. Use `bd` commands instead of markdown TODOs. See AGENTS.md for workflow details.

## Project Overview

Grid is a Terraform/OpenTofu remote state management system consisting of:
- **gridapi**: HTTP server implementing Terraform HTTP backend protocol + Connect RPC service
- **gridctl**: CLI for managing states (create, list, init) and their dependencies
- **pkg/sdk**: Go SDK for programmatic access to Grid API
- **api**: Generated protobuf/Connect code from proto definitions

## Architecture

### Go Workspace Structure
This is a Go 1.24+ workspace-based monorepo. The workspace is defined in `go.work` with 5 modules:
- `./api` - Generated protobuf/Connect code
- `./cmd/gridapi` - API server binary
- `./cmd/gridctl` - CLI binary
- `./pkg/sdk` - SDK library
- `./tests` - Integration/contract tests

**Important**: When adding dependencies, run `go get` in the specific module directory, not at workspace root.

### State Management Model
States are identified by two IDs:
- **GUID**: Client-generated UUIDv7 (immutable, used in HTTP backend URLs)
- **logic-id**: User-provided string (mutable, human-readable identifier)

### Dual API Surface
The server exposes two APIs on the same port:
1. **Connect RPC** (`/state.v1.StateService/*`): Management operations (create, list, config)
2. **Terraform HTTP Backend** (`/tfstate/{guid}`, `/tfstate/{guid}/lock`, `/tfstate/{guid}/unlock`): State storage and locking per Terraform HTTP backend spec

### Database Layer
- **ORM**: Bun (lightweight, SQL-focused)
- **Migrations**: Embedded in `cmd/gridapi/internal/migrations/` using bun/migrate
- **Repository Pattern**: Interface in `internal/repository/interface.go`, Bun implementation in `internal/repository/bun_state_repository.go`
- **Model**: `internal/db/models/state.go` with embedded lock metadata as JSONB

### Code Generation
- **Protobuf**: Defined in `proto/state/v1/state.proto`
- **Generation**: `buf generate` produces Go (Connect + protobuf) and TypeScript (js/sdk/gen)
- **Always regenerate** after changing .proto files: `buf generate`

## Common Commands

### Building
```bash
make build              # Build both gridapi and gridctl to bin/
cd cmd/gridapi && go build -o ../../bin/gridapi .
cd cmd/gridctl && go build -o ../../bin/gridctl .
```

### Database
```bash
make db-up              # Start PostgreSQL via docker compose
make db-down            # Stop PostgreSQL
make db-reset           # Fresh database (removes volumes)

# Initialize migration tables
./bin/gridapi db init --db-url="postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"

# Run migrations manually
./bin/gridapi db migrate --db-url="postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"
```

### Running the Server
```bash
./bin/gridapi serve --server-addr :8080 --db-url "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"

# Or with environment variables
export DATABASE_URL="postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"
export SERVER_ADDR="localhost:8080"
./bin/gridapi serve
```

### CLI Usage
```bash
./bin/gridctl state -h         # Show state command help
```

### Testing
```bash
make test-unit          # Unit tests (no external dependencies)
make test-unit-db       # Repository tests (requires PostgreSQL)
make test-integration   # Integration tests (automated setup via TestMain)
make test-all           # All test suites
make test-clean         # Clean test artifacts

# Run specific test
cd cmd/gridapi/internal/repository && go test -v -run TestBunStateRepository_Create

# Run with race detector
go test -race ./...
```

### Code Generation
```bash
buf generate            # Generate Go + TypeScript from .proto files
```

## Development Patterns

### Adding a New RPC Method
1. Add method to `proto/state/v1/state.proto`
2. Run `buf generate`
3. Implement handler in `cmd/gridapi/internal/server/connect_handlers.go`
4. Add repository method in `internal/repository/interface.go` and `bun_state_repository.go`
5. Add service logic in `internal/state/service.go`
6. Write handler test in `connect_handlers_test.go`
7. Write repository test in `bun_state_repository_test.go`

### Database Schema Changes
1. Create migration file in `cmd/gridapi/internal/migrations/` following naming: `YYYYMMDDHHMMSS_description.go`
2. Implement `Up` and `Down` methods
3. Register in `migrations.Collection` in `main.go`
4. Test with `./bin/gridapi db migrate`

### Repository Testing Pattern
Repository tests use a real PostgreSQL database (not mocks). The test setup:
1. Connects to test database
2. Runs migrations
3. Uses `t.Cleanup()` to truncate tables after each test
4. See `cmd/gridapi/internal/repository/bun_state_repository_test.go` for examples

### Integration Testing Pattern
Integration tests in `tests/integration/`:
- Use `TestMain` to start gridapi server automatically
- Server runs in background with random port
- Each test gets isolated state via unique logic-ids
- See `tests/integration/main_test.go` for setup pattern

## File Organization

```
.
├── api/                    # Generated protobuf/Connect code (don't edit manually)
├── proto/                  # Protobuf definitions (source of truth)
├── cmd/
│   ├── gridapi/           # API server
│   │   ├── cmd/           # Cobra commands (serve, db)
│   │   ├── internal/
│   │   │   ├── config/    # Configuration loading
│   │   │   ├── db/        # Database models and provider
│   │   │   ├── migrations/# Schema migrations
│   │   │   ├── repository/# Data access layer
│   │   │   ├── server/    # HTTP handlers (Connect + Terraform)
│   │   │   └── state/     # Business logic service
│   └── gridctl/           # CLI
│       └── cmd/           # Cobra commands (state create/list/init)
├── pkg/sdk/               # Go SDK for Grid API
├── tests/
│   ├── contract/          # API contract tests (TODO)
│   └── integration/       # End-to-end tests
├── js/sdk/gen/            # Generated TypeScript SDK
└── specs/                 # Feature specifications and design docs
```
