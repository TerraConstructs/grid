# Data Model: Output Schema Support - Phase 2

**Feature Branch**: `010-output-schema-support`
**Date**: 2025-11-25

## Entity Changes

### 1. StateOutput (Extended)

**Table**: `state_outputs`

**Current Schema** (Phase 1 complete):
```sql
CREATE TABLE state_outputs (
    state_guid   UUID NOT NULL REFERENCES states(guid) ON DELETE CASCADE,
    output_key   TEXT NOT NULL,
    sensitive    BOOLEAN NOT NULL DEFAULT FALSE,
    state_serial BIGINT NOT NULL,
    schema_json  TEXT,  -- Phase 1: JSON Schema definition
    created_at   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (state_guid, output_key)
);
```

**Phase 2 Extensions**:

```sql
-- Migration: 20251125_add_schema_source_and_validation.go

ALTER TABLE state_outputs
ADD COLUMN schema_source TEXT CHECK (schema_source IN ('manual', 'inferred'));

ALTER TABLE state_outputs
ADD COLUMN validation_status TEXT CHECK (validation_status IN ('valid', 'invalid', 'error'));

ALTER TABLE state_outputs
ADD COLUMN validation_error TEXT;

ALTER TABLE state_outputs
ADD COLUMN validated_at TIMESTAMPTZ;

-- Index for validation status queries
CREATE INDEX idx_state_outputs_validation_status
ON state_outputs(state_guid)
WHERE validation_status IS NOT NULL;
```

**Go Model** (`internal/db/models/state_output.go`):

```go
type StateOutput struct {
    bun.BaseModel `bun:"table:state_outputs,alias:so"`

    // Primary key
    StateGUID   string    `bun:"state_guid,pk,type:uuid,notnull"`
    OutputKey   string    `bun:"output_key,pk,type:text,notnull"`

    // Core fields
    Sensitive   bool      `bun:"sensitive,notnull,default:false"`
    StateSerial int64     `bun:"state_serial,notnull"`

    // Schema fields (Phase 1)
    SchemaJSON  *string   `bun:"schema_json,type:text,nullzero"`

    // Schema source (Phase 2A: Inference)
    SchemaSource *string  `bun:"schema_source,type:text,nullzero"` // "manual" | "inferred"

    // Validation fields (Phase 2B: Validation)
    ValidationStatus *string    `bun:"validation_status,type:text,nullzero"` // "valid" | "invalid" | "error"
    ValidationError  *string    `bun:"validation_error,type:text,nullzero"`
    ValidatedAt      *time.Time `bun:"validated_at,type:timestamptz,nullzero"`

    // Timestamps
    CreatedAt   time.Time `bun:"created_at,notnull,default:current_timestamp"`
    UpdatedAt   time.Time `bun:"updated_at,notnull,default:current_timestamp"`

    // Relationships
    State *State `bun:"rel:belongs-to,join:state_guid=guid"`
}
```

### 2. Edge (Extended)

**Table**: `edges`

**New Status Values**: `clean-invalid`, `dirty-invalid`

**Design Change (2025-11-27)**: Edge status now captures **TWO orthogonal dimensions**:
1. **Drift**: clean (in_digest == out_digest) vs dirty (in_digest != out_digest)
2. **Validation**: valid (passes schema) vs invalid (fails schema)

**Go Model** (`internal/db/models/edge.go`):

```go
type EdgeStatus string

const (
    EdgeStatusPending          EdgeStatus = "pending"           // Initial state, no observation yet
    EdgeStatusClean            EdgeStatus = "clean"             // in_digest == out_digest && valid
    EdgeStatusCleanInvalid     EdgeStatus = "clean-invalid"     // in_digest == out_digest && invalid (NEW)
    EdgeStatusDirty            EdgeStatus = "dirty"             // in_digest != out_digest && valid
    EdgeStatusDirtyInvalid     EdgeStatus = "dirty-invalid"     // in_digest != out_digest && invalid (NEW)
    EdgeStatusPotentiallyStale EdgeStatus = "potentially-stale" // Transitive upstream dirty
    EdgeStatusMock             EdgeStatus = "mock"              // Using mock value, real output not yet exists
    EdgeStatusMissingOutput    EdgeStatus = "missing-output"    // Producer output key removed
)
```

**Composite Status Matrix**:

| Drift / Validation | Valid (passes schema or no schema) | Invalid (fails schema) |
|--------------------|------------------------------------|------------------------|
| **Clean** (in_digest == out_digest) | `clean` | `clean-invalid` |
| **Dirty** (in_digest != out_digest) | `dirty` | `dirty-invalid` |

**Note**: No database migration needed - `status` is TEXT type, new values added in application.

---

## Repository Interface Changes

### StateOutputRepository (Extended)

**File**: `internal/repository/interface.go`

```go
// OutputKey represents a Terraform output with metadata
type OutputKey struct {
    Key              string
    Sensitive        bool
    SchemaJSON       *string  // Phase 1
    SchemaSource     *string  // Phase 2A: "manual" | "inferred"
    ValidationStatus *string  // Phase 2B: "valid" | "invalid" | "error"
    ValidationError  *string  // Phase 2B
    ValidatedAt      *time.Time // Phase 2B
}

// ValidationResult represents the outcome of schema validation
type ValidationResult struct {
    Status      string     // "valid", "invalid", "error"
    Error       *string    // Error message if status != "valid"
    ValidatedAt time.Time
}

// OutputKeyWithValidation pairs an output with its validation result
type OutputKeyWithValidation struct {
    OutputKey
    Validation *ValidationResult // nil if no schema to validate
}

type StateOutputRepository interface {
    // Existing methods (unchanged)
    UpsertOutputs(ctx context.Context, stateGUID string, serial int64, outputs []OutputKey) error
    GetOutputsByState(ctx context.Context, stateGUID string) ([]OutputKey, error)
    SearchOutputsByKey(ctx context.Context, outputKey string) ([]StateOutputRef, error)
    DeleteOutputsByState(ctx context.Context, stateGUID string) error
    SetOutputSchema(ctx context.Context, stateGUID string, outputKey string, schemaJSON string) error
    GetOutputSchema(ctx context.Context, stateGUID string, outputKey string) (string, error)

    // Phase 2A: Schema inference
    SetOutputSchemaWithSource(ctx context.Context, stateGUID, outputKey, schemaJSON, source string) error
    GetOutputsWithoutSchema(ctx context.Context, stateGUID string) ([]string, error) // Output keys needing inference

    // Phase 2B: Validation
    UpsertOutputsWithValidation(ctx context.Context, stateGUID string, serial int64, outputs []OutputKeyWithValidation) error
    UpdateValidationStatus(ctx context.Context, stateGUID, outputKey string, result ValidationResult) error
    GetSchemasForState(ctx context.Context, stateGUID string) (map[string]string, error) // outputKey -> schemaJSON
}
```

---

## Service Layer Types

### Validation Service

**Package**: `internal/services/validation`

```go
// ValidationStatus represents the outcome of validating an output
type ValidationStatus string

const (
    ValidationStatusValid   ValidationStatus = "valid"
    ValidationStatusInvalid ValidationStatus = "invalid"
    ValidationStatusError   ValidationStatus = "error"
)

// OutputValidation pairs an output key with its validation result
type OutputValidation struct {
    OutputKey string
    Result    *ValidationResult
}

// ValidationResult contains validation outcome details
type ValidationResult struct {
    Status      ValidationStatus
    Error       *string    // Error message with path information
    ValidatedAt time.Time
}

// Validator validates output values against JSON Schemas
type Validator interface {
    // ValidateOutputs validates all outputs that have schemas
    // Returns validation results only for outputs with schemas
    ValidateOutputs(ctx context.Context, stateGUID string, outputs map[string]interface{}) ([]OutputValidation, error)

    // InvalidateCache clears cached schemas for a state/output
    InvalidateCache(stateGUID, outputKey string)
}
```

### Inference Service

**Package**: `internal/services/inference`

```go
// InferredSchema represents a schema generated from output data
type InferredSchema struct {
    OutputKey  string
    SchemaJSON string
}

// SchemaInferrer infers JSON Schemas from output values
type SchemaInferrer interface {
    // InferSchemas infers schemas for outputs without existing schemas
    // Only infers for outputs in needsSchema list
    InferSchemas(ctx context.Context, stateGUID string, outputs map[string]interface{}, needsSchema []string) ([]InferredSchema, error)
}
```

---

## Proto Changes

### OutputKey Message (Extended)

**File**: `proto/state/v1/state.proto`

