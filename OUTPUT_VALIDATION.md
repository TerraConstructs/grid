# Output Schema Validation Implementation Plan

## Overview

This document describes the implementation plan for validating Terraform state output values against their associated JSON Schemas when a tfstate POST request is processed. This feature extends the [output schema storage capability](OUTPUT_SCHEMA_IMPLEMENTATION.md) by adding runtime validation.

## Current State Upload Flow

### 1. Terraform HTTP Backend Endpoint
**File:** `cmd/gridapi/internal/server/tfstate_handlers.go:74` (`UpdateState`)

```
POST /tfstate/{guid}
  ├─> Read JSON body
  ├─> Validate JSON syntax
  ├─> Extract lock ID from query param
  └─> Call service.UpdateStateContent(ctx, guid, body, lockID)
```

### 2. State Service Layer
**File:** `cmd/gridapi/internal/services/state/service.go:217` (`UpdateStateContent`)

```go
func (s *Service) UpdateStateContent(ctx, guid, content, lockID) (*StateUpdateResult, error) {
  // 1. Parse state once to get serial, keys, and values
  parsed, err := tfstate.ParseState(content)

  // 2. Atomic update: state content + output cache (FR-027 compliance)
  err = s.repo.UpdateContentAndUpsertOutputs(ctx, guid, content, lockID,
                                              parsed.Serial, parsed.Keys)

  // 3. Return parsed values for edge job
  return &StateUpdateResult{
    Summary:      &summary,
    OutputValues: parsed.Values,
  }
}
```

### 3. Edge Update Job (Asynchronous)
**File:** `cmd/gridapi/internal/server/update_edges.go:46` (`UpdateEdgesWithOutputs`)

```go
// Triggered asynchronously after state upload (fire-and-forget)
go h.edgeUpdater.UpdateEdgesWithOutputs(ctx, guid, result.OutputValues)

// Updates edge statuses based on:
// - Output existence (missing-output status)
// - Output fingerprint changes (dirty vs clean)
// - Mock → real transitions
```

## JSON Schema Validation Libraries (Go)

### Recommended: `github.com/santhosh-tekuri/jsonschema/v6`

**Rationale:**
- ✅ **Draft 7 compliance** - Matches the schema format used in our test fixtures
- ✅ **Performance** - Pre-compiles schemas for fast validation
- ✅ **Detailed errors** - Provides validation error paths and descriptions
- ✅ **Active maintenance** - Latest version (v6) released 2024
- ✅ **No CGO dependencies** - Pure Go implementation
- ✅ **Remote schema support** - Can resolve `$ref` to external schemas
- ✅ **Custom formats** - Extensible validation for domain-specific formats

**Installation:**
```bash
cd cmd/gridapi
go get github.com/santhosh-tekuri/jsonschema/v6@latest
```

**Basic Usage:**
```go
import "github.com/santhosh-tekuri/jsonschema/v6"

// Compile schema once (cache this)
compiler := jsonschema.NewCompiler()
schema, err := compiler.Compile("schema.json", schemaJSON)

// Validate data
var outputValue interface{}
err = schema.Validate(outputValue)
if err != nil {
  // err contains detailed validation errors
  validationErr := err.(*jsonschema.ValidationError)
  // validationErr.InstanceLocation - path to invalid data
  // validationErr.Message - human-readable error
}
```

### Alternative Libraries (Not Recommended)

| Library | Draft | Pros | Cons |
|---------|-------|------|------|
| `github.com/xeipuuv/gojsonschema` | Draft 4 | Mature, widely used | Draft 4 only, no recent updates |
| `github.com/qri-io/jsonschema` | Draft 7 | Good errors | Less active, smaller community |
| `github.com/go-openapi/validate` | Swagger | OpenAPI focus | Not general-purpose JSON Schema |

## Implementation Design

### Architecture Overview

