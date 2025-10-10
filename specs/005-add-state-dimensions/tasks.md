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

## Progress Notes
- 2025-10-10: `go test ./cmd/gridapi/internal/server` (pass) covering updated handlers for T035-T038.
- 2025-10-10: `ListStates` avoids sending `state_content` by selecting `length(state_content) AS size_bytes`; verify equivalent `length` usage when SQLite backend is enabled.
- 2025-10-10: `go test ./pkg/sdk/...` (pass) after adding label helpers, ListStates options, and policy/labels client wrappers (T069).
- 2025-10-10: `go test ./cmd/gridctl/...` (pass) verifying T040-T042 CLI label support powered by pkg/sdk wrappers.
- 2025-10-10: `go test ./cmd/gridctl/... ./pkg/sdk/...` (pass) after adding policy command group (T046-T049) and SDK label validator for compliance.
- 2025-10-10: `npm run test` under `js/sdk` (pass) covering new bexpr utilities and StateInfo label support (T050-T052).
- 2025-10-10: `go test ./tests/integration -run TestLabel` (pass) exercising new label lifecycle/policy/filtering compliance suites (T071-T075).
- 2025-10-10: Fixed PolicyDefinition.Value() to return string(bytes) for JSONB compatibility; simplified Scan() logic; updated GetLabelPolicy handler type assertion. All repository and integration tests passing.
- 2025-10-10: Created js/sdk README.md with comprehensive label/bexpr documentation (T053); built React components (LabelList, updated DetailView/ListView) for label display (T054-T057); implemented useLabelPolicy hook with SDK abstraction (T059). Dashboard ready for filter UI (T060-T062).

## Phase 3.1: Setup & Dependencies
- [x] **T001** Add go-bexpr dependency to cmd/gridapi module (`cd cmd/gridapi && go get github.com/hashicorp/go-bexpr`)
- [x] **T002** [P] Add go-bexpr dependency to cmd/gridctl module (`cd cmd/gridctl && go get github.com/hashicorp/go-bexpr`)

## Phase 3.2: Database Schema & Models
⚠️ **CRITICAL: Complete migration before any implementation tasks**

- [x] **T003** Create LabelMap and PolicyDefinition types in `cmd/gridapi/internal/db/models/state.go` (add LabelMap type with Scan/Value methods per data-model.md lines 102-123)
- [x] **T004** [P] Create LabelPolicy model in `cmd/gridapi/internal/db/models/label_policy.go` (per data-model.md lines 154-202)
- [x] **T005** Create migration `cmd/gridapi/internal/migrations/20251009000001_add_state_labels.go` (add labels JSONB column to states table, create label_policy table with single-row constraint, add GIN index; per data-model.md lines 207-290)
- [x] **T006** Run migration to verify schema changes (`make db-reset && make build && ./bin/gridapi db init && ./bin/gridapi db migrate`)

## Phase 3.3: Tests First (TDD) ⚠️ MUST COMPLETE BEFORE 3.4
**CRITICAL: These tests MUST be written and MUST FAIL before ANY implementation**

### Repository Layer Tests
- [x] **T007** [P] Write test for BunStateRepository.Create with labels in `cmd/gridapi/internal/repository/bun_state_repository_test.go` (verify labels persist correctly in JSONB column)
- [x] **T008** [P] Write test for BunStateRepository.Update with label changes in `cmd/gridapi/internal/repository/bun_state_repository_test.go` (verify label updates apply atomically AND updated_at timestamp changes per FR-009)
- [x] **T009** [P] Write test for BunStateRepository.Update preserving other state fields in `cmd/gridapi/internal/repository/bun_state_repository_test.go` (verify logic_id, state_content, lock_info stay intact when only labels change)
- [x] **T010** [P] Write test for BunStateRepository.ListWithFilter with bexpr in `cmd/gridapi/internal/repository/bun_state_repository_test.go` (verify in-memory bexpr filtering per data-model.md lines 360-411)
- [x] **T011** [P] Write test for deterministic label ordering in `cmd/gridapi/internal/repository/bun_state_repository_test.go` (verify labels returned in alphabetical key order per FR-007)
- [x] **T012** [P] Write test for BunLabelPolicyRepository.GetPolicy in `cmd/gridapi/internal/repository/bun_label_policy_repository_test.go` (verify single-row retrieval)
- [x] **T013** [P] Write test for BunLabelPolicyRepository.SetPolicy in `cmd/gridapi/internal/repository/bun_label_policy_repository_test.go` (verify version increment and policy_json update)

