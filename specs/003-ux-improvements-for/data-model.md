# Data Model: CLI Context-Aware State Management

**Feature**: 003-ux-improvements-for
**Date**: 2025-10-03

## Client-Side Entities

### DirectoryContext
**Purpose**: Associates a working directory with a Grid state for context-aware CLI operations

**Storage**: JSON file at `.grid` in current working directory

**Schema**:
```go
type DirectoryContext struct {
    Version      string    `json:"version"`        // Schema version (always "1")
    StateGUID    string    `json:"state_guid"`     // Immutable UUIDv7
    StateLogicID string    `json:"state_logic_id"` // Current logic ID (mutable)
    ServerURL    string    `json:"server_url"`     // Grid API base URL
    CreatedAt    time.Time `json:"created_at"`     // ISO 8601 timestamp
    UpdatedAt    time.Time `json:"updated_at"`     // ISO 8601 timestamp
}
```

**Validation Rules**:
- `version` MUST equal "1"
- `state_guid` MUST be valid UUIDv7 format
- `state_logic_id` MUST be non-empty string
- `server_url` SHOULD be valid HTTP/HTTPS URL (optional field)

**Lifecycle**:
1. **Created**: When `gridctl state create` runs in directory without `.grid`
2. **Updated**: When state logic-id changes (not implemented in Phase 1)
3. **Deleted**: Manual user action or `gridctl state unlink` (future command)

**Concurrency**:
- Write conflicts handled by "first write wins" policy
- Second concurrent write returns error: "`.grid` exists for different state; use --force`"

---

## Server-Side Entities

### StateOutput (Protobuf Message)
**Purpose**: Represents a single Terraform/OpenTofu output key from a state

**Definition** (proto/state/v1/state.proto):
```protobuf
message OutputKey {
  string key = 1;           // Terraform output name (e.g., "vpc_id")
  bool sensitive = 2;       // Marked sensitive in TF state metadata
}
```

**Source**: Parsed from `outputs` object in Terraform state JSON

**Example Terraform State JSON**:
```json
{
  "version": 4,
  "terraform_version": "1.6.0",
  "serial": 5,
  "outputs": {
    "vpc_id": {
      "value": "vpc-abc123",
      "type": "string",
      "sensitive": false
    },
    "db_password": {
      "value": "secret",
      "type": "string",
      "sensitive": true
    }
  }
}
```

**Parsing Logic** (server-side):

Already implemented in `cmd/gridapi/internal/tfstate/parser.go`
currently only used in `UpdateEdges` (`cmd/gridapi/internal/server/update_edges.go`)

Re-use for state outputs listing and info retrieval.

file: cmd/gridapi/internal/tfstate/parser.go
```go
// TFOutputs represents the structure of Terraform state outputs
type TFOutputs struct {
	Outputs map[string]OutputValue `json:"outputs"`
}
// OutputValue represents a single Terraform output value
type OutputValue struct {
	Value     interface{} `json:"value"`
	Sensitive bool        `json:"sensitive,omitempty"`
}
// ParseOutputs extracts output values from Terraform state JSON
func ParseOutputs(tfstateJSON []byte) (map[string]interface{}, error) {
	if len(tfstateJSON) == 0 {
		return make(map[string]interface{}), nil
	}

	var state TFOutputs
	if err := json.Unmarshal(tfstateJSON, &state); err != nil {
		return nil, fmt.Errorf("failed to parse tfstate JSON: %w", err)
	}

	outputs := make(map[string]interface{}, len(state.Outputs))
	for k, v := range state.Outputs {
		outputs[k] = v.Value
	}

	return outputs, nil
}
```

---

### StateOutput (Database Cache Table)
**Purpose**: Cached Terraform/OpenTofu outputs for fast cross-state search and retrieval

**Storage**: PostgreSQL table `state_outputs`

