# Research: Terraform State Management Framework

**Feature**: 001-develop-the-grid
**Date**: 2025-09-30
**Status**: Complete

## Research Questions

### 1. Terraform HTTP Backend Protocol Specification

**Decision**: Implement the official Terraform HTTP Backend protocol with GET, POST, LOCK, UNLOCK custom HTTP methods

**Rationale**:
- Well-documented protocol at https://developer.hashicorp.com/terraform/language/settings/backends/http
- Required endpoints with **custom HTTP methods**:
  - `GET /tfstate/{guid}` - Retrieve current state
  - `POST /tfstate/{guid}` - Update state (configurable via `update_method`, default POST)
  - **`LOCK /tfstate/{guid}/lock`** - Acquire lock (configurable via `lock_method`, default LOCK)
  - **`UNLOCK /tfstate/{guid}/unlock`** - Release lock (configurable via `unlock_method`, default UNLOCK)
- State locking uses separate lock info JSON with ID, operation, timestamp
- Compatible with both Terraform and OpenTofu CLIs

**Notes from Source Code Review**:
From `github.com/opentofu/opentofu/internal/backend/remote-state/http/backend.go`:
```go
"lock_method": &schema.Schema{
    DefaultFunc: schema.EnvDefaultFunc("TF_HTTP_LOCK_METHOD", "LOCK"),
},
"unlock_method": &schema.Schema{
    DefaultFunc: schema.EnvDefaultFunc("TF_HTTP_UNLOCK_METHOD", "UNLOCK"),
},
```

**Terraform by default uses LOCK and UNLOCK as custom HTTP methods, not PUT!**

**Method Configurability**:
- Our config generator can override methods in backend configuration:
  ```hcl
  backend "http" {
    address        = "..."
    lock_address   = "..."
    unlock_address = "..."
    lock_method    = "PUT"    # Override default LOCK
    unlock_method  = "DELETE" # Override default UNLOCK
    update_method  = "PATCH"  # Override default POST
  }
  ```
- Server may support custom methods for default Terraform behavior (less config generation, easier drop-in)
- Chi router supports custom HTTP methods via `r.Method("LOCK", "/path", handler)`

**Alternatives Considered**:
- S3-compatible API: Too complex, requires more endpoints
- Custom REST API: requires more Terraform backend config overrides
- File-based backend: Not suitable for remote collaboration
- Standard REST verbs only: Would require Terraform configuration override (which we control via our config generation)

**Implementation Notes** (Chi Router with Method Whitelist):
```go
// Method whitelist approach (from unit test analysis)
// Support common methods, reject arbitrary strings

// Lock endpoint - accept LOCK, PUT, POST
lockMethods := []string{"LOCK", "PUT", "POST"}
for _, method := range lockMethods {
    r.Method(method, "/tfstate/{guid}/lock", lockHandler)
}

// Unlock endpoint - accept UNLOCK, PUT, DELETE, POST
unlockMethods := []string{"UNLOCK", "PUT", "DELETE", "POST"}
for _, method := range unlockMethods {
    r.Method(method, "/tfstate/{guid}/unlock", unlockHandler)
}

// Update endpoint - accept POST, PUT, PATCH
updateMethods := []string{"POST", "PUT", "PATCH"}
for _, method := range updateMethods {
    r.Method(method, "/tfstate/{guid}", updateHandler)
}

// Unsupported methods return 405 Method Not Allowed
```

**Method Whitelist Rationale**:
- Terraform tests show methods are fully configurable (ANY string accepted)
- Test uses "BLAH", "BLIP", "BLOOP" to prove flexibility
- Grid implements whitelist: Support common methods, reject arbitrary
- Constitutional Principle VII: Simplicity - don't support every possible string
- If user needs unusual method, they can configure to supported alternative

**Locking is Optional**:
- Terraform allows omitting lock_address (no locking)
- Grid always provides lock/unlock addresses in backend config
- Users can configure Terraform to ignore locking if desired
- Lock/unlock addresses derived from state address: `{address}/lock`, `{address}/unlock`
- Terraform allows independent addresses, but Grid simplifies (lock belongs to state)

