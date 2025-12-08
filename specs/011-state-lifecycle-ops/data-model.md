# Data Model: State Lifecycle Operations

**Feature**: 011-state-lifecycle-ops
**Date**: 2025-12-08

## Entity Changes

### State (Extended)

The existing `State` entity is extended with lifecycle status tracking.

#### Current Fields (Unchanged)

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| `guid` | UUID | PK | Immutable UUIDv7 identifier |
| `logic_id` | VARCHAR(255) | UNIQUE, NOT NULL | Mutable human-readable identifier |
| `state_content` | BYTEA | | Terraform state JSON |
| `locked` | BOOLEAN | NOT NULL, DEFAULT false | Terraform lock status |
| `lock_info` | JSONB | | Lock metadata |
| `labels` | JSONB | NOT NULL, DEFAULT '{}' | User-defined labels |
| `created_at` | TIMESTAMP | NOT NULL | Creation timestamp |
| `updated_at` | TIMESTAMP | NOT NULL | Last modification timestamp |

#### New Fields (Lifecycle)

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| `status` | VARCHAR(20) | NOT NULL, DEFAULT 'active' | Lifecycle status: 'active' or 'tombstoned' |
| `tombstoned_at` | TIMESTAMP | NULL | When state was tombstoned |
| `tombstoned_by` | VARCHAR(255) | NULL | Principal ID who performed tombstone |
| `retention_days` | INTEGER | NOT NULL, DEFAULT 30 | Days before eligible for purge |

#### Go Struct Update

```go
// StateStatus represents the lifecycle status of a state
type StateStatus string

const (
    StateStatusActive     StateStatus = "active"
    StateStatusTombstoned StateStatus = "tombstoned"
)

type State struct {
    // Existing fields...
    GUID         string    `bun:"guid,pk,type:uuid"`
    LogicID      string    `bun:"logic_id,notnull,unique"`
    StateContent []byte    `bun:"state_content,type:bytea"`
    SizeBytes    int64     `bun:"size_bytes,scanonly"`
    Locked       bool      `bun:"locked,notnull,default:false"`
    LockInfo     *LockInfo `bun:"lock_info,type:jsonb"`
    Labels       LabelMap  `bun:"labels,type:jsonb,notnull,default:'{}'"`
    CreatedAt    time.Time `bun:"created_at,notnull,default:current_timestamp"`
    UpdatedAt    time.Time `bun:"updated_at,notnull,default:current_timestamp"`

    // NEW: Lifecycle fields
    Status        StateStatus `bun:"status,notnull,default:'active'"`
    TombstonedAt  *time.Time  `bun:"tombstoned_at"`
    TombstonedBy  *string     `bun:"tombstoned_by"`
    RetentionDays int         `bun:"retention_days,notnull,default:30"`

    // Relationships (unchanged)
    Outputs       []*StateOutput `bun:"rel:has-many,join:guid=state_guid"`
    OutgoingEdges []*Edge        `bun:"rel:has-many,join:guid=from_state"`
    IncomingEdges []*Edge        `bun:"rel:has-many,join:guid=to_state"`

    // Computed counts (unchanged)
    DependenciesCount int `bun:"dependencies_count,scanonly"`
    DependentsCount   int `bun:"dependents_count,scanonly"`
    OutputsCount      int `bun:"outputs_count,scanonly"`
}

// Helper methods
func (s *State) IsTombstoned() bool {
    return s.Status == StateStatusTombstoned
}

func (s *State) IsActive() bool {
    return s.Status == StateStatusActive
}

func (s *State) PurgeEligibleAt() *time.Time {
    if s.TombstonedAt == nil {
        return nil
    }
    t := s.TombstonedAt.AddDate(0, 0, s.RetentionDays)
    return &t
}

func (s *State) IsPurgeEligible() bool {
    eligible := s.PurgeEligibleAt()
    if eligible == nil {
        return false
    }
    return time.Now().After(*eligible)
}
```

## Database Migration

### Migration: Add Lifecycle Columns