**Schema**:
```sql
CREATE TABLE state_outputs (
    state_guid UUID NOT NULL REFERENCES states(guid) ON DELETE CASCADE,
    output_key TEXT NOT NULL,
    sensitive BOOLEAN NOT NULL DEFAULT FALSE,
    state_serial BIGINT NOT NULL,  -- TF state serial/version for cache invalidation
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (state_guid, output_key)
);

CREATE INDEX idx_state_outputs_guid ON state_outputs(state_guid);  -- Bulk lookup by state
CREATE INDEX idx_state_outputs_key ON state_outputs(output_key);   -- Cross-state search
```

**Bun ORM Model** (`cmd/gridapi/internal/db/models/state_output.go`):
```go
type StateOutput struct {
    bun.BaseModel `bun:"table:state_outputs,alias:so"`

    StateGUID   string    `bun:"state_guid,pk,type:uuid"`
    OutputKey   string    `bun:"output_key,pk,type:text"`
    Sensitive   bool      `bun:"sensitive,notnull,default:false"`
    StateSerial int64     `bun:"state_serial,notnull"`
    CreatedAt   time.Time `bun:"created_at,notnull,default:now()"`
    UpdatedAt   time.Time `bun:"updated_at,notnull,default:now()"`
}
```

**Repository Interface** (`cmd/gridapi/internal/repository/interface.go`):
```go
type StateOutputRepository interface {
    // UpsertOutputs atomically replaces all outputs for a state
    // Deletes old outputs where state_serial != serial, inserts new ones
    UpsertOutputs(ctx context.Context, stateGUID string, serial int64, outputs []OutputKey) error

    // GetOutputsByState returns all cached outputs for a state
    // Returns empty slice if no outputs exist (not an error)
    GetOutputsByState(ctx context.Context, stateGUID string) ([]OutputKey, error)

    // SearchOutputsByKey finds all states with output matching key (exact match)
    // Used for cross-state dependency discovery
    SearchOutputsByKey(ctx context.Context, outputKey string) ([]StateOutputRef, error)

    // DeleteOutputsByState removes all cached outputs for a state
    // Cascade handles this on state deletion, but explicit method useful for testing
    DeleteOutputsByState(ctx context.Context, stateGUID string) error
}

type OutputKey struct {
    Key       string
    Sensitive bool
}

type StateOutputRef struct {
    StateGUID    string
    StateLogicID string
    OutputKey    string
    Sensitive    bool
}
```

**Migration** (`cmd/gridapi/internal/migrations/20251003140000_add_state_outputs_cache.go`):
```go
package migrations

import (
    "context"
    "fmt"

    "github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
    "github.com/uptrace/bun"
)

func init() {
    Migrations.MustRegister(up_20251003140000, down_20251003140000)
}

// up_20251003140000 creates the state_outputs table for caching TF outputs
func up_20251003140000(ctx context.Context, db *bun.DB) error {
    fmt.Print(" [up] creating state_outputs table...")

    // Create state_outputs table
    _, err := db.NewCreateTable().
        Model((*models.StateOutput)(nil)).
        IfNotExists().
        Exec(ctx)

    if err != nil {
        return fmt.Errorf("failed to create state_outputs table: %w", err)
    }

    // Create index on state_guid for bulk lookup
    _, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_state_outputs_guid ON state_outputs(state_guid)`)
    if err != nil {
        return fmt.Errorf("failed to create index on state_guid: %w", err)
    }

    // Create index on output_key for cross-state search
    _, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_state_outputs_key ON state_outputs(output_key)`)
    if err != nil {
        return fmt.Errorf("failed to create index on output_key: %w", err)
    }

    // Add foreign key constraint with cascade delete
    _, err = db.Exec(`
        ALTER TABLE state_outputs
        ADD CONSTRAINT fk_state_outputs_state_guid
        FOREIGN KEY (state_guid) REFERENCES states(guid) ON DELETE CASCADE
    `)
    if err != nil {
        return fmt.Errorf("failed to add FK constraint on state_guid: %w", err)
    }

    fmt.Println(" OK")
    return nil
}

// down_20251003140000 drops the state_outputs table
func down_20251003140000(ctx context.Context, db *bun.DB) error {
    fmt.Print(" [down] dropping state_outputs table...")

    _, err := db.NewDropTable().
        Model((*models.StateOutput)(nil)).
        IfExists().
        Exec(ctx)

    if err != nil {
        return fmt.Errorf("failed to drop state_outputs table: %w", err)
    }

    fmt.Println(" OK")
    return nil
}
```

