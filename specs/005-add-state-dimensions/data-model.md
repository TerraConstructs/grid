# Data Model: State Labels

**Date**: 2025-10-09 (Scope Reduction Applied)
**Feature Branch**: `005-add-state-dimensions`

## Overview

This document describes the simplified data model using a JSON column approach with in-memory bexpr filtering. Following the scope reduction, we avoid EAV tables, JSON Schema validation, and facet projection complexity.

## Schema Changes

### 1. State Model (Extended)

**Description**: Extend existing `states` table with a `labels` JSONB column.

**Existing Fields** (from `models.State`):
- `guid` (string/uuid, PK)
- `logic_id` (string, unique, notnull)
- `state_content` (bytea)
- `locked` (bool, default false)
- `lock_info` (jsonb, nullable)
- `created_at` (timestamp)
- `updated_at` (timestamp)

**New Field** (added by migration):
- `labels` (jsonb, notnull, default '{}') - stores typed label key/value pairs

**Label JSON Structure**:
```json
{
  "env": "staging",
  "team": "platform",
  "region": "us-west",
  "active": true,
  "generation": 3
}
```

**Supported Value Types**:
- `string`: any UTF-8 string ≤256 chars
- `number`: JSON number (stored as float64 in Go)
- `boolean`: true or false

### 2. Label Policy Model (New Table)

**Description**: Single-row table storing the active label policy with versioning.

**Table**: `label_policy`

**Fields**:
- `id` (integer, PK, CHECK id = 1) - enforces single-row table
- `version` (integer, notnull) - monotonically increasing version number
- `policy_json` (text, notnull) - JSON blob with policy definition
- `created_at` (timestamp, notnull, default current_timestamp)
- `updated_at` (timestamp, notnull, default current_timestamp)

**Policy JSON Structure**:
```json
{
  "allowed_keys": {
    "env": {},
    "team": {},
    "region": {},
    "cost_center": {}
  },
  "allowed_values": {
    "env": ["staging", "prod", "development"],
    "region": ["us-west", "us-east", "eu-west-1"]
  },
  "reserved_prefixes": ["grid.io/", "kubernetes.io/"],
  "max_keys": 32,
  "max_value_len": 256
}
```

**Policy Fields**:
- `allowed_keys`: Map of permitted keys (null/empty → any key matching regex allowed)
- `allowed_values`: Per-key enum constraints (missing key → any value allowed)
- `reserved_prefixes`: List of forbidden key prefixes
- `max_keys`: Maximum labels per state (default: 32)
- `max_value_len`: Maximum string value length (default: 256)

## Bun ORM Models

### Updated State Model

```go
// cmd/gridapi/internal/db/models/state.go
package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// LabelMap represents typed label values (string | float64 | bool)
type LabelMap map[string]any

// Scan implements sql.Scanner for reading from database
func (lm *LabelMap) Scan(value any) error {
	if value == nil {
		*lm = make(LabelMap)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan LabelMap: expected []byte, got %T", value)
	}
	return json.Unmarshal(bytes, lm)
}

// Value implements driver.Valuer for writing to database
func (lm LabelMap) Value() (driver.Value, error) {
	if lm == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(lm)
}

type State struct {
	bun.BaseModel `bun:"table:states,alias:s"`

	GUID         string    `bun:"guid,pk,type:uuid"`
	LogicID      string    `bun:"logic_id,notnull,unique"`
	StateContent []byte    `bun:"state_content,type:bytea"`
	Locked       bool      `bun:"locked,notnull,default:false"`
	LockInfo     *LockInfo `bun:"lock_info,type:jsonb"`
	CreatedAt    time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt    time.Time `bun:"updated_at,notnull,default:current_timestamp"`

	// NEW: Labels column
	Labels LabelMap `bun:"labels,type:jsonb,notnull,default:'{}'"`
}

// LockInfo remains unchanged...
type LockInfo struct {
	ID        string    `json:"ID"`
	Operation string    `json:"Operation"`
	Info      string    `json:"Info"`
	Who       string    `json:"Who"`
	Version   string    `json:"Version"`
	Created   time.Time `json:"Created"`
	Path      string    `json:"Path"`
}
```

### Label Policy Model

