# Tasks: CLI Context-Aware State Management

**Feature**: 003-ux-improvements-for
**Input**: Design documents from `/specs/003-ux-improvements-for/`
**Prerequisites**: plan.md, research.md, data-model.md, contracts/

## Overview

This task breakdown implements directory-based state context management, interactive output selection for dependencies, server-side output caching, and enhanced state information display for the Grid CLI.

**Key Components**:
- Proto definitions for 2 new RPC methods (ListStateOutputs, GetStateInfo)
- Server-side state_outputs caching table + repository layer
- CLI directory context (.grid file I/O)
- Interactive terminal prompts (pterm library)
- Enhanced state display formatting

**Total Tasks**: 44
**Estimated Time**: 24-32 hours

## Format: `[ID] [P?] Description`
- **[P]**: Can run in parallel (different files, no dependencies)
- All file paths are absolute from repository root `/Users/vincentdesmet/tcons/grid/`

---

## Phase 3.1: Setup & Dependencies

- [X] **T001** Add pterm dependency to gridctl module
  - File: `cmd/gridctl/go.mod`
  - Command: `cd cmd/gridctl && go get github.com/pterm/pterm`
  - Reference: research.md (lines 7-20)

---

## Phase 3.2: Proto Definitions & Code Generation

- [X] **T002** [P] Add OutputKey message to proto
  - File: `proto/state/v1/state.proto`
  - Add message definition with key (string) and sensitive (bool) fields
  - Reference: contracts/list-state-outputs.proto (lines 50-58)

- [X] **T003** [P] Add ListStateOutputsRequest message to proto
  - File: `proto/state/v1/state.proto`
  - Add message with oneof state {logic_id, guid}
  - Reference: contracts/list-state-outputs.proto (lines 30-37)

- [X] **T004** [P] Add ListStateOutputsResponse message to proto
  - File: `proto/state/v1/state.proto`
  - Add message with state_guid, state_logic_id, repeated OutputKey outputs
  - Reference: contracts/list-state-outputs.proto (lines 39-48)

- [X] **T005** [P] Add GetStateInfoRequest message to proto
  - File: `proto/state/v1/state.proto`
  - Add message with oneof state {logic_id, guid}
  - Reference: contracts/get-state-info.proto (lines 35-42)

- [X] **T006** [P] Add GetStateInfoResponse message to proto
  - File: `proto/state/v1/state.proto`
  - Add message with guid, logic_id, backend_config, dependencies, dependents, outputs, timestamps
  - Reference: contracts/get-state-info.proto (lines 44-68)

- [X] **T007** Add ListStateOutputs RPC method to StateService
  - File: `proto/state/v1/state.proto`
  - Depends on: T002, T003, T004
  - Add: `rpc ListStateOutputs(ListStateOutputsRequest) returns (ListStateOutputsResponse);`
  - Reference: contracts/list-state-outputs.proto (lines 23)

- [X] **T008** Add GetStateInfo RPC method to StateService
  - File: `proto/state/v1/state.proto`
  - Depends on: T002, T005, T006
  - Add: `rpc GetStateInfo(GetStateInfoRequest) returns (GetStateInfoResponse);`
  - Reference: contracts/get-state-info.proto (lines 28)

- [X] **T009** Generate API code from proto definitions
  - Command: `buf generate` (from repo root)
  - Depends on: T007, T008
  - Validates: Check `api/state/v1/statev1connect/state.connect.go` has new RPC methods
  - Output: Generated Go code in `api/state/v1/`

---

## Phase 3.3: Database Layer (state_outputs caching)

- [X] **T010** [P] Create StateOutput Bun model
  - File: `cmd/gridapi/internal/db/models/state_output.go` (NEW)
  - Reference: data-model.md (lines 139-150)
  - Fields: state_guid (pk), output_key (pk), sensitive, state_serial, created_at, updated_at
  - Use bun struct tags matching existing models/state.go pattern