**Lifecycle**:
1. **Created**: When Terraform state JSON uploaded via HTTP backend PUT endpoint
2. **Updated**: When state serial changes (atomic delete + insert)
3. **Deleted**: Cascade when parent state deleted from `states` table

**Invalidation Strategy**:
- Parse TF state serial from uploaded JSON
- Compare with cached `state_serial` in `state_outputs`
- If different: Transaction { delete old rows, insert new rows }
- Hook in `cmd/gridapi/internal/server/terraform_backend.go` PUT handler

**Query Patterns**:
```go
// Fast: Indexed lookup by state GUID
outputs, _ := repo.GetOutputsByState(ctx, stateGUID)

// Fast: Indexed search by output key (e.g., "find all states with 'vpc_id' output")
stateRefs, _ := repo.SearchOutputsByKey(ctx, "vpc_id")

// Atomic update on state write
repo.UpsertOutputs(ctx, stateGUID, newSerial, parsedOutputs)
```

---

### Enhanced State Information (Protobuf Response)
**Purpose**: Comprehensive state view including dependencies, dependents, and outputs

**Definition** (proto/state/v1/state.proto):
```protobuf
message GetStateInfoRequest {
  oneof state {
    string logic_id = 1;
    string guid = 2;
  }
}

message GetStateInfoResponse {
  string guid = 1;
  string logic_id = 2;
  BackendConfig backend_config = 3;

  // Incoming dependency edges (this state consumes outputs from these states)
  repeated DependencyEdge dependencies = 4;

  // Outgoing dependency edges (other states consume this state's outputs)
  repeated DependencyEdge dependents = 5;

  // Available outputs from this state's Terraform JSON
  repeated OutputKey outputs = 6;

  google.protobuf.Timestamp created_at = 7;
  google.protobuf.Timestamp updated_at = 8;
}
```

**Relationships**:
- `dependencies`: Where `to_guid == this.guid` (incoming edges)
- `dependents`: Where `from_guid == this.guid` (outgoing edges)
- `outputs`: Parsed from Terraform state JSON

**Query Strategy**:
```sql
-- Fetch dependencies (incoming edges)
SELECT * FROM edges WHERE to_guid = $1;

-- Fetch dependents (outgoing edges)
SELECT * FROM edges WHERE from_guid = $1;

-- Parse outputs from state JSON
SELECT data FROM states WHERE guid = $1;
-- Then parse outputs in application code
```

---

## Data Flow Diagrams

### State Creation with Directory Context
```
User runs: gridctl state create my-app
                 ↓
    1. Check .grid exists?
       → Yes + different GUID → Error
       → Yes + same logic-id → Skip write
       → No → Proceed
                 ↓
    2. Generate UUIDv7
                 ↓
    3. Call SDK.CreateState(guid, logic-id)
                 ↓
    4. Server creates state in DB
                 ↓
    5. Server returns BackendConfig
                 ↓
    6. CLI writes .grid file
       {
         "version": "1",
         "state_guid": "01JXXX...",
         "state_logic_id": "my-app",
         "server_url": "http://localhost:8080",
         "created_at": "2025-10-03T14:22:00Z",
         "updated_at": "2025-10-03T14:22:00Z"
       }
                 ↓
    7. CLI prints success + backend config
```

### Interactive Dependency Creation
```
User runs: gridctl deps add --from networking
                 ↓
    1. Read .grid → to_guid (default for --to)
                 ↓
    2. Check --non-interactive flag?
       → Yes → Require --output, error if not provided
       → No → Proceed to prompt
                 ↓
    3. Call SDK.ListStateOutputs("networking")
                 ↓
    4. Server reads state JSON, parses outputs
                 ↓
    5. Server returns []OutputKey
       [
         {key: "vpc_id", sensitive: false},
         {key: "subnet_id", sensitive: false},
         {key: "db_password", sensitive: true}
       ]
                 ↓
    6. CLI shows survey.MultiSelect:
       ☐ vpc_id
       ☐ subnet_id
       ☐ db_password (⚠️  sensitive)
                 ↓
    7. User selects: vpc_id, subnet_id
                 ↓
    8. CLI calls SDK.AddDependency() twice:
       - networking.vpc_id → my-app
       - networking.subnet_id → my-app
                 ↓
    9. Server creates 2 dependency edges
                 ↓
   10. CLI prints success for each edge
```

