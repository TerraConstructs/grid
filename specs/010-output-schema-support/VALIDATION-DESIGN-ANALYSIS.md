# Output Validation Design Analysis

**Date**: 2025-11-27
**Feature**: Output Schema Support - Phase 2B/2C
**Context**: Design review for validation timing, purge logic, and race condition handling

## Executive Summary

This document analyzes critical design decisions for Phase 2B (Output Validation) and Phase 2C (Edge Status Updates):

1. **Purge Logic Fix**: Current logic retains ALL outputs with schemas (`schema_json IS NOT NULL`), which prevents cleanup of removed outputs when they have inferred schemas
2. **Validation Timing**: Sync vs async validation tradeoffs and coordination with EdgeUpdateJob
3. **Race Conditions**: Serial ordering, rapid POSTs, and edge status atomicity

**Key Recommendations**:
- ✅ Fix purge logic to only retain `schema_source='manual'` rows
- ✅ Run validation **in-sync** (before EdgeUpdateJob fires)
- ✅ Add serial checks to async inference to prevent resurrection
- ✅ Use database transaction atomicity for edge status updates

---

## Problem 1: Purge Logic for Inferred Schemas

### Current Behavior (BROKEN)

**Location**: `cmd/gridapi/internal/repository/bun_state_repository.go:155-165`

```go
// Delete old outputs with different serial (cache invalidation)
// IMPORTANT: Do NOT delete outputs that have schemas (schema_json IS NOT NULL)
_, err = tx.NewDelete().
    Model((*models.StateOutput)(nil)).
    Where("state_guid = ?", guid).
    Where("state_serial != ?", serial).
    Where("schema_json IS NULL").  // ❌ PROBLEM: Keeps inferred schemas forever
    Exec(ctx)
```

**Problem**: When outputs get inferred schemas (after Phase 2A), they are **never purged** even when removed from Terraform state. This creates:
- ✅ Desired: Manual schemas (schema_source='manual') persist even when output absent (user pre-declared)
- ❌ Broken: Inferred schemas (schema_source='inferred') persist forever, cluttering the output table
- ❌ Broken: Removed outputs with inferred schemas show as `state_serial=<old>` but are never cleaned up

### Root Cause

The purge logic was written for Phase 1 where **all schemas were manual**. Phase 2A added inference, which creates `schema_source='inferred'` rows that should be ephemeral (tied to output presence).

### Required Fix

**New Logic**:
```go
// Delete old outputs with different serial (cache invalidation)
// Retain only manual schema-only rows (schema_source='manual', often state_serial=0)
// Remove outputs with inferred schemas when absent from new state
_, err = tx.NewDelete().
    Model((*models.StateOutput)(nil)).
    Where("state_guid = ?", guid).
    Where("state_serial != ?", serial).
    Where("schema_source IS NULL OR schema_source = 'inferred'").  // ✅ FIX
    Exec(ctx)
```

**Logic Summary**:
- Delete if `state_serial != newSerial` AND (`schema_source IS NULL` OR `schema_source='inferred'`)
- Keep if `schema_source='manual'` (even when state_serial=0, meaning output not yet in state)

**Behavior Table**:

| Output Key | schema_source | state_serial | New State Has Output? | Action |
|------------|---------------|--------------|----------------------|--------|
| vpc_id     | NULL          | 10           | No                   | DELETE (no schema) |
| vpc_id     | 'inferred'    | 10           | No                   | DELETE (ephemeral) |
| vpc_id     | 'manual'      | 0            | No                   | KEEP (user pre-declared) |
| vpc_id     | 'manual'      | 10           | No                   | KEEP (user pre-declared) |
| subnet_id  | 'inferred'    | 10           | Yes (serial=11)      | UPDATE (in ON CONFLICT clause) |

### Implementation Notes

1. **Preserves Manual Schemas**: Schema-only rows (`state_serial=0`, `schema_source='manual'`) survive indefinitely
2. **Cleans Inferred Schemas**: When output removed, inferred schema is deleted
3. **Handles Transitions**: If user sets manual schema on output with inferred schema, manual schema takes precedence