```go
// cmd/gridapi/internal/db/models/label_policy.go
package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// PolicyDefinition represents label validation rules
type PolicyDefinition struct {
	AllowedKeys      map[string]struct{}            `json:"allowed_keys,omitempty"`
	AllowedValues    map[string][]string            `json:"allowed_values,omitempty"`
	ReservedPrefixes []string                       `json:"reserved_prefixes,omitempty"`
	MaxKeys          int                            `json:"max_keys"`
	MaxValueLen      int                            `json:"max_value_len"`
}

// Scan implements sql.Scanner
func (pd *PolicyDefinition) Scan(value any) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan PolicyDefinition: expected []byte, got %T", value)
	}
	return json.Unmarshal(bytes, pd)
}

// Value implements driver.Valuer
func (pd PolicyDefinition) Value() (driver.Value, error) {
	return json.Marshal(pd)
}

type LabelPolicy struct {
	bun.BaseModel `bun:"table:label_policy,alias:lp"`

	ID         int               `bun:"id,pk"`
	Version    int               `bun:"version,notnull"`
	PolicyJSON PolicyDefinition  `bun:"policy_json,type:text,notnull"`
	CreatedAt  time.Time         `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt  time.Time         `bun:"updated_at,notnull,default:current_timestamp"`
}
```

## Migration

### Migration File Pattern

Following the existing pattern from `20251003140000_add_state_outputs_cache.go`:

```go
// cmd/gridapi/internal/migrations/20251009000001_add_state_labels.go
package migrations

import (
	"context"
	"fmt"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(up_20251009000001, down_20251009000001)
}

// up_20251009000001 adds labels column to states and creates label_policy table
func up_20251009000001(ctx context.Context, db *bun.DB) error {
	fmt.Print(" [up] adding labels column to states...")

	// 1. Add labels column to states table (PostgreSQL)
	_, err := db.Exec(`ALTER TABLE states ADD COLUMN IF NOT EXISTS labels JSONB NOT NULL DEFAULT '{}'::jsonb`)
	if err != nil {
		return fmt.Errorf("failed to add labels column: %w", err)
	}

	fmt.Println(" OK")
	fmt.Print(" [up] creating label_policy table...")

	// 2. Create label_policy table
	_, err = db.NewCreateTable().
		Model((*models.LabelPolicy)(nil)).
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create label_policy table: %w", err)
	}

	// 3. Add single-row constraint (PostgreSQL)
	_, err = db.Exec(`
		ALTER TABLE label_policy
		ADD CONSTRAINT label_policy_single_row CHECK (id = 1)
	`)
	if err != nil {
		return fmt.Errorf("failed to add single-row constraint: %w", err)
	}

	// 4. Optional: Create GIN index for future SQL push-down optimization
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_states_labels_gin ON states USING gin (labels jsonb_path_ops)`)
	if err != nil {
		return fmt.Errorf("failed to create GIN index on labels: %w", err)
	}

	fmt.Println(" OK")
	return nil
}

// down_20251009000001 removes labels column and drops label_policy table
func down_20251009000001(ctx context.Context, db *bun.DB) error {
	fmt.Print(" [down] dropping label_policy table...")

	_, err := db.NewDropTable().
		Model((*models.LabelPolicy)(nil)).
		IfExists().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to drop label_policy table: %w", err)
	}

	fmt.Println(" OK")
	fmt.Print(" [down] removing labels column from states...")

	_, err = db.Exec(`ALTER TABLE states DROP COLUMN IF EXISTS labels`)
	if err != nil {
		return fmt.Errorf("failed to drop labels column: %w", err)
	}

	fmt.Println(" OK")
	return nil
}
```

### SQLite Compatibility Notes

For SQLite support (future), the migration would use:
- `labels TEXT NOT NULL DEFAULT '{}'` instead of JSONB
- Trigger-based single-row enforcement for label_policy
- Expression-based indexes instead of GIN

## Validation Rules

### Label Key Format
```go
// Regex pattern matching spec.md FR-008
// Compatible with go-bexpr default Identifier grammar (bexpr-grammar.peg:186)
// Permits: lowercase letter start, then alphanumeric + underscore + forward-slash
var labelKeyRE = regexp.MustCompile(`^[a-z][a-z0-9_/]{0,31}$`)
```

### Label Constraints
- **Keys**: Must match regex pattern, ≤32 chars
- **Values**:
  - Strings: ≤256 chars
  - Numbers: JSON number (float64)
  - Booleans: true/false only
- **Count**: Maximum 32 labels per state
- **Reserved prefixes**: e.g., `grid.io/`, `kubernetes.io/`

### Policy Enforcement
Validation happens synchronously before state create/update:
1. Check label count ≤ max_keys
2. Validate each key format (regex)
3. Check reserved prefix violations
4. Validate against allowed_keys (if defined)
5. Validate against allowed_values enum (if defined)
6. Check string length ≤ max_value_len

## Repository Updates

