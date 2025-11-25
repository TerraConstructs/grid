# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

**Note**: This project uses [bd (beads)](https://github.com/steveyegge/beads) for issue tracking. Use `bd` commands instead of markdown TODOs. See AGENTS.md for workflow details.

## Beads (Issue Tracking) Best Practices

### ⚠️ CRITICAL: Avoid Context Exhaustion with bd list

**NEVER run `bd list` without rigorous filtering or limits!** This will kill your context:

```bash
# ❌ DANGER: Even CLI can consume 5k-15k tokens without filters
bd list  # Lists ALL issues with full descriptions

# ✅ SAFE: Always use specific filters
bd list --status open --priority 1 --limit 5

# ✅ BETTER: Use targeted queries
bd ready --limit 5 # CLI: Find unblocked issues
bd show grid-xxxx  # CLI: View one issue
```

**Why this matters:**
- CLI output with full descriptions consumes significant tokens
- Always filter by: `status`, `priority`, `issue_type`, `assignee`, or `limit`

**Safe filtering patterns:**
```bash
# Find work
bd ready --limit 5          # ✅ Only unblocked issues (best for starting work)
bd list --status open -p 1  # ✅ High-priority open issues only

# Specific queries
bd show grid-xxxx           # ✅ One issue by ID
bd list --status in_progress # ✅ Only active tasks
bd list --issue-type task --limit 10  # ✅ Limited results

# Stats (no descriptions)
bd stats                    # ✅ Summary only
```

### MCP Tool Failure Handling
When `bd update` fails, **ALWAYS** fall back to `bd comments add`:

```bash
# ❌ If bd update fails with notes parameter
bd update "grid-xxxx" --notes "..."

# ✅ Immediately use comments as fallback
bd comments add grid-xxxx "Your context/notes here..."
```

**Lesson learned:** Never skip documentation just because the bd CLI failed. Comments are persistent and viewable with `bd show` or `bd comments`.

### Workflow Pattern
1. Create issue: `bd create` with full description
2. Add context: `bd comments add` (more reliable than notes field)
3. Update status: `bd update` for status/priority changes
4. Close issue: `bd close` with reason

### Common Commands
```bash
bd show grid-xxxx          # View issue details + comments
bd comments grid-xxxx      # View all comments
bd comments add grid-xxxx "..." # Add context (reliable fallback)
bd ready --limit 5          # Find issues ready to work on (SAFE)
bd list --status open -p 1 --limit 5 # List filtered issues (ALWAYS FILTER)
bd stats                   # Project statistics (no descriptions)
```

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

## Active Technologies
- TypeScript 5.x (webapp), React 18 (UI framework) + React, @connectrpc/connect-web (RPC client), Vite (build tool), Tailwind CSS (styling), Lucide React (icons) (007-webapp-auth)
- Browser localStorage/sessionStorage for session management, httpOnly cookies for auth tokens (managed by gridapi) (007-webapp-auth)
- YAML (GitHub Actions workflows), Go 1.24+ (existing project), Node.js 20+ (pnpm workspaces) (008-cicd-workflows)
- N/A (CI/CD infrastructure only) (008-cicd-workflows)
- Go 1.24+, TypeScript 5.x (webapp) (010-output-schema-support)
- PostgreSQL (existing), new columns in `state_outputs` table (010-output-schema-support)

## Recent Changes
- 007-webapp-auth: Added TypeScript 5.x (webapp), React 18 (UI framework) + React, @connectrpc/connect-web (RPC client), Vite (build tool), Tailwind CSS (styling), Lucide React (icons)
- 007-webapp-auth-refactor (2025-11-13): Refactored gridapi authentication architecture
  * Introduced IAM service layer with immutable group→role cache
  * Eliminated race condition in authentication middleware (was causing 30% test failure rate)
  * Implemented Authenticator pattern for pluggable authentication (JWT, Session)
  * Removed 26 layering violations (handlers/middleware now use services → repositories)
  * Performance: 67% faster (150ms → <50ms), 70% fewer DB queries (9 → 2-3 per request)
  * Zero lock contention (lock-free cache reads with atomic.Value)
  * All integration tests passing (32/32) with race detector clean