---

## Problem 2: Async Inference Serial Check (Resurrection Risk)

### Current Behavior (RACE CONDITION)

**Location**: `cmd/gridapi/internal/services/state/service.go:304`

```go
// Save inferred schemas with source="inferred"
for _, schema := range inferred {
    err := s.outputRepo.SetOutputSchemaWithSource(inferCtx, guid, schema.OutputKey, schema.SchemaJSON, "inferred")
    if err != nil {
        // Log error but continue with other schemas
        fmt.Printf("SetOutputSchemaWithSource failed for output %s in state %s: %v\n", schema.OutputKey, guid, err)
    }
}
```

**Problem**: No serial check before writing inferred schema. Consider this scenario:

1. POST A (serial=10): uploads `vpc_id` output, fires inference goroutine
2. POST B (serial=11): removes `vpc_id` output, deletes row (purged per fixed logic above)
3. Inference goroutine (from POST A) completes, writes `vpc_id` schema with `state_serial=0`
4. **Result**: `vpc_id` resurrected with `state_serial=0`, `schema_source='inferred'` despite being absent in serial=11

### Required Fix

**Add serial check before writing inferred schema**:

```go
// In SetOutputSchemaWithSource (or before calling it):
// 1. Fetch current state serial
currentState, err := s.repo.GetByGUID(inferCtx, guid)
if err != nil {
    return // State deleted or error, skip inference
}

// 2. Check if output still exists at current serial
output, err := s.outputRepo.GetOutputByKey(inferCtx, guid, schema.OutputKey)
if err != nil || output.StateSerial < currentState.Serial {
    // Output removed in newer upload, skip inference
    continue
}

// 3. Only write if serial matches
err := s.outputRepo.SetOutputSchemaWithSource(inferCtx, guid, schema.OutputKey, schema.SchemaJSON, "inferred")
```

**Alternative**: Pass serial to inference job, check before writing:
```go
// In inference goroutine startup:
inferSerial := parsed.Serial  // Capture serial from POST

// Before writing schema:
if output.StateSerial != inferSerial {
    continue // Skip, output was updated/removed by newer POST
}
```

---

## Problem 3: Validation Sync vs Async

### Option A: Async Validation (Current Plan)

**Design**:
```
POST /tfstate/{guid} → UpdateContentAndUpsertOutputs → Response 200 OK
                     ↓ (fire-and-forget)
                     go ValidationJob.ValidateOutputs() → writes validation_status
                     ↓ (separate goroutine)
                     go EdgeUpdateJob.UpdateEdges() → reads validation_status
```

**Risks**:
1. ⚠️ **Race Condition**: EdgeUpdateJob may read `validation_status=NULL` before ValidationJob writes it
2. ⚠️ **Inconsistent Edge Status**: Edge marked `dirty` instead of `schema-invalid` due to race
3. ⚠️ **User Confusion**: Graph shows "dirty" briefly, then flips to "schema-invalid" seconds later

**Mitigation Options**:
- Shared per-state mutex (both jobs acquire same lock)
- EdgeUpdateJob waits for validation completion (blocks edge updates)
- Accept eventual consistency (edge status updates later)

**Code Pattern**:
```go
// In handler after UpdateContentAndUpsertOutputs:
go h.validationJob.ValidateOutputs(ctx, guid, outputs)  // Job 1
go h.edgeUpdateJob.UpdateEdges(ctx, guid, tfstateJSON)  // Job 2 (may race Job 1)
```

### Option B: In-Sync Validation (RECOMMENDED)

**Design**:
```
POST /tfstate/{guid} → UpdateContentAndUpsertOutputs
                     → ValidationJob.ValidateOutputs() (synchronous)
                     → Response 200 OK
                     ↓ (fire-and-forget after validation completes)
                     go EdgeUpdateJob.UpdateEdges() → reads validation_status (guaranteed set)
```

