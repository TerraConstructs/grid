# Feature Specification: State Lifecycle Operations

**Feature Branch**: `011-state-lifecycle-ops`
**Created**: 2025-12-08
**Status**: Draft
**Input**: User description: "We need to add support for day 2 operations on terraform states such as renaming logic identity of a state as well as deleting (tombstone) and purging tombstoned states"

## Terminology

- **Authorized user**: A user who has been granted permission to perform a specific action (rename, tombstone, restore, purge) on states within their permitted scope. Access is determined by policy rules that may restrict which states a user can manage based on labels, ownership, or other criteria.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Rename State Logic ID (Priority: P1)

As an authorized user, I need to rename the logic ID of an existing Terraform state (within my permitted scope) to reflect organizational changes (project renames, team restructures, naming convention updates) without disrupting active Terraform workflows or losing state history.

**Why this priority**: Renaming is the most common day-2 operation and directly impacts operational workflows. Teams frequently need to reorganize states as projects evolve, and this operation should be non-disruptive.

**Independent Test**: Can be fully tested by creating a state with one logic ID, renaming it, and verifying that both the new name works and that Terraform operations continue to function using the same GUID-based backend URL.

**Acceptance Scenarios**:

1. **Given** an existing state with logic_id "project-a/dev" within the user's permitted scope, **When** an authorized user renames it to "project-b/dev", **Then** the state is accessible by the new logic_id and all existing Terraform configurations continue to work (GUID unchanged).

2. **Given** an existing state with logic_id "old-name", **When** an authorized user attempts to rename it to a logic_id that already exists (active), **Then** the system rejects the rename with a clear error message indicating the conflict.

3. **Given** an existing state with logic_id "old-name", **When** an authorized user attempts to rename it to a logic_id that is currently tombstoned, **Then** the system rejects the rename and indicates the target name is tombstoned (must be purged first).

4. **Given** a state that is currently locked by a Terraform operation, **When** an authorized user attempts to rename it, **Then** the system rejects the rename and indicates the state is locked.

---

### User Story 2 - Soft Delete (Tombstone) State (Priority: P2)

As an authorized user, I need to mark a Terraform state (within my permitted scope) as deleted (tombstoned) so that it no longer appears in normal listings and cannot be used for new operations, while preserving the data for audit, recovery, or compliance purposes.

**Why this priority**: Soft delete is essential for safe state removal. Accidental deletions of Terraform states can be catastrophic, so a recoverable delete mechanism is critical before supporting permanent deletion.

**Independent Test**: Can be fully tested by creating a state, tombstoning it, verifying it no longer appears in default listings, and confirming Terraform operations against it are rejected with an appropriate error.

**Acceptance Scenarios**:

1. **Given** an existing active state within the user's permitted scope, **When** an authorized user tombstones it, **Then** the state is marked as deleted, no longer appears in default state listings, and Terraform operations (lock, unlock, get, update) are rejected with "state has been deleted" error.

2. **Given** a tombstoned state within the user's permitted scope, **When** an authorized user lists states with "include deleted" filter, **Then** the tombstoned state appears with a clear deleted indicator.

3. **Given** a tombstoned state within the recovery window, **When** an authorized user restores it, **Then** the state becomes active again and Terraform operations resume normally.

4. **Given** a state that is currently locked, **When** an authorized user attempts to tombstone it, **Then** the system rejects the operation and indicates the state must be unlocked first.

5. **Given** a state "network/vpc" that has active dependents "app/frontend" and "app/backend", **When** an authorized user attempts to tombstone "network/vpc", **Then** the system rejects the operation and lists the dependent states that must be tombstoned first.

---

### User Story 3 - Purge Tombstoned State (Priority: P3)

As an authorized user, I need to permanently delete tombstoned states (within my permitted scope) after the retention period to free up resources and ensure compliance with data retention policies.

**Why this priority**: Purge is a destructive, irreversible operation that should only be available after soft delete. It's necessary for compliance and resource management but is less frequently used than rename or soft delete.

**Independent Test**: Can be fully tested by tombstoning a state, waiting for/bypassing the retention period, purging it, and verifying the state data is completely removed and cannot be recovered.

**Acceptance Scenarios**:

1. **Given** a tombstoned state within the user's permitted scope that has exceeded the retention period, **When** an authorized user purges it, **Then** the state and all associated data (versions, locks, metadata) are permanently removed.

2. **Given** a tombstoned state within the retention period, **When** an authorized user attempts to purge it, **Then** the system rejects the purge with a message indicating when the state becomes eligible for purge.

3. **Given** a tombstoned state within the retention period, **When** an authorized user force-purges with explicit confirmation, **Then** the state is permanently removed (with audit trail).

4. **Given** an active (non-tombstoned) state, **When** an authorized user attempts to purge it, **Then** the system rejects the operation and requires tombstoning first.

---

### Edge Cases (Resolved)

| Edge Case | Resolution |
|-----------|------------|
| Renaming an active state to a tombstoned logic_id | **Block**: Rename rejected if target name is tombstoned. User must purge the tombstoned state first, rename the tombstoned state, or choose a different name. |
| Renaming a tombstoned state | **Allow**: Tombstoned states can be renamed to free up their logic_id for reuse without waiting for purge. |
| Concurrent rename operations | **Optimistic locking**: First operation wins, subsequent attempts fail with ABORTED error. |
| Tombstoning a state with dependents | **Block**: Cannot tombstone a state if other active states depend on it. Dependents must be tombstoned first. |
| Purge with dependency links | **N/A**: Since states cannot be tombstoned with dependents, purge will never encounter dependency links. |
| Adding dependencies to tombstoned states | **Block**: Cannot add a tombstoned state as a dependency. |
| Stale Terraform config after rename | **Clear error**: Logic_id lookup returns NOT_FOUND - users update config manually. GUID-based backend URLs continue working. |
| Force purge scope | **Tombstoned only**: Force flag bypasses retention period check, but state MUST still be tombstoned first. Cannot force-purge active states. |

## Requirements *(mandatory)*

### Functional Requirements

#### Authorization
- **FR-001**: System MUST enforce permission checks for each lifecycle action (rename, tombstone, restore, purge) based on the user's granted permissions and scope restrictions.
- **FR-002**: System MUST reject operations on states outside the user's permitted scope with an appropriate authorization error.

#### Rename Operations
- **FR-003**: System MUST allow authorized users to rename the logic_id of an existing state (active or tombstoned) while preserving its GUID and all historical data.
- **FR-004**: System MUST reject rename operations when the target logic_id already exists (active or tombstoned).
- **FR-004a**: System MUST allow renaming tombstoned states (to free up logic_id for reuse without purging).
- **FR-005**: System MUST reject rename operations on locked states (active states only; tombstoned states cannot be locked).
- **FR-006**: System MUST use optimistic locking for rename operations to handle concurrent requests (first wins, others fail with conflict).

#### Tombstone (Soft Delete) Operations
- **FR-007**: System MUST support soft deletion (tombstoning) of states, marking them as deleted without removing data.
- **FR-008**: System MUST reject all Terraform backend operations (GET, PUT, LOCK, UNLOCK) on tombstoned states with an appropriate error message.
- **FR-009**: System MUST support filtering state listings to include or exclude tombstoned states (respecting user's scope).
- **FR-010**: System MUST support restoring tombstoned states to active status within the retention period.
- **FR-011**: System MUST block tombstoning of states that have active dependents - dependents must be tombstoned first.
- **FR-012**: System MUST reject adding dependencies to tombstoned states.

#### Purge (Permanent Delete) Operations
- **FR-013**: System MUST support permanent deletion (purge) of tombstoned states after the retention period.
- **FR-014**: System MUST enforce a retention period before tombstoned states can be purged (default: 30 days).
- **FR-015**: System MUST support force-purge of tombstoned states within the retention period with explicit confirmation.
- **FR-016**: System MUST reject purge operations on non-tombstoned (active) states.
- **FR-017**: System MUST allow reuse of logic_ids from purged states (not from tombstoned states).

#### CLI
- **FR-019**: CLI MUST provide commands for rename, delete (tombstone), restore, and purge operations.
- **FR-019a**: CLI MUST use `--all` flag for listing states including tombstoned (not `--include-deleted`).

#### Audit (Deferred)
- **FR-018**: System SHOULD log lifecycle operations using existing `log.Printf` pattern for basic observability.

> **Note**: Compliance-grade audit logging (structured slog, dedicated audit table, tamper-proof records) is tracked in ROADMAP.md, not in scope for this feature. Basic logging via `log.Printf` is sufficient for initial implementation.

### Key Entities

- **State**: Extended with lifecycle status (active, tombstoned), tombstone timestamp, and deletion metadata.
- **RetentionDays**: Global configuration for retention period duration (configured via gridapi CLI `--retention-days` flag, default: 30 days). No per-state override mechanism.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Authorized users can rename state logic_ids (single operation) in under 5 seconds without affecting active Terraform workflows.
- **SC-002**: Tombstoned states are excluded from default listings (`gridctl state list` without `--all`), reducing clutter for active state management.
- **SC-003**: Zero accidental permanent data loss due to state deletion (all deletions go through tombstone first; force-purge only bypasses retention, not tombstone requirement).
- **SC-004**: Basic lifecycle operation logging via existing `log.Printf` pattern (compliance-grade audit deferred to ROADMAP.md).
- **SC-005**: Users attempting Terraform operations on tombstoned states receive clear error messages (HTTP 410 Gone with descriptive body) within 1 second.
- **SC-006**: Tombstoned states can be restored within the retention period with zero data loss.
- **SC-007**: Users without appropriate permissions receive clear authorization errors when attempting lifecycle operations.

### Previous work

Related features and tasks from Beads:

- **001-develop-the-grid**: Initial Grid development including state GUID/logic_id model
- **002-add-state-dependency**: State dependency management (relevant for tombstone behavior with dependents)
- **006-authz-authn-rbac**: Authorization framework (lifecycle operations need permission controls)

## Assumptions

- **Retention period**: 30 days default, configured globally via gridapi `--retention-days` CLI flag. No per-state override mechanism (YAGNI).
- **Lock state interaction**: States must be unlocked before tombstoning to prevent orphaned locks. Tombstoned states cannot be locked.
- **Logic_id uniqueness**: Logic_ids must be unique across both active and tombstoned states to prevent confusion (purged state IDs can be reused).
- **Tombstoned state renaming**: Tombstoned states CAN be renamed to free up their logic_id for reuse without waiting for purge.
- **Dependency graph integrity**: States with active dependents cannot be tombstoned (enforces clean dependency graph). Dependencies cannot be added to tombstoned states.
- **CLI-first**: All operations will be exposed via CLI (gridctl) with API support for future webapp integration.
- **CLI terminology**: CLI uses "delete" command (user-friendly) which performs tombstone operation internally.
- **Audit logging**: Existing `log.Printf` pattern will be used. Structured logging (slog) migration and dedicated audit table are separate ROADMAP.md items.
- **No alias/redirect support**: After rename, old logic_id lookups return NOT_FOUND error - no hints or temporary redirects.
