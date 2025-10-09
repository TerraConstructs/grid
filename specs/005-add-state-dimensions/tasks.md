# Tasks: State Labels

**Input**: Design documents from `/specs/005-add-state-dimensions/`
**Prerequisites**: plan.md (required), research.md, data-model.md, contracts/, quickstart.md

## Execution Flow (main)
```
1. Load plan.md from feature directory
   → SUCCESS: JSON column with in-memory bexpr filtering approach
   → Extract: Go 1.24.4, Connect RPC, Bun ORM, PostgreSQL/SQLite, go-bexpr
2. Load optional design documents:
   → data-model.md: Labels (JSONB column), LabelPolicy (single-row table)
   → contracts/: state-tags.md (labels only; schemas/facets deferred)
   → research.md: JSON column > EAV, bexpr > SQL translation, lightweight policy > JSON Schema
   → quickstart.md: Label lifecycle, policy validation, bexpr filtering scenarios
3. Generate tasks by category:
   → Setup: go-bexpr dependency, migrations
   → Tests: Repository tests, service tests, handler tests
   → Core: Models, migration, repository, service layer, validators
   → API: Connect handlers, protobuf updates
   → CLI: gridctl state (--label flags), gridctl policy commands
   → Integration: End-to-end quickstart scenarios
4. Apply task rules:
   → Different files = mark [P] for parallel
   → Same file = sequential (no [P])
   → Tests before implementation (TDD)
   → Migration before all implementation
5. Number tasks sequentially (T001, T002...)
6. Validate task completeness:
   → Migration creates schema ✓
   → Models support labels ✓
   → Repository CRUD + filtering ✓
   → Policy validation ✓
   → CLI commands ✓
   → Quickstart scenarios covered ✓
7. Return: SUCCESS (tasks ready for execution)
```

## Format: `[ID] [P?] Description`
- **[P]**: Can run in parallel (different files, no dependencies)
- Include exact file paths in descriptions

## Path Conventions
- **API Server**: `cmd/gridapi/internal/` (models, repository, service, handlers, migrations)
- **CLI**: `cmd/gridctl/cmd/` (Cobra commands)
- **SDK**: `pkg/sdk/` (Go SDK for label operations)
- **Protobuf**: `proto/state/v1/state.proto` (API contracts)
- **Tests**: `tests/integration/`, `cmd/gridapi/internal/repository/` (repository tests)

---

## Phase 3.1: Setup & Dependencies
- [ ] **T001** Add go-bexpr dependency to cmd/gridapi module (`cd cmd/gridapi && go get github.com/hashicorp/go-bexpr`)
- [ ] **T002** [P] Add go-bexpr dependency to cmd/gridctl module (`cd cmd/gridctl && go get github.com/hashicorp/go-bexpr`)

## Phase 3.2: Database Schema & Models
⚠️ **CRITICAL: Complete migration before any implementation tasks**

- [ ] **T003** Create LabelMap and PolicyDefinition types in `cmd/gridapi/internal/db/models/state.go` (add LabelMap type with Scan/Value methods per data-model.md lines 102-123)
- [ ] **T004** [P] Create LabelPolicy model in `cmd/gridapi/internal/db/models/label_policy.go` (per data-model.md lines 154-202)
- [ ] **T005** Create migration `cmd/gridapi/internal/migrations/20251009000001_add_state_labels.go` (add labels JSONB column to states table, create label_policy table with single-row constraint, add GIN index; per data-model.md lines 207-290)
- [ ] **T006** Run migration to verify schema changes (`make db-reset && ./bin/gridapi db migrate`)

## Phase 3.3: Tests First (TDD) ⚠️ MUST COMPLETE BEFORE 3.4
**CRITICAL: These tests MUST be written and MUST FAIL before ANY implementation**

### Repository Layer Tests
- [ ] **T007** [P] Write test for BunStateRepository.Create with labels in `cmd/gridapi/internal/repository/bun_state_repository_test.go` (verify labels persist correctly in JSONB column)
- [ ] **T008** [P] Write test for BunStateRepository.Update with label changes in `cmd/gridapi/internal/repository/bun_state_repository_test.go` (verify label updates apply atomically)
- [ ] **T009** [P] Write test for BunStateRepository.ListWithFilter with bexpr in `cmd/gridapi/internal/repository/bun_state_repository_test.go` (verify in-memory bexpr filtering per data-model.md lines 360-411)
- [ ] **T010** [P] Write test for BunLabelPolicyRepository.GetPolicy in `cmd/gridapi/internal/repository/bun_label_policy_repository_test.go` (verify single-row retrieval)
- [ ] **T011** [P] Write test for BunLabelPolicyRepository.SetPolicy in `cmd/gridapi/internal/repository/bun_label_policy_repository_test.go` (verify version increment and policy_json update)

