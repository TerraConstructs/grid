# Quickstart: State Lifecycle Operations

**Feature**: 011-state-lifecycle-ops
**Date**: 2025-12-08
**Updated**: 2025-12-08 (post-analysis refinements)

This guide demonstrates the state lifecycle operations: rename, delete (soft), restore, and purge.

## Decision Log

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Tombstoned state rename | **Allowed** | Allows freeing up logic_id without waiting for purge |
| Restore vs undelete | **restore** | Matches SDK terminology, standard database term |
| List flag for deleted | **--all** | Simple, intuitive - shows all states regardless of status |
| Force purge scope | **Tombstoned only** | Force bypasses retention, not tombstone requirement (safer two-step) |
| CLI command | **delete** | User-friendly; internally maps to tombstone operation |
| Retention config | **Global only** | Configured via gridapi CLI, not per-state |

## Prerequisites

- Grid API server running (`./bin/gridapi serve`)
- Grid CLI authenticated (`gridctl auth login`)
- At least one state created

## Scenario 1: Rename a State

### Use Case

Project "alpha" is being renamed to "beta". All state logic IDs need updating.

### Steps

```bash
# 1. Navigate to state directory (with .grid context) and check current state
cd /path/to/alpha-terraform
gridctl state get
# Output:
# GUID:      01JEDA...
# Logic ID:  alpha/production
# Status:    active
# Created:   2025-12-01 10:00:00

# 2. Rename the state (uses dirCtx for current state)
gridctl state rename beta/production
# Output:
# State renamed successfully
# GUID:      01JEDA... (unchanged)
# Old Name:  alpha/production
# New Name:  beta/production

# 3. Verify the rename
gridctl state get
# Output:
# GUID:      01JEDA...
# Logic ID:  beta/production
# Status:    active

# 4. Terraform continues to work (GUID unchanged)
terraform plan
# Works - backend uses GUID-based URL, not logic_id
```

### Without dirCtx (explicit reference)

```bash
# When not in a state directory, use --logic-id or --guid
gridctl state rename --logic-id alpha/production beta/production
gridctl state rename --guid 01JEDA... beta/production
```

### Renaming a Tombstoned State

Tombstoned states can be renamed to free up their logic_id for reuse without waiting for purge:

```bash
# 1. Find the tombstoned state
gridctl state list --all
# Output: legacy/test [DELETED]

# 2. Rename the tombstoned state to a new name
gridctl state rename --logic-id legacy/test archived/legacy-test
# Output:
# State renamed successfully
# GUID:      01JEDC... (unchanged)
# Old Name:  legacy/test
# New Name:  archived/legacy-test

# 3. Now "legacy/test" is available for active states
gridctl state create legacy/test
# Works - creates new state with new GUID
```

### Error Cases

```bash
# Rename to existing name
gridctl state rename beta/production
# Error: target logic_id "beta/production" already exists

# Rename locked state
gridctl state rename gamma/staging
# Error: cannot rename locked state (unlock first or wait for Terraform to release)

# Rename active state to tombstoned name
gridctl state rename legacy/test
# Error: target logic_id "legacy/test" is tombstoned (purge it first or rename the tombstoned state)
```

## Scenario 2: Delete (Soft Delete) a State

### Use Case

Project "legacy" is being decommissioned. States should be hidden but preserved for compliance.

### Steps