```
┌────────────────────────────────────────────────────────────────┐
│ POST /tfstate/{guid}  (tfstate_handlers.go)                    │
└───────────────────┬────────────────────────────────────────────┘
                    │
                    ▼
┌────────────────────────────────────────────────────────────────┐
│ service.UpdateStateContent  (state/service.go)                 │
│                                                                 │
│  1. Parse tfstate → serial, keys, values                       │
│  2. **NEW: Validate outputs against schemas**                  │
│  3. Atomic update: content + outputs + validation results      │
│  4. Return: summary + values + **validation results**          │
└───────────────────┬────────────────────────────────────────────┘
                    │
                    ├─> (synchronous) repo.UpdateContentAndUpsertOutputs
                    │
                    └─> (async) EdgeUpdateJob.UpdateEdgesWithOutputs
                         ├─> Update based on fingerprints
                         └─> **NEW: Update based on validation results**
```

### Key Design Decisions

#### 1. **Where to Validate**
**Location:** `service.UpdateStateContent()` - after parsing, before repository update

**Rationale:**
- ✅ Service layer owns business logic
- ✅ Can access outputRepo to fetch schemas
- ✅ Happens in same transaction context
- ✅ Results available for both DB update and edge job

#### 2. **Schema Caching Strategy**
**Approach:** In-memory LRU cache with TTL (5 minutes default)

**Rationale:**
- ✅ Schemas rarely change (set manually, not on every state upload)
- ✅ Avoids DB query per output per state upload
- ✅ TTL prevents stale schemas after updates
- ✅ LRU prevents unbounded memory growth

**Implementation:**
```go
type SchemaCache struct {
  cache *lru.Cache // github.com/hashicorp/golang-lru
  ttl   time.Duration
}

type cachedSchema struct {
  compiled  *jsonschema.Schema
  expiresAt time.Time
}

func (c *SchemaCache) Get(stateGUID, outputKey string) (*jsonschema.Schema, error) {
  key := fmt.Sprintf("%s:%s", stateGUID, outputKey)
  if val, ok := c.cache.Get(key); ok {
    cached := val.(*cachedSchema)
    if time.Now().Before(cached.expiresAt) {
      return cached.compiled, nil
    }
  }
  return nil, ErrCacheMiss
}
```

#### 3. **Validation Result Storage**
**Approach:** New column `validation_status` in `state_outputs` table

**Schema:**
```sql
ALTER TABLE state_outputs ADD COLUMN validation_status TEXT;
-- Values: NULL (no schema), 'valid', 'invalid', 'error'

ALTER TABLE state_outputs ADD COLUMN validation_error TEXT;
-- Stores validation error message if status='invalid' or 'error'

ALTER TABLE state_outputs ADD COLUMN validated_at TIMESTAMPTZ;
-- When validation last ran
```

**Repository Interface Extension:**
```go
type ValidationResult struct {
  Status    ValidationStatus // valid, invalid, error
  Error     *string          // Validation error message
  ValidatedAt time.Time
}

type ValidationStatus string

const (
  ValidationStatusValid   ValidationStatus = "valid"
  ValidationStatusInvalid ValidationStatus = "invalid"
  ValidationStatusError   ValidationStatus = "error"
)

// Update repository interface
type StateOutputRepository interface {
  // ... existing methods ...

  // UpsertOutputsWithValidation atomically updates outputs and validation results
  UpsertOutputsWithValidation(ctx context.Context, stateGUID string, serial int64,
                               outputs []OutputKeyWithValidation) error

  // GetValidationResult retrieves validation status for an output
  GetValidationResult(ctx context.Context, stateGUID string, outputKey string) (*ValidationResult, error)
}

type OutputKeyWithValidation struct {
  OutputKey
  Validation *ValidationResult
}
```

#### 4. **Edge Status Updates**
**New Edge Status:** `schema-invalid`

**Status Transition Logic:**
```
Current Logic (output_edges.go:114):
  if inDigest != outDigest → status = dirty
  if inDigest == outDigest → status = clean
  if inDigest == "" → status = pending
  if output missing → status = missing-output

NEW Logic:
  if validation_status == 'invalid' → status = schema-invalid (highest priority)
  else if inDigest != outDigest → status = dirty
  else if inDigest == outDigest → status = clean
  ...
```

**Rationale:**
- Schema validation failures are more severe than drift
- Prevents consumers from using invalid data
- Clear signal that producer needs to fix outputs

#### 5. **Error Handling**
**Approach:** Validation errors do NOT block state upload

