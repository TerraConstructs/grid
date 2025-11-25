# Handoff: Output Schema Support - Phase 1 Bug Fix Complete

**Date**: 2025-11-25
**Branch**: `010-output-schema-support`
**Status**: Phase 1 Bug Fixed ✅ - Ready for Phase 2 Implementation

## Summary

Phase 1 (schema declaration & storage) had a critical bug preventing schema preservation during Terraform state uploads. This bug has been **completely fixed** and all 8 integration tests now pass.

---

## Bug Fix: Schema Preservation During State Upload

### Problem Identified

**Symptom**: Schemas set via `SetOutputSchema` were being deleted when Terraform uploaded state.

**Root Cause**: Two repository functions were deleting outputs with `state_serial != <new_serial>` WITHOUT checking if they had schemas:
1. `bun_state_output_repository.go` - `UpsertOutputs()`
2. `bun_state_repository.go` - `UpdateContentAndUpsertOutputs()`

Pre-declared schemas have `state_serial=0`, so they were deleted when the first state upload arrived with `serial > 0`.

### Fix Applied

**Files Modified**:
1. `cmd/gridapi/internal/repository/bun_state_output_repository.go`
2. `cmd/gridapi/internal/repository/bun_state_repository.go`

**Changes Made** (both files):
```go
// BEFORE: Deleted ALL outputs with different serial
_, err := tx.NewDelete().
    Model((*models.StateOutput)(nil)).
    Where("state_guid = ?", stateGUID).
    Where("state_serial != ?", serial).  // ← Bug: deletes schema-only outputs
    Exec(ctx)

// AFTER: Only delete outputs WITHOUT schemas
// 1. Fetch existing schemas BEFORE deletion
var existingOutputs []models.StateOutput
err := tx.NewSelect().
    Model(&existingOutputs).
    Where("state_guid = ?", stateGUID).
    Where("schema_json IS NOT NULL").
    Scan(ctx)

// Build map of output_key -> schema_json
existingSchemas := make(map[string]*string)
for i := range existingOutputs {
    existingSchemas[existingOutputs[i].OutputKey] = existingOutputs[i].SchemaJSON
}

// 2. Only delete outputs with NULL schemas
_, err = tx.NewDelete().
    Model((*models.StateOutput)(nil)).
    Where("state_guid = ?", stateGUID).
    Where("state_serial != ?", serial).
    Where("schema_json IS NULL").  // ← Fix: preserve schemas
    Exec(ctx)

// 3. Preserve schemas when inserting outputs
for _, out := range outputs {
    model := models.StateOutput{...}
    if existingSchema, ok := existingSchemas[out.Key]; ok {
        model.SchemaJSON = existingSchema  // ← Preserve existing schema
    }
    outputModels = append(outputModels, model)
}

// 4. Include schema_json in ON CONFLICT update
tx.NewInsert().
    Model(&outputModels).
    On("CONFLICT (state_guid, output_key) DO UPDATE").
    Set("sensitive = EXCLUDED.sensitive").
    Set("state_serial = EXCLUDED.state_serial").
    Set("updated_at = EXCLUDED.updated_at").
    Set("schema_json = EXCLUDED.schema_json").  // ← Preserve schemas
    Exec(ctx)
```

---

## Test Isolation Fix

**File Modified**: `cmd/gridctl/internal/client/provider.go`

**Problem**: OIDC warning was written to stdout, polluting JSON output in tests.

**Fix**: Redirect pterm warnings to stderr:
```go
// BEFORE
pterm.Warning.Printf("OIDC authentication disabled for %s; proceeding without credentials.\n", p.serverURL)

// AFTER
pterm.Warning.WithWriter(os.Stderr).Printf("OIDC authentication disabled for %s; proceeding without credentials.\n", p.serverURL)
```

**Test Helper Added**: `mustRunGridctlStdOut()` in `tests/integration/helpers.go` to capture only stdout (not combined output).

**Test Fixed**: `TestSchemaWithGridctl` now uses `--force` flag and `mustRunGridctlStdOut()` helper.

---

## Test Results

### Before Fix (4 failures)
```
✅ TestBasicSchemaOperations           - PASS
❌ TestSchemaPreDeclaration            - FAIL (schema lost)
✅ TestSchemaUpdate                    - PASS
❌ TestSchemaPreservationDuringStateUpload - FAIL (schema lost)
✅ TestSchemaWithDependencies          - PASS
❌ TestComplexSchemas                  - FAIL (schema lost)
❌ TestSchemaWithGridctl               - FAIL (.grid file conflict)
✅ TestStateReferenceResolution        - PASS
```

### After Fix (all passing)
```
✅ TestBasicSchemaOperations           - PASS
✅ TestSchemaPreDeclaration            - PASS ✨ (fixed)
✅ TestSchemaUpdate                    - PASS
✅ TestSchemaPreservationDuringStateUpload - PASS ✨ (fixed)
✅ TestSchemaWithDependencies          - PASS
✅ TestComplexSchemas                  - PASS ✨ (fixed)
✅ TestSchemaWithGridctl               - PASS ✨ (fixed)
✅ TestStateReferenceResolution        - PASS
```

**Status**: All 8 schema integration tests passing (33 total integration tests passing)

---

## Verification Steps

Run the full test suite to verify the fix:

```bash
# Clean slate
make db-reset && sleep 2 && make db-migrate

# Rebuild
make build

# Run all integration tests
make test-integration

# Expected result: All tests pass, including:
# - TestSchemaPreDeclaration
# - TestSchemaPreservationDuringStateUpload
# - TestComplexSchemas
# - TestSchemaWithGridctl
```

---

## Phase 2 Readiness

With the Phase 1 bug fixed, the foundation is solid for Phase 2 implementation:

### Phase 2A: Schema Inference (Next)
- **Blocked By**: None ✅
- **Library**: `github.com/JLugagne/jsonschema-infer`
- **Estimated**: 2-3 days

### Phase 2B: Schema Validation
- **Blocked By**: Phase 2A completion
- **Library**: `github.com/santhosh-tekuri/jsonschema/v6`
- **Estimated**: 3-4 days

### Phase 2C: Edge Status Updates
- **Blocked By**: Phase 2B completion
- **Estimated**: 1-2 days

---

## Files Changed

| File | Lines Changed | Purpose |
|------|---------------|---------|
| `cmd/gridapi/internal/repository/bun_state_output_repository.go` | +31/-6 | Schema preservation in UpsertOutputs |
| `cmd/gridapi/internal/repository/bun_state_repository.go` | +38/-7 | Schema preservation in UpdateContentAndUpsertOutputs |
| `cmd/gridctl/internal/client/provider.go` | +2/-1 | Redirect warnings to stderr |
| `tests/integration/output_schema_test.go` | +1/-1 | Use --force flag |
| `tests/integration/helpers.go` | +8/+0 | Add mustRunGridctlStdOut helper |

**Total**: ~80 lines changed across 5 files

---

## Debug Logging

The fix includes debug logging (can be removed after Phase 2 stabilizes):

```go
fmt.Printf("DEBUG bun_state_repository: Found existing schema for key='%s' has_schema=%v\n", ...)
fmt.Printf("DEBUG bun_state_repository: Total existing schemas: %d\n", ...)
fmt.Printf("DEBUG bun_state_repository: Preserving schema for key='%s' schema_is_not_nil=%v\n", ...)
```

These logs helped verify schema preservation during testing. Remove after Phase 2 implementation is complete.

---

## Next Steps

1. ✅ **Phase 1 Bug Fix** - Complete
2. **Run `/speckit.tasks`** - Generate Beads task breakdown for Phase 2
3. **Write Integration Tests** (TDD approach):
   - 9 tests for Phase 2A (inference)
   - 7 tests for Phase 2B (validation)
   - 4 tests for Phase 2C (edge status)
4. **Implement Phase 2A** - Schema inference using `JLugagne/jsonschema-infer`
5. **Implement Phase 2B** - Schema validation using `santhosh-tekuri/jsonschema/v6`
6. **Implement Phase 2C** - Edge status updates for schema violations

---

## Documentation

All planning artifacts completed in `/specs/010-output-schema-support/`:

| Document | Status | Purpose |
|----------|--------|---------|
| `plan.md` | ✅ Complete | Implementation plan with phases, tests, decisions |
| `research.md` | ✅ Complete | Technology decisions + bug documentation |
| `data-model.md` | ✅ Complete | Entity extensions, repository interfaces |
| `contracts/state.proto.diff` | ✅ Complete | Proto field additions |
| `contracts/repository-interface.go` | ✅ Complete | Extended repository interface |
| `quickstart.md` | ✅ Complete | Usage examples for CLI, SDK |
| `HANDOFF.md` | ✅ Complete | This document |

---

## Lessons Learned

1. **Always test schema preservation**: The bug existed because state uploads were tested but schema+upload interaction wasn't.
2. **Separate stdout/stderr**: Machine-readable output (JSON) must not be polluted by warnings.
3. **Test isolation matters**: The `.grid` file from `TestDuplicateLogicID` caused failures in `TestSchemaWithGridctl`.
4. **Database resets are critical**: Some test failures were due to stale data, not code bugs.

---

## References

- **Original Feature Spec**: `specs/010-output-schema-support/spec.md`
- **Integration Test Plan**: `tests/integration/OUTPUT_SCHEMA_TEST_PLAN.md`
- **Phase 1 Implementation Guide**: `OUTPUT_SCHEMA_IMPLEMENTATION.md`
- **Phase 2B Validation Plan**: `OUTPUT_VALIDATION.md`
- **Webapp UI Design**: `specs/webapp-output-schema-design.md`