```sql
-- Up migration
ALTER TABLE states ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'active';
ALTER TABLE states ADD COLUMN tombstoned_at TIMESTAMP NULL;
ALTER TABLE states ADD COLUMN tombstoned_by VARCHAR(255) NULL;
ALTER TABLE states ADD COLUMN retention_days INTEGER NOT NULL DEFAULT 30;

-- Create index for efficient status filtering
CREATE INDEX idx_states_status ON states(status);

-- Composite index for tombstone retention queries
CREATE INDEX idx_states_tombstone ON states(status, tombstoned_at)
    WHERE status = 'tombstoned';

-- Add check constraint for valid status values (PostgreSQL)
ALTER TABLE states ADD CONSTRAINT chk_states_status
    CHECK (status IN ('active', 'tombstoned'));

-- Down migration
DROP INDEX IF EXISTS idx_states_tombstone;
DROP INDEX IF EXISTS idx_states_status;
ALTER TABLE states DROP CONSTRAINT IF EXISTS chk_states_status;
ALTER TABLE states DROP COLUMN IF EXISTS retention_days;
ALTER TABLE states DROP COLUMN IF EXISTS tombstoned_by;
ALTER TABLE states DROP COLUMN IF EXISTS tombstoned_at;
ALTER TABLE states DROP COLUMN IF EXISTS status;
```

### SQLite Compatibility

SQLite has several limitations that affect the migration approach:

| Feature | PostgreSQL | SQLite | Migration Impact |
|---------|------------|--------|------------------|
| CHECK constraints | ALTER TABLE ADD CONSTRAINT | Not supported via ALTER | Enforce in application layer |
| Partial indexes | `WHERE status = 'tombstoned'` | Supported | Same syntax works |
| Timestamps | TIMESTAMP type | TEXT (ISO8601 strings) | Bun handles conversion |
| Interval arithmetic | `+ interval '30 days'` | `datetime(ts, '+30 days')` | Repository handles in Go |
| JSONB | Native JSONB type | TEXT | Bun handles serialization |

```sql
-- SQLite version (no ALTER TABLE ADD CONSTRAINT)
ALTER TABLE states ADD COLUMN status TEXT NOT NULL DEFAULT 'active';
ALTER TABLE states ADD COLUMN tombstoned_at TEXT NULL;
ALTER TABLE states ADD COLUMN tombstoned_by TEXT NULL;
ALTER TABLE states ADD COLUMN retention_days INTEGER NOT NULL DEFAULT 30;

CREATE INDEX idx_states_status ON states(status);
CREATE INDEX idx_states_tombstone ON states(status, tombstoned_at);
-- Note: Partial index syntax works in SQLite 3.8.0+ but we use simple composite index for compatibility
```

### Go Migration Implementation

Following the existing pattern in `20251203000000_init_schema.go`:

```go
func up_lifecycle(ctx context.Context, db *bun.DB) error {
    // Add columns (same SQL works for both PG and SQLite via Bun)
    _, err := db.Exec(`ALTER TABLE states ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'active'`)
    if err != nil {
        return fmt.Errorf("add status column: %w", err)
    }
    _, err = db.Exec(`ALTER TABLE states ADD COLUMN tombstoned_at TIMESTAMP NULL`)
    if err != nil {
        return fmt.Errorf("add tombstoned_at column: %w", err)
    }
    _, err = db.Exec(`ALTER TABLE states ADD COLUMN tombstoned_by VARCHAR(255) NULL`)
    if err != nil {
        return fmt.Errorf("add tombstoned_by column: %w", err)
    }
    _, err = db.Exec(`ALTER TABLE states ADD COLUMN retention_days INTEGER NOT NULL DEFAULT 30`)
    if err != nil {
        return fmt.Errorf("add retention_days column: %w", err)
    }

    // Create indexes (same syntax for both)
    _, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_states_status ON states(status)`)
    if err != nil {
        return fmt.Errorf("create status index: %w", err)
    }
    _, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_states_tombstone ON states(status, tombstoned_at)`)
    if err != nil {
        return fmt.Errorf("create tombstone index: %w", err)
    }

    // CHECK constraint only for PostgreSQL
    if IsPostgreSQL(db) {
        _, err = db.Exec(`ALTER TABLE states ADD CONSTRAINT chk_states_status CHECK (status IN ('active', 'tombstoned'))`)
        if err != nil {
            return fmt.Errorf("add status check constraint: %w", err)
        }
    }
    // For SQLite: Status validation enforced in StateService and repository layer

    return nil
}
```

### Application-Level Validation (SQLite)

Since SQLite cannot enforce CHECK constraints via ALTER TABLE, the service layer validates status values:

```go
// In StateService or repository
func validateStatus(status StateStatus) error {
    switch status {
    case StateStatusActive, StateStatusTombstoned:
        return nil
    default:
        return fmt.Errorf("invalid state status: %s", status)
    }
}
```

## State Transitions