**OpenAPI Limitation**:
- OpenAPI 3.0 does not support custom HTTP methods
- Specification documents PUT as supported by our Chi router configuration
- Actual implementation should handle LOCK/UNLOCK methods

### 2. Bun ORM Migration Strategy

**Decision**: Use Bun's migration system with Go-based CreateTable (not raw SQL strings)

**Rationale**:
- Native PostgreSQL support with type-safe queries
- **Go migrations** (not SQL strings) for better IDE support and type safety
- Migration files in `cmd/gridapi/internal/migrations/`
- Version tracking via `bun_migrations` table
- CLI command pattern: `gridapi db migrate up` and `gridapi db migrate down`
- Supports transactional migrations for schema changes
- Bun's schema builder generates correct DDL for CreateTable

**Alternatives Considered**:
- Raw SQL migrations: Less type-safe, harder to refactor
- golang-migrate: Extra dependency, Bun provides built-in solution
- Manual SQL execution: No version tracking, error-prone
- ORM auto-migrations: Not recommended for production, loses control

**Implementation Notes**:
```go
// Migration file: cmd/gridapi/internal/migrations/20250930000001_create_states_table.go
package migrations

import (
  "context"
  "github.com/uptrace/bun"
  "github.com/uptrace/bun/migrate"
)

func init() {
  Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
    // Up migration
    _, err := db.NewCreateTable().
      Model((*State)(nil)).
      IfNotExists().
      Exec(ctx)
    return err
  }, func(ctx context.Context, db *bun.DB) error {
    // Down migration
    _, err := db.NewDropTable().
      Model((*State)(nil)).
      IfExists().
      Exec(ctx)
    return err
  })
}

// State model with Bun tags
type State struct {
  bun.BaseModel `bun:"table:states,alias:s"`
  GUID          string    `bun:"guid,pk,type:uuid"`
  LogicID       string    `bun:"logic_id,notnull,unique"`
  StateContent  []byte    `bun:"state_content,type:bytea"`
  Locked        bool      `bun:"locked,notnull,default:false"`
  LockInfo      []byte    `bun:"lock_info,type:jsonb"`
  CreatedAt     time.Time `bun:"created_at,notnull,default:current_timestamp"`
  UpdatedAt     time.Time `bun:"updated_at,notnull,default:current_timestamp"`
}
```

**No Postgres Extensions Required**:
- UUID type is built-in (since Postgres 8.3)
- JSONB type is built-in (since Postgres 9.4)
- Client generates UUIDs, no need for uuid-ossp or pgcrypto extensions

### 3. Repository & Service Layers Inside API Server

**Decision**: Define the persistence interfaces and Bun implementation under `cmd/gridapi/internal/repository`, consumed only by internal services in `cmd/gridapi/internal/state`.

**Rationale**:
- Upholds the Contract-Centric principle: SDKs stay limited to generated Connect clients while all business logic remains server-side.
- Keeps persistence concerns encapsulated so future storage changes do not leak across the public surface area.
- Enables focused unit testing by mocking the repository when exercising the internal service layer.

**Repository Interface (internal only)**:
```go
// cmd/gridapi/internal/repository/interface.go
type StateRepository interface {
  Create(ctx context.Context, state *State) error  // State includes client-generated GUID
  GetByGUID(ctx context.Context, guid string) (*State, error)
  GetByLogicID(ctx context.Context, logicID string) (*State, error)
  Update(ctx context.Context, state *State) error
  List(ctx context.Context) ([]State, error)
  Lock(ctx context.Context, guid string, lockInfo LockInfo) error
  Unlock(ctx context.Context, guid string, lockID string) error
}
```

**Internal Service Layer**:
- `cmd/gridapi/internal/state/service.go` orchestrates validation, repository access, and DTO conversion for Connect handlers.
- Service methods expose the operations required by the RPC layer (create, list, config lookup, lock orchestration).
- Tests (`service_test.go`) mock the repository to validate business rules (logic_id uniqueness, lock state transitions, size warnings).

**SDK Integration**:
- Go (`pkg/sdk`) and Node.js (`js/sdk`) packages talk exclusively to the Connect RPC endpoints; they never import repository or service packages.
- DTO serialization stays in SDK land, while persistence-specific structs (with Bun tags) remain in `cmd/gridapi/internal`.

