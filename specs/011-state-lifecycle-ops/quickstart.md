# Quickstart: State Lifecycle Operations

**Feature**: 011-state-lifecycle-ops
**Date**: 2025-12-08

This guide demonstrates the state lifecycle operations: rename, tombstone, restore, and purge.

## Prerequisites

- Grid API server running (`./bin/gridapi serve`)
- Grid CLI authenticated (`gridctl auth login`)
- At least one state created

## Scenario 1: Rename a State

### Use Case

Project "alpha" is being renamed to "beta". All state logic IDs need updating.

### Steps

```bash
# 1. Check current state
gridctl state get alpha/production
# Output:
# GUID:      01JEDA...
# Logic ID:  alpha/production
# Status:    active
# Created:   2025-12-01 10:00:00

# 2. Rename the state
gridctl state rename alpha/production beta/production
# Output:
# State renamed successfully
# GUID:      01JEDA... (unchanged)
# Old Name:  alpha/production
# New Name:  beta/production

# 3. Verify the rename
gridctl state get beta/production
# Output:
# GUID:      01JEDA...
# Logic ID:  beta/production
# Status:    active

# 4. Terraform continues to work (GUID unchanged)
cd /path/to/terraform
terraform plan
# Works - backend uses GUID-based URL, not logic_id
```

### Error Cases

```bash
# Rename to existing name
gridctl state rename alpha/dev beta/production
# Error: target logic_id "beta/production" already exists

# Rename locked state
gridctl state rename alpha/staging gamma/staging
# Error: cannot rename locked state (unlock first or wait for Terraform to release)

# Rename to tombstoned name
gridctl state rename alpha/test legacy/test
# Error: target logic_id "legacy/test" is tombstoned (purge it first)
```

## Scenario 2: Tombstone (Soft Delete) a State

### Use Case

Project "legacy" is being decommissioned. States should be hidden but preserved for compliance.

### Steps

```bash
# 1. Check state dependencies first
gridctl state get legacy/production
# Output shows:
# Dependents: app/frontend, app/backend  <-- These depend on this state

# 2. Tombstone dependent states first (leaf nodes)
gridctl state tombstone app/frontend
# Output:
# State tombstoned successfully
# GUID:      01JEDB...
# Logic ID:  app/frontend
# Status:    tombstoned
# Purge eligible: 2026-01-07 (30 days)

gridctl state tombstone app/backend
# Output:
# State tombstoned successfully

# 3. Now tombstone the producer state
gridctl state tombstone legacy/production
# Output:
# State tombstoned successfully
# GUID:      01JEDA...
# Logic ID:  legacy/production
# Status:    tombstoned
# Purge eligible: 2026-01-07 (30 days)

# 4. Verify state is hidden from default listings
gridctl state list
# Output: legacy/production NOT shown

gridctl state list --include-deleted
# Output: Shows legacy/production with [DELETED] indicator

# 5. Terraform operations are blocked
cd /path/to/legacy-terraform
terraform plan
# Error: state has been deleted (HTTP 410 Gone)
```

### Error Cases

```bash
# Tombstone locked state
gridctl state tombstone active/locked-state
# Error: cannot tombstone locked state (unlock first)

# Tombstone state with active dependents
gridctl state tombstone network/vpc
# Error: cannot tombstone state with active dependents:
#   - app/frontend
#   - app/backend
# Tombstone these states first.
```

## Scenario 3: Restore a Tombstoned State

### Use Case

The decommissioning was cancelled. States need to be restored.

### Steps

```bash
# 1. List tombstoned states
gridctl state list --include-deleted
# Output shows: legacy/production [DELETED]

# 2. Restore the state
gridctl state restore legacy/production
# Output:
# State restored successfully
# GUID:      01JEDA...
# Logic ID:  legacy/production
# Status:    active

# 3. Verify restoration
gridctl state get legacy/production
# Output shows status: active

# 4. Terraform operations work again
cd /path/to/legacy-terraform
terraform plan
# Works normally
```

### Error Cases

```bash
# Restore active state
gridctl state restore active/production
# Error: state is not tombstoned

# Restore after retention period
gridctl state restore very-old/expired-state
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
gridctl state list --include-deleted
# Output shows: old/project [DELETED] (purge eligible)

# 2. Purge the state
gridctl state purge old/project
# Output:
# State purged successfully
# GUID:      01JED9...
# Logic ID:  old/project
# WARNING: This action is irreversible. All data has been permanently deleted.

# 3. Verify purge
gridctl state get old/project
# Error: state not found

# 4. Logic ID is now reusable
gridctl state create old/project
# Works - creates new state with new GUID
```

### Force Purge (Within Retention Period)

```bash
# Force purge before retention period expires
gridctl state purge recent/tombstoned --force
# Output:
# WARNING: State is within retention period (eligible: 2026-01-15)
# Are you sure you want to permanently delete this state? [y/N]: y
# State purged successfully
```

### Error Cases

```bash
# Purge active state
gridctl state purge active/production
# Error: cannot purge active state (tombstone first)

# Purge within retention without force
gridctl state purge recent/tombstoned
# Error: state is within retention period
# Tombstoned: 2025-12-01
# Purge eligible: 2025-12-31
# Use --force to purge immediately (data will be permanently lost)
```

## CLI Reference

### Rename

```bash
gridctl state rename [--logic-id <current>] [--guid <guid>] <new-logic-id>

# Examples:
gridctl state rename alpha/prod beta/prod          # By logic_id (positional)
gridctl state rename --logic-id alpha/prod beta/prod  # By logic_id (flag)
gridctl state rename --guid 01JEDA... beta/prod    # By GUID
```

### Tombstone

```bash
gridctl state tombstone [--logic-id <id>] [--guid <guid>] [<logic-id>]

# Examples:
gridctl state tombstone legacy/prod               # By logic_id
gridctl state tombstone --guid 01JEDA...          # By GUID
```

### Restore

```bash
gridctl state restore [--logic-id <id>] [--guid <guid>] [<logic-id>]

# Examples:
gridctl state restore legacy/prod                 # By logic_id
gridctl state restore --guid 01JEDA...            # By GUID
```

### Purge

```bash
gridctl state purge [--logic-id <id>] [--guid <guid>] [--force] [<logic-id>]

# Examples:
gridctl state purge old/project                   # Normal purge (after retention)
gridctl state purge --force recent/project        # Force purge (within retention)
gridctl state purge --guid 01JEDA... --force      # Force purge by GUID
```

### List with Tombstoned

```bash
gridctl state list [--include-deleted]

# Examples:
gridctl state list                    # Active states only (default)
gridctl state list --include-deleted  # Include tombstoned states
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

// List with tombstoned
states, err := client.ListStatesWithOptions(ctx, sdk.ListStatesOptions{
    IncludeTombstoned: true,
})
```

## Verification Tests

These scenarios should be validated as integration tests:

1. **Rename happy path**: Create state, rename, verify GUID unchanged
2. **Rename conflict**: Attempt rename to existing name, expect error
3. **Tombstone happy path**: Create state, tombstone, verify hidden from listings
4. **Tombstone with dependents**: Create dependency, attempt tombstone producer, expect error
5. **Restore happy path**: Tombstone state, restore, verify active
6. **Restore expired**: Tombstone state, set past retention, attempt restore, expect error
7. **Purge happy path**: Tombstone state, wait retention, purge, verify deleted
8. **Purge active**: Attempt purge active state, expect error
9. **Force purge**: Tombstone state, force purge within retention, verify deleted
10. **Terraform 410**: Tombstone state, run terraform plan, expect 410 Gone