### Service Layer Tests
- [ ] **T012** [P] Write test for LabelValidator.Validate with policy constraints in `cmd/gridapi/internal/state/label_validator_test.go` (test key format regex, enum validation, reserved prefixes, size limits per data-model.md lines 299-327)
- [ ] **T013** [P] Write test for StateService.CreateState with labels in `cmd/gridapi/internal/state/service_test.go` (verify label validation runs before persistence)
- [ ] **T014** [P] Write test for StateService.UpdateLabels in `cmd/gridapi/internal/state/service_test.go` (verify add/remove/replace operations and policy enforcement)

### Handler Tests
- [ ] **T015** [P] Write test for UpdateStateTags RPC handler in `cmd/gridapi/internal/server/connect_handlers_test.go` (verify adds/removals apply correctly, validation errors return INVALID_ARGUMENT per contracts/state-tags.md)
- [ ] **T016** [P] Write test for GetLabelPolicy RPC handler in `cmd/gridapi/internal/server/connect_handlers_test.go` (verify policy retrieval)
- [ ] **T017** [P] Write test for SetLabelPolicy RPC handler in `cmd/gridapi/internal/server/connect_handlers_test.go` (verify policy update and versioning)
- [ ] **T018** [P] Write test for ListStates with filter parameter in `cmd/gridapi/internal/server/connect_handlers_test.go` (verify bexpr filter delegation to repository)

## Phase 3.4: Core Implementation (ONLY after tests are failing)

### Protobuf Updates
- [ ] **T019** Update `proto/state/v1/state.proto` to add LabelValue message (oneof string_value/number_value/bool_value), add labels field to State message, add filter field to ListStatesRequest, add UpdateStateLabelsRequest/Response, add GetLabelPolicyRequest/Response, add SetLabelPolicyRequest/Response, add GetLabelEnumRequest/Response
- [ ] **T020** Run `buf generate` to regenerate Go and TypeScript code
- [ ] **T021** Run `buf lint` to verify proto changes

### Repository Layer
- [ ] **T022** Implement BunStateRepository.ListWithFilter in `cmd/gridapi/internal/repository/bun_state_repository.go` (add bexpr filtering per data-model.md lines 360-411; fetch states, compile bexpr evaluator, filter in-memory, trim to page_size)
- [ ] **T023** [P] Create BunLabelPolicyRepository in `cmd/gridapi/internal/repository/bun_label_policy_repository.go` (implement GetPolicy, SetPolicy, GetEnumValues per data-model.md lines 350-355)
- [ ] **T024** Update StateRepository interface in `cmd/gridapi/internal/repository/interface.go` to add ListWithFilter method signature
- [ ] **T025** [P] Create LabelPolicyRepository interface in `cmd/gridapi/internal/repository/interface.go` (add GetPolicy, SetPolicy, GetEnumValues methods)

### Service Layer
- [ ] **T026** Create LabelValidator in `cmd/gridapi/internal/state/label_validator.go` (implement Validate method with key format regex `^[a-z][a-z0-9_/]{0,31}$`, enum checks, reserved prefix checks, size limits per data-model.md lines 299-327)
- [ ] **T027** Add StateService.UpdateLabels method in `cmd/gridapi/internal/state/service.go` (implement add/remove/replace logic with policy validation, atomic updates)
- [ ] **T028** Update StateService.CreateState in `cmd/gridapi/internal/state/service.go` to validate labels via LabelValidator before persisting
- [ ] **T029** [P] Create PolicyService in `cmd/gridapi/internal/state/policy_service.go` (implement GetPolicy, SetPolicy, ValidateLabels dry-run, GetEnumValues methods)

### Connect RPC Handlers
- [ ] **T030** Implement UpdateStateLabels RPC handler in `cmd/gridapi/internal/server/connect_handlers.go` (delegate to StateService.UpdateLabels, map errors per contracts/state-tags.md)
- [ ] **T031** Implement GetLabelPolicy RPC handler in `cmd/gridapi/internal/server/connect_handlers.go` (delegate to PolicyService.GetPolicy)
- [ ] **T032** Implement SetLabelPolicy RPC handler in `cmd/gridapi/internal/server/connect_handlers.go` (delegate to PolicyService.SetPolicy, increment version)
- [ ] **T033** Implement GetLabelEnum RPC handler in `cmd/gridapi/internal/server/connect_handlers.go` (delegate to PolicyService.GetEnumValues)
- [ ] **T034** Update ListStates RPC handler in `cmd/gridapi/internal/server/connect_handlers.go` to support filter parameter (delegate to StateRepository.ListWithFilter with bexpr string)

