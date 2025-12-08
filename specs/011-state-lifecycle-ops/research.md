# Research: State Lifecycle Operations

**Feature**: 011-state-lifecycle-ops
**Date**: 2025-12-08

## Prior Work Summary

### Related Specs

| Spec | Relevance |
|------|-----------|
| 001-develop-the-grid | Initial State model with GUID/logic_id dual-identifier design |
| 002-add-state-dependency | Dependency graph - impacts tombstone blocking logic |
| 006-authz-authn-rbac | Authorization framework - pattern for new lifecycle actions |
| 010-output-schema-support | Recent state extension pattern (added schema fields) |

### Existing Patterns Identified

1. **State Model Extension** (from 010-output-schema-support):
   - Add columns via migration
   - Extend `models.State` struct with Bun tags
   - Update repository interface and implementation
   - No breaking changes to existing code

2. **Authorization Actions** (from 006-authz-authn-rbac):
   - Actions defined as constants in `auth/actions.go`
   - Pattern: `{object}:{verb}` (e.g., `state:create`, `tfstate:lock`)
   - Wildcard support: `state:*` grants all state actions
   - `ValidateAction()` and `ExpandWildcard()` must be updated

3. **Service Layer Pattern** (from codebase exploration):
   - Services own business logic, repositories handle persistence
   - Builder pattern with `With*()` methods for optional dependencies
   - Error wrapping with context: `fmt.Errorf("operation: %w", err)`

4. **CLI Command Pattern** (from codebase exploration):
   - One file per subcommand in `cmd/gridctl/cmd/state/`
   - State reference resolution: flags > args > .grid context
   - 10-second timeout on all RPC calls

## Technical Decisions

### Decision 1: Lifecycle Status Storage

**Decision**: Add `status` column (VARCHAR) to states table rather than `deleted_at` timestamp pattern.

**Rationale**:
- Explicit status is clearer for querying (`WHERE status = 'active'`)
- Allows future status values if needed (e.g., 'archived', 'migrating')
- Matches existing `EdgeStatus` pattern in codebase
- `tombstoned_at` timestamp still stored for retention calculations

**Alternatives Rejected**:
- `deleted_at` NULL/non-NULL pattern: Less explicit, harder to extend
- Separate tombstone table: More complex, joins for every query

### Decision 2: Tombstone Blocking in HTTP Backend

**Decision**: Return HTTP 410 Gone for Terraform operations on tombstoned states.

**Rationale**:
- 410 indicates resource was intentionally removed (not just missing)
- Terraform will surface this clearly to users
- Distinguishes from 404 (never existed or purged)

**Alternatives Rejected**:
- 404 Not Found: Ambiguous, could mean typo in GUID
- 403 Forbidden: Implies permission issue, not lifecycle state

### Decision 3: Retention Period Storage

**Decision**: Store `retention_days` per-state with system default fallback.

**Rationale**:
- Allows per-state override for compliance requirements
- System default (30 days) covers common case
- Simple integer comparison for purge eligibility

**Alternatives Rejected**:
- Separate retention policy table: Over-engineered for current needs
- Environment variable only: No per-state flexibility

### Decision 4: Dependency Blocking Strategy

**Decision**: Block tombstone if active dependents exist; check in service layer.

**Rationale**:
- Service layer has access to EdgeRepository for dependency queries
- Clear error message can list blocking dependents
- Consistent with existing dependency cycle checks

**Alternatives Rejected**:
- Database constraint: Can't provide helpful error messages
- Allow tombstone with orphaned edges: Breaks dependency graph integrity

### Decision 5: Audit Trail Approach

**Decision**: Use existing `log.Printf` pattern for lifecycle operations; defer structured logging and audit table to ROADMAP.md.

**Rationale**:
- Codebase uses `log.Printf` extensively (72 occurrences in 15 files)
- Introducing `slog` just for lifecycle creates inconsistency
- Structured logging migration is already tracked in ROADMAP.md as separate work
- Dedicated audit table deferred to avoid scope creep

**Implementation**:
```go
// In service layer, log lifecycle operations (matches existing codebase pattern)
log.Printf("lifecycle: %s state %s (logic_id=%s) by %s",
    "tombstone", guid, logicID, principalID)
```

**Alternatives Rejected**:
- Use slog now: Creates inconsistency with rest of codebase
- New audit_events table now: Adds ~1 day work, deferred to ROADMAP.md
- External audit system: Out of scope for this feature

**Future Work**: See ROADMAP.md for:
1. "Logging Library" - structured logging migration (slog)
2. "OTEL Support" - log shipping to aggregation systems
3. "Compliance-Grade Audit Trail" - queryable audit_events table

### Decision 6: Concurrent Rename Handling

**Decision**: Optimistic locking using existing `updated_at` column check (no schema change).

**Rationale**:
- Uses existing `updated_at` column already present on State model
- Matches pattern used in other repositories (RowsAffected check)
- No schema migration required
- First write wins, subsequent writes fail with detectable error