```
                    ┌─────────────────────────┐
                    │                         │
                    ▼                         │
    ┌────────┐  create   ┌────────┐  restore │
    │  NEW   │ ────────► │ ACTIVE │ ◄────────┤
    └────────┘           └────────┘          │
                              │              │
                         tombstone           │
                              │              │
                              ▼              │
                        ┌────────────┐       │
                        │ TOMBSTONED │ ──────┘
                        └────────────┘
                              │
                         purge (after retention)
                              │
                              ▼
                        ┌────────────┐
                        │  DELETED   │ (no record)
                        └────────────┘
```

### Transition Rules

| From | To | Conditions | Action |
|------|-----|------------|--------|
| (none) | ACTIVE | Create state | `Status = 'active'` |
| ACTIVE | TOMBSTONED | Not locked, no active dependents | Set `Status`, `TombstonedAt`, `TombstonedBy` |
| TOMBSTONED | ACTIVE | Within retention period | Clear tombstone fields |
| TOMBSTONED | (deleted) | Past retention OR force flag | Hard delete row |

## Validation Rules

### Rename Operation

| Rule | Description | Error Code |
|------|-------------|------------|
| State exists | GUID must exist | NOT_FOUND |
| Not tombstoned | Cannot rename tombstoned state | FAILED_PRECONDITION |
| Not locked | Cannot rename locked state | FAILED_PRECONDITION |
| Target unique | New logic_id must not exist (active or tombstoned) | ALREADY_EXISTS |
| Valid format | Logic_id must match pattern `^[a-zA-Z0-9/_-]+$` | INVALID_ARGUMENT |

### Tombstone Operation

| Rule | Description | Error Code |
|------|-------------|------------|
| State exists | GUID must exist | NOT_FOUND |
| Is active | Cannot tombstone already tombstoned state | FAILED_PRECONDITION |
| Not locked | Cannot tombstone locked state | FAILED_PRECONDITION |
| No dependents | No active states depend on this state | FAILED_PRECONDITION |

### Restore Operation

| Rule | Description | Error Code |
|------|-------------|------------|
| State exists | GUID must exist | NOT_FOUND |
| Is tombstoned | Cannot restore active state | FAILED_PRECONDITION |
| Within retention | `TombstonedAt + RetentionDays > now` | FAILED_PRECONDITION |

### Purge Operation

| Rule | Description | Error Code |
|------|-------------|------------|
| State exists | GUID must exist | NOT_FOUND |
| Is tombstoned | Cannot purge active state | FAILED_PRECONDITION |
| Past retention | Must be past retention (unless force=true) | FAILED_PRECONDITION |

## Query Patterns

### List Active States (Default)

```sql
SELECT * FROM states
WHERE status = 'active'
ORDER BY updated_at DESC;
```

### List Including Tombstoned

```sql
SELECT * FROM states
ORDER BY
    CASE WHEN status = 'active' THEN 0 ELSE 1 END,
    updated_at DESC;
```

### Check Active Dependents

```sql
SELECT COUNT(*) > 0
FROM edges e
JOIN states s ON e.from_state = s.guid
WHERE e.to_state = $1
  AND s.status = 'active';
```

### Find Purge-Eligible States

PostgreSQL:
```sql
SELECT * FROM states
WHERE status = 'tombstoned'
  AND tombstoned_at + (retention_days || ' days')::interval < NOW();
```

SQLite:
```sql
SELECT * FROM states
WHERE status = 'tombstoned'
  AND datetime(tombstoned_at, '+' || retention_days || ' days') < datetime('now');
```

**Recommended Go Implementation** (database-agnostic):
```go
// Calculate eligibility in Go, query by status only
func (r *BunStateRepository) ListPurgeEligible(ctx context.Context) ([]*models.State, error) {
    var states []*models.State
    err := r.db.NewSelect().
        Model(&states).
        Where("status = ?", models.StateStatusTombstoned).
        Scan(ctx)
    if err != nil {
        return nil, err
    }

    // Filter in Go for database-agnostic date arithmetic
    var eligible []*models.State
    now := time.Now()
    for _, s := range states {
        if s.IsPurgeEligible() { // Uses Go time.AddDate()
            eligible = append(eligible, s)
        }
    }
    return eligible, nil
}
```

## Impact on Existing Queries

### ListStates

- Default behavior: Filter `WHERE status = 'active'`
- With `include_tombstoned=true`: No status filter

### GetByLogicID

- Continue to return state regardless of status
- Caller checks status for their use case

### GetByGUID

- Continue to return state regardless of status
- Caller checks status for their use case

### Terraform HTTP Backend

- Check `status = 'active'` in middleware
- Return 410 Gone for tombstoned states

### Dependency Operations

- AddDependency: Reject if target is tombstoned
- ListDependencies/Dependents: Include tombstoned for visibility