```bash
# 1. Check state dependencies first
gridctl state get --logic-id legacy/production
# Output shows:
# Dependents: app/frontend, app/backend  <-- These depend on this state

# 2. Delete dependent states first (leaf nodes) - soft delete by default
cd /path/to/app-frontend-terraform
gridctl state delete
# Output:
# State deleted (tombstoned) successfully
# GUID:      01JEDB...
# Logic ID:  app/frontend
# Status:    tombstoned
# Purge eligible: 2026-01-07 (30 days)

cd /path/to/app-backend-terraform
gridctl state delete
# Output:
# State deleted (tombstoned) successfully

# 3. Now delete the producer state
gridctl state delete --logic-id legacy/production
# Output:
# State deleted (tombstoned) successfully
# GUID:      01JEDA...
# Logic ID:  legacy/production
# Status:    tombstoned
# Purge eligible: 2026-01-07 (30 days)

# 4. Verify state is hidden from default listings
gridctl state list
# Output: legacy/production NOT shown

gridctl state list --all
# Output: Shows legacy/production with [DELETED] indicator

# 5. Terraform operations are blocked
cd /path/to/legacy-terraform
terraform plan
# Error: state has been deleted (HTTP 410 Gone)
```

### Error Cases

```bash
# Delete locked state
gridctl state delete
# Error: cannot delete locked state (unlock first)

# Delete state with active dependents
gridctl state delete --logic-id network/vpc
# Error: cannot delete state with active dependents:
#   - app/frontend
#   - app/backend
# Delete these states first.
```

## Scenario 3: Restore a Tombstoned State

### Use Case

The decommissioning was cancelled. States need to be restored.

### Steps

```bash
# 1. List tombstoned states
gridctl state list --all
# Output shows: legacy/production [DELETED]

# 2. Restore the state (uses dirCtx or explicit reference)
cd /path/to/legacy-terraform
gridctl state restore
# Output:
# State restored successfully
# GUID:      01JEDA...
# Logic ID:  legacy/production
# Status:    active

# Or without dirCtx:
gridctl state restore --logic-id legacy/production

# 3. Verify restoration
gridctl state get
# Output shows status: active

# 4. Terraform operations work again
terraform plan
# Works normally
```

### Error Cases

```bash
# Restore active state
gridctl state restore
# Error: state is not tombstoned

# Restore after retention period
gridctl state restore --logic-id very-old/expired-state
# Error: cannot restore state past retention period
# Tombstoned: 2025-10-01
# Retention: 30 days
# Expired: 2025-10-31
```

## Scenario 4: Purge (Permanently Delete) a State

### Use Case

After compliance retention period, permanently remove old states.

### Steps

```bash
# 1. Find purge-eligible states
gridctl state list --all
# Output shows: old/project [DELETED] (purge eligible)

# 2. Purge the state (uses dirCtx or explicit reference)
cd /path/to/old-project-terraform
gridctl state delete --purge
# Output:
# State purged successfully
# GUID:      01JED9...
# Logic ID:  old/project
# WARNING: This action is irreversible. All data has been permanently deleted.

# Or without dirCtx:
gridctl state delete --purge --logic-id old/project

# 3. Verify purge
gridctl state get --logic-id old/project
# Error: state not found

# 4. Logic ID is now reusable
gridctl state create old/project
# Works - creates new state with new GUID
```

### Force Purge (Within Retention Period)

```bash
# Force purge before retention period expires
gridctl state delete --purge --force --logic-id recent/tombstoned
# Output:
# WARNING: State is within retention period (eligible: 2026-01-15)
# Are you sure you want to permanently delete this state? [y/N]: y
# State purged successfully

# Or with dirCtx:
cd /path/to/recent-tombstoned-terraform
gridctl state delete --purge --force
```

### Error Cases

```bash
# Purge active state
gridctl state delete --purge
# Error: cannot purge active state (delete without --purge first)

# Purge within retention without force
gridctl state delete --purge
# Error: state is within retention period
# Tombstoned: 2025-12-01
# Purge eligible: 2025-12-31
# Use --force to purge immediately (data will be permanently lost)
```

## CLI Reference

### Rename

```bash
gridctl state rename <new-logic-id>                    # Uses dirCtx for current state
gridctl state rename --logic-id <current> <new-logic-id>  # Explicit current state
gridctl state rename --guid <guid> <new-logic-id>      # By GUID

# Examples (in state directory with .grid context):
gridctl state rename beta/prod                         # Rename current state

# Examples (explicit reference):
gridctl state rename --logic-id alpha/prod beta/prod
gridctl state rename --guid 01JEDA... beta/prod
```