**Implementation**:
```go
// In repository UpdateLogicID method
func (r *BunStateRepository) UpdateLogicID(ctx context.Context, guid, newLogicID string, originalUpdatedAt time.Time) error {
    now := time.Now()
    result, err := r.db.NewUpdate().
        Model((*models.State)(nil)).
        Set("logic_id = ?", newLogicID).
        Set("updated_at = ?", now).
        Where("guid = ?", guid).
        Where("updated_at = ?", originalUpdatedAt). // Optimistic lock
        Exec(ctx)
    if err != nil {
        return fmt.Errorf("rename state: %w", err)
    }
    rows, _ := result.RowsAffected()
    if rows == 0 {
        return fmt.Errorf("concurrent modification detected or state not found")
    }
    return nil
}
```

**Existing Pattern Reference**: See `bun_role_repository.go:75-97` and `bun_label_policy_repository.go:41-92` for similar RowsAffected patterns.

### Decision 7: Logic ID Uniqueness Scope

**Decision**: Logic IDs unique across active AND tombstoned states.

**Rationale**:
- Prevents confusion when referencing by logic_id
- User must explicitly purge tombstoned state to reuse name
- Matches spec requirement FR-004

**Implementation**:
- Existing unique constraint on `logic_id` column remains
- Rename to tombstoned name fails at DB level
- Service layer provides clear error message

## Integration Points

### Authorization Interceptor Updates

New cases in `authz_interceptor.go` switch statement:

```go
case statev1connect.StateServiceRenameStateProcedure:
    obj = auth.ObjectTypeState
    action = auth.StateRename
    // Load state labels for dynamic check

case statev1connect.StateServiceTombstoneStateProcedure:
    obj = auth.ObjectTypeState
    action = auth.StateTombstone
    // Load state labels for dynamic check

case statev1connect.StateServiceRestoreStateProcedure:
    obj = auth.ObjectTypeState
    action = auth.StateRestore
    // Load state labels for dynamic check (tombstoned states still have labels)

case statev1connect.StateServicePurgeStateProcedure:
    obj = auth.ObjectTypeState
    action = auth.StatePurge
    // Load state labels for dynamic check
```

### HTTP Backend Middleware Updates

In `authz.go`, add tombstone check before authorization:

```go
// Early exit for tombstoned states
if state.Status == models.StateStatusTombstoned {
    http.Error(w, "state has been deleted", http.StatusGone)
    return
}
```

### Service Layer Operations

New methods in `StateService`:

| Method | Description | Checks |
|--------|-------------|--------|
| `RenameState(ctx, guid, newLogicID)` | Update logic_id | Not locked, not tombstoned, target unique |
| `TombstoneState(ctx, guid, principalID)` | Set status=tombstoned | Not locked, no active dependents |
| `RestoreState(ctx, guid)` | Set status=active | Is tombstoned, within retention |
| `PurgeState(ctx, guid, force)` | Hard delete | Is tombstoned, past retention (or force) |

### Repository Layer Operations

New methods in `StateRepository`:

| Method | Description |
|--------|-------------|
| `UpdateLogicID(ctx, guid, newLogicID)` | Atomic rename with optimistic lock |
| `SetTombstoned(ctx, guid, principalID)` | Set tombstone fields |
| `ClearTombstone(ctx, guid)` | Clear tombstone fields |
| `Delete(ctx, guid)` | Hard delete (existing, but verify cascade) |
| `ListWithStatus(ctx, status, filter)` | Filter by lifecycle status |
| `HasActiveDependents(ctx, guid)` | Check for blocking edges |

### Decision 8: Purge Data Cleanup Scope

**Decision**: Purge deletes all state-related data via CASCADE DELETE.

Side note: Application logic (service layer) should ensure no edges or outputs remain before purge.

**Data Deleted on Purge**:
| Table | FK Relationship | Cascade Behavior |
|-------|-----------------|------------------|
| `states` | (primary) | Deleted row |
| `state_outputs` | `state_guid → states(guid)` | ON DELETE CASCADE |
| `edges` (outgoing) | `from_state → states(guid)` | ON DELETE CASCADE |
| `edges` (incoming) | `to_state → states(guid)` | ON DELETE CASCADE |

**What is NOT deleted**:
- `label_policy` - Global config, no FK to states
- User/role/session data - No FK to states

**Implementation**: No custom cleanup code needed - database CASCADE handles everything.

**Verification**: The existing migration (`20251203000000_init_schema.go`) already defines:
```go
// edges table
"FOREIGN KEY (from_state) REFERENCES states(guid) ON DELETE CASCADE"
"FOREIGN KEY (to_state) REFERENCES states(guid) ON DELETE CASCADE"

// state_outputs table
"FOREIGN KEY (state_guid) REFERENCES states(guid) ON DELETE CASCADE"
```

### Decision 9: ListStates Proto Change

**Decision**: Add `optional bool include_tombstoned = 4` field to `ListStatesRequest`.

**Rationale**:
- Backward compatible - old clients default to `false` (active only)
- Simple boolean is clearer than enum for this use case
- Matches existing optional field pattern in ListStatesRequest