**Rationale:**
- ✅ Terraform workflow should not break if schema is wrong
- ✅ Validation is advisory, not enforcement (for now)
- ✅ Failed uploads break IaC pipelines (too disruptive)
- ✅ Edge status provides visibility without blocking

**Error Scenarios:**
| Scenario | Behavior |
|----------|----------|
| Schema JSON is invalid | Log error, set status='error', continue |
| Schema compilation fails | Log error, set status='error', continue |
| Output value doesn't match schema | Set status='invalid', store error, continue |
| Output has no schema | Skip validation, status=NULL |
| DB failure storing validation | Log error, continue (best effort) |

#### 6. **Future: Strict Mode**
**Feature:** Optional enforcement mode (reject upload if validation fails)

**Configuration:**
```go
// Per-state or global flag
type State struct {
  // ... existing fields ...
  ValidateSchemaStrict bool `bun:"validate_schema_strict,notnull,default:false"`
}
```

**Implementation Note:** Add this AFTER initial soft validation is proven stable.

## Implementation Steps

### Phase 1: Foundation (Days 1-2)

#### Step 1.1: Add Validation Columns
**File:** `cmd/gridapi/internal/migrations/YYYYMMDDHHMMSS_add_output_validation.go`

```go
func up_YYYYMMDDHHMMSS(ctx context.Context, db *bun.DB) error {
  _, err := db.Exec(`
    ALTER TABLE state_outputs
    ADD COLUMN IF NOT EXISTS validation_status TEXT,
    ADD COLUMN IF NOT EXISTS validation_error TEXT,
    ADD COLUMN IF NOT EXISTS validated_at TIMESTAMPTZ
  `)
  return err
}
```

#### Step 1.2: Update Models
**File:** `cmd/gridapi/internal/db/models/state_output.go`

```go
type StateOutput struct {
  // ... existing fields ...

  SchemaJSON  *string   `bun:"schema_json,type:text,nullzero"`

  // NEW: Validation fields
  ValidationStatus *string    `bun:"validation_status,type:text,nullzero"`
  ValidationError  *string    `bun:"validation_error,type:text,nullzero"`
  ValidatedAt      *time.Time `bun:"validated_at,type:timestamptz,nullzero"`
}
```

#### Step 1.3: Install Dependency
```bash
cd cmd/gridapi
go get github.com/santhosh-tekuri/jsonschema/v6@latest
go get github.com/hashicorp/golang-lru/v2@latest
```

### Phase 2: Validation Service (Days 3-4)

#### Step 2.1: Create Validator Service
**File:** `cmd/gridapi/internal/services/validation/validator.go`