### Interface Changes

```go
// cmd/gridapi/internal/repository/interface.go

type StateRepository interface {
	// Existing methods...
	Create(ctx context.Context, state *models.State) error
	GetByGUID(ctx context.Context, guid string) (*models.State, error)
	GetByLogicID(ctx context.Context, logicID string) (*models.State, error)
	Update(ctx context.Context, state *models.State) error

	// NEW: Filtered list with bexpr support
	ListWithFilter(ctx context.Context, filter string, pageSize int, offset int) ([]models.State, error)

	// Existing methods...
	Lock(ctx context.Context, guid string, lockInfo *models.LockInfo) error
	Unlock(ctx context.Context, guid string, lockID string) error
}

// NEW: Label policy repository
type LabelPolicyRepository interface {
	GetPolicy(ctx context.Context) (*models.LabelPolicy, error)
	SetPolicy(ctx context.Context, policy *models.PolicyDefinition) error
	GetEnumValues(ctx context.Context, key string) ([]string, error)
}
```

### Implementation Pattern (Bexpr Filtering)

```go
// cmd/gridapi/internal/repository/bun_state_repository.go

import bexpr "github.com/hashicorp/go-bexpr"

func (r *BunStateRepository) ListWithFilter(ctx context.Context, filter string, pageSize int, offset int) ([]models.State, error) {
	// 1. Fetch states from DB (over-fetch for in-memory filtering)
	var states []models.State
	fetchSize := pageSize * 3 // heuristic: 3x over-fetch

	err := r.db.NewSelect().
		Model(&states).
		Column("guid", "logic_id", "locked", "created_at", "updated_at", "labels").
		Order("updated_at DESC").
		Limit(fetchSize).
		Offset(offset).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list states: %w", err)
	}

	// 2. If no filter, return results
	if filter == "" {
		if len(states) > pageSize {
			states = states[:pageSize]
		}
		return states, nil
	}

	// 3. Compile bexpr evaluator
	evaluator, err := bexpr.CreateEvaluator(filter)
	if err != nil {
		return nil, fmt.Errorf("invalid filter expression: %w", err)
	}

	// 4. Filter in-memory
	filtered := make([]models.State, 0, pageSize)
	for _, state := range states {
		match, err := evaluator.Evaluate(map[string]any(state.Labels))
		if err != nil {
			return nil, fmt.Errorf("filter evaluation error: %w", err)
		}
		if match {
			filtered = append(filtered, state)
			if len(filtered) >= pageSize {
				break
			}
		}
	}

	return filtered, nil
}
```

## Design Decisions

### Why JSON Column over EAV?
- **Simplicity**: Single ALTER TABLE vs 3 new tables with foreign keys
- **Performance**: At 100-500 state scale, fetching all and filtering in-memory is <50ms p99
- **Maintenance**: No dictionary normalization, no join complexity
- **Migration path**: Easy to add GIN index or migrate to EAV later if needed

### Why go-bexpr over SQL Translation?
- **Safety**: Avoids SQL injection risks from user-provided filter expressions
- **Completeness**: Full boolean logic support without custom parser
- **Battle-tested**: Used in Consul, Nomad, Vault
- **No dependencies**: Only uses standard library beyond bexpr itself

### Why Policy over JSON Schema?
- **Simplicity**: Regex + enum maps vs external validation library
- **Performance**: Direct Go type assertions, no schema compilation
- **Sufficient**: Covers 95% of validation needs (key format, enums, size limits)
- **Extensibility**: Can add JSON Schema later if richer validation needed

### Future Optimization Paths
1. **SQL Push-down** (when >1000 states):
   - Translate simple bexpr subset to SQL WHERE clauses
   - Use GIN index for JSONB queries
   - Fall back to in-memory for complex expressions

2. **EAV Migration** (when >10k states or high cardinality):
   - Create normalized tables
   - Backfill from labels column
   - Keep labels column as source of truth for display

## Summary

**Schema Changes**:
- ✅ Add `labels` JSONB column to `states` table
- ✅ Create `label_policy` single-row table
- ✅ Optional GIN index for future optimization

**No Additional Complexity**:
- ❌ No EAV tables
- ❌ No JSON Schema validation
- ❌ No facet projection
- ❌ No audit log infrastructure

**Performance**:
- In-memory bexpr filtering: <50ms p99 for 500 states
- Over-fetch + filter + trim pagination
- Optional GIN index for future SQL push-down

**Migration**:
- Single migration following existing Bun patterns
- Zero-downtime: labels defaults to `{}`
- Backward compatible: existing states get empty labels