## Phase 3.5: CLI Integration

### gridctl state commands
- [ ] **T035** Add --label flag support to `gridctl state create` in `cmd/gridctl/cmd/state_create.go` (accept repeated --label key=value flags, build labels map, pass to CreateState RPC)
- [ ] **T036** [P] Create `gridctl state set` command in `cmd/gridctl/cmd/state_set.go` (parse --label key=value for adds, --label -key for removals, call UpdateStateLabels RPC per spec.md FR-012, FR-013)
- [ ] **T037** Update `gridctl state list` command in `cmd/gridctl/cmd/state_list.go` to display labels in output table
- [ ] **T038** Add --filter flag to `gridctl state list` in `cmd/gridctl/cmd/state_list.go` (accept bexpr filter expression, pass to ListStates RPC per spec.md FR-016)
- [ ] **T039** Add --label flag shortcut to `gridctl state list` in `cmd/gridctl/cmd/state_list.go` (convert repeated --label key=value to bexpr AND expression per spec.md FR-016a)
- [ ] **T040** Update `gridctl state get` command in `cmd/gridctl/cmd/state_get.go` to display labels section in output

### gridctl policy commands
- [ ] **T041** Create `gridctl policy` command group in `cmd/gridctl/cmd/policy.go` (parent command with subcommands)
- [ ] **T042** [P] Create `gridctl policy get` command in `cmd/gridctl/cmd/policy_get.go` (call GetLabelPolicy RPC, display policy JSON per spec.md FR-031)
- [ ] **T043** [P] Create `gridctl policy set` command in `cmd/gridctl/cmd/policy_set.go` (read policy JSON from --file flag, call SetLabelPolicy RPC per quickstart.md lines 28-46)
- [ ] **T044** [P] Create `gridctl policy enum` command in `cmd/gridctl/cmd/policy_enum.go` (accept key argument, call GetLabelEnum RPC, display allowed values per spec.md FR-017b)
- [ ] **T045** [P] Optional: Create `gridctl policy compliance` command in `cmd/gridctl/cmd/policy_compliance.go` (revalidate all states against current policy, list violations per spec.md FR-017c, quickstart.md lines 78-84)

## Phase 3.6: SDK Updates
- [ ] **T046** Add LabelMap helper functions to `pkg/sdk/client.go` (ConvertProtoLabels, BuildBexprFilter for SDK consumers)
- [ ] **T047** Update SDK documentation in `pkg/sdk/README.md` to cover label operations and bexpr filter construction per spec.md FR-017

## Phase 3.7: Integration Testing
- [ ] **T048** Create integration test in `tests/integration/labels_lifecycle_test.go` (verify quickstart.md scenario 1: create state with labels, verify persistence and retrieval)
- [ ] **T049** [P] Create integration test in `tests/integration/labels_policy_test.go` (verify quickstart.md scenario 2: set policy, submit invalid label, verify validation error)
- [ ] **T050** [P] Create integration test in `tests/integration/labels_update_test.go` (verify quickstart.md scenario 3: add/remove labels via state set, verify atomic updates)
- [ ] **T051** [P] Create integration test in `tests/integration/labels_filtering_test.go` (verify quickstart.md scenario 5: bexpr filter expressions return correct states)
- [ ] **T052** [P] Create integration test in `tests/integration/labels_compliance_test.go` (verify quickstart.md scenario 7: policy update, compliance report shows violations)

## Phase 3.8: Polish & Documentation
- [ ] **T053** [P] Add unit tests for label key regex validation in `cmd/gridapi/internal/state/label_validator_test.go` (test edge cases: hyphens rejected, underscores/slashes allowed, uppercase rejected per spec.md FR-008)
- [ ] **T054** [P] Add unit tests for LabelMap Scan/Value methods in `cmd/gridapi/internal/db/models/state_test.go` (verify JSON marshaling for string/number/bool types)
- [ ] **T055** Run quickstart.md manual verification (`make db-up`, start gridapi, execute all quickstart commands, verify expected outputs)
- [ ] **T056** Update `CHANGELOG.md` with State Labels feature summary
- [ ] **T057** Add bexpr filter examples to `README.md` or docs/ (show compound expressions, escape rules, best practices per research.md)
- [ ] **T058** Performance test: verify <50ms p99 latency for bexpr filtering with 500 states (load test script in `tests/performance/labels_filter_bench_test.go`)

---

## Dependencies

