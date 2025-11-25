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

**New Status Value**: `schema-invalid`

**Go Model** (`internal/db/models/edge.go`):

```go
type EdgeStatus string

const (
    EdgeStatusPending       EdgeStatus = "pending"
    EdgeStatusDirty         EdgeStatus = "dirty"
    EdgeStatusClean         EdgeStatus = "clean"
    EdgeStatusMock          EdgeStatus = "mock"
    EdgeStatusMissingOutput EdgeStatus = "missing-output"
    EdgeStatusSchemaInvalid EdgeStatus = "schema-invalid"  // NEW
)
```

**Note**: No database migration needed - `status` is TEXT type, new value added in application.

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

### EdgeStatus Enum (Extended)

**File**: `proto/state/v1/state.proto`

```protobuf
enum EdgeStatus {
  EDGE_STATUS_UNSPECIFIED = 0;
  EDGE_STATUS_PENDING = 1;
  EDGE_STATUS_CLEAN = 2;
  EDGE_STATUS_DIRTY = 3;
  EDGE_STATUS_POTENTIALLY_STALE = 4;
  EDGE_STATUS_MOCK = 5;
  EDGE_STATUS_MISSING_OUTPUT = 6;
  EDGE_STATUS_SCHEMA_INVALID = 7;  // NEW
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

export type EdgeStatus =
  | 'pending'
  | 'clean'
  | 'dirty'
  | 'potentially-stale'
  | 'mock'
  | 'missing-output'
  | 'schema-invalid';  // NEW
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

### Edge Status with Schema Validation

```
┌──────────────────────────────────────────────────────────────┐
│              Edge Status Priority                             │
├──────────────────────────────────────────────────────────────┤
│                                                               │
│  [Output Validation Status Check] (highest priority)          │
│       │                                                       │
│       ├──[validation_status = "invalid"]──► status = "schema-invalid"
│       │                                                       │
│       └──[validation_status ≠ "invalid"]                      │
│            │                                                  │
│            ▼                                                  │
│  [Output Existence Check]                                     │
│       │                                                       │
│       ├──[Output missing]──► status = "missing-output"        │
│       │                                                       │
│       └──[Output exists]                                      │
│            │                                                  │
│            ▼                                                  │
│  [Fingerprint Check]                                          │
│       │                                                       │
│       ├──[Digest changed]──► status = "dirty"                 │
│       │                                                       │
│       └──[Digest unchanged]──► status = "clean"               │
│                                                               │
└──────────────────────────────────────────────────────────────┘
```

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

### Edge Status

- `schema-invalid` status takes priority over fingerprint-based statuses
- Edge status MUST be updated atomically with validation status
- Clearing `schema-invalid` requires validation to pass on next state upload