### Service Layer Tests
- [x] **T014** [P] Write test for LabelValidator.Validate with policy constraints in `cmd/gridapi/internal/state/label_validator_test.go` (test key format regex, enum validation, reserved prefixes, size limits per data-model.md lines 299-327)
- [x] **T015** [P] Write test for StateService.CreateState with labels in `cmd/gridapi/internal/state/service_test.go` (verify label validation runs before persistence)
- [x] **T016** [P] Write test for StateService.UpdateLabels in `cmd/gridapi/internal/state/service_test.go` (verify add/remove/replace operations and policy enforcement)
- [x] **T017** [P] Write test for StateService.UpdateLabels updating updated_at in `cmd/gridapi/internal/state/service_test.go` (verify FR-009: updated_at timestamp bumps when labels change)
- [x] **T017a** [P] Write test for PolicyService.SetPolicy with malformed JSON in `cmd/gridapi/internal/state/policy_service_test.go` (verify FR-029: invalid JSON rejected before activation with clear error)
- [x] **T017b** [P] Write test for PolicyService.SetPolicy with invalid schema in `cmd/gridapi/internal/state/policy_service_test.go` (verify FR-029: structurally valid JSON with wrong schema rejected, e.g., missing required fields, wrong types)

### Handler Tests
- [x] **T018** [P] Write test for UpdateStateLabels RPC handler in `cmd/gridapi/internal/server/connect_handlers_test.go` (verify adds/removals apply correctly, validation errors return INVALID_ARGUMENT per contracts/state-tags.md)
- [x] **T019** [P] Write test for GetLabelPolicy RPC handler in `cmd/gridapi/internal/server/connect_handlers_test.go` (verify policy retrieval)
- [x] **T020** [P] Write test for SetLabelPolicy RPC handler in `cmd/gridapi/internal/server/connect_handlers_test.go` (verify policy update and versioning)
- [x] **T020a** [P] Write test for SetLabelPolicy RPC handler with invalid policy in `cmd/gridapi/internal/server/connect_handlers_test.go` (verify FR-029: malformed/invalid policy returns INVALID_ARGUMENT status with validation details)
- [x] **T021** [P] Write test for ListStates with filter parameter in `cmd/gridapi/internal/server/connect_handlers_test.go` (verify bexpr filter delegation to repository)

### CLI Tests
- [x] **T022** [P] Write test for duplicate --label flag handling in `cmd/gridctl/cmd/state_create_test.go` (verify last value wins and user is informed per spec.md FR-014a)

## Phase 3.4: Core Implementation (ONLY after tests are failing)

### Protobuf Updates
- [x] **T023** Update `proto/state/v1/state.proto` to add LabelValue message (oneof string_value/number_value/bool_value), add labels field to State message, add filter field and include_labels field (default true) to ListStatesRequest per FR-020a, add UpdateStateLabelsRequest/Response, add GetLabelPolicyRequest/Response, add SetLabelPolicyRequest/Response (GetLabelEnum REMOVED per 2025-10-09 clarification)
- [x] **T024** Run `buf generate` to regenerate Go and TypeScript code
- [x] **T025** Run `buf lint` to verify proto changes

### Repository Layer
- [x] **T026** Implement BunStateRepository.ListWithFilter with deterministic ordering in `cmd/gridapi/internal/repository/bun_state_repository.go` (add bexpr filtering per data-model.md lines 360-411; fetch states, compile bexpr evaluator, filter in-memory, trim to page_size; sort labels alphabetically by key per FR-007)
- [x] **T027** Update BunStateRepository.Update to bump updated_at in `cmd/gridapi/internal/repository/bun_state_repository.go` (ensure updated_at changes when labels modified per FR-009)
- [x] **T028** [P] Create BunLabelPolicyRepository in `cmd/gridapi/internal/repository/bun_label_policy_repository.go` (implement GetPolicy, SetPolicy per data-model.md lines 350-355; GetEnumValues REMOVED per 2025-10-09 clarification)
- [x] **T029** Update StateRepository interface in `cmd/gridapi/internal/repository/interface.go` to add ListWithFilter method signature
- [x] **T030** [P] Create LabelPolicyRepository interface in `cmd/gridapi/internal/repository/interface.go` (add GetPolicy, SetPolicy methods; GetEnumValues REMOVED)

### Service Layer
- [x] **T031** Create LabelValidator in `cmd/gridapi/internal/state/label_validator.go` (implement Validate method with key format regex `^[a-z][a-z0-9_/]{0,31}$`, enum checks, reserved prefix checks, size limits per data-model.md lines 299-327)
- [x] **T032** Add StateService.UpdateLabels method in `cmd/gridapi/internal/state/service.go` (implement add/remove/replace logic with policy validation, atomic updates, ensure updated_at bumps)
- [x] **T033** Update StateService.CreateState in `cmd/gridapi/internal/state/service.go` to validate labels via LabelValidator before persisting
- [x] **T034** [P] Create PolicyService in `cmd/gridapi/internal/state/policy_service.go` (implement GetPolicy, SetPolicy with JSON/schema validation per FR-029, ValidateLabels dry-run; GetEnumValues REMOVED)
- [x] **T034a** [P] Create PolicyValidator in `cmd/gridapi/internal/state/policy_validator.go` (implement ValidatePolicyStructure: check valid JSON, required fields present [AllowedKeys, AllowedValues, etc.], correct types, sensible limits per FR-029)

### Connect RPC Handlers
- [x] **T035** Implement UpdateStateLabels RPC handler in `cmd/gridapi/internal/server/connect_handlers.go` (delegate to StateService.UpdateLabels, map errors per contracts/state-tags.md; covered by `go test ./cmd/gridapi/internal/server`)
- [x] **T036** Implement GetLabelPolicy RPC handler in `cmd/gridapi/internal/server/connect_handlers.go` (delegate to PolicyService.GetPolicy; validated via `go test ./cmd/gridapi/internal/server`)
- [x] **T037** Implement SetLabelPolicy RPC handler in `cmd/gridapi/internal/server/connect_handlers.go` (delegate to PolicyService.SetPolicy which validates via PolicyValidator per FR-029, increment version, return INVALID_ARGUMENT for malformed policy; tested with handler suite)
- [x] **T038** Update ListStates RPC handler in `cmd/gridapi/internal/server/connect_handlers.go` to support filter parameter, include_labels toggle (default true per FR-020a), and return sorted labels (delegates to StateService.ListStatesWithFilter; `go test ./cmd/gridapi/internal/server` passing)

## Phase 3.5: CLI Integration

### gridctl state commands
- [x] **T040** Add --label flag support to `gridctl state create` in `cmd/gridctl/cmd/state/create.go` (uses SDK `UpdateStateLabels` wrapper post-create; duplicate handling covered by `parseLabelArgs` + unit tests)
- [x] **T041** [P] Create `gridctl state set` command in `cmd/gridctl/cmd/state/set.go` (parses key=value/-key via shared helpers, applies mutations via SDK `UpdateStateLabels` and resolves .grid context)
- [x] **T042** Update `gridctl state list` command in `cmd/gridctl/cmd/state/list.go` to display labels using SDK summaries (sorted, 32-char preview) while leveraging `ListStatesWithOptions`
- [x] **T043** Add --filter flag to `gridctl state list` in `cmd/gridctl/cmd/state/list.go` (pipes expression into `sdk.ListStatesWithOptions`)
- [x] **T044** Add --label flag shortcut to `gridctl state list` in `cmd/gridctl/cmd/state/list.go` (uses shared parsers + `sdk.BuildBexprFilter` for AND expression)
- [x] **T045** Update `gridctl state get` command in `cmd/gridctl/cmd/state/get.go` to display full labels sorted alphabetically (retrieved via SDK summaries)

### gridctl policy commands
- [x] **T046** Create `gridctl policy` command group in `cmd/gridctl/cmd/policy/policy.go` (parent command with subcommands)
- [x] **T047** [P] Create `gridctl policy get` command in `cmd/gridctl/cmd/policy/get.go` (call GetLabelPolicy RPC, display policy JSON per spec.md FR-031)
- [x] **T048** [P] Create `gridctl policy set` command in `cmd/gridctl/cmd/policy/set.go` (read policy JSON from --file flag, call SetLabelPolicy RPC per quickstart.md lines 28-46)
- [x] **T049** Create `gridctl policy compliance` command in `cmd/gridctl/cmd/policy/compliance.go` (revalidate all states against current policy, list violations per spec.md FR-017b and FR-028b)

## Phase 3.6: Web Dashboard Integration

### TypeScript SDK Updates (Required before Dashboard)
- [x] **T050** Update StateInfo interface in `js/sdk/src/models/state-info.ts` to add `labels?: Record<string, string | number | boolean>` field (aligns with protobuf LabelValue oneof after T023-T025)
- [x] **T051** [P] Create bexpr filter utilities in `js/sdk/src/filters/bexpr.ts` (implement buildEqualityFilter, buildInFilter, combineFilters for simple bexpr string concatenation per FR-017 and 2025-10-09 clarification)
- [x] **T052** [P] Add unit tests for bexpr utilities in `js/sdk/src/__tests__/bexpr.test.ts` (test equality, in, AND/OR combinations, escaping)
- [x] **T053** [P] Update js/sdk documentation in `js/sdk/README.md` to cover bexpr filter helpers and label operations

### Dashboard Label Display (Read-Only)
- [x] **T054** [P] Create LabelList component in `webapp/src/components/LabelList.tsx` (display labels as key=value pairs sorted alphabetically per FR-007, FR-040; reusable component for DetailView)
- [x] **T055** Update DetailView in `webapp/src/components/DetailView.tsx` to add "Labels" tab to existing tab navigation (overview/json/dependencies/dependents/labels) with LabelList component per FR-040; use lucide-react Tag icon
- [x] **T056** [P] Update ListView in `webapp/src/components/ListView.tsx` to add Labels column to states table showing label count or preview (e.g., "3 labels" or "env:prod, team:core") per FR-041
- [x] **T057** [P] Add empty state handling to LabelList in `webapp/src/components/LabelList.tsx` (display "No labels" placeholder message when labels map is empty per FR-043)
- [x] **T058** [P] SKIP: GraphView does NOT display labels visually on nodes (per FR-041 - avoids UI crowding); label metadata used only for filtering (see T060)

### Dashboard Filtering (Read-Only) - MUST use js/sdk per FR-050 ...
- [x] **T059** [P] Create useLabelPolicy hook in `webapp/src/hooks/useLabelPolicy.ts` (fetch policy via js/sdk client wrapper calling GetLabelPolicy RPC, extract enums client-side per FR-044; MUST NOT call generated Connect client directly per FR-045)
- [ ] **T060** [P] Create LabelFilter component in `webapp/src/components/LabelFilter.tsx` (reusable filter UI with key dropdown, value dropdown/input based on enums from useLabelPolicy, "Add Filter" button; uses bexpr utilities from js/sdk per T051)
- [ ] **T061** Add LabelFilter to ListView in `webapp/src/components/ListView.tsx` (place filter row above states table; when filters change, call js/sdk listStates wrapper with bexpr filter string per FR-041a and FR-045)
- [ ] **T062** [P] Add LabelFilter to GraphView in `webapp/src/components/GraphView.tsx` (place filter controls in toolbar/sidebar; when filters change, call js/sdk listStates wrapper per FR-041a and FR-045)

### Dashboard Tests
- [ ] **T063** [P] Write test for LabelList component in `webapp/src/__tests__/dashboard_label_list.test.tsx` (verify alphabetical sorting, "No labels" empty state per FR-043, key=value formatting)
- [ ] **T064** [P] Write test for LabelFilter component in `webapp/src/__tests__/dashboard_label_filter.test.tsx` (verify enum dropdowns extracted from policy, free-text input, filter building with js/sdk bexpr utilities)
- [ ] **T065** [P] Write test for DetailView labels tab in `webapp/src/__tests__/dashboard_detail_view.test.tsx` (add test case verifying labels tab renders with LabelList)
- [ ] **T066** [P] Write test for ListView labels column in `webapp/src/__tests__/dashboard_list_view.test.tsx` (add test case verifying labels column displays count/preview)
- [ ] **T067** [P] Write test for ListView filtering in `webapp/src/__tests__/dashboard_list_view.test.tsx` (verify LabelFilter triggers js/sdk listStates with bexpr filter per FR-045)
- [ ] **T068** [P] Write test for GraphView filtering in `webapp/src/__tests__/dashboard_graph_view.test.tsx` (verify LabelFilter triggers js/sdk filtered state fetch per FR-045)

## Phase 3.7: Go SDK Updates
- [x] **T069** Add LabelMap helper functions to `pkg/sdk/client.go` (ConvertProtoLabels, BuildBexprFilter, SortLabels for SDK consumers per FR-017)
- [ ] **T070** Update SDK documentation in `pkg/sdk/README.md` to cover label operations and bexpr filter construction per FR-017

## Phase 3.8: Integration Testing
- [x] **T071** Create integration test in `tests/integration/labels_test.go` (verify quickstart.md scenario 1: create state with labels via CLI and confirm persistence via SDK summary)
- [x] **T072** [P] Create integration test in `tests/integration/labels_test.go` (verify quickstart.md scenario 2: set policy, submit invalid label, verify validation error)
- [x] **T073** [P] Create integration test in `tests/integration/labels_test.go` (verify quickstart.md scenario 3: add/remove labels via state set, verify atomic updates)
- [x] **T074** [P] Create integration test in `tests/integration/labels_test.go` (verify quickstart.md scenario 5: bexpr filter expressions return correct states and CLI filters)
- [x] **T075** [P] Create integration test in `tests/integration/labels_test.go` (verify quickstart.md scenario 7-8: policy update, compliance command detects and resolves violations)
- [ ] **T076** [P] Create integration test in `tests/integration/labels_dashboard_test.go` (verify dashboard uses js/sdk wrappers per FR-045, displays labels in detail view, list/graph view filtering with enums extracted from GetLabelPolicy per FR-044)

## Phase 3.9: Polish & Documentation
- [ ] **T077** [P] Add unit tests for label key regex validation in `cmd/gridapi/internal/state/label_validator_test.go` (test edge cases: hyphens rejected, underscores/slashes allowed, uppercase rejected per spec.md FR-008)
- [ ] **T078** [P] Add unit tests for LabelMap Scan/Value methods in `cmd/gridapi/internal/db/models/state_test.go` (verify JSON marshaling for string/number/bool types)
- [ ] **T079** [P] Add unit tests for CLI truncation in `cmd/gridctl/cmd/state_list_test.go` (verify comma-separated format with 32-char truncation per FR-015)
- [ ] **T080** Run quickstart.md manual verification (`make db-reset && make build && ./bin/gridapi db init && ./bin/gridapi db migrate`, start gridapi, execute all quickstart commands, verify expected outputs)
- [ ] **T081** Update `CHANGELOG.md` with State Labels feature summary
- [ ] **T082** Add bexpr filter examples to `README.md` or docs/ (show compound expressions, escape rules, best practices per research.md)
- [ ] **T083** Performance test: verify <50ms p99 latency for bexpr filtering with 500 states (load test script in `tests/performance/labels_filter_bench_test.go`)

---

## Dependencies

### Hard Blockers (Must Complete Before)
- **T003-T006** (schema/models/migration) block ALL implementation tasks
- **T007-T022** (tests) block T023-T076 (implementation)
- **T023-T025** (protobuf) block T035-T038 (handlers), T040-T049 (CLI), T050-T068 (SDK/dashboard), T069-T070 (Go SDK)

### Soft Dependencies (Recommended Order)
- **T026-T030** (repository) before T031-T034a (service)
- **T031-T034a** (service + validators) before T035-T038 (handlers)
- **T034a** (PolicyValidator) before T034 (PolicyService uses validator)
- **T035-T038** (handlers) before T040-T070 (CLI/dashboard/SDK)
- **T050** (TypeScript StateInfo update) before T054-T068 (dashboard components/tests)
- **T051-T053** (js/sdk bexpr utilities + tests + docs) before T060 (LabelFilter component uses utilities)
- **T054** (LabelList component) before T055 (DetailView uses LabelList)
- **T059** (useLabelPolicy hook) before T060-T062 (LabelFilter components use hook)
- **T060** (LabelFilter component) before T061-T062 (ListView/GraphView use LabelFilter)
- **T071-T076** (integration tests) after T040-T070 (CLI/dashboard/SDK complete)
- **T077-T083** (polish) after all implementation complete

### No Dependencies (Parallel Safe)
- T001, T002 (different modules)
- T003, T004 (different files)
- T007-T013 (different test files)
- T014-T017b (different test files, different classes; T017a/T017b are policy_service_test.go)
- T018-T020a (different handler test scenarios, same file but independent test functions)
- T028, T029, T030 (different files from T026-T027)
- T034, T034a (different files: policy_service.go vs policy_validator.go)
- T047, T048 (different CLI command files)
- T051, T052, T053 (different js/sdk files: utilities, tests, docs)
- T054, T056, T057, T058 (different dashboard component files)
- T059, T060 (different files: hook vs component)
- T063-T068 (different dashboard test files)
- T072-T076 (different integration test files)
- T077, T078, T079, T081, T082 (different polish tasks)

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
# Launch T007-T013 together (all [P]):
# Task T007: BunStateRepository.Create with labels test
# Task T008: BunStateRepository.Update with labels + updated_at test
# Task T009: BunStateRepository.Update preserving other fields test
# Task T010: BunStateRepository.ListWithFilter test
# Task T011: Deterministic label ordering test
# Task T012: BunLabelPolicyRepository.GetPolicy test
# Task T013: BunLabelPolicyRepository.SetPolicy test
```

### Phase 3.3: Service Tests (Run in Parallel)
```bash
# Launch T014-T017 together (all [P]):
# Task T014: LabelValidator.Validate test
# Task T015: StateService.CreateState with labels test
# Task T016: StateService.UpdateLabels test
# Task T017: StateService.UpdateLabels updating updated_at test
```

### Phase 3.3: Handler & CLI Tests (Run in Parallel)
```bash
# Launch T018-T022 together (all [P]):
# Task T018: UpdateStateLabels RPC handler test
# Task T019: GetLabelPolicy RPC handler test
# Task T020: SetLabelPolicy RPC handler test
# Task T021: ListStates with filter test
# Task T022: Duplicate --label flag handling test
```

### Phase 3.5: CLI Policy Commands (Run in Parallel)
```bash
# Launch T047-T049 together (different files):
# Task T047: gridctl policy get
# Task T048: gridctl policy set
# Task T049: gridctl policy enum
# (T050 compliance command is sequential - required feature)
```

### Phase 3.6: Dashboard Components (Staged Parallel Execution)
```bash
# Stage 1: T051 (TypeScript types) - REQUIRED FIRST
# Stage 2: Launch T052, T054, T055, T056, T057 together (all [P]):
#   Task T052: LabelList component
#   Task T054: ListView labels column
#   Task T055: Empty state in LabelList
#   Task T056: SKIP (GraphView no visual display)
#   Task T057: useLabelPolicy hook
# Stage 3: After T052, T057 complete:
#   Task T053: DetailView tab (needs T052 LabelList)
#   Task T058: LabelFilter component (needs T057 hook) [P with T053]
# Stage 4: After T058 complete, launch T059-T060 together:
#   Task T059: ListView filtering (needs T058 LabelFilter)
#   Task T060: GraphView filtering (needs T058 LabelFilter)
# Stage 5: Launch T061-T066 together (all [P]):
#   Task T061-T066: Dashboard tests
```

### Phase 3.8: Integration Tests (Run in Parallel)
```bash
# Launch T069-T074 together (all [P]):
# Task T069: labels_lifecycle_test.go
# Task T070: labels_policy_test.go
# Task T071: labels_update_test.go
# Task T072: labels_filtering_test.go
# Task T073: labels_compliance_test.go
# Task T074: labels_dashboard_test.go (includes filter testing)
```

---

## Notes
- **[P]** tasks target different files with no shared dependencies
- **Verify tests fail** before implementing (TDD discipline)
- **Commit after each task** to track incremental progress
- **Scope boundary**: Labels only; state-schemas.md and state-facets.md contracts deferred to future milestone per scope-reduction.md
- **Bexpr grammar**: Document identifier constraints (`[a-z][a-z0-9_/]*`) in error messages per spec.md FR-008
- **SQLite parity**: Migration uses PostgreSQL JSONB; SQLite adaptation deferred to future appliance deployment
- **Terminology**: All code uses "labels" not "tags" (contracts/state-tags.md updated 2025-10-09)
- **Compliance command**: Required per FR-028b (not optional); T049 is mandatory
- **Deterministic ordering**: All label displays must sort alphabetically by key per FR-007
- **Timestamp tracking**: updated_at MUST bump when labels change per FR-009
- **Dashboard stance**: Read-only in this milestone per FR-042; inline editing deferred
- **Policy validation (FR-029)**: PolicyValidator (T034a) MUST reject malformed JSON and invalid schema (missing fields, wrong types) before SetPolicy persists to database; tests T017a, T017b, T020a verify rejection with clear error messages

---

## Validation Checklist
*GATE: Checked before execution*

- [x] All contracts have corresponding tests (state-tags.md → T018; schemas/facets deferred)
- [x] All entities have model tasks (State with Labels → T003; LabelPolicy → T004)
- [x] All tests come before implementation (T007-T022 before T023-T076)
- [x] Parallel tasks truly independent (all [P] tasks use different files)
- [x] Each task specifies exact file path (verified with actual webapp structure)
- [x] No task modifies same file as another [P] task (verified; T055, T061, T062 sequential where needed)
- [x] Migration runs before implementation (T003-T006 before all others)
- [x] Protobuf generation before handlers/CLI/SDK/dashboard (T023-T025 before T035-T070)
- [x] Quickstart scenarios covered (T071-T075 map to quickstart.md scenarios)
- [x] **Constitution Principle III enforced (2025-10-09 update)**:
  - [x] FR-045: Dashboard MUST use js/sdk wrappers, not generated Connect clients (T059, T061, T062 enforce this)
  - [x] All dashboard tasks reference js/sdk client usage explicitly (T059-T062, T064, T067-T068, T076)
  - [x] Integration test T076 verifies FR-045 compliance
- [x] **GetLabelEnum RPC removed (2025-10-09 update)**:
  - [x] T023 proto excludes GetLabelEnum, includes only GetLabelPolicy
  - [x] T028, T030, T034 repository/service exclude GetEnumValues methods
  - [x] T038 handler removed (was GetLabelEnum handler)
  - [x] T049 CLI command renumbered (was T050; T049 policy enum removed)
  - [x] FR-044 updated: UIs extract enums from GetLabelPolicy (T059 useLabelPolicy hook)
- [x] **TypeScript SDK bexpr utilities added (2025-10-09 update)**:
  - [x] T051: bexpr filter utilities (buildEqualityFilter, buildInFilter, combineFilters)
  - [x] T052: Unit tests for bexpr utilities
  - [x] T053: js/sdk documentation updated
  - [x] T060: LabelFilter uses bexpr utilities from T051
  - [x] Aligns with FR-017: Go SDK rich builders, TypeScript SDK simple string utilities
- [x] **FR-020a include_labels field added**:
  - [x] T023 proto includes include_labels field (default true) on ListStatesRequest
  - [x] T038 handler implements include_labels toggle
- [x] **FR-015 CLI output format clarified (2025-10-09 update)**:
  - [x] T042: state list shows comma-separated key=value truncated at 32 chars
  - [x] T045: state get shows full labels without truncation
  - [x] T079: Unit tests verify truncation behavior
- [x] **FR-029 policy validation (2025-10-09 update)**:
  - [x] T017a: PolicyService.SetPolicy rejects malformed JSON with clear error
  - [x] T017b: PolicyService.SetPolicy rejects invalid schema (missing fields, wrong types)
  - [x] T020a: SetLabelPolicy RPC handler returns INVALID_ARGUMENT for malformed policy
  - [x] T034: PolicyService.SetPolicy calls PolicyValidator before persistence
  - [x] T034a: PolicyValidator.ValidatePolicyStructure implementation
  - [x] T037: SetLabelPolicy handler delegates validation to PolicyService
- [x] Web dashboard requirements covered (T050-T068 satisfy FR-040–FR-045)
  - [x] FR-040: DetailView labels tab (T055 adds Labels tab with LabelList component T054)
  - [x] FR-041: ListView labels column (T056); GraphView NO visual display (T058 SKIP)
  - [x] FR-041a: ListView filtering (T060-T061); GraphView filtering (T060, T062)
  - [x] FR-042: Read-only stance documented in all task descriptions
  - [x] FR-043: Empty state handling (T057 "No labels" placeholder in LabelList component)
  - [x] FR-044: Policy enum extraction from GetLabelPolicy (T059 useLabelPolicy hook)
  - [x] FR-045: Dashboard uses js/sdk wrappers (T059, T061-T062 enforce Constitution Principle III)
- [x] Dashboard filtering requirements clarified:
  - [x] LabelFilter component (T060) reusable across ListView and GraphView
  - [x] Enum dropdown extracted from GetLabelPolicy per FR-044 (T059, T060)
  - [x] Free-text input when no policy or no enums for key (T060, FR-044)
  - [x] Both views call js/sdk listStates wrapper with bexpr filter (T061, T062, FR-041a, FR-045)
- [x] Deterministic label ordering addressed (T011, T026, FR-007)
- [x] Timestamp tracking addressed (T008, T017, T027, T032, FR-009)
- [x] Preserve-other-metadata addressed (T009)
- [x] Duplicate-flag handling addressed (T022, T040, FR-014a)
- [x] Compliance command scope clarified (T049 required per FR-017b and FR-028b)
- [x] TypeScript SDK types updated before dashboard (T050 before T054-T068)
- [x] Webapp paths verified: webapp/src/components/, webapp/src/hooks/, webapp/src/__tests__/
- [x] GraphView UI crowding concern addressed (T058 SKIP, filtering only via T062)
- [x] **Updated task count: 87 tasks** (up from original 80, +7 for js/sdk bexpr utilities T051-T053, CLI truncation test T079, FR-029 policy validation T017a/T017b/T020a/T034a)