**Benefits**:
1. ✅ **No Race Condition**: validation_status guaranteed written before EdgeUpdateJob reads it
2. ✅ **Atomic Edge Status**: Edge status reflects validation result immediately
3. ✅ **Simpler Code**: No coordination primitives needed
4. ✅ **Predictable Behavior**: Graph always shows correct status

**Drawbacks**:
1. ⚠️ **Latency**: Adds ~10-50ms to POST response time (validation is fast)
2. ⚠️ **Blocking**: Large schemas (>1MB) may delay response (mitigated by timeout)

**Performance Analysis**:
- Typical schema size: <10KB → validation <10ms (per SC-003)
- Worst case: 1MB schema → validation ~100ms (still acceptable)
- Timeout: 30s per validation job (prevents runaway validation)
- **Conclusion**: Latency cost is minimal, correctness gain is significant

**Code Pattern**:
```go
// In handler after UpdateContentAndUpsertOutputs:
// Run validation synchronously (non-blocking write to client, but blocks edge job)
h.validationJob.ValidateOutputs(ctx, guid, outputs)  // Sync call

// Fire edge update after validation completes
go h.edgeUpdateJob.UpdateEdges(ctx, guid, tfstateJSON)
```

### Recommendation: **Option B (In-Sync Validation)**

**Rationale**:
1. **Constitution VII (Simplicity & Pragmatism)**: Simpler design, no coordination primitives
2. **Correctness > Performance**: Edge status must reflect validation accurately
3. **Performance Acceptable**: <50ms latency for typical use case (SC-003)
4. **User Experience**: Consistent graph state, no flipping

---

## Problem 4: Edge Status Model - Composite Drift + Validation

### Critical Design Clarification (2025-11-27)

**Original Plan**: Add single `schema-invalid` status as highest priority
**Revised Design**: **Composite status model** capturing TWO orthogonal dimensions

**Why This Matters**:
- An output can be **both** dirty (fingerprint changed) **and** invalid (fails schema)
- User needs to know: "Is my dependency clean/dirty?" AND "Is the value valid/invalid?"
- Status like `clean-invalid` means: "Consumer is up-to-date, but value violates schema"
- Status like `dirty-invalid` means: "Consumer is stale AND value violates schema"

### Revised EdgeStatus Enum

**New Constants** (replaces simple list):

```go
// In cmd/gridapi/internal/db/models/edge.go:
type EdgeStatus string

const (
    EdgeStatusPending          EdgeStatus = "pending"           // Initial state, no observation yet
    EdgeStatusClean            EdgeStatus = "clean"             // in_digest == out_digest && valid
    EdgeStatusCleanInvalid     EdgeStatus = "clean-invalid"     // in_digest == out_digest && invalid
    EdgeStatusDirty            EdgeStatus = "dirty"             // in_digest != out_digest && valid
    EdgeStatusDirtyInvalid     EdgeStatus = "dirty-invalid"     // in_digest != out_digest && invalid
    EdgeStatusPotentiallyStale EdgeStatus = "potentially-stale" // Transitive upstream dirty
    EdgeStatusMock             EdgeStatus = "mock"              // Using mock value, real output not yet exists
    EdgeStatusMissingOutput    EdgeStatus = "missing-output"    // Producer output key removed
)
```

**Status Derivation Logic**:

```
Dimension 1: Drift (fingerprint comparison)
- clean: in_digest == out_digest
- dirty: in_digest != out_digest

Dimension 2: Validation (schema compliance)
- valid: validation_status == "valid" OR validation_status IS NULL (no schema)
- invalid: validation_status == "invalid"

Combined Matrix:
┌─────────────┬──────────┬────────────┐
│             │ Valid    │ Invalid    │
├─────────────┼──────────┼────────────┤
│ Clean       │ clean    │ clean-inv  │
│ Dirty       │ dirty    │ dirty-inv  │
└─────────────┴──────────┴────────────┘

Special Cases:
- Missing output → "missing-output" (overrides all)
- No observation yet → "pending"
- Mock edge → "mock" (until real output appears)
```