**Alternatives Considered**:
- Placing the repository in `pkg/sdk`: rejected because it would re-introduce a dependency from server internals to SDK code.
- Exposing repository interfaces publicly: unnecessary for initial scope and risks future leakage of persistence details.
- Skipping the service layer: would force handlers to mix transport and validation logic, reducing testability.

### 4. Connect RPC Service Definition for State Management

**Decision**: Define StateService in proto/state/v1/state.proto with Create, List, GetConfig, GetStateLock, and UnlockState RPCs

**Rationale**:
- Follows constitutional requirement for Connect RPC
- Service methods:
  - `CreateState(CreateStateRequest) → CreateStateResponse` - Creates state, returns GUID + endpoints
  - `ListStates(ListStatesRequest) → ListStatesResponse` - Returns all states
  - `GetStateConfig(GetStateConfigRequest) → GetStateConfigResponse` - Get backend config for existing state
  - `GetStateLock(GetStateLockRequest) → GetStateLockResponse` - Inspect current lock metadata via SDK
  - `UnlockState(UnlockStateRequest) → UnlockStateResponse` - Unlock using lock ID when automation needs to recover
- Separate from Terraform HTTP Backend (which is REST for Terraform CLI consumption)
- Enables future SDK consumers (webapp, other tools) to manage states

**Alternatives Considered**:
- Single REST API for everything: Violates constitution IV (SDK-first via protobuf)
- gRPC only: Less HTTP/2 friendly, Connect provides better web compatibility
- No RPC layer: CLI would hit REST endpoints directly, violates SDK-first

**Implementation Notes**:
```protobuf
service StateService {
  rpc CreateState(CreateStateRequest) returns (CreateStateResponse);
  rpc ListStates(ListStatesRequest) returns (ListStatesResponse);
  rpc GetStateConfig(GetStateConfigRequest) returns (GetStateConfigResponse);
  rpc GetStateLock(GetStateLockRequest) returns (GetStateLockResponse);
  rpc UnlockState(UnlockStateRequest) returns (UnlockStateResponse);
}
```

### 5. CLI Template Embedding Strategy

**Decision**: Use Go embed package with .hcl.tmpl template in cmd/gridctl/internal/templates/

