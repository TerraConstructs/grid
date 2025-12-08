# Implementation Plan: State Lifecycle Operations

**Branch**: `011-state-lifecycle-ops` | **Date**: 2025-12-08 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/011-state-lifecycle-ops/spec.md`

## Summary

Add day-2 lifecycle operations for Terraform states: **rename** (change logic_id), **tombstone** (soft delete with retention), **restore** (recover tombstoned), and **purge** (permanent delete). These operations extend the existing State model with lifecycle status tracking and integrate with the existing Casbin-based RBAC authorization system using new action constants.

## Technical Context

**Language/Version**: Go 1.24+
**Primary Dependencies**: Bun ORM, Connect RPC, Casbin (RBAC), bexpr (label filtering)
**Storage**: PostgreSQL (primary), SQLite (fallback)
**Testing**: go test, table-driven tests, integration tests with real DB
**Target Platform**: Linux server (gridapi), macOS/Linux CLI (gridctl)
**Project Type**: monorepo (Go workspace with multiple modules)
**Performance Goals**: Operations complete in <5 seconds, state listings remain fast with tombstone filtering
**Constraints**: Must preserve GUID immutability, maintain backward compatibility with Terraform HTTP backend
**Scale/Scope**: Existing infrastructure supports thousands of states

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Go Workspace Architecture | PASS | Changes span gridapi, gridctl, pkg/sdk - all existing modules |
| II. Contract-Centric SDKs | PASS | New RPCs defined in proto first, SDK wraps Connect clients |
| III. Dependency Flow Discipline | PASS | gridapi → internal services → repositories; gridctl → pkg/sdk |
| IV. Cross-Language Parity | PASS | Proto-first design, Go SDK implementation |
| V. Test Strategy | PASS | Contract tests + integration tests planned |
| VI. Versioning & Releases | PASS | Non-breaking changes (additive proto fields/RPCs) |
| VII. Simplicity & Pragmatism | PASS | Extending existing patterns, no new abstractions |
| VIII. Service Exposure Discipline | PASS | New RPCs go through existing authz interceptor |
| IX. API Server Internal Layering | PASS | Handlers → StateService → StateRepository pattern |

**Gate Result**: PASS - No constitution violations identified.

## Project Structure

### Documentation (this feature)

```
specs/011-state-lifecycle-ops/
├── spec.md              # Feature specification (complete)
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (proto additions)
├── checklists/          # Quality checklists
│   └── requirements.md  # Spec quality checklist (complete)
└── tasks.md             # Phase 2 output (via /speckit.tasks)
```

### Source Code (repository root)

```
# API Server (gridapi)
cmd/gridapi/
├── internal/
│   ├── db/models/
│   │   └── state.go           # MODIFY: Add lifecycle status fields
│   ├── migrations/
│   │   └── YYYYMMDD_lifecycle.go  # NEW: Add lifecycle columns (carefully handle SQLite limitations)
│   ├── repository/
│   │   ├── interface.go       # MODIFY: Add lifecycle methods
│   │   └── bun_state_repository.go  # MODIFY: Implement lifecycle queries
│   ├── services/state/
│   │   └── service.go         # MODIFY: Add lifecycle operations
│   ├── server/
│   │   └── connect_handlers.go  # MODIFY: Add lifecycle handlers
│   ├── middleware/
│   │   ├── authz_interceptor.go  # MODIFY: Add lifecycle action checks
│   │   └── authz.go           # MODIFY: Block tombstoned states in HTTP backend
│   └── auth/
│       └── actions.go         # MODIFY: Add lifecycle action constants

# Proto Definitions
proto/state/v1/
└── state.proto                # MODIFY: Add lifecycle RPCs and messages

# Generated Code
pkg/api/state/v1/
├── state.pb.go               # REGENERATE
└── statev1connect/           # REGENERATE

# Go SDK
pkg/sdk/
└── state_client.go           # MODIFY: Add lifecycle methods wrappers

# CLI
cmd/gridctl/cmd/state/
├── rename.go                 # NEW: Rename command
├── tombstone.go              # NEW: Tombstone command
├── restore.go                # NEW: Restore command
└── purge.go                  # NEW: Purge command