### Updated Status Priority Logic

**Revised Priority** (replaces data-model.md lines 363-388):

```
1. Output Existence Check (highest priority):
   - If output missing → "missing-output"

2. Drift + Validation Check (combined):
   - Compute drift: clean = (in_digest == out_digest), dirty = (in_digest != out_digest)
   - Compute validation: valid = (validation_status != "invalid"), invalid = (validation_status == "invalid")
   - Combine:
     * clean + valid   → "clean"
     * clean + invalid → "clean-invalid"
     * dirty + valid   → "dirty"
     * dirty + invalid → "dirty-invalid"

3. Special Cases:
   - Mock edge transitioning → "pending" or computed status
   - No in_digest yet → "pending"
```

### Implications for Implementation

**Database Changes**:
- No `error_message` column needed on edges (validation_error is on state_outputs)
- EdgeStatus enum expanded from 6 to 8 values (adding clean-invalid, dirty-invalid)

**Go Model Changes** (`cmd/gridapi/internal/db/models/edge.go`):
```go
const (
    EdgeStatusPending          EdgeStatus = "pending"           // Initial state, no observation yet
    EdgeStatusClean            EdgeStatus = "clean"             // in_digest == out_digest && valid
    EdgeStatusCleanInvalid     EdgeStatus = "clean-invalid"     // in_digest == out_digest && invalid
    EdgeStatusDirty            EdgeStatus = "dirty"             // in_digest != out_digest && valid
    EdgeStatusDirtyInvalid     EdgeStatus = "dirty-invalid"     // in_digest != out_digest && invalid
    EdgeStatusPotentiallyStale EdgeStatus = "potentially-stale" // Transitive upstream dirty
    EdgeStatusMock             EdgeStatus = "mock"              // Using mock value, real output not yet exists
    EdgeStatusMissingOutput    EdgeStatus = "missing-output"    // Producer output key removed
)
```

**Proto Comments** (`proto/state/v1/state.proto`):
```protobuf
// Edge represents a dependency relationship between states
message Edge {
    // ... other fields ...

    // Status indicates the synchronization and validation state of the edge.
    // Edge status combines two orthogonal dimensions:
    // 1. Drift: clean (in_digest == out_digest) vs dirty (in_digest != out_digest)
    // 2. Validation: valid (passes schema) vs invalid (fails schema)
    //
    // Possible values:
    // - "pending": Initial state, no observation yet
    // - "clean": in_digest == out_digest && output passes schema validation
    // - "clean-invalid": in_digest == out_digest && output fails schema validation
    // - "dirty": in_digest != out_digest && output passes schema validation
    // - "dirty-invalid": in_digest != out_digest && output fails schema validation
    // - "potentially-stale": Transitive upstream dirty
    // - "mock": Using mock_value, real output doesn't exist yet
    // - "missing-output": Producer doesn't have the required output key
    string status = 8;
}
```