### Hard Blockers (Must Complete Before)
- **T003-T006** (schema/models/migration) block ALL implementation tasks
- **T007-T018** (tests) block T019-T052 (implementation)
- **T019-T021** (protobuf) block T030-T034 (handlers), T035-T045 (CLI), T046-T047 (SDK)

### Soft Dependencies (Recommended Order)
- **T022-T025** (repository) before T026-T029 (service)
- **T026-T029** (service) before T030-T034 (handlers)
- **T030-T034** (handlers) before T035-T047 (CLI/SDK)
- **T048-T052** (integration tests) after T035-T047 (CLI complete)
- **T053-T058** (polish) after all implementation complete

### No Dependencies (Parallel Safe)
- T001, T002 (different modules)
- T003, T004 (different files)
- T007-T011 (different test files)
- T012-T018 (different test files, different classes)
- T023, T024, T025 (different files from T022)
- T029 (different file from T026-T028)
- T042-T045 (different CLI command files)
- T048-T052 (different integration test files)
- T053, T054, T056, T057, T058 (different polish tasks)

---

## Parallel Execution Examples

### Phase 3.1: Setup (Run in Parallel)
```bash
# Launch T001-T002 together:
cd cmd/gridapi && go get github.com/hashicorp/go-bexpr &
cd cmd/gridctl && go get github.com/hashicorp/go-bexpr &
wait
```

### Phase 3.2: Models (Run in Parallel)
```bash
# Launch T003-T004 together (different files):
# Task T003: Create LabelMap in cmd/gridapi/internal/db/models/state.go
# Task T004: Create LabelPolicy model in cmd/gridapi/internal/db/models/label_policy.go
# Then T005 (migration) sequentially, then T006 (run migration)
```

### Phase 3.3: Repository Tests (Run in Parallel)
```bash
# Launch T007-T011 together (all [P]):
# Task T007: BunStateRepository.Create with labels test
# Task T008: BunStateRepository.Update with labels test
# Task T009: BunStateRepository.ListWithFilter test
# Task T010: BunLabelPolicyRepository.GetPolicy test
# Task T011: BunLabelPolicyRepository.SetPolicy test
```

### Phase 3.3: Service Tests (Run in Parallel)
```bash
# Launch T012-T014 together (all [P]):
# Task T012: LabelValidator.Validate test
# Task T013: StateService.CreateState with labels test
# Task T014: StateService.UpdateLabels test
```

### Phase 3.3: Handler Tests (Run in Parallel)
```bash
# Launch T015-T018 together (all [P]):
# Task T015: UpdateStateTags RPC handler test
# Task T016: GetLabelPolicy RPC handler test
# Task T017: SetLabelPolicy RPC handler test
# Task T018: ListStates with filter test
```

### Phase 3.5: CLI Commands (Run in Parallel)
```bash
# Launch T042-T045 together (different files):
# Task T042: gridctl policy get
# Task T043: gridctl policy set
# Task T044: gridctl policy enum
# Task T045: gridctl policy compliance
```

### Phase 3.7: Integration Tests (Run in Parallel)
```bash
# Launch T048-T052 together (all [P]):
# Task T048: labels_lifecycle_test.go
# Task T049: labels_policy_test.go
# Task T050: labels_update_test.go
# Task T051: labels_filtering_test.go
# Task T052: labels_compliance_test.go
```

---

## Notes
- **[P]** tasks target different files with no shared dependencies
- **Verify tests fail** before implementing (TDD discipline)
- **Commit after each task** to track incremental progress
- **Scope boundary**: Labels only; state-schemas.md and state-facets.md contracts deferred to future milestone per scope-reduction.md
- **Bexpr grammar**: Document identifier constraints (`[a-z][a-z0-9_/]*`) in error messages per spec.md FR-008
- **SQLite parity**: Migration uses PostgreSQL JSONB; SQLite adaptation deferred to future appliance deployment

---

## Validation Checklist
*GATE: Checked before execution*

- [x] All contracts have corresponding tests (state-tags.md → T015; schemas/facets deferred)
- [x] All entities have model tasks (State with Labels → T003; LabelPolicy → T004)
- [x] All tests come before implementation (T007-T018 before T019-T052)
- [x] Parallel tasks truly independent (all [P] tasks use different files)
- [x] Each task specifies exact file path
- [x] No task modifies same file as another [P] task (verified)
- [x] Migration runs before implementation (T003-T006 before all others)
- [x] Protobuf generation before handlers/CLI/SDK (T019-T021 before T030-T047)
- [x] Quickstart scenarios covered (T048-T052 map to quickstart.md scenarios)