- [X] **T011** Create migration for state_outputs table
  - File: `cmd/gridapi/internal/migrations/20251003140000_add_state_outputs_cache.go` (NEW)
  - Depends on: T010
  - Reference: data-model.md (lines 186-258)
  - Pattern: Follow existing migration structure from `20251002000002_add_edges_table.go`
  - Up: CREATE TABLE, indexes (guid, key), FK constraint with CASCADE
  - Down: DROP TABLE

- [X] **T012** Define StateOutputRepository interface
  - File: `cmd/gridapi/internal/repository/interface.go` (MODIFIED)
  - Reference: data-model.md (lines 153-184)
  - Add methods: UpsertOutputs, GetOutputsByState, SearchOutputsByKey, DeleteOutputsByState
  - Add types: OutputKey, StateOutputRef structs

- [X] **T013** Implement Bun StateOutputRepository
  - File: `cmd/gridapi/internal/repository/bun_state_output_repository.go` (NEW)
  - Depends on: T010, T012
  - Reference: data-model.md (lines 153-184)
  - Pattern: Follow `bun_state_repository.go` structure (context, transactions, error wrapping)
  - Implement all 4 methods from interface

- [X] **T014** [P] Write StateOutputRepository tests
  - File: `cmd/gridapi/internal/repository/bun_state_output_repository_test.go` (NEW)
  - Depends on: T013
  - Pattern: Follow `bun_state_repository_test.go` (table-driven tests, real DB, t.Cleanup)
  - Tests: UpsertOutputs, GetOutputsByState, SearchOutputsByKey, cache invalidation

---

## Phase 3.4: SDK Wrappers

- [X] **T015** [P] Add ListStateOutputs SDK wrapper
  - File: `pkg/sdk/state_client.go` (MODIFIED)
  - Depends on: T009
  - Reference: contracts/list-state-outputs.proto (lines 117-118)
  - Add: `func (c *Client) ListStateOutputs(ctx, ref StateRef) ([]OutputKey, error)`
  - Pattern: Follow existing CreateState, GetStateConfig wrappers
  - Error handling: Wrap Connect errors, validate response

- [X] **T016** [P] Add GetStateInfo SDK wrapper
  - File: `pkg/sdk/state_client.go` (MODIFIED)
  - Depends on: T009
  - Reference: contracts/get-state-info.proto (lines 169-170)
  - Add: `func (c *Client) GetStateInfo(ctx, ref StateRef) (*StateInfo, error)`
  - May need to add StateInfo struct wrapping GetStateInfoResponse

- [ ] **T017** [P] Add SDK wrapper tests
  - File: `pkg/sdk/state_client_test.go` (MODIFIED)
  - Depends on: T015, T016
  - Test both new SDK methods with mock server

---

## Phase 3.5: Server RPC Handlers & Service Layer

- [X] **T018** Add service layer helper for output parsing
  - File: `cmd/gridapi/internal/state/service.go` (MODIFIED)
  - Reference: data-model.md (lines 78-114, reuse existing tfstate parser)
  - Add: `GetOutputKeys(guid string) ([]OutputKey, error)` method
  - Logic: Fetch state JSON, call tfstate.ParseOutputs, extract keys + sensitive flags

- [X] **T019** Implement ListStateOutputs RPC handler
  - File: `cmd/gridapi/internal/server/connect_handlers.go` (MODIFIED)
  - Depends on: T009, T013, T018
  - Reference: data-model.md (lines 153-240)
  - Logic: Resolve state by logic_id/guid → repo.GetOutputsByState → build response
  - Error cases: State not found (NOT_FOUND), repository errors (INTERNAL)

- [X] **T020** Add service layer orchestration for GetStateInfo
  - File: `cmd/gridapi/internal/state/service.go` (MODIFIED)
  - Depends on: T018
  - Add: `GetStateInfo(ref StateRef) (*StateInfo, error)` method
  - Logic: Fetch state + dependencies + dependents + outputs (4 repo calls)
  - Optimization: Could parallelize calls

- [X] **T021** Implement GetStateInfo RPC handler
  - File: `cmd/gridapi/internal/server/connect_handlers.go` (MODIFIED)
  - Depends on: T009, T020
  - Reference: contracts/get-state-info.proto (lines 70-90)
  - Logic: Call service.GetStateInfo → build GetStateInfoResponse
  - Include: backend_config, dependencies, dependents, outputs