# Tests
tests/integration/
└── lifecycle_test.go         # NEW: Lifecycle integration tests
```

**Structure Decision**: Extends existing monorepo structure. No new modules required - all changes fit within existing cmd/gridapi, pkg/sdk, cmd/gridctl modules.

## Complexity Tracking

*No constitution violations requiring justification.*

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| N/A | N/A | N/A |

## Authorization Design

### New Action Constants (cmd/gridapi/internal/auth/actions.go)

Based on existing action taxonomy pattern:

```go
// State Lifecycle Actions
const (
    StateRename    = "state:rename"     // Rename logic_id
    StateTombstone = "state:tombstone"  // Soft delete
    StateRestore   = "state:restore"    // Recover from tombstone
    StatePurge     = "state:purge"      // Permanent delete
)
```

### Authorization Flow

All lifecycle operations follow the existing two-phase pattern:
1. **Authentication** (middleware): Extract principal from JWT/session
2. **Authorization** (interceptor): Check action permission against state labels

The authz interceptor will load state labels for dynamic checks, matching the existing pattern for `GetStateConfig`, `UpdateStateLabels`, etc.

### Seed Policy Updates

Add lifecycle actions to roles in migration seed data:

```go
// In 20251203000000_init_schema.go or new migration
// platform-engineer already has wildcard: {Ptype: "p", V0: "role:platform-engineer", V1: "*", V2: "*", V4: "allow"}
// This covers state:rename, state:tombstone, state:restore, state:purge automatically

// product-engineer gets lifecycle actions SCOPED to their label permissions
// Add explicit policies for lifecycle actions:
{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "state:rename", V3: "env == 'dev'", V4: "allow"},
{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "state:tombstone", V3: "env == 'dev'", V4: "allow"},
{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "state:restore", V3: "env == 'dev'", V4: "allow"},
{Ptype: "p", V0: "role:product-engineer", V1: "state", V2: "state:purge", V3: "env == 'dev'", V4: "allow"},

// service-account does NOT get lifecycle actions (automation shouldn't rename/delete)
```

**Rationale**: Product engineers can manage lifecycle of states within their scope (determined by label-based access control). Platform engineers have unrestricted access. Service accounts are blocked from destructive operations.

### Tombstoned State Handling

- **Connect RPC**: New lifecycle RPCs check tombstone status in service layer
- **Terraform HTTP Backend**: Middleware checks tombstone status via `StateService.GetStateByGUID()` (Constitution Principle IX compliant - no direct repository access) and returns 410 Gone with descriptive JSON error body for tombstoned states
- **Rename**: Tombstoned states CAN be renamed (to free up logic_id without purging). Active states CANNOT be renamed to a tombstoned logic_id.
- **Dependencies**: Cannot add dependencies to tombstoned states

## Data Model Changes

### State Table Additions

| Column | Type | Default | Description |
|--------|------|---------|-------------|
| `status` | VARCHAR(20) | 'active' | Lifecycle status: 'active' or 'tombstoned' |
| `tombstoned_at` | TIMESTAMP | NULL | When state was tombstoned |
| `tombstoned_by` | VARCHAR(255) | NULL | Principal ID who tombstoned |
| `retention_days` | INT | 30 | Days before purge eligible (captured from global config at tombstone time) |

**Note**: `retention_days` is set from global server config (`--retention-days` flag) when tombstoning. No per-state override API.

### Index Additions

- `idx_states_status` on `status` column for efficient filtering
- Composite index for tombstone queries: `(status, tombstoned_at)`

## API Additions (Proto)

### New RPCs

```protobuf
// Lifecycle Operations
rpc RenameState(RenameStateRequest) returns (RenameStateResponse);
rpc TombstoneState(TombstoneStateRequest) returns (TombstoneStateResponse);
rpc RestoreState(RestoreStateRequest) returns (RestoreStateResponse);
rpc PurgeState(PurgeStateRequest) returns (PurgeStateResponse);
```

### Modified RPCs

- `ListStatesRequest`: Add `include_tombstoned` filter option
- `ListStatesResponse`: States include lifecycle status indicator

## CLI Commands

| Command | Description | Flags |
|---------|-------------|-------|
| `gridctl state rename <new-logic-id>` | Rename state logic_id | `--logic-id`, `--guid` (for explicit ref) |
| `gridctl state delete` | Soft delete (tombstone) state | `--logic-id`, `--guid`, `--purge`, `--force` |
| `gridctl state restore` | Restore tombstoned state | `--logic-id`, `--guid` |
| `gridctl state list` | List states | `--all` (include tombstoned) |

**CLI Design Notes**:
- `delete` without flags = tombstone (soft delete)
- `delete --purge` = permanent delete (requires tombstoned state)
- `delete --purge --force` = permanent delete within retention period
- Uses dirCtx (`.grid` context) by default, explicit `--logic-id` or `--guid` when needed

## Testing Strategy

### Test Layers

| Layer | Scope | Database | Pattern |
|-------|-------|----------|---------|
| Repository | CRUD operations, optimistic locking | Real DB | `bun_state_repository_test.go` |
| Service | Business logic, validation rules | Mocked repos | `service_test.go` |
| Integration | Full flow CLI → SDK → API → DB | Real DB | `tests/integration/lifecycle_test.go` |
| Authorization | Permission checks per action | Real DB | Embedded in integration tests |

### Integration Test Configuration

```go
// Test retention period: 30 seconds for fast iteration
const testRetentionSeconds = 30