**TypeScript SDK Changes** (`js/sdk/src/models/state-info.ts`):
```typescript
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

**EdgeUpdateJob Changes**:
- Replace priority-based if/else with **composite status derivation**
- Single function: `deriveEdgeStatusWithValidation(inDigest, outDigest, validationStatus)`
- Must handle: drift unchanged but validation changed (e.g., user adds schema, existing output fails)

**User Experience**:
- Graph shows: "This edge is clean but value is invalid" vs "This edge is dirty AND value is invalid"
- Webapp can color-code: red for any invalid, yellow for dirty, green for clean+valid
- Users know whether to: (1) fix schema, (2) run terraform apply, or (3) both

---

## Problem 4B: Edge Status Atomicity (Updated)

### Requirement (FR-037)

> "Edge status update must be atomic with validation status to prevent race conditions"

### Current EdgeUpdateJob Pattern

**Location**: `cmd/gridapi/internal/server/update_edges.go:64-122`

```go
func (j *EdgeUpdateJob) updateOutgoingEdges(ctx context.Context, stateGUID string, outputs map[string]interface{}) error {
    outgoingEdges, err := j.edgeRepo.GetOutgoingEdges(ctx, stateGUID)  // Read edges

    for _, edge := range outgoingEdges {
        // ... compute new status ...

        if err := j.edgeRepo.Update(ctx, &edge); err != nil {  // Write edge (separate tx)
            log.Printf("EdgeUpdateJob: failed to update edge %d: %v", edge.ID, err)
        }
    }
}
```

**Problem**: Edge read and write are **separate transactions**. If validation status changes between read and write, edge status may be inconsistent.

### Required Fix: Read Validation Status in Same Query

**Extend GetOutgoingEdges to join validation status**:

```go
// In repository/interface.go:
type EdgeWithValidation struct {
    Edge             models.Edge
    ValidationStatus *string  // From state_outputs.validation_status
    ValidationError  *string  // From state_outputs.validation_error
}

// In EdgeRepository interface:
GetOutgoingEdgesWithValidation(ctx context.Context, stateGUID string) ([]EdgeWithValidation, error)
```

> PREFER Bun Relationship tags on Edge model for JOIN! for example
```go
// example file cmd/gridapi/internal/db/models/state.go
// State represents the persisted Terraform state
type State struct {
	bun.BaseModel `bun:"table:states,alias:s"`
    // other fields omitted for brevity

	// Relationships for eager loading (populated only when using Relation())
	Outputs        []*StateOutput `bun:"rel:has-many,join:guid=state_guid"`
	OutgoingEdges  []*Edge        `bun:"rel:has-many,join:guid=from_state"`
	IncomingEdges  []*Edge        `bun:"rel:has-many,join:guid=to_state"`
    // ...
}
```

And Use relation in like so:
```go
// example file cmd/gridapi/internal/repository/bun_state_repository.go

// GetByGUIDWithRelations fetches a state with specified relations preloaded.
// Relations can be: "Outputs", "IncomingEdges", "OutgoingEdges"
// This allows flexible eager loading based on what data is needed.
func (r *BunStateRepository) GetByGUIDWithRelations(ctx context.Context, guid string, relations ...string) (*models.State, error) {
	state := new(models.State)
	query := r.db.NewSelect().Model(state).Where("guid = ?", guid)

	// Add each requested relation
	for _, rel := range relations {
		query = query.Relation(rel)
	}
    // ...
}
```

**Conceptual / expected SQL Query**:
```sql
SELECT
    e.*,
    so.validation_status,
    so.validation_error
FROM edges e
LEFT JOIN state_outputs so
    ON e.from_state_guid = so.state_guid
    AND e.from_output = so.output_key
WHERE e.from_state_guid = ?
```

**Updated EdgeUpdateJob Logic**:
```go
func (j *EdgeUpdateJob) updateOutgoingEdges(ctx context.Context, stateGUID string, outputs map[string]interface{}) error {
    edgesWithValidation, err := j.edgeRepo.GetOutgoingEdgesWithValidation(ctx, stateGUID)

    for _, ewv := range edgesWithValidation {
        edge := ewv.Edge

        // PRIORITY 1: Check validation status (highest priority)
        if ewv.ValidationStatus != nil && *ewv.ValidationStatus == "invalid" {
            edge.Status = models.EdgeStatusSchemaInvalid
            edge.ErrorMessage = ewv.ValidationError  // Set error details
            if err := j.edgeRepo.Update(ctx, &edge); err != nil {
                log.Printf("EdgeUpdateJob: failed to mark edge as schema-invalid: %v", err)
            }
            continue  // Skip fingerprint check
        }

        // PRIORITY 2: Check output existence
        outputValue, outputExists := outputs[edge.FromOutput]
        if !outputExists {
            // ... missing-output logic ...
            continue
        }

        // PRIORITY 3: Check fingerprint (existing logic)
        // ... dirty/clean logic ...
    }
}
```

### Atomicity Guarantee

**Database ensures atomicity**:
1. `LEFT JOIN` reads edge + validation_status in **one snapshot** (MVCC isolation)
2. Edge update writes new status in **one transaction**
3. No race between read and write (single UPDATE statement)

**Edge Update Transaction**:
```go
// edgeRepo.Update() already runs in transaction:
_, err := r.db.NewUpdate().
    Model(&edge).
    Where("id = ?", edge.ID).
    Exec(ctx)  // Single atomic UPDATE