**Rationale**:
- Templates compiled into binary (no external file dependencies)
- Standard library solution (//go:embed directive)
- text/template for HCL generation
- Template receives: GUID, base_url, address, lock_address, unlock_address

**Alternatives Considered**:
- External template files: Requires distribution, version mismatch risk
- Hardcoded string formatting: Not maintainable, no validation
- HCL library generation: Over-engineering, template is simpler

**Implementation Notes**:
```go
//go:embed templates/*.hcl.tmpl
var templates embed.FS

func GenerateBackendConfig(guid, baseURL string) (string, error) {
  tmpl := template.Must(template.ParseFS(templates, "templates/backend.hcl.tmpl"))
  // Execute template with data
}
```

### 6. GUID Generation Strategy

**Decision**: Use google/uuid library for UUIDv7 (timestamp-ordered UUIDs)

**Rationale**:
- **Client-side generation** per specification: "The client CLI generates immutable GUID"
- UUIDv7 provides timestamp ordering for better Postgres index performance (B-tree locality)
- Cryptographically secure random component
- Industry standard (RFC 9562), universally unique
- Well-maintained library (google/uuid v1.6+)
- URL-safe format (hyphenated)
- Saves API roundtrip: CLI generates GUID, creates state in single request

**Alternatives Considered**:
- UUIDv4: Random only, poor index performance (random distribution causes page splits)
- ULIDs: Less standard, not native UUID type in Postgres
- Server-side generation: Violates spec, adds unnecessary roundtrip
- Sequential IDs: Not globally unique, exposes state count

**Implementation Notes**:
```go
import "github.com/google/uuid"

// In CLI (cmd/gridctl/cmd/state/create.go)
guid := uuid.Must(uuid.NewV7()).String() // e.g., "018e8c5e-7890-7000-8000-123456789abc"

// Send to API server in CreateStateRequest
req := &statev1.CreateStateRequest{
  Guid:    guid,
  LogicId: logicID,
}
```

**Postgres Storage**:
- Column type: `UUID` (native Postgres type, 16 bytes)
- No extensions required (uuid type is built-in since Postgres 8.3)
- Index performance: UUIDv7 timestamp prefix provides sequential writes

### 7. State Size Warning Implementation

**Decision**: Calculate content length on POST, log warning and include in response when >10MB

**Rationale**:
- Specification requires warning at 10MB threshold (FR-023)
- Non-blocking: accept all sizes, warn only
- Warning logged server-side
- Optional warning header in response: `X-Grid-State-Size-Warning: exceeds-threshold`
- CLI can optionally display warning to user

**Alternatives Considered**:
- Hard limit with rejection: Violates spec (accept all sizes)
- No warning: Doesn't help users identify performance issues
- Client-side checking: Server is source of truth

**Implementation Notes**:
```go
const StateSizeWarningThreshold = 10 * 1024 * 1024 // 10MB

if len(stateContent) > StateSizeWarningThreshold {
  log.Warn("State size exceeds threshold", "size", len(stateContent), "guid", guid)
  w.Header().Set("X-Grid-State-Size-Warning", "exceeds-threshold")
}
```

### 8. Cobra Command Structure

**Decision**:
- API Server: root → serve, db (with db migrate subcommands)
- CLI: root → state (with state create, list, init subcommands)

**Rationale**:
- Idiomatic Cobra pattern with command groups
- API server commands:
  - `gridapi serve` - Start HTTP server
  - `gridapi db migrate up` - Run migrations
  - `gridapi db migrate down` - Rollback migrations
  - `gridapi db migrate status` - Show migration status
- CLI commands:
  - `gridctl state create <logic-id>` - Create new state
  - `gridctl state list` - List all states
  - `gridctl state init <logic-id>` - Generate backend config in current directory

**Alternatives Considered**:
- Flat command structure: Less organized for future expansion
- Single serve command: Mixing concerns, harder to test
- Combined server/CLI binary: Violates separation of concerns

**Implementation Notes**:
- Each command in separate file
- Shared flags via persistent flags on root command
- API server base URL configurable via flag/env var

### 9. Testing Strategy for Terraform Integration

**Decision**: Integration tests using real Terraform CLI with test fixtures

**Rationale**:
- Constitution requires Terraform CLI compatibility tests
- Test approach:
  1. Start test API server with in-memory DB
  2. Use CLI to create state
  3. Write sample .tf file with null resources
  4. Execute `terraform init` → verify success
  5. Execute `terraform apply` → verify state stored
  6. Modify resources in .tf file
  7. Execute `terraform plan` → verify diff calculated
  8. Execute `terraform apply` → verify state updated
- Tests in tests/integration/terraform/
- Requires Terraform binary in CI environment

**Alternatives Considered**:
- Mock Terraform: Doesn't validate real protocol compatibility
- Manual testing only: Not repeatable, blocks CI/CD
- Unit tests only: Can't catch protocol issues

**Implementation Notes**:
```go
func TestTerraformIntegration(t *testing.T) {
  // Setup: start API server, create state via SDK
  // Execute: run terraform commands via exec.Command
  // Assert: verify state content in DB matches expected
}
```

### 10. Locking Mechanism: Column-Based vs PostgreSQL Advisory Locks

**Decision**: Use column-based locking (boolean + JSONB) via repository abstraction, not PostgreSQL advisory locks

**Column-Based Locking (Chosen)**:
```go
// In repository
func (r *BunRepository) Lock(ctx context.Context, guid string, lockInfo *LockInfo) error {
  result, err := r.db.NewUpdate().
    Model(&State{}).
    Set("locked = ?", true).
    Set("lock_info = ?", lockInfo).
    Set("updated_at = ?", time.Now()).
    Where("guid = ?", guid).
    Where("locked = ?", false).  // Atomic check-and-set
    Exec(ctx)

  if result.RowsAffected() == 0 {
    // Already locked, fetch existing lock info for 423 response
    return ErrAlreadyLocked
  }
  return err
}
```

**PostgreSQL Advisory Locks (Considered, Rejected for Initial Version)**:
```go
// Alternative approach (NOT implementing initially)
func (r *BunRepository) Lock(ctx context.Context, guid string, lockInfo *LockInfo) error {
  // Convert GUID to int64 hash for advisory lock
  lockID := hashGUIDToInt64(guid)

  var acquired bool
  err := r.db.NewRaw(`SELECT pg_try_advisory_lock(?)`, lockID).Scan(ctx, &acquired)
  if err != nil {
    return err
  }

  if !acquired {
    return ErrAlreadyLocked
  }

  // Still need to store lock_info for Terraform 423 responses
  _, err = r.db.NewUpdate().
    Model(&State{}).
    Set("lock_info = ?", lockInfo).
    Where("guid = ?", guid).
    Exec(ctx)

  return err
}

func (r *BunRepository) Unlock(ctx context.Context, guid string, lockID string) error {
  // Release advisory lock
  lockHash := hashGUIDToInt64(guid)
  r.db.NewRaw(`SELECT pg_advisory_unlock(?)`, lockHash).Exec(ctx)

  // Clear lock_info
  _, err := r.db.NewUpdate().
    Model(&State{}).
    Set("lock_info = ?", nil).
    Where("guid = ?", guid).
    Exec(ctx)

  return err
}
```

**Comparison**:

| Aspect | Column-Based (Chosen) | PostgreSQL Advisory Locks |
|--------|----------------------|--------------------------|
| **Database portability** | ✅ Works with SQLite, MySQL, any SQL DB | ❌ PostgreSQL-specific |
| **Lock info persistence** | ✅ Stored in table, survives crashes | ⚠️ Requires separate storage for 423 responses |
| **Automatic cleanup** | ❌ Manual (future: expiry mechanism) | ✅ Auto-released on connection close |
| **Performance** | ⚠️ Row update on lock/unlock | ✅ Kernel-level, no table updates |
| **Race conditions** | ✅ Atomic with WHERE locked=false | ✅ Atomic pg_try_advisory_lock |
| **Observability** | ✅ Query table to see all locks | ⚠️ Must query pg_locks system table |
| **Terraform compatibility** | ✅ Lock info in 423 response | ⚠️ Still need table storage for lock info |
| **Complexity** | ✅ Simple, standard SQL | ⚠️ Postgres-specific, requires hash function |

**Rationale for Column-Based Approach**:
1. **Database agnostic**: Repository pattern enables future SQLite dialect for single-machine setups (user's use case)
2. **Terraform HTTP Backend requirement**: Spec requires lock info JSON in 423 responses - must store lock_info regardless
3. **Observability**: Lock status visible via `SELECT * FROM states WHERE locked=true`
4. **Simplicity** (Constitution VII): Standard SQL, no database-specific features
5. **No proven pain**: Advisory locks are optimization, not requirement
6. **Repository abstraction**: Already justified for testing/mocking - lock mechanism detail is hidden

**Future Optimization Path**:
- If PostgreSQL-only deployment shows lock contention issues, can add pg_advisory_lock *behind existing repository interface* without SDK changes
- Hybrid approach: Advisory locks for atomicity + column storage for observability
- Lock expiry: Add `lock_expires_at` column with background cleanup job

**SQLite Compatibility Note**:
- SQLite has limited concurrency (write locks entire database)
- Column-based locking works but won't prevent SQLite-level write locks
- Still useful for single-machine, single-operator scenarios
- Future: SQLite repository implementation can use EXCLUSIVE transactions

**Repository Abstraction Benefit**:
```go
// Repository interface hides locking mechanism
type StateRepository interface {
  Lock(ctx context.Context, guid string, lockInfo *LockInfo) error
  Unlock(ctx context.Context, guid string, lockID string) error
}

// Can implement as:
// - BunPostgresRepository (column-based)
// - BunPostgresAdvisoryLockRepository (pg_advisory_lock)
// - BunSQLiteRepository (column-based with EXCLUSIVE transactions)
// All implementations remain within cmd/gridapi/internal/repository; SDKs stay unaware
// of persistence details and continue to use Connect RPC clients only.
```

### 11. Lock Conflict Error Response

**Decision**: Return HTTP 423 (Locked) with JSON lock info body, no retry logic server-side

**Rationale**:
- Terraform HTTP Backend spec uses 423 for lock conflicts
- Specification requires immediate return (FR-014a)
- Terraform CLI handles retry logic automatically
- Response body contains existing lock info:
```json
{
  "ID": "lock-uuid",
  "Operation": "apply",
  "Info": "",
  "Who": "user@host",
  "Version": "1.5.0",
  "Created": "2025-09-30T10:00:00Z",
  "Path": "production-us-east"
}
```

**Alternatives Considered**:
- Queueing with wait: Violates spec, adds complexity
- Custom retry logic: Terraform already implements exponential backoff
- Different status code: Would break Terraform compatibility

**Implementation Notes**:
- Store lock info in separate locks table or state table column
- Atomic check-and-set for lock acquisition
- Lock expiry handling (future enhancement, not in initial version)

### 12. Go Version and Tooling

**Decision**: Use Go 1.24 with mise for version management (Node.js 22 LTS for JS tooling)

**Rationale**:
- Project uses mise (see mise.toml in repo root) for tool version management
- Go 1.24.4 available via mise: `mise ls -l` shows go 1.24.4
- Node.js 22 LTS aligns with Connect tooling and matches constitution v2.0.0 guidance
- Compatible with all dependencies (Bun ORM, Chi, Cobra, Connect RPC, google/uuid v1.6+)

**Implementation Notes**:
```toml
# mise.toml (already exists)
[tools]
go = "1.24"
node = "22"
terraform = "1.10.5"
opentofu = "1.9.3"
```

**Module Configuration**:
```go
// go.mod (all modules)
module github.com/terraconstructs/grid/[module]

go 1.24

toolchain go1.24.4
```

### 13. Docker Compose for PostgreSQL

**Decision**: Use docker-compose.yml with postgres:17-alpine and persistent volumes

**Rationale**:
- Reproducible local development environment
- Persistent data via named volumes
- Port 5432 exposed on localhost for API server and migrations
- Alpine variant reduces image size (276MB vs 400MB+ for standard)
- postgres:17-alpine already available locally (docker image ls shows it)
- Compose v2 syntax (docker compose, not docker-compose)

**Implementation Notes**:
```yaml
# docker-compose.yml (repository root)
version: '3.8'

services:
  postgres:
    image: postgres:17-alpine
    container_name: grid-postgres
    environment:
      POSTGRES_USER: grid
      POSTGRES_PASSWORD: gridpass
      POSTGRES_DB: grid
    ports:
      - "5432:5432"
    volumes:
      - grid-postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-USERSHELL", "pg_isready", "-U", "grid"]
      interval: 5s
      timeout: 3s
      retries: 5

volumes:
  grid-postgres-data:
    driver: local
```

**Connection String** (for API server and migrations):
```
postgres://grid:gridpass@localhost:5432/grid?sslmode=disable
```

**Commands**:
- Start: `docker compose up -d`
- Stop: `docker compose down`
- Reset: `docker compose down -v` (deletes volumes)
- Logs: `docker compose logs -f postgres`

## Research Summary

All technical unknowns resolved. Implementation approach validated against:
- Terraform HTTP Backend protocol specification
- Constitutional requirements (Connect RPC + REST exception)
- Go best practices (embed, uuid v7, Cobra patterns)
- Bun ORM patterns (Go migrations with CreateTable, repository pattern)
- Testing strategy (unit, contract, integration with real Terraform)
- Go 1.24 toolchain via mise
- Docker Compose for reproducible Postgres environment
- Client-side UUIDv7 generation (per specification)
- Column-based locking (database-agnostic, supports future SQLite dialect)

**Key Architectural Decisions**:
1. **Client-side UUIDv7**: Saves roundtrip, optimal B-tree performance
2. **Column-based locking**: Portable across databases, repository abstraction enables future pg_advisory_lock optimization
3. **Repository pattern**: Justified for testing and multi-dialect support (Postgres, SQLite)
4. **Go migrations**: Type-safe CreateTable with Bun
5. **Docker Compose**: Reproducible Postgres 17-alpine with volumes

No blockers identified. Ready for Phase 1 design artifacts.