**Proto Change**:
```protobuf
message ListStatesRequest {
  optional string filter = 1;
  optional bool include_labels = 2;
  optional bool include_status = 3;
  optional bool include_tombstoned = 4;  // NEW: Include soft-deleted states
}
```

### Decision 10: Test Retention Period

**Decision**: Use 30-second retention period for integration tests.

**Rationale**:
- Fast enough for development iteration
- Reliable enough for CI environments
- Allows testing both "within retention" and "past retention" scenarios

**Test Configuration**:
```go
// In test setup, override default retention
const testRetentionSeconds = 30

// Test "within retention" - immediately after tombstone
// Test "past retention" - sleep 31 seconds, then verify purge allowed
```

### Decision 11: Role Permissions for Lifecycle Actions

**Decision**: Lifecycle actions restricted to **platform-engineer role only**.

**Rationale**:
- Rename, tombstone, restore, purge are potentially destructive operations
- platform-engineer already has wildcard (`*:*`) so no new policy rules needed
- product-engineer should not be able to delete states (even in their scope)
- service-account should not perform lifecycle operations (automation risk)

**Implementation**:
- No new Casbin policy rules required
- platform-engineer wildcard automatically covers new actions
- Authorization tests verify product-engineer/service-account are denied

### Decision 12: Time-Based Test Strategy

**Decision**: Skip time-based retention tests in CI; use `slow` build tag.

**Rationale**:
- Tests requiring 30-second waits slow down CI significantly
- Use Go build tags to separate fast vs slow tests
- CI runs fast tests only; manual/nightly runs include slow tests

**Implementation**:
```go
//go:build slow

func TestRestoreState_PastRetention(t *testing.T) { ... }
func TestPurgeState_HappyPath(t *testing.T) { ... }
```

**Execution**:
```bash
# CI (fast only - default)
make test-integration

# Manual/nightly (include slow)
go test -tags=slow ./tests/integration/...
```

## Open Questions (Resolved)

| Question | Resolution |
|----------|------------|
| Rename to tombstoned name? | Block - user must purge first |
| Tombstone with dependents? | Block - dependents must be tombstoned first |
| Add dependency to tombstoned? | Block - service layer rejects |
| Stale config after rename? | Clear 404 error - no hints |
| Concurrent renames? | Optimistic locking using updated_at - first wins |
| What does purge delete? | State row + CASCADE to outputs and edges |
| TF apply on tombstoned? | HTTP 410 Gone response |
| List API change? | Add `include_tombstoned` optional bool field |
| Test retention period? | 30 seconds for integration tests |
| Role permissions? | platform-engineer only (Decision 11) |
| Slow test handling? | Build tag `slow`, skip in CI (Decision 12) |

## Testing Strategy

### Integration Test Approach

Based on existing patterns in `tests/integration/`:

1. **Server Management**: TestMain starts gridapi, tests use SDK/CLI (prefer CLI where possible, fall back to SDK if not possible through CLI, avoid direct http/rpc endpoint calls)
2. **Isolation**: Each test uses unique logic_id (UUID suffix)
3. **No DB Truncation**: Tests are independent via unique IDs
4. **Dual Database**: Run with both PostgreSQL (`make test-integration`) and SQLite (`make test-integration-sqlite`)

### Lifecycle Test Scenarios

| Test | Description | Retention |
|------|-------------|-----------|
| `TestRenameState_HappyPath` | Create, rename, verify GUID unchanged | N/A |
| `TestRenameState_Conflict` | Rename to existing name | N/A |
| `TestRenameState_Locked` | Rename locked state fails | N/A |
| `TestRenameState_Tombstoned` | Rename tombstoned state fails | N/A |
| `TestTombstoneState_HappyPath` | Tombstone, verify listing hidden | N/A |
| `TestTombstoneState_WithDependents` | Tombstone with dependents fails | N/A |
| `TestTombstoneState_TerraformBlocked` | Terraform plan returns 410 | N/A |
| `TestRestoreState_HappyPath` | Tombstone, restore, verify active | N/A |
| `TestRestoreState_PastRetention` | Restore expired state fails | 30s |
| `TestPurgeState_HappyPath` | Tombstone, wait, purge | 30s |
| `TestPurgeState_WithinRetention` | Purge within retention fails | 30s |
| `TestPurgeState_Force` | Force purge within retention | 30s |
| `TestPurgeState_Active` | Purge active state fails | N/A |
| `TestAddDependency_ToTombstoned` | Add dep to tombstoned fails | N/A |

### Test Execution

```bash
# PostgreSQL (default)
make test-integration  # Includes lifecycle tests

# SQLite
make test-integration-sqlite  # Same tests, different DB

# Run specific lifecycle tests
cd tests/integration && go test -v -run "TestLifecycle"
```

## References

- Terraform HTTP Backend Spec: https://developer.hashicorp.com/terraform/language/settings/backends/http
- Bun ORM Documentation: https://bun.uptrace.dev/
- Connect RPC Testing: https://connectrpc.com/docs/web/testing