### Enhanced State Info Display
```
User runs: gridctl state get
                 ↓
    1. Read .grid → logic_id
                 ↓
    2. Call SDK.GetStateInfo(logic_id)
                 ↓
    3. Server fetches:
       - State row (guid, logic_id, timestamps)
       - Edges WHERE to_guid = X (dependencies)
       - Edges WHERE from_guid = X (dependents)
       - Parse outputs from state JSON
                 ↓
    4. Server returns GetStateInfoResponse
                 ↓
    5. CLI formats output:
       State: my-app (guid: 01JXXX...)
       Created: 2025-10-03 14:22:00

       Dependencies (consuming from):
         networking.vpc_id → vpc_id_input
         networking.subnet_id → subnet_id_input

       Dependents (consumed by):
         frontend.app_subnet_id
         backend.db_subnet_id

       Outputs:
         app_url
         app_version (⚠️  sensitive)
```

---

## Database Schema Changes

**No schema changes required for Phase 1** (outputs parsed on-demand from existing `states.data` JSONB column)

**Future Optimization** (Phase 2, if needed):
```sql
-- Optional caching table for frequently-accessed outputs
CREATE TABLE state_outputs (
    state_guid UUID NOT NULL REFERENCES states(guid) ON DELETE CASCADE,
    output_key TEXT NOT NULL,
    sensitive BOOLEAN NOT NULL DEFAULT FALSE,
    state_serial BIGINT NOT NULL,  -- For cache invalidation
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (state_guid, output_key)
);

CREATE INDEX idx_state_outputs_guid ON state_outputs(state_guid);
```

**Invalidation Strategy**: When Terraform state updated via HTTP backend PUT, compare `serial` field and delete rows where `state_guid = X AND state_serial != new_serial`.

---

## Validation & Constraints

### DirectoryContext File
- **Size limit**: <1KB (typical: ~200 bytes)
- **Permissions**: 0644 (user read/write, group/other read-only)
- **Location**: Must be named `.grid` in current working directory
- **Encoding**: UTF-8 JSON with 2-space indentation

### OutputKey
- **Key length**: 1-64 characters (Terraform output name constraint)
- **Sensitive flag**: Boolean (from TF state metadata)
- **Uniqueness**: Per-state (no duplicate output names within same state)

### GetStateInfoResponse
- **Max dependencies**: 1000 (sanity limit, prevents pathological graphs)
- **Max dependents**: 1000
- **Max outputs**: 200 (reasonable Terraform state limit)

---

## State Transitions

### .grid File Lifecycle
```
[No .grid] --create--> [.grid exists, GUID=X]
                             |
                             |--force flag--> [.grid exists, GUID=Y]
                             |
                             |--corrupted--> [Warning logged, ignored]
                             |
                             |--state deleted on server--> [Error: run create]
```

### State Output Availability
```
[State created, no TF state JSON] --> outputs = []
                ↓
    [TF state uploaded via backend] --> outputs parsed from JSON
                ↓
    [TF state updated, serial++] --> outputs re-parsed (cache invalidated)
```

---

## API Contract Summary

### New RPC Methods

1. **ListStateOutputs**
   - Input: `logic_id` or `guid`
   - Output: `[]OutputKey` (key + sensitive flag)
   - Used by: Interactive deps add prompt

2. **GetStateInfo** (enhanced/new)
   - Input: `logic_id` or `guid`
   - Output: State metadata + dependencies + dependents + outputs
   - Used by: `gridctl state get` command

### Modified RPC Methods
None (existing CreateState, AddDependency unchanged)

---

This data model supports all functional requirements (FR-001 through FR-022) with minimal schema changes and follows Constitution Principle VII (Simplicity).