```go
package validation

import (
  "context"
  "encoding/json"
  "fmt"
  "sync"
  "time"

  lru "github.com/hashicorp/golang-lru/v2"
  "github.com/santhosh-tekuri/jsonschema/v6"
  "github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
)

type ValidationStatus string

const (
  ValidationStatusValid   ValidationStatus = "valid"
  ValidationStatusInvalid ValidationStatus = "invalid"
  ValidationStatusError   ValidationStatus = "error"
)

type ValidationResult struct {
  Status      ValidationStatus
  Error       *string
  ValidatedAt time.Time
}

type OutputValidation struct {
  OutputKey string
  Result    *ValidationResult
}

type Validator struct {
  outputRepo  repository.StateOutputRepository
  cache       *SchemaCache
  compiler    *jsonschema.Compiler
  mu          sync.RWMutex
}

type SchemaCache struct {
  cache *lru.Cache[string, *cachedSchema]
  ttl   time.Duration
}

type cachedSchema struct {
  compiled  *jsonschema.Schema
  expiresAt time.Time
}

func NewValidator(outputRepo repository.StateOutputRepository) *Validator {
  cache, _ := lru.New[string, *cachedSchema](1000) // Cache up to 1000 schemas

  return &Validator{
    outputRepo: outputRepo,
    cache: &SchemaCache{
      cache: cache,
      ttl:   5 * time.Minute,
    },
    compiler: jsonschema.NewCompiler(),
  }
}

// ValidateOutputs validates all outputs that have schemas
func (v *Validator) ValidateOutputs(ctx context.Context, stateGUID string,
                                     outputs map[string]interface{}) ([]OutputValidation, error) {
  results := make([]OutputValidation, 0)

  for outputKey, outputValue := range outputs {
    result := v.validateSingleOutput(ctx, stateGUID, outputKey, outputValue)
    if result != nil {
      results = append(results, OutputValidation{
        OutputKey: outputKey,
        Result:    result,
      })
    }
  }

  return results, nil
}

func (v *Validator) validateSingleOutput(ctx context.Context, stateGUID, outputKey string,
                                          outputValue interface{}) *ValidationResult {
  now := time.Now()

  // 1. Get schema (from cache or DB)
  schema, err := v.getCompiledSchema(ctx, stateGUID, outputKey)
  if err != nil {
    if err == repository.ErrNotFound || err == ErrNoSchema {
      return nil // No schema = skip validation
    }

    // Schema fetch/compile failed
    errMsg := fmt.Sprintf("failed to get schema: %v", err)
    return &ValidationResult{
      Status:      ValidationStatusError,
      Error:       &errMsg,
      ValidatedAt: now,
    }
  }

  // 2. Validate output value against schema
  err = schema.Validate(outputValue)
  if err != nil {
    // Validation failed
    errMsg := err.Error()
    return &ValidationResult{
      Status:      ValidationStatusInvalid,
      Error:       &errMsg,
      ValidatedAt: now,
    }
  }

  // Validation passed
  return &ValidationResult{
    Status:      ValidationStatusValid,
    Error:       nil,
    ValidatedAt: now,
  }
}

var ErrNoSchema = fmt.Errorf("no schema defined")

func (v *Validator) getCompiledSchema(ctx context.Context, stateGUID, outputKey string) (*jsonschema.Schema, error) {
  cacheKey := fmt.Sprintf("%s:%s", stateGUID, outputKey)

  // Check cache
  if cached := v.cache.get(cacheKey); cached != nil {
    return cached, nil
  }

  // Cache miss - fetch from DB
  schemaJSON, err := v.outputRepo.GetOutputSchema(ctx, stateGUID, outputKey)
  if err != nil {
    return nil, err
  }
  if schemaJSON == "" {
    return nil, ErrNoSchema
  }

  // Compile schema
  schema, err := v.compiler.Compile(cacheKey, []byte(schemaJSON))
  if err != nil {
    return nil, fmt.Errorf("compile schema: %w", err)
  }

  // Store in cache
  v.cache.put(cacheKey, schema)

  return schema, nil
}

func (c *SchemaCache) get(key string) *jsonschema.Schema {
  if val, ok := c.cache.Get(key); ok {
    if time.Now().Before(val.expiresAt) {
      return val.compiled
    }
    c.cache.Remove(key) // Expired
  }
  return nil
}

func (c *SchemaCache) put(key string, schema *jsonschema.Schema) {
  c.cache.Add(key, &cachedSchema{
    compiled:  schema,
    expiresAt: time.Now().Add(c.ttl),
  })
}

// InvalidateCache clears cached schemas for a state (call after SetOutputSchema)
func (v *Validator) InvalidateCache(stateGUID, outputKey string) {
  cacheKey := fmt.Sprintf("%s:%s", stateGUID, outputKey)
  v.cache.cache.Remove(cacheKey)
}
```

#### Step 2.2: Update Repository Interface
**File:** `cmd/gridapi/internal/repository/interface.go`

```go
type OutputKeyWithValidation struct {
  OutputKey
  Validation *ValidationResult
}

type ValidationResult struct {
  Status      ValidationStatus
  Error       *string
  ValidatedAt time.Time
}

type ValidationStatus string

const (
  ValidationStatusValid   ValidationStatus = "valid"
  ValidationStatusInvalid ValidationStatus = "invalid"
  ValidationStatusError   ValidationStatus = "error"
)

type StateOutputRepository interface {
  // ... existing methods ...

  // UpsertOutputsWithValidation atomically updates outputs and validation results
  UpsertOutputsWithValidation(ctx context.Context, stateGUID string, serial int64,
                               outputs []OutputKeyWithValidation) error
}
```

### Phase 3: Integration (Days 5-6)

#### Step 3.1: Update State Service
**File:** `cmd/gridapi/internal/services/state/service.go`

```go
import (
  "github.com/terraconstructs/grid/cmd/gridapi/internal/services/validation"
)

type Service struct {
  repo       repository.StateRepository
  outputRepo repository.StateOutputRepository
  edgeRepo   repository.EdgeRepository
  policyRepo repository.LabelPolicyRepository
  validator  *validation.Validator
  serverURL  string
}

func NewService(repo repository.StateRepository, serverURL string) *Service {
  return &Service{
    repo:      repo,
    serverURL: serverURL,
  }
}

func (s *Service) WithOutputRepository(outputRepo repository.StateOutputRepository) *Service {
  s.outputRepo = outputRepo
  // Initialize validator when output repo is available
  if outputRepo != nil {
    s.validator = validation.NewValidator(outputRepo)
  }
  return s
}

// UpdateStateContent with validation
func (s *Service) UpdateStateContent(ctx context.Context, guid string, content []byte, lockID string) (*StateUpdateResult, error) {
  if len(content) == 0 {
    return nil, fmt.Errorf("state content must not be empty")
  }

  // 1. Parse state once
  parsed, err := tfstate.ParseState(content)
  if err != nil {
    return nil, fmt.Errorf("parse state: %w", err)
  }

  // 2. Validate outputs against schemas (if validator available)
  var validations []validation.OutputValidation
  if s.validator != nil {
    validations, err = s.validator.ValidateOutputs(ctx, guid, parsed.Values)
    if err != nil {
      // Log but don't fail - validation is best effort
      log.Printf("WARNING: output validation failed for state %s: %v", guid, err)
    }
  }

  // 3. Merge validation results with output keys
  outputsWithValidation := mergeValidationResults(parsed.Keys, validations)

  // 4. Atomic update: state content + outputs + validation results
  err = s.repo.UpdateContentAndUpsertOutputsWithValidation(ctx, guid, content, lockID,
                                                            parsed.Serial, outputsWithValidation)
  if err != nil {
    return nil, fmt.Errorf("update state content: %w", err)
  }

  // 5. Fetch updated state for summary
  record, err := s.repo.GetByGUID(ctx, guid)
  if err != nil {
    return nil, fmt.Errorf("get updated state: %w", err)
  }

  summary := toSummary(record)
  return &StateUpdateResult{
    Summary:      &summary,
    OutputValues: parsed.Values,
    Validations:  validations,
  }, nil
}

func mergeValidationResults(keys []repository.OutputKey, validations []validation.OutputValidation) []repository.OutputKeyWithValidation {
  validationMap := make(map[string]*validation.ValidationResult)
  for _, v := range validations {
    validationMap[v.OutputKey] = v.Result
  }

  result := make([]repository.OutputKeyWithValidation, len(keys))
  for i, key := range keys {
    result[i] = repository.OutputKeyWithValidation{
      OutputKey:  key,
      Validation: validationMap[key.Key], // nil if no validation
    }
  }

  return result
}
```

#### Step 3.2: Update Repository Implementation
**File:** `cmd/gridapi/internal/repository/bun_state_output_repository.go`

```go
func (r *BunStateOutputRepository) UpsertOutputsWithValidation(ctx context.Context, stateGUID string,
                                                                serial int64, outputs []OutputKeyWithValidation) error {
  if len(outputs) == 0 {
    return nil
  }

  now := time.Now()
  outputModels := make([]*models.StateOutput, len(outputs))

  for i, out := range outputs {
    model := &models.StateOutput{
      StateGUID:   stateGUID,
      OutputKey:   out.Key,
      Sensitive:   out.Sensitive,
      StateSerial: serial,
      CreatedAt:   now,
      UpdatedAt:   now,
    }

    // Include validation results if present
    if out.Validation != nil {
      status := string(out.Validation.Status)
      model.ValidationStatus = &status
      model.ValidationError = out.Validation.Error
      model.ValidatedAt = &out.Validation.ValidatedAt
    }

    outputModels[i] = model
  }

  _, err := r.db.NewInsert().
    Model(&outputModels).
    On("CONFLICT (state_guid, output_key) DO UPDATE").
    Set("sensitive = EXCLUDED.sensitive").
    Set("state_serial = EXCLUDED.state_serial").
    Set("validation_status = EXCLUDED.validation_status").
    Set("validation_error = EXCLUDED.validation_error").
    Set("validated_at = EXCLUDED.validated_at").
    Set("updated_at = EXCLUDED.updated_at").
    // NOTE: schema_json is still NOT updated here
    Exec(ctx)

  return err
}
```

#### Step 3.3: Update Edge Status Logic
**File:** `cmd/gridapi/internal/server/update_edges.go`

```go
// NEW: Edge status constants
const (
  EdgeStatusSchemaInvalid models.EdgeStatus = "schema-invalid"
)

func (j *EdgeUpdateJob) updateOutgoingEdges(ctx context.Context, stateGUID string, outputs map[string]interface{}) error {
  outgoingEdges, err := j.edgeRepo.GetOutgoingEdges(ctx, stateGUID)
  if err != nil {
    return fmt.Errorf("get outgoing edges: %w", err)
  }

  // Fetch validation results for all outputs
  validationMap := j.getValidationResults(ctx, stateGUID)

  for _, edge := range outgoingEdges {
    outputValue, outputExists := outputs[edge.FromOutput]

    if !outputExists {
      // Output removed - mark as missing-output
      if edge.Status != models.EdgeStatusMissingOutput {
        edge.Status = models.EdgeStatusMissingOutput
        j.edgeRepo.Update(ctx, &edge)
      }
      continue
    }

    // Check validation status (highest priority)
    if validation, ok := validationMap[edge.FromOutput]; ok {
      if validation.Status == ValidationStatusInvalid || validation.Status == ValidationStatusError {
        if edge.Status != models.EdgeStatusSchemaInvalid {
          edge.Status = models.EdgeStatusSchemaInvalid
          j.edgeRepo.Update(ctx, &edge)
        }
        continue // Don't check fingerprint if schema invalid
      }
    }

    // Existing fingerprint logic...
    newDigest := tfstate.ComputeFingerprint(outputValue)
    if newDigest == "" {
      continue
    }

    // ... rest of existing logic
  }

  return nil
}

func (j *EdgeUpdateJob) getValidationResults(ctx context.Context, stateGUID string) map[string]ValidationResult {
  // Query state_outputs for validation status
  var outputs []models.StateOutput
  err := j.db.NewSelect().
    Model(&outputs).
    Where("state_guid = ?", stateGUID).
    Where("validation_status IS NOT NULL").
    Scan(ctx)

  if err != nil {
    log.Printf("EdgeUpdateJob: failed to get validation results: %v", err)
    return make(map[string]ValidationResult)
  }

  results := make(map[string]ValidationResult, len(outputs))
  for _, out := range outputs {
    if out.ValidationStatus != nil {
      results[out.OutputKey] = ValidationResult{
        Status: ValidationStatus(*out.ValidationStatus),
        Error:  out.ValidationError,
      }
    }
  }

  return results
}
```

#### Step 3.4: Invalidate Cache on Schema Update
**File:** `cmd/gridapi/internal/services/state/service.go`

```go
func (s *Service) SetOutputSchema(ctx context.Context, guid string, outputKey string, schemaJSON string) error {
  // ... existing validation ...

  err := s.outputRepo.SetOutputSchema(ctx, guid, outputKey, schemaJSON)
  if err != nil {
    return err
  }

  // Invalidate cache so new schema is used on next validation
  if s.validator != nil {
    s.validator.InvalidateCache(guid, outputKey)
  }

  return nil
}
```

### Phase 4: API & Testing (Days 7-8)

#### Step 4.1: Expose Validation Results in API
**File:** `proto/state/v1/state.proto`

```protobuf
message OutputKey {
  string key = 1;
  bool sensitive = 2;
  optional string schema_json = 3;

  // NEW: Validation metadata
  optional string validation_status = 4;  // "valid", "invalid", "error", or absent
  optional string validation_error = 5;   // Error message if status != "valid"
  optional google.protobuf.Timestamp validated_at = 6;
}
```

**File:** `cmd/gridapi/internal/server/connect_handlers_deps.go`

```go
func (h *StateServiceHandler) ListStateOutputs(...) {
  // ... existing code ...

  protoOutputs := make([]*statev1.OutputKey, len(outputs))
  for i, out := range outputs {
    protoOut := &statev1.OutputKey{
      Key:       out.Key,
      Sensitive: out.Sensitive,
    }

    if out.SchemaJSON != nil && *out.SchemaJSON != "" {
      protoOut.SchemaJson = out.SchemaJSON
    }

    // Include validation results
    if out.ValidationStatus != nil {
      protoOut.ValidationStatus = out.ValidationStatus
      protoOut.ValidationError = out.ValidationError
      if out.ValidatedAt != nil {
        protoOut.ValidatedAt = timestamppb.New(*out.ValidatedAt)
      }
    }

    protoOutputs[i] = protoOut
  }
  // ...
}
```

#### Step 4.2: Update Edge Model
**File:** `cmd/gridapi/internal/db/models/edge.go`

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

#### Step 4.3: Integration Tests
**File:** `tests/integration/output_validation_test.go`

```go
func TestOutputValidation_ValidSchema(t *testing.T) {
  // 1. Create state
  // 2. Set schema for output (e.g., VPC ID pattern)
  // 3. Upload tfstate with VALID VPC ID
  // 4. Verify validation_status = 'valid'
  // 5. Verify edge status != 'schema-invalid'
}

func TestOutputValidation_InvalidSchema(t *testing.T) {
  // 1. Create state
  // 2. Set schema for output (e.g., subnet-xxx pattern)
  // 3. Upload tfstate with INVALID value (vpc-xxx instead of subnet-xxx)
  // 4. Verify validation_status = 'invalid'
  // 5. Verify validation_error contains meaningful message
}

func TestOutputValidation_EdgeStatusSchemaInvalid(t *testing.T) {
  // 1. Create producer and consumer states
  // 2. Set schema on producer output
  // 3. Create dependency edge
  // 4. Upload INVALID tfstate to producer
  // 5. Verify edge status = 'schema-invalid'
}

func TestOutputValidation_CacheInvalidation(t *testing.T) {
  // 1. Create state with output
  // 2. Set schema v1 (requires string)
  // 3. Upload tfstate with string → valid
  // 4. Update schema v2 (requires number)
  // 5. Upload SAME tfstate → now invalid (cache was cleared)
}

func TestOutputValidation_NoSchemaSkipped(t *testing.T) {
  // 1. Create state
  // 2. Upload tfstate (no schema set)
  // 3. Verify validation_status IS NULL
  // 4. Verify edge status computed from fingerprint only
}
```

## Migration Strategy

### Existing Deployments

**Database Migration:**
```sql
-- Non-blocking: Add columns as nullable
ALTER TABLE state_outputs
  ADD COLUMN IF NOT EXISTS validation_status TEXT,
  ADD COLUMN IF NOT EXISTS validation_error TEXT,
  ADD COLUMN IF NOT EXISTS validated_at TIMESTAMPTZ;

-- Create index for edge update queries
CREATE INDEX CONCURRENTLY idx_state_outputs_validation
  ON state_outputs(state_guid)
  WHERE validation_status IS NOT NULL;
```

**Backfill Strategy:**
- New uploads automatically get validation
- Existing outputs have `validation_status = NULL`
- No backfill needed (validation happens on next upload)

### Rollout Plan

1. **Week 1:** Deploy validation columns + API fields (validation disabled)
2. **Week 2:** Enable validation in staging (monitor performance)
3. **Week 3:** Enable validation in production (soft mode - log only)
4. **Week 4:** Expose validation status in UI/CLI
5. **Future:** Add strict mode (reject invalid uploads)

## Performance Considerations

### Expected Impact

**Schema Compilation:**
- One-time cost per unique schema
- Cached for 5 minutes (adjustable)
- ~1-5ms per schema compile

**Validation Overhead:**
- ~0.1-1ms per output (depends on complexity)
- Typical state: 5-10 outputs = ~1-10ms total
- Large state: 100 outputs = ~10-100ms total

**Database Updates:**
- 3 additional columns per output row
- Atomic update in same transaction (no extra round-trip)

**Mitigation:**
- Schema caching reduces repeated compilation
- Validation runs asynchronously from Terraform's perspective (state upload completes first)
- Best-effort validation (failures don't block upload)

### Monitoring Metrics

**Suggested Prometheus Metrics:**
```go
validation_total{status="valid|invalid|error"}
validation_duration_seconds{operation="compile|validate"}
validation_cache_hit_ratio
```

## Future Enhancements

### 1. **Strict Validation Mode** (Q2 2025)
- Per-state configuration: `validate_schema_strict: true`
- Reject tfstate upload if validation fails
- Gated behind feature flag

### 2. **Custom Error Messages** (Q3 2025)
- Allow schemas to define human-readable error messages
- Use JSON Schema `$comment` or custom extensions

### 3. **Remote Schema References** (Q3 2025)
- Support `$ref` to external schema URLs
- Centralized schema registry

### 4. **Validation Webhooks** (Q4 2025)
- Notify external systems on validation failures
- Integration with Slack, PagerDuty, etc.

### 5. **Schema Versioning** (Q4 2025)
- Track schema history
- Allow rollback to previous schema versions

## Security Considerations

### Schema Complexity Attacks
**Risk:** Malicious schemas with exponential validation time

**Mitigation:**
- Validation timeout (5 seconds default)
- Schema size limit (1MB max)
- Regex complexity limits (jsonschema library has built-in protection)

### Schema Injection
**Risk:** Schemas containing malicious JSON

**Mitigation:**
- JSON syntax validation before storage
- Schema compilation validates structure
- No code execution (pure data validation)

## References

- [JSON Schema Specification (Draft 7)](https://json-schema.org/draft-07/json-schema-release-notes.html)
- [santhosh-tekuri/jsonschema Documentation](https://github.com/santhosh-tekuri/jsonschema)
- [Terraform State Format](https://www.terraform.io/docs/language/state/index.html)
- [Output Schema Implementation](OUTPUT_SCHEMA_IMPLEMENTATION.md)
- [Integration Test Plan](tests/integration/OUTPUT_SCHEMA_TEST_PLAN.md)

## Appendix: Example Validation Flow

```
┌─────────────────────────────────────────────────────────────┐
│ Terraform applies changes                                   │
└────────────┬────────────────────────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────────────────────────┐
│ POST /tfstate/abc-123                                       │
│ Body: {"version":4,"serial":5,"outputs":{                   │
│   "vpc_id": {"value": "vpc-12345", "sensitive": false}      │
│ }}                                                           │
└────────────┬────────────────────────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────────────────────────┐
│ service.UpdateStateContent()                                │
│  ├─> tfstate.ParseState() → outputs["vpc_id"] = "vpc-12345" │
│  ├─> validator.ValidateOutputs()                            │
│  │    ├─> Get schema for (abc-123, vpc_id)                  │
│  │    │    ├─> Check cache → MISS                           │
│  │    │    └─> DB query → {"type":"string","pattern":"^vpc"}│
│  │    ├─> Compile schema → jsonschema.Schema                │
│  │    └─> schema.Validate("vpc-12345") → ✓ VALID            │
│  └─> repo.UpsertOutputsWithValidation()                     │
│       └─> INSERT ... ON CONFLICT UPDATE                     │
│           validation_status='valid', validated_at=now()     │
└────────────┬────────────────────────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────────────────────────┐
│ EdgeUpdateJob.UpdateEdgesWithOutputs() (async)              │
│  ├─> Get validation results                                 │
│  │    └─> Query: validation_status='valid' for vpc_id       │
│  ├─> For each outgoing edge:                                │
│  │    ├─> Check validation → VALID (skip schema-invalid)    │
│  │    ├─> Compute fingerprint                               │
│  │    └─> Update edge status (dirty/clean based on digest)  │
│  └─> For each incoming edge:                                │
│       └─> Update consumer observation                       │
└─────────────────────────────────────────────────────────────┘
```

**Database State After Upload:**
```sql
SELECT output_key, validation_status, validation_error, validated_at
FROM state_outputs
WHERE state_guid = 'abc-123';

┌────────────┬────────────────────┬───────────────────┬─────────────────────┐
│ output_key │ validation_status  │ validation_error  │ validated_at        │
├────────────┼────────────────────┼───────────────────┼─────────────────────┤
│ vpc_id     │ valid              │ NULL              │ 2025-11-23 10:30:00 │
└────────────┴────────────────────┴───────────────────┴─────────────────────┘
```