// Test scenarios requiring retention timing:
// 1. "within retention" - immediately after tombstone
// 2. "past retention" - sleep 31 seconds, then verify
```

### Time-Based Test Strategy

Tests that require waiting for retention period are **excluded from CI** using build tags:

```go
//go:build slow
// +build slow

func TestRestoreState_PastRetention(t *testing.T) {
    // This test requires 30+ second wait
    // Run manually: go test -tags=slow -v -run TestRestoreState_PastRetention
}
```

**Test Categories**:
| Category | Build Tag | CI Behavior | When to Run |
|----------|-----------|-------------|-------------|
| Fast tests | (default) | Always run | Every PR |
| Time-based tests | `slow` | Skipped | Manual/nightly |

**Execution Commands**:
```bash
# CI (fast tests only - default)
make test-integration

# Manual (include slow tests)
go test -tags=slow ./tests/integration/... -v -timeout 5m

# Nightly job (all tests)
make test-integration-slow
```

### Dual Database Testing

Both PostgreSQL and SQLite must pass all tests:

```bash
# PostgreSQL (default)
make test-integration

# SQLite
make test-integration-sqlite
```

### Test Scenarios

| Test | Description | Build Tag | Retention |
|------|-------------|-----------|-----------|
| `TestRenameState_HappyPath` | Create, rename, verify GUID unchanged | default | N/A |
| `TestRenameState_Conflict` | Rename to existing name fails | default | N/A |
| `TestRenameState_Locked` | Rename locked state fails | default | N/A |
| `TestRenameState_Tombstoned` | Rename tombstoned state fails | default | N/A |
| `TestRenameState_Concurrent` | Optimistic lock conflict detection | default | N/A |
| `TestTombstoneState_HappyPath` | Tombstone, verify hidden from listings | default | N/A |
| `TestTombstoneState_WithDependents` | Tombstone with dependents fails | default | N/A |
| `TestTombstoneState_TerraformBlocked` | Terraform plan returns 410 | default | N/A |
| `TestRestoreState_HappyPath` | Tombstone, restore, verify active | default | N/A |
| `TestRestoreState_PastRetention` | Restore expired state fails | **slow** | 30s wait |
| `TestPurgeState_HappyPath` | Tombstone, wait, purge | **slow** | 30s wait |
| `TestPurgeState_WithinRetention` | Purge within retention fails | default | Immediate |
| `TestPurgeState_Force` | Force purge within retention | default | Immediate |
| `TestPurgeState_Active` | Purge active state fails | default | N/A |
| `TestAddDependency_ToTombstoned` | Add dep to tombstoned fails | default | N/A |

### Test Execution

```bash
# Run all lifecycle tests
cd tests/integration && go test -v -run "TestLifecycle"

# Run specific test with verbose output
go test -v -run "TestPurgeState_Force" -timeout 120s
```

### Authorization Test Matrix

Each lifecycle action requires authorization testing. Product engineers can perform lifecycle operations on states within their label scope.

| Action | Role: platform-engineer | Role: product-engineer (scoped) | Role: service-account |
|--------|------------------------|--------------------------------|----------------------|
| `state:rename` | ✅ Allow | ✅ Allow (in scope) | ❌ Deny |
| `state:tombstone` | ✅ Allow | ✅ Allow (in scope) | ❌ Deny |
| `state:restore` | ✅ Allow | ✅ Allow (in scope) | ❌ Deny |
| `state:purge` | ✅ Allow | ✅ Allow (in scope) | ❌ Deny |

**Scope enforcement**: Product engineers' label-based policies determine which states they can manage. Out-of-scope operations return PERMISSION_DENIED.