```

---

## Problem 5: Race Condition Test Coverage

### Required Integration Tests

#### Test 1: Out-of-Order Serial POST

**Scenario**: POST serial=11, then POST serial=10 (late arrival)

**Expected Behavior**:
- Serial=10 POST should be **rejected** or **no-op** (state serial already higher)
- Outputs should remain at serial=11
- Validation status should remain from serial=11

**Test Code**:
```go
func TestOutOfOrderSerialRejected(t *testing.T) {
    // POST serial=10
    _, err := sdk.UpdateState(guid, tfstateSerial10)
    require.NoError(t, err)

    // POST serial=11 (newer)
    _, err = sdk.UpdateState(guid, tfstateSerial11)
    require.NoError(t, err)

    // POST serial=10 again (late arrival)
    _, err = sdk.UpdateState(guid, tfstateSerial10)
    // TODO: Should this error or no-op? Check existing behavior

    // Verify state remains at serial=11
    state, err := sdk.GetState(guid)
    require.NoError(t, err)
    assert.Equal(t, int64(11), state.Serial)
}
```

**Note**: Current implementation may **accept** out-of-order serials (no serial check in UpdateContentAndUpsertOutputs). This is a **pre-existing issue**, not introduced by validation.

#### Test 2: Rapid POST A,B then A,C (Resurrection)

**Scenario**:
1. POST A (serial=10): creates `vpc_id` output, fires inference
2. POST B (serial=11): removes `vpc_id` output, purges row
3. Inference from POST A completes (late)
4. POST C (serial=12): adds new outputs

**Expected Behavior**:
- `vpc_id` should **not reappear** after POST B
- Inference from POST A should **skip** writing (serial check fails)
- POST C should not resurrect `vpc_id`

**Test Code**:
```go
func TestInferenceDoesNotResurrectRemovedOutput(t *testing.T) {
    // POST A: Create vpc_id
    _, err := sdk.UpdateState(guid, tfstateWithVpcID_Serial10)
    require.NoError(t, err)
    time.Sleep(50 * time.Millisecond)  // Let inference start

    // POST B: Remove vpc_id (purge)
    _, err = sdk.UpdateState(guid, tfstateWithoutVpcID_Serial11)
    require.NoError(t, err)

    // Wait for inference to complete
    time.Sleep(200 * time.Millisecond)

    // Verify vpc_id does NOT exist
    outputs, err := sdk.ListOutputs(guid)
    require.NoError(t, err)
    for _, out := range outputs {
        assert.NotEqual(t, "vpc_id", out.Key, "vpc_id should not be resurrected")
    }
}
```

#### Test 3: Manual Schema Survives When Output Absent

**Scenario**:
1. SetOutputSchema(vpc_id, schema) → creates row with `state_serial=0`, `schema_source='manual'`
2. POST serial=10: does **not** include `vpc_id` output
3. Purge logic runs

**Expected Behavior**:
- Manual schema row **survives** (not purged)
- Row remains: `state_serial=0`, `schema_source='manual'`, `schema_json=<user_schema>`

**Test Code**:
```go
func TestManualSchemaOnlyRowSurvivesPurge(t *testing.T) {
    // Pre-declare schema before output exists
    err := sdk.SetOutputSchema(guid, "vpc_id", vpcSchema)
    require.NoError(t, err)

    // Verify schema-only row created
    output, err := sdk.GetOutputSchema(guid, "vpc_id")
    require.NoError(t, err)
    assert.Equal(t, "manual", output.SchemaSource)
    assert.Equal(t, int64(0), output.StateSerial)  // Schema-only row

    // POST state WITHOUT vpc_id (should trigger purge logic)
    _, err = sdk.UpdateState(guid, tfstateWithoutVpcID_Serial10)
    require.NoError(t, err)

    // Verify manual schema still exists (not purged)
    output, err = sdk.GetOutputSchema(guid, "vpc_id")
    require.NoError(t, err)
    assert.Equal(t, "manual", output.SchemaSource)
    assert.NotNil(t, output.SchemaJSON)
}
```

#### Test 4: Edge Status Honors Schema-Invalid and Clears

**Scenario**:
1. POST state with `vpc_id` output
2. SetOutputSchema(vpc_id, strict_pattern_schema)
3. POST state with invalid `vpc_id` value (fails validation)
4. EdgeUpdateJob runs
5. POST state with valid `vpc_id` value (passes validation)
6. EdgeUpdateJob runs again

**Expected Behavior**:
- Step 4: Edge status = `schema-invalid`
- Step 6: Edge status = `clean` or `dirty` (validation cleared)

**Test Code**:
```go
func TestEdgeStatusSchemaInvalidThenClears(t *testing.T) {
    // Create producer and consumer states with dependency
    producerGUID := createState(t, "producer")
    consumerGUID := createState(t, "consumer")
    createDependency(t, consumerGUID, producerGUID, "vpc_id")

    // POST producer with valid vpc_id
    _, err := sdk.UpdateState(producerGUID, tfstateValidVpcID_Serial10)
    require.NoError(t, err)
    time.Sleep(100 * time.Millisecond)  // Wait for edge update

    // Set strict schema on producer output
    err = sdk.SetOutputSchema(producerGUID, "vpc_id", strictVpcSchema)
    require.NoError(t, err)

    // POST producer with INVALID vpc_id (violates pattern)
    _, err = sdk.UpdateState(producerGUID, tfstateInvalidVpcID_Serial11)
    require.NoError(t, err)
    time.Sleep(100 * time.Millisecond)  // Wait for validation + edge update

    // Verify edge status = schema-invalid
    edges, err := sdk.ListDependencies(consumerGUID)
    require.NoError(t, err)
    require.Len(t, edges, 1)
    assert.Equal(t, "schema-invalid", edges[0].Status)
    assert.Contains(t, edges[0].ErrorMessage, "pattern")

    // POST producer with VALID vpc_id (passes validation)
    _, err = sdk.UpdateState(producerGUID, tfstateValidVpcID_Serial12)
    require.NoError(t, err)
    time.Sleep(100 * time.Millisecond)  // Wait for validation + edge update

    // Verify edge status cleared (clean or dirty)
    edges, err = sdk.ListDependencies(consumerGUID)
    require.NoError(t, err)
    require.Len(t, edges, 1)
    assert.NotEqual(t, "schema-invalid", edges[0].Status)
    assert.Empty(t, edges[0].ErrorMessage)
}
```

---

## Summary of Design Decisions

| Component | Current Design | Recommended Change | Rationale |
|-----------|----------------|-------------------|-----------|
| **Purge Logic** | `WHERE schema_json IS NULL` | `WHERE schema_source IS NULL OR schema_source='inferred'` | Only manual schemas should persist when output absent |
| **Inference Serial Check** | No check | Add serial check before writing schema | Prevent resurrection of removed outputs |
| **Validation Timing** | Async (planned) | **In-Sync** (before response) | Prevent race with EdgeUpdateJob, simpler design |
| **Edge Status Model** | Simple priority (schema-invalid > dirty > clean) | **Composite model** (drift × validation dimensions) | Capture both "is it stale?" AND "is it valid?" |
| **EdgeStatus Enum** | 5 values (no validation statuses) | **7 values** (clean, clean-invalid, dirty, dirty-invalid, ...) | Two orthogonal dimensions need 4 combinations |
| **Edge Status Atomicity** | Separate read/write | Join validation_status in GetOutgoingEdges | Database MVCC ensures snapshot consistency |
| **Integration Tests** | Phase 2B tests | Add 4 race condition tests | Cover out-of-order, resurrection, purge, edge clearing |

---

## Implementation Checklist

### Code Changes Required

1. **Purge Logic Fix** (`cmd/gridapi/internal/repository/bun_state_repository.go`):
   - Line 161: Change `WHERE schema_json IS NULL` to `WHERE schema_source IS NULL OR schema_source = 'inferred'`
   - Same fix in `bun_state_output_repository.go:49`

2. **Inference Serial Check** (`cmd/gridapi/internal/services/state/service.go`):
   - Line 304: Add serial check before `SetOutputSchemaWithSource`
   - Verify output still exists and serial matches before writing inferred schema

3. **EdgeStatus Enum Expansion** (`cmd/gridapi/internal/db/models/edge.go`):
   - Add `EdgeStatusCleanInvalid` and `EdgeStatusDirtyInvalid` constants
   - Update comments to reflect composite drift+validation model

4. **Proto Comments Update** (`proto/state/v1/state.proto`):
   - Update `Edge.status` field comment with composite status explanation
   - List all 8 possible values with descriptions

5. **TypeScript SDK Types** (`js/sdk/src/models/state-info.ts`):
   - Add `'clean-invalid'` and `'dirty-invalid'` to EdgeStatus union type
   - Update comments to explain drift vs validation dimensions

6. **Validation Service** (NEW: `cmd/gridapi/internal/services/validation/validator.go`):
   - Implement synchronous validation (not async)
   - LRU cache for compiled schemas
   - 30-second timeout per validation

7. **EdgeUpdateJob Extension** (`cmd/gridapi/internal/server/update_edges.go`):
   - Replace `deriveEdgeStatus` with `deriveEdgeStatusWithValidation`
   - Implement composite status logic (drift × validation matrix)

8. **Repository Interface** (`cmd/gridapi/internal/repository/interface.go`):
   - Add `EdgeWithValidation` struct
   - Add `GetOutgoingEdgesWithValidation` method to EdgeRepository interface

9. **Repository Implementation** (`cmd/gridapi/internal/repository/bun_edge_repository.go`):
   - Implement `GetOutgoingEdgesWithValidation` with LEFT JOIN to state_outputs
   - Return validation_status and validation_error with edges

### Integration Tests Required

10. **Race Condition Tests** (`tests/integration/output_validation_race_test.go` - NEW):
    - `TestOutOfOrderSerialRejected`: Verify serial=10 after serial=11 is no-op
    - `TestInferenceDoesNotResurrectRemovedOutput`: Verify inference doesn't resurrect purged outputs
    - `TestManualSchemaOnlyRowSurvivesPurge`: Verify schema_source='manual' rows survive
    - `TestEdgeStatusComposite`: Verify clean-invalid, dirty-invalid status transitions

### Documentation Updates

11. **Plan.md**: ✅ DONE (updated with sync validation decision)
12. **Data-model.md**: Update edge status priority logic (lines 363-388)
13. **Beads Tasks**: Add comments to grid-1c39, grid-cc87 with design clarifications

---

## Next Steps

1. ✅ Update `plan.md` with sync validation decision - **COMPLETED**
2. ✅ Document composite edge status model - **COMPLETED**
3. ⏭️ Update task `grid-1c39` (validation job) to reflect sync design
4. ⏭️ Update task `grid-cc87` (edge update) to include composite status logic
5. ⏭️ Update task `grid-5d3e` or create new task for purge logic fix
6. ⏭️ Create task for inference serial check fix
7. ⏭️ Create task for proto/TS SDK type updates
8. ⏭️ Create tasks for 4 integration tests (race condition coverage)