# Implementation Plan: Output Schema Support - Phase 2

**Branch**: `010-output-schema-support` | **Date**: 2025-11-25 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/010-output-schema-support/spec.md`

**Status**: Phase 2A Complete (2025-11-26), Phase 2B In Progress

## Summary

Add comprehensive JSON Schema validation and inference for Terraform/OpenTofu state outputs. Phase 1 (schema declaration & storage) is complete. This plan covers:

- **Phase 2A**: Automatic schema inference from output values during state upload
- **Phase 2B**: Runtime validation of outputs against declared/inferred schemas
- **Phase 2C**: Dependency edge status updates for schema violations
- **Phase 3**: Webapp UI for schema and validation display (deferred to separate plan)

Technical approach: `JLugagne/jsonschema-infer` for schema inference + `santhosh-tekuri/jsonschema/v6` for validation, fire-and-forget goroutines for async processing (consistent with existing EdgeUpdateJob pattern).

## Technical Context

**Language/Version**: Go 1.24+, TypeScript 5.x (webapp)
**Primary Dependencies**:
- `github.com/santhosh-tekuri/jsonschema/v6` - JSON Schema validation
- `github.com/hashicorp/golang-lru/v2` - Schema caching
- `github.com/JLugagne/jsonschema-infer` - JSON Schema inference from samples

**Storage**: PostgreSQL (existing), new columns in `state_outputs` table
**Testing**: Go integration tests (`tests/integration/`), table-driven unit tests
**Target Platform**: Linux server (gridapi), CLI (gridctl), React webapp
**Project Type**: Monorepo with Go workspace (existing structure)
**Performance Goals**: <2s validation for schemas <10KB, outputs <100KB (SC-003)
**Constraints**: Validation non-blocking (advisory, not enforcement)
**Scale/Scope**: Typical state has 5-10 outputs, validation is per-output

## Constitution Check

*GATE: All principles satisfied - one pragmatic exception documented*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Go Workspace Architecture | ✅ | All changes within existing modules |
| II. Contract-Centric SDKs | ✅ | Proto changes for new fields, SDKs wrap |
| III. Dependency Flow Discipline | ✅ | No new cross-module dependencies |
| IV. Cross-Language Parity | ✅ | Proto generates Go + TS, SDK wraps |
| V. Test Strategy | ✅ | Integration tests before implementation |
| VI. Versioning & Releases | ✅ | Minor version bump (new fields) |
| VII. Simplicity & Pragmatism | ✅ | Fire-and-forget, no external queues |
| VIII. Service Exposure Discipline | ✅ | No new endpoints, uses existing auth |
| IX. API Server Internal Layering | ⚠️ | **Exception**: SchemaValidationJob in server/ to match EdgeUpdateJob pattern (see note below) |

**Note on Principle IX Exception**: SchemaValidationJob will be placed in `internal/server/` to match the existing EdgeUpdateJob pattern (also in server/). While this technically violates "business logic in service layer," Constitution §VII (Simplicity & Pragmatism) prioritizes consistency with production-proven patterns. Both jobs should be moved to service layer together in future refactoring. This is documented technical debt, not a new violation.

## Project Structure

### Documentation (this feature)

```
specs/010-output-schema-support/
├── plan.md              # This file
├── spec.md              # Feature specification (complete)
├── research.md          # Technology decisions (complete)
├── data-model.md        # Entity/interface definitions (complete)
├── quickstart.md        # Usage examples (complete)
├── contracts/           # API contract changes
│   ├── state.proto.diff     # Proto field additions
│   └── repository-interface.go  # Repository interface extensions
├── checklists/
│   └── requirements.md  # Spec quality validation (complete)
└── tasks.md             # Beads task breakdown (to be generated)
```

### Source Code (repository root)

```
cmd/gridapi/
├── internal/
│   ├── db/models/
│   │   └── state_output.go      # Extended model (+4 fields)
│   ├── migrations/
│   │   └── 20251125_add_schema_source_and_validation.go  # New migration
│   ├── repository/
│   │   ├── interface.go         # Extended interface (+4 methods)
│   │   └── bun_state_output_repository.go  # Implementation
│   ├── services/
│   │   ├── validation/          # NEW: Validation service
│   │   │   ├── validator.go     # JSON Schema validation
│   │   │   └── cache.go         # LRU schema cache
│   │   ├── inference/           # NEW: Inference service
│   │   │   └── inferrer.go      # Schema inference from values
│   │   └── state/
│   │       └── service.go       # Extended with validation/inference calls
│   └── server/
│       ├── tfstate_handlers.go  # Trigger validation job
│       └── schema_validation_job.go  # Background validation job

proto/state/v1/
└── state.proto                  # Extended OutputKey message

api/state/v1/
└── (generated)                  # buf generate

pkg/sdk/
└── state_types.go               # Extended OutputKey struct

js/sdk/
└── src/models/state-info.ts     # Extended TypeScript types

tests/integration/
├── output_schema_test.go        # Existing Phase 1 tests
├── output_inference_test.go     # NEW: Inference tests
└── output_validation_test.go    # NEW: Validation tests
```

**Structure Decision**: Web application structure (backend + frontend). Changes span gridapi (backend), webapp (frontend), and SDKs. No new modules created.

## Previous Work

### Phase 1 Completed (7 commits, 6,176 lines)

| Component | Implementation |
|-----------|----------------|
| Proto | `SetOutputSchema`, `GetOutputSchema` RPCs, `schema_json` field |
| Database | `schema_json TEXT` column, migration |
| Repository | `SetOutputSchema`, `GetOutputSchema` methods |
| Service | Schema CRUD via `state.Service` |
| Handlers | Connect RPC handlers, schema preservation on upload |
| Authorization | `state-output:schema-write`, `state-output:schema-read` |
| Go SDK | `SetOutputSchema`, `GetOutputSchema` client methods |
| CLI | `set-output-schema`, `get-output-schema` commands |
| Tests | 8 integration tests, 9 fixtures |

### Related Documentation

| Document | Purpose |
|----------|---------|
| `OUTPUT_SCHEMA_IMPLEMENTATION.md` | Phase 1 implementation guide |
| `OUTPUT_VALIDATION.md` | Phase 2B detailed plan (1,057 lines) |
| `specs/010-output-schema-support/webapp-output-schema-design.md` | Phase 3 UI design (1,034 lines) |

## Implementation Phases

### Phase 1 Bug Fix: Schema Preservation (PREREQUISITE)

**Status**: Bug identified in Phase 1 - must be fixed before Phase 2

**Problem**: Pre-declared schemas (via `SetOutputSchema`) are deleted when Terraform state uploads occur. The `UpsertOutputs` function deletes outputs with `state_serial != <new_serial>`, but schemas have `state_serial=0`.

**Fix Location**: `cmd/gridapi/internal/repository/bun_state_output_repository.go:27-34`

**Affected Tests** (4 failures):
- `TestSchemaPreDeclaration`
- `TestSchemaPreservationDuringStateUpload`
- `TestComplexSchemas`
- `TestSchemaWithGridctl` (separate issue: `.grid` file conflict)

**Estimated Effort**: 0.5 days

---

### Phase 2A: Schema Inference (FR-019 through FR-028) - ✅ COMPLETED (2025-11-26)

**Goal**: Automatically generate JSON Schema from output values when no schema exists.

**Implementation Summary**:
1. ✅ Library: `github.com/JLugagne/jsonschema-infer v0.1.2` (Draft-07 output)
2. ✅ Service: `cmd/gridapi/internal/services/inference/inferrer.go`
3. ✅ Format detection: date-time, email, UUID, IPv4/IPv6, URL (built-in)
4. ✅ Database: `schema_source`, `validation_status`, `validation_error`, `validated_at` columns
5. ✅ Repository: `SetOutputSchemaWithSource()`, `GetOutputsWithoutSchema()` methods
6. ✅ Integration: Fire-and-forget async via goroutine in state upload workflow
7. ✅ Proto/SDK: `schema_source` field added to OutputKey message
8. ✅ Tests: 10 integration tests passing (TestSchemaInference*)

**Critical Bug Fixed**: JSON double-encoding in `inferrer.go` - library returns JSON string directly, removed unnecessary `json.Marshal()` call

**Tasks Closed**: grid-daf8, grid-5d22, grid-9461, grid-befd, grid-3f9b, grid-1845, grid-d219, grid-aeba, grid-4ab5, grid-1049

**Actual Effort**: 3 days (including bug fix and comprehensive testing)

### Phase 2B: Schema Validation (FR-029 through FR-035)

**Goal**: Validate output values against schemas during state upload.

**Key Implementation Points**:
1. Validation service using `santhosh-tekuri/jsonschema/v6`
2. LRU cache for compiled schemas (1000 entries, 5-min TTL)
3. Fire-and-forget goroutine for async validation
4. Per-state mutex to prevent concurrent validations
5. 30-second timeout per validation job
6. `validation_status`: valid | invalid | error | not_validated (null when no schema exists)
7. Structured `validation_error` with JSON path
8. Best-effort (validation errors don't block uploads)

**Estimated Effort**: 3-4 days

### Phase 2C: Edge Status Updates (FR-036 through FR-039)

**Goal**: Mark dependency edges as `schema-invalid` when producer outputs fail validation.

**Key Implementation Points**:
1. Add `schema-invalid` to EdgeStatus enum
2. Extend EdgeUpdateJob to check validation status
3. Priority: schema-invalid > missing-output > dirty > clean
4. Atomic edge status update with validation
5. Clear status when subsequent validation passes

**Estimated Effort**: 1-2 days

---

### Integration Testing Phase

**Location**: `tests/integration/output_schema_test.go` (existing) + new test files

#### Existing Tests (Phase 1 - Need Bug Fix)

| Test Function | User Story | Status | Notes |
|---------------|------------|--------|-------|
| `TestBasicSchemaOperations` | US-1 | ✅ PASS | Set/Get schemas via SDK |
| `TestSchemaPreDeclaration` | US-1 | ❌ FAIL | Schema lost on state upload |
| `TestSchemaUpdate` | US-2 | ✅ PASS | Update existing schema |
| `TestSchemaPreservationDuringStateUpload` | US-3 | ❌ FAIL | Schema lost on state upload |
| `TestSchemaWithDependencies` | US-1 | ✅ PASS | Schema + dependency interaction |
| `TestComplexSchemas` | US-1 | ❌ FAIL | Schema lost on state upload |
| `TestSchemaWithGridctl` | US-4 | ❌ FAIL | .grid file conflict (unrelated) |
| `TestStateReferenceResolution` | US-4 | ✅ PASS | Logic-id and GUID both work |

**Fix Required**: 4 tests fail due to schema preservation bug (Phase 1 Bug Fix section)

#### New Tests for Phase 2A: Schema Inference

| Test Function | User Story | Requirements |
|---------------|------------|--------------|
| `TestSchemaInferenceFromString` | US-5 | FR-019, FR-021 - Infer string type |
| `TestSchemaInferenceFromNumber` | US-5 | FR-021 - Infer number/integer types |
| `TestSchemaInferenceFromBoolean` | US-5 | FR-021 - Infer boolean type |
| `TestSchemaInferenceFromArray` | US-5 | FR-021, FR-022 - Infer array with items |
| `TestSchemaInferenceFromObject` | US-5 | FR-021, FR-022 - Nested object inference |
| `TestSchemaInferenceDateTime` | US-5 | FR-023 - ISO 8601 format detection |
| `TestSchemaInferencePreserveManual` | US-5 | FR-025 - Manual schemas not overwritten |
| `TestSchemaSourceMetadata` | US-5 | FR-026, FR-028 - schema_source field |
| `TestSchemaInferenceOnceOnly` | US-5 | FR-027 - Only first upload triggers inference |

**Test Fixtures Needed**:
- `testdata/tfstate_string_output.json` - State with string output
- `testdata/tfstate_complex_types.json` - State with number, boolean, array, object
- `testdata/tfstate_datetime_output.json` - State with ISO 8601 timestamp

#### New Tests for Phase 2B: Schema Validation

| Test Function | User Story | Requirements |
|---------------|------------|--------------|
| `TestValidationPassPattern` | US-6 | FR-029, FR-031 - Pattern validation pass |
| `TestValidationFailPattern` | US-6 | FR-029, FR-030 - Pattern validation fail |
| `TestValidationNoSchema` | US-6 | FR-033 - Skip validation when no schema |
| `TestValidationComplexSchema` | US-6 | FR-029 - Nested object validation |
| `TestValidationStatusInResponse` | US-6 | FR-034 - Status in ListStateOutputs |
| `TestValidationErrorMessage` | US-6 | FR-035 - Structured error with path |
| `TestValidationAsync` | US-6 | FR-032 - Non-blocking state upload |

**Test Fixtures Needed**:
- `testdata/schema_pattern_strict.json` - Schema with pattern constraint
- `testdata/tfstate_valid_pattern.json` - State matching pattern
- `testdata/tfstate_invalid_pattern.json` - State violating pattern

#### New Tests for Phase 2C: Edge Status

| Test Function | User Story | Requirements |
|---------------|------------|--------------|
| `TestEdgeStatusSchemaInvalid` | US-6 | FR-036 - Edge marked schema-invalid |
| `TestEdgeStatusSchemaClears` | US-6 | FR-038 - Status clears on valid upload |
| `TestEdgeStatusSchemaInResponse` | US-6 | FR-039 - Status in dependency query |
| `TestEdgeStatusPriority` | US-6 | FR-037 - schema-invalid > dirty |

**Estimated Effort**: 2-3 days (writing tests + fixtures)

---

### Phase 3: Webapp UI (FR-040) - Deferred

Covered by separate design document: `specs/010-output-schema-support/webapp-output-schema-design.md`

## Complexity Tracking

*No Constitution violations - no entries needed*

## Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Validation library | `santhosh-tekuri/jsonschema/v6` | Draft 7 support, detailed errors, active |
| Inference library | `JLugagne/jsonschema-infer` | Purpose-built, Draft-07 output, format detection |
| Background processing | Fire-and-forget goroutine | Matches EdgeUpdateJob, no external deps |
| Schema source tracking | Separate column | Query efficiency, not embedded JSON |
| Validation storage | Same table (state_outputs) | 1:1 with outputs, no extra joins |
| Edge status addition | New enum value | Higher priority than drift status |

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Validation performance | Low | Medium | Schema caching, async processing |
| Inference accuracy | Medium | Low | Conservative inference, manual override |
| Edge update race | Low | Medium | Per-state mutex, atomic updates |
| Schema size limits | Low | Low | Document 1MB limit, log warnings |

## Success Criteria Mapping

| SC | Requirement | Implementation |
|----|-------------|----------------|
| SC-003 | <2s validation | Caching + async |
| SC-009 | Best-effort inference | Try/catch, log, continue |
| SC-010 | 95% inference accuracy | Type detection, format hints |

## Completion Log

### Phase 2A: Schema Inference - COMPLETED (2025-11-26)

**Summary**: Automatic schema inference from Terraform state output values is now fully operational.

**What Was Built**:
- Inference service using `jsonschema-infer v0.1.2` library
- Database schema with `schema_source` and validation metadata columns
- Repository methods for schema-source-aware operations
- Fire-and-forget async inference on state upload
- Proto/SDK extensions for schema source tracking
- 10 comprehensive integration tests (100% passing)

**Critical Issues Resolved**:
- Fixed JSON double-encoding bug in `inferrer.go` (library returns JSON string, not object)
- All inference tests now passing after fix

**Key Learnings**:
- The `jsonschema-infer` library returns JSON strings directly (not Go structs)
- Fire-and-forget pattern works well for non-blocking operations
- Database reset requires using `make db-reset` from project root, not tests directory
- Architecture layering verified compliant (Services → Repositories)

**Files Modified/Created**:
- `cmd/gridapi/internal/services/inference/inferrer.go` (new)
- `cmd/gridapi/internal/migrations/20251125000002_add_schema_source_and_validation.go` (new)
- `cmd/gridapi/internal/repository/interface.go` (extended)
- `cmd/gridapi/internal/repository/bun_state_output_repository.go` (extended)
- `cmd/gridapi/internal/services/state/service.go` (extended)
- `proto/state/v1/state.proto` (extended)
- `pkg/sdk/state_types.go` (extended)
- `tests/integration/output_inference_test.go` (new - 10 tests)

## Next Steps

1. ✅ ~~Fix Phase 1 Bug~~ - COMPLETED
2. ✅ ~~Implement Phase 2A (inference)~~ - COMPLETED
3. **Phase 2B: Schema Validation** - IN PROGRESS
   - Write integration tests first (TDD approach)
   - Implement validation service with `santhosh-tekuri/jsonschema/v6`
   - Add background validation job
   - **Ready Tasks**: grid-c833, grid-bef1, grid-1c39
4. Implement Phase 2C (edge updates) - add `schema-invalid` status
5. Implement Phase 3 (webapp UI) - display schema and validation status