```protobuf
message OutputKey {
  string key = 1;
  bool sensitive = 2;

  // Schema fields (Phase 1 + 2A)
  optional string schema_json = 3;
  optional string schema_source = 4;  // "manual" | "inferred"

  // Validation fields (Phase 2B)
  optional string validation_status = 5;  // "valid" | "invalid" | "error"
  optional string validation_error = 6;
  optional google.protobuf.Timestamp validated_at = 7;
}
```

### Edge Message (Extended)

**File**: `proto/state/v1/state.proto`

**Note**: EdgeStatus is stored as `string` (not protobuf enum) in Edge message.

```protobuf
message Edge {
    // ... other fields ...

    // Status indicates the synchronization and validation state of the edge.
    // Edge status combines two orthogonal dimensions:
    // 1. Drift: clean (in_digest == out_digest) vs dirty (in_digest != out_digest)
    // 2. Validation: valid (passes schema) vs invalid (fails schema)
    //
    // Possible values:
    // - "pending": Initial state, no observation yet
    // - "clean": in_digest == out_digest && output passes schema validation (or no schema)
    // - "clean-invalid": in_digest == out_digest && output fails schema validation
    // - "dirty": in_digest != out_digest && output passes schema validation (or no schema)
    // - "dirty-invalid": in_digest != out_digest && output fails schema validation
    // - "potentially-stale": Transitive upstream dirty
    // - "mock": Using mock_value, real output doesn't exist yet
    // - "missing-output": Producer doesn't have the required output key
    string status = 8;
}
```

---

## TypeScript SDK Types

**File**: `js/sdk/src/models/state-info.ts`

```typescript
export interface OutputKey {
  /** Output name from Terraform state */
  key: string;

  /** Whether output is marked sensitive in Terraform */
  sensitive: boolean;

  /** JSON Schema definition (optional) */
  schema_json?: string;

  /** Schema source: "manual" (explicitly set) or "inferred" (auto-generated) */
  schema_source?: 'manual' | 'inferred';

  /** Validation status (present if schema is set) */
  validation_status?: 'valid' | 'invalid' | 'error';

  /** Validation error message (present if validation_status != 'valid') */
  validation_error?: string;

  /** Last validation timestamp (ISO 8601) */
  validated_at?: string;
}

/** Edge synchronization and validation status */
export type EdgeStatus =
  | 'pending'           // Edge created, no digest values yet
  | 'clean'             // in_digest === out_digest && valid (synchronized & valid)
  | 'clean-invalid'     // in_digest === out_digest && invalid (synchronized but fails schema)
  | 'dirty'             // in_digest !== out_digest && valid (out of sync but valid)
  | 'dirty-invalid'     // in_digest !== out_digest && invalid (out of sync AND fails schema)
  | 'potentially-stale' // Producer updated, consumer not re-evaluated
  | 'mock'              // Using mock_value_json
  | 'missing-output';   // Producer doesn't have required output
```

---

## State Transitions

### Schema Source Lifecycle

```
┌──────────────────────────────────────────────────────────────┐
│                      Schema Source                            │
├──────────────────────────────────────────────────────────────┤
│                                                               │
│  [Output Created]                                             │
│       │                                                       │
│       ▼                                                       │
│   schema_source = NULL (no schema)                            │
│       │                                                       │
│       ├──[SetOutputSchema called]──► schema_source = "manual" │
│       │                                                       │
│       └──[State upload + inference]──► schema_source = "inferred"
│                                                               │
│   schema_source = "inferred"                                  │
│       │                                                       │
│       └──[SetOutputSchema called]──► schema_source = "manual" │
│          (overwrites inferred schema)                         │
│                                                               │
│   schema_source = "manual"                                    │
│       │                                                       │
│       └──[SetOutputSchema called]──► schema_source = "manual" │
│          (updates existing manual schema)                     │
│                                                               │
└──────────────────────────────────────────────────────────────┘
```

### Validation Status Lifecycle

```
┌──────────────────────────────────────────────────────────────┐
│                    Validation Status                          │
├──────────────────────────────────────────────────────────────┤
│                                                               │
│  [State Upload Received]                                      │
│       │                                                       │
│       ├──[No schema exists]──► validation_status = NULL       │
│       │                                                       │
│       └──[Schema exists]                                      │
│            │                                                  │
│            ├──[Value matches schema]──► validation_status = "valid"
│            │                                                  │
│            ├──[Value fails schema]──► validation_status = "invalid"
│            │                          validation_error = "<details>"
│            │                                                  │
│            └──[Validation system error]──► validation_status = "error"
│                                            validation_error = "<details>"
│                                                               │
│  [All paths]──► validated_at = NOW()                          │
│                                                               │
└──────────────────────────────────────────────────────────────┘
```

### Edge Status Derivation (Composite Model)

**Updated 2025-11-27**: Edge status now combines drift and validation dimensions.

```
┌────────────────────────────────────────────────────────────────┐
│           Edge Status Derivation Logic                         │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│  [1. Output Existence Check] (highest priority)                 │
│       │                                                         │
│       ├──[Output missing]──► status = "missing-output"          │
│       │                     (overrides all other checks)        │
│       │                                                         │
│       └──[Output exists]                                        │
│            │                                                    │
│            ▼                                                    │
│  [2. Compute Drift Dimension]                                   │
│       │                                                         │
│       ├──[in_digest == out_digest]──► drift = "clean"           │
│       │                                                         │
│       └──[in_digest != out_digest]──► drift = "dirty"           │
│            │                                                    │
│            ▼                                                    │
│  [3. Compute Validation Dimension]                              │
│       │                                                         │
│       ├──[validation_status == "invalid"]──► validation = "invalid"
│       │                                                         │
│       └──[validation_status != "invalid" OR NULL]──► validation = "valid"
│            │                              (no schema = valid)   │
│            ▼                                                    │
│  [4. Combine Dimensions]                                        │
│       │                                                         │
│       ├──[drift="clean" && validation="valid"]──► "clean"       │
│       ├──[drift="clean" && validation="invalid"]──► "clean-invalid"
│       ├──[drift="dirty" && validation="valid"]──► "dirty"       │
│       └──[drift="dirty" && validation="invalid"]──► "dirty-invalid"
│                                                                 │
│  [Special Cases]                                                │
│   • No observation yet (in_digest=NULL) → "pending"             │
│   • Mock edge (mock_value set) → "mock"                         │
│                                                                 │
└────────────────────────────────────────────────────────────────┘
```

**Status Matrix**:

| Condition | Drift | Validation | Result Status |
|-----------|-------|------------|---------------|
| Output missing | N/A | N/A | `missing-output` |
| in == out && valid | clean | valid | `clean` |
| in == out && invalid | clean | invalid | `clean-invalid` |
| in != out && valid | dirty | valid | `dirty` |
| in != out && invalid | dirty | invalid | `dirty-invalid` |
| No observation | N/A | N/A | `pending` |
| Mock value | N/A | N/A | `mock` |

**User Experience**:
- `clean`: "Your dependency is synchronized and valid" ✅
- `clean-invalid`: "Your dependency is synchronized but violates schema" ⚠️
- `dirty`: "Your dependency is out of sync (run terraform apply)" 🔄
- `dirty-invalid`: "Your dependency is out of sync AND violates schema" ❌

---

## Data Integrity Constraints

### Schema Source

- `schema_source` MUST be `manual` when set via `SetOutputSchema` RPC
- `schema_source` MUST be `inferred` when set via inference during state upload
- `schema_source` CAN be NULL only when `schema_json` is NULL
- Setting `schema_json` MUST also set `schema_source`

### Validation Status

- `validation_status` CAN be NULL (no schema exists)
- `validation_error` MUST be NULL when `validation_status = "valid"`
- `validation_error` SHOULD be non-NULL when `validation_status` is `"invalid"` or `"error"`
- `validated_at` MUST be set whenever `validation_status` is non-NULL

### Edge Status (Updated 2025-11-27)

- Edge status combines **drift** (clean/dirty) and **validation** (valid/invalid) dimensions
- `missing-output` takes highest priority (overrides drift and validation)
- Edge status MUST be derived from both `in_digest`/`out_digest` comparison AND `validation_status`
- Edge status MUST be updated atomically with validation status (via JOIN in query)
- Validation dimension changes when:
  - User adds/updates schema on existing output (may transition clean → clean-invalid)
  - User removes schema (transitions *-invalid → *)
  - Output value changes and validation re-runs (may transition valid ↔ invalid)
- Status transitions preserve information about both dimensions:
  - `clean` → `clean-invalid`: Schema added and output fails (consumer still synchronized)
  - `dirty-invalid` → `dirty`: Validation passes after schema fix (consumer still stale)
  - `dirty` → `clean`: Consumer applies terraform (validation already passing)