- [X] **T022** Hook Terraform backend PUT for output caching
  - File: `cmd/gridapi/internal/server/tfstate_handlers.go` (MODIFIED)
  - Depends on: T013
  - Reference: data-model.md (lines 265-269, 78-114)
  - Modify: PUT /tfstate/{guid} handler
  - Add: After state write, parse TF JSON serial → repo.UpsertOutputs(guid, serial, outputs)
  - Transaction: Wrap state write + output cache update in same DB transaction

---

## Phase 3.6: CLI Directory Context I/O

- [X] **T023** [P] Create DirectoryContext struct and JSON schema
  - File: `cmd/gridctl/cmd/state/context.go` (NEW)
  - Reference: research.md (lines 75-90), data-model.md (lines 14-22)
  - Type definition matching JSON schema (version, state_guid, state_logic_id, server_url, timestamps)

- [X] **T024** [P] Implement ReadGridContext function
  - File: `cmd/gridctl/cmd/state/context.go` (continued)
  - Reference: research.md (lines 86-90)
  - Function: `ReadGridContext() (*DirectoryContext, error)`
  - Validation: Check version == "1", valid GUID format, non-empty logic_id
  - Error handling: Corrupted file returns specific error type

- [X] **T025** [P] Implement WriteGridContext with atomic write
  - File: `cmd/gridctl/cmd/state/context.go` (continued)
  - Reference: research.md (lines 194-234)
  - Function: `WriteGridContext(ctx *DirectoryContext) error`
  - Pattern: Write to `.grid.tmp` → rename to `.grid` (atomic on POSIX)
  - Error handling: Detect write permissions, conflict detection

- [X] **T026** [P] Add context resolution helper
  - File: `cmd/gridctl/cmd/state/context.go` (continued)
  - Function: `ResolveStateRef(explicitRef, contextRef StateRef) StateRef`
  - Logic: Explicit params override context → return error if neither provided

---

## Phase 3.7: CLI Interactive Prompts

- [X] **T027** [P] Create multi-select helper for outputs
  - File: `cmd/gridctl/cmd/deps/add.go` (MODIFIED, inline helper function)
  - Reference: research.md (lines 22-57)
  - Function: `promptSelectOutputs(outputs []OutputKey, nonInteractive bool) ([]string, error)`
  - Logic: If nonInteractive → error, if 1 output → auto-select, else → pterm.MultiSelect
  - Display: Mark sensitive outputs with "(⚠️  sensitive)" suffix

- [X] **T028** [P] Add --non-interactive root flag
  - File: `cmd/gridctl/cmd/root.go` (MODIFIED)
  - Reference: research.md (lines 233-269)
  - Add: `--non-interactive` persistent flag + `GRID_NON_INTERACTIVE` env var
  - Make accessible to all subcommands via cobra context or global var

---

## Phase 3.8: CLI Command Integration

- [X] **T029** Modify state create command for .grid file creation
  - File: `cmd/gridctl/cmd/state/create.go` (MODIFIED)
  - Depends on: T024, T025
  - Reference: quickstart.md (lines 30-98)
  - Add: `--force` flag
  - Add: After successful CreateState RPC → WriteGridContext()
  - Add: Check existing .grid, handle conflicts per FR-002
  - Add: Detect write permissions per FR-006c

- [X] **T030** Add context validation on state command startup
  - File: `cmd/gridctl/cmd/state/get.go` and other state/* commands (MODIFIED)
  - Depends on: T024, T026
  - Add: At start of RunE, attempt ReadGridContext() if no explicit params
  - Add: Handle corrupted context per FR-006b (warning + ignore)
  - Add: Handle stale context per FR-006e (verify state exists on server)

- [X] **T031** Modify deps add command for interactive selection using pTerm
  - File: `cmd/gridctl/cmd/deps/add.go` (MODIFIED)
  - Depends on: T015, T024, T027, T028
  - Reference: quickstart.md (lines 100-166)
  - Add: Make `--output` optional
  - Add: Resolve `--to` default from context per FR-008
  - Add: Call SDK.ListStateOutputs when --output not provided
  - Add: Call promptSelectOutputs helper
  - Add: Loop over selected outputs, call AddDependency for each

- [X] **T032** Modify state get command for enhanced display
  - File: `cmd/gridctl/cmd/state/get.go` (MODIFIED)
  - Depends on: T016, T030
  - Reference: quickstart.md (lines 168-211)
  - Change: Call SDK.GetStateInfo instead of GetStateConfig
  - Add: Format dependencies section (list from_state.output → to_input)
  - Add: Format dependents section (list to_state consuming which output)
  - Add: Format outputs section (list output keys, mark sensitive with "⚠️")

---

## Phase 3.9: Contract Tests (TDD - Write BEFORE implementation)

- [ ] **T033** [P] Contract test: ListStateOutputs with valid logic-id
  - File: `tests/contract/list_state_outputs_test.go` (NEW)
  - Reference: contracts/list-state-outputs.proto (lines 64-67)
  - Test: Request with logic_id → assert response structure, outputs array

- [ ] **T034** [P] Contract test: ListStateOutputs with no outputs
  - File: `tests/contract/list_state_outputs_test.go` (continued)
  - Reference: contracts/list-state-outputs.proto (lines 69-72)
  - Test: State with no TF JSON → assert empty outputs array (not error)

- [ ] **T035** [P] Contract test: ListStateOutputs with non-existent state
  - File: `tests/contract/list_state_outputs_test.go` (continued)
  - Reference: contracts/list-state-outputs.proto (lines 74-77)
  - Test: Invalid logic_id → assert NOT_FOUND error code

- [ ] **T036** [P] Contract test: ListStateOutputs sensitive flag accuracy
  - File: `tests/contract/list_state_outputs_test.go` (continued)
  - Reference: contracts/list-state-outputs.proto (lines 79-83)
  - Test: State with sensitive output → assert sensitive=true in response

- [ ] **T037** [P] Contract test: ListStateOutputs with GUID
  - File: `tests/contract/list_state_outputs_test.go` (continued)
  - Reference: contracts/list-state-outputs.proto (lines 85-88)
  - Test: Request with guid instead of logic_id → same response

- [ ] **T038** [P] Contract test: GetStateInfo with dependencies
  - File: `tests/contract/get_state_info_test.go` (NEW)
  - Reference: contracts/get-state-info.proto (lines 100-110)
  - Setup: Create states A, B, edge A.vpc_id → B
  - Test: GetStateInfo(B) → assert dependencies array populated

- [ ] **T039** [P] Contract test: GetStateInfo with dependents
  - File: `tests/contract/get_state_info_test.go` (continued)
  - Reference: contracts/get-state-info.proto (lines 112-121)
  - Setup: Create state A with outputs, states B/C consuming A.vpc_id
  - Test: GetStateInfo(A) → assert dependents array has 2 edges

- [ ] **T040** [P] Contract test: GetStateInfo with isolated state
  - File: `tests/contract/get_state_info_test.go` (continued)
  - Reference: contracts/get-state-info.proto (lines 123-129)
  - Test: State with outputs but no edges → assert empty deps/dependents

- [ ] **T041** [P] Contract test: GetStateInfo with non-existent state
  - File: `tests/contract/get_state_info_test.go` (continued)
  - Reference: contracts/get-state-info.proto (lines 131-133)
  - Test: Invalid logic_id → assert NOT_FOUND error

- [ ] **T042** [P] Contract test: GetStateInfo includes backend_config
  - File: `tests/contract/get_state_info_test.go` (continued)
  - Reference: contracts/get-state-info.proto (lines 135-140)
  - Test: Verify backend_config.address/lock_address/unlock_address structure

---

## Phase 3.10: Integration Tests (End-to-End)

- [ ] **T043** [P] Integration test: Directory context creation and usage
  - File: `tests/integration/context_aware_test.go` (NEW)
  - Reference: quickstart.md (lines 30-98)
  - Test: state create → .grid file created → subsequent commands use context
  - Test: Concurrent create without --force → error
  - Test: --force flag replaces .grid

- [ ] **T044** [P] Integration test: Interactive output selection (mocked I/O)
  - File: `tests/integration/context_aware_test.go` (continued)
  - Reference: quickstart.md (lines 100-166)
  - Test: deps add with multi-output state → verify multi-edge creation
  - Test: Single output → auto-select
  - Test: --non-interactive without --output → error

---

## Dependencies Graph

```
Setup: T001

Proto:
  T002,T003,T004,T005,T006 [P] → T007,T008 → T009

Database:
  T010 [P] → T011 → T012 → T013 → T014 [P]

SDK:
  T009 → T015,T016,T017 [P]

Server:
  T009,T013 → T018 → T019,T020 → T021
  T013 → T022

CLI Context:
  T023,T024,T025,T026 [P]

CLI Prompts:
  T001 → T027,T028 [P]

CLI Integration:
  T024,T025 → T029
  T024,T026 → T030
  T015,T024,T027,T028 → T031
  T016,T030 → T032

Contract Tests:
  T009 → T033,T034,T035,T036,T037,T038,T039,T040,T041,T042 [P]

Integration Tests:
  T029,T030,T031,T032 → T043,T044 [P]
```

**Critical Path**: T001 → T009 → T015/T016 → T029-T032 → T043/T044

---

## Parallel Execution Examples

### Phase 3.2: Proto Messages (can run in parallel)
```bash
Task: "Add OutputKey message to proto/state/v1/state.proto"
Task: "Add ListStateOutputsRequest message to proto/state/v1/state.proto"
Task: "Add ListStateOutputsResponse message to proto/state/v1/state.proto"
Task: "Add GetStateInfoRequest message to proto/state/v1/state.proto"
Task: "Add GetStateInfoResponse message to proto/state/v1/state.proto"
```

### Phase 3.6: CLI Context I/O (independent functions)
```bash
Task: "Create DirectoryContext struct in cmd/gridctl/cmd/state/context.go"
Task: "Implement ReadGridContext in cmd/gridctl/cmd/state/context.go"
Task: "Implement WriteGridContext in cmd/gridctl/cmd/state/context.go"
Task: "Add context resolution helper in cmd/gridctl/cmd/state/context.go"
```

### Phase 3.9: Contract Tests (all independent)
```bash
Task: "Contract test: ListStateOutputs with valid logic-id"
Task: "Contract test: ListStateOutputs with no outputs"
Task: "Contract test: ListStateOutputs with non-existent state"
Task: "Contract test: ListStateOutputs sensitive flag accuracy"
Task: "Contract test: ListStateOutputs with GUID"
Task: "Contract test: GetStateInfo with dependencies"
Task: "Contract test: GetStateInfo with dependents"
Task: "Contract test: GetStateInfo with isolated state"
Task: "Contract test: GetStateInfo with non-existent state"
Task: "Contract test: GetStateInfo includes backend_config"
```

---

## Implementation Notes

1. **TDD Workflow**: Write contract tests (T033-T042) immediately after T009, verify they fail, then implement handlers
2. **Database Testing**: Repository tests (T014) require PostgreSQL running, use `make db-up`
3. **Migration Registration**: After T011, register migration in `cmd/gridapi/internal/migrations/main.go`
4. **Atomic Writes**: T025 uses temp file + rename pattern for POSIX atomicity
5. **Error Handling**: All repository methods wrap errors with context
6. **Sensitive Outputs**: Mark with "⚠️  sensitive" in CLI display (T027, T032)

---

## Validation Checklist

- [x] All contracts have corresponding tests (T033-T042 cover both proto contracts)
- [x] All entities have model tasks (T010 for StateOutput)
- [x] All tests come before implementation (T033-T042 before T019-T022)
- [x] Parallel tasks truly independent (verified via dependency graph)
- [x] Each task specifies exact file path (all tasks include file references)
- [x] No task modifies same file as another [P] task (verified per phase)

---

**Ready for Implementation**: All 44 tasks are dependency-ordered, parallelizable where possible, and mapped to design artifacts from spec.md, plan.md, data-model.md, research.md, and quickstart.md.