### Delete (Soft Delete / Tombstone)

```bash
gridctl state delete                          # Uses dirCtx, soft delete (tombstone)
gridctl state delete --logic-id <id>          # Explicit reference
gridctl state delete --guid <guid>            # By GUID

# Examples:
gridctl state delete                          # Soft delete current state
gridctl state delete --logic-id legacy/prod   # Soft delete by logic_id
gridctl state delete --guid 01JEDA...         # Soft delete by GUID
```

### Delete with Purge (Permanent Delete)

```bash
gridctl state delete --purge                  # Purge tombstoned state (uses dirCtx)
gridctl state delete --purge --force          # Force purge within retention period
gridctl state delete --purge --logic-id <id>  # Explicit reference

# Examples:
gridctl state delete --purge                            # Purge current tombstoned state
gridctl state delete --purge --logic-id old/project     # Purge by logic_id
gridctl state delete --purge --force --guid 01JEDA...   # Force purge by GUID
```

### Restore

```bash
gridctl state restore                         # Uses dirCtx
gridctl state restore --logic-id <id>         # Explicit reference
gridctl state restore --guid <guid>           # By GUID

# Examples:
gridctl state restore                         # Restore current tombstoned state
gridctl state restore --logic-id legacy/prod  # Restore by logic_id
gridctl state restore --guid 01JEDA...        # Restore by GUID
```

### List with All States

```bash
gridctl state list [--all]

# Examples:
gridctl state list        # Active states only (default)
gridctl state list --all  # Include tombstoned (deleted) states
```

## SDK Usage (Go)

```go
import "github.com/terraconstructs/grid/pkg/sdk"

// Rename
result, err := client.RenameState(ctx, sdk.RenameStateInput{
    State:      sdk.StateRef{LogicID: "alpha/prod"},
    NewLogicID: "beta/prod",
})

// Tombstone
result, err := client.TombstoneState(ctx, sdk.StateRef{LogicID: "legacy/prod"})

// Restore
result, err := client.RestoreState(ctx, sdk.StateRef{GUID: "01JEDA..."})

// Purge
result, err := client.PurgeState(ctx, sdk.PurgeStateInput{
    State: sdk.StateRef{LogicID: "old/project"},
    Force: true,
})

// List all states (including tombstoned)
// Note: SDK uses 'IncludeTombstoned', CLI uses '--all' for user-friendliness
states, err := client.ListStatesWithOptions(ctx, sdk.ListStatesOptions{
    IncludeTombstoned: true,
})
```

## Verification Tests

These scenarios should be validated as integration tests:

1. **Rename happy path**: Create state, rename, verify GUID unchanged
2. **Rename conflict**: Attempt rename to existing name, expect error
3. **Rename locked**: Attempt rename of locked state, expect error
4. **Rename tombstoned state**: Rename a tombstoned state to free up logic_id
5. **Rename to tombstoned name**: Attempt rename active state to tombstoned name, expect error
6. **Rename concurrent conflict**: Two concurrent renames, first wins, second gets ABORTED
7. **Lookup old logic_id**: After rename, lookup by old logic_id returns NOT_FOUND
8. **Tombstone happy path**: Create state, tombstone, verify hidden from listings
9. **Tombstone with dependents**: Create dependency, attempt tombstone producer, expect error
10. **Restore happy path**: Tombstone state, restore, verify active
11. **Restore expired**: Tombstone state, set past retention, attempt restore, expect error (slow)
12. **Purge happy path**: Tombstone state, wait retention, purge, verify deleted (slow)
13. **Purge active**: Attempt purge active state, expect error
14. **Force purge**: Tombstone state, force purge within retention, verify deleted
15. **Terraform 410**: Tombstone state, run terraform plan, expect 410 Gone with error body
16. **Add dependency to tombstoned**: Attempt add dependency to tombstoned state, expect error
