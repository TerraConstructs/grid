# Implementation Plan: CLI Context-Aware State Management

**Branch**: `003-ux-improvements-for` | **Date**: 2025-10-03 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/003-ux-improvements-for/spec.md`

## Execution Flow (/plan command scope)
```
1. Load feature spec from Input path
   → If not found: ERROR "No feature spec at {path}"
2. Fill Technical Context (scan for NEEDS CLARIFICATION)
   → Detect Project Type from file system structure or context (web=frontend+backend, mobile=app+api)
   → Set Structure Decision based on project type
3. Fill the Constitution Check section based on the content of the constitution document.
4. Evaluate Constitution Check section below
   → If violations exist: Document in Complexity Tracking
   → If no justification possible: ERROR "Simplify approach first"
   → Update Progress Tracking: Initial Constitution Check
5. Execute Phase 0 → research.md
   → If NEEDS CLARIFICATION remain: ERROR "Resolve unknowns"
6. Execute Phase 1 → contracts, data-model.md, quickstart.md, agent-specific template file (e.g., `CLAUDE.md` for Claude Code, `.github/copilot-instructions.md` for GitHub Copilot, `GEMINI.md` for Gemini CLI, `QWEN.md` for Qwen Code or `AGENTS.md` for opencode).
7. Re-evaluate Constitution Check section
   → If new violations: Refactor design, return to Phase 1
   → Update Progress Tracking: Post-Design Constitution Check
8. Plan Phase 2 → Describe task generation approach (DO NOT create tasks.md)
9. STOP - Ready for /tasks command
```

**IMPORTANT**: The /plan command STOPS at step 8. Phases 2-4 are executed by other commands:
- Phase 2: /tasks command creates tasks.md
- Phase 3-4: Implementation execution (manual or via tools)

## Summary
This feature enhances CLI/SDK user experience by adding directory-based state context management, interactive output selection for dependencies, and enriched state information display. Users can work in a directory that remembers the associated Grid state, interactively select from available outputs when creating dependencies, and view complete dependency/dependent/output information when inspecting states.

## Technical Context
**Language/Version**: Go 1.24+
**Primary Dependencies**:
- Cobra (CLI framework, already in use)
- Connect RPC client (pkg/sdk, already in use)
- Interactive terminal UI library: `github.com/pterm/pterm` (multi-select, minimal footprint)

**Storage**:
- Client-side: `.grid` file in user's working directory (JSON format, version-controlled)
- Server-side: PostgreSQL database
  - `states` table: Raw Terraform/OpenTofu state JSON
  - `state_outputs` table: Cached output keys for fast cross-state search (Phase 1)

**Testing**: Go testing framework (`go test`), table-driven tests, integration tests

**Target Platform**: CLI binary for Linux/macOS (Note: WSL for Windows)

**Project Type**: Monorepo with Go workspace (5 modules: api, cmd/gridapi, cmd/gridctl, pkg/sdk, tests)

**Performance Goals**:
- Interactive prompts respond <100ms for typical output counts (<20 outputs)
- State lookups from .grid file <10ms
- Server output fetching <500ms p95

**Constraints**:
- `.grid` file must be version-control friendly (JSON, deterministic formatting)
- Interactive prompts must gracefully degrade with --non-interactive flag
- Output caching must invalidate atomically with state updates (transaction integrity)

**Scale/Scope**:
- Typical users: 10-100 states per project
- Outputs per state: 1-50 typical, 200 max reasonable
- Directory contexts: 1 per Terraform/OpenTofu State

**Additional Technical Details** (from user input):
- `.grid` file recommended to be added to version control for end users
- Server currently stores raw Terraform/OpenTofu state JSON in states table
- Outputs NOT currently stored separately; EdgeUpdateJob parses outputs in-memory from JSON
- For improved performance, basic (non-sensitive) outputs information could be cached separately in database regardless of dependency edge existence (upon state put -> parse outputs and insert both state json and outputs into separate table)

## Constitution Check
*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Go Workspace Architecture ✅ PASS
- Changes confined to existing `cmd/gridctl` module (CLI updates)
- May add new RPC methods to `proto/state/v1/state.proto` for fetching outputs list (Alpha -> no version bumps needed). Add/Update state fetching RPC with dependencies/dependents/outputs to avoid n+1 calls.
- Generated code goes to `./api` module per standard workflow
- SDK wrapper updates in `pkg/sdk` to expose new RPC methods (follow DX friendly wrapper types and methods)
- No new modules required

### Contract-Centric SDKs ✅ PASS
- Interactive CLI prompts are CLI-specific UX, not SDK responsibility
- `.grid` file I/O is CLI-specific (directory context management)
- SDK will expose output-fetching RPC method (wraps proto-generated client)
- Server-side output caching is internal to `cmd/gridapi/internal`
- No persistence logic leaking into SDK

### Dependency Flow Discipline ✅ PASS
- CLI (`cmd/gridctl`) depends on SDK (`pkg/sdk`) ✓
- SDK depends on generated clients in `./api` ✓
- No circular dependencies introduced

### Cross-Language Parity via Connect RPC ⚠️ CONSIDERATION
- New RPC method needed: `ListStateOutputs(logic_id/guid) → []OutputKey`
- This breaks parity temporarily until Node.js SDK also implements wrapper
- **Mitigation**: Document in proto comments, add to Node.js SDK backlog
- **Justification**: CLI-first feature, web UI doesn't need this immediately

### Test Strategy ✅ PASS
- Contract tests for new `ListStateOutputs` RPC
- Integration tests for:
  - `.grid` file creation, reading, corruption handling
  - Interactive prompt flow (mocked terminal I/O)
  - Default parameter behavior with/without .grid context
- Quickstart scenarios validate end-to-end UX

### Versioning & Releases ✅ PASS
- Proto change is additive (new RPC method), backward compatible
- CLI can release independently with new features
- SDK minor version bump to expose new method

### Simplicity & Pragmatism ✅ PASS
- `.grid` file: simple JSON, no custom config language
- Interactive prompts: use existing library (survey/promptui), don't build custom TUI
- Output caching: optional optimization, not required for MVP
- No new abstraction layers

**Initial Gate**: ✅ PASS (with Node.js SDK parity tracked)

## Project Structure

### Documentation (this feature)
```
specs/003-ux-improvements-for/
├── plan.md              # This file (/plan command output)
├── research.md          # Phase 0 output (/plan command)
├── data-model.md        # Phase 1 output (/plan command)
├── quickstart.md        # Phase 1 output (/plan command)
├── contracts/           # Phase 1 output (/plan command)
│   └── list-state-outputs.yaml  # OpenAPI/proto for new RPC
└── tasks.md             # Phase 2 output (/tasks command - NOT created by /plan)
```

### Source Code (repository root)
```
proto/state/v1/
└── state.proto          # Add RPC method(s) (for example ListStateOutputs and enhanced GetStateInfo)

api/state/v1/            # Generated from proto (buf generate)
└── [generated files]

cmd/gridctl/
├── cmd/
│   ├── state/
│   │   ├── create.go    # MODIFIED: Add .grid file creation, --force flag, --non-interactive flag
│   │   ├── get.go       # MODIFIED: Show dependencies/dependents/outputs via GetStateInfo RPC
│   │   ├── init.go      # (existing, may combine functionality with create through --init flag)
│   │   └── context.go   # NEW: Shared .grid file I/O logic (read/write/validate)
│   └── deps/
│       └── add.go       # MODIFIED: Interactive output selection (pterm), use .grid for --to default
└── go.mod               # Add github.com/pterm/pterm dependency

pkg/sdk/
├── state_client.go      # MODIFIED: Add new RPC Wrappers and types (follow existing patterns)
└── state_client_test.go # NEW: Test for new features such as ListStateOutputs

cmd/gridapi/internal/
├── server/
│   ├── connect_handlers.go  # MODIFIED: Implement ListStateOutputs, GetStateInfo RPC handlers
│   └── terraform_backend.go # MODIFIED: Hook output cache updates on state PUT
├── state/
│   └── service.go           # MODIFIED: Add GetOutputKeys, GetStateInfo methods
├── repository/
│   ├── interface.go         # MODIFIED: Add StateOutputRepository interface
│   └── bun_state_output_repository.go  # NEW: Bun implementation for output caching
├── db/models/
│   └── state_output.go      # NEW: Bun model for state_outputs table
└── migrations/
    └── 20251003140000_add_state_outputs_cache.go  # NEW: Migration for state_outputs table

tests/integration/
└── context_aware_test.go  # NEW: End-to-end tests for directory context + interactive selection
```

**Structure Decision**: Go workspace monorepo (single project structure). All changes fit within existing modules: proto definitions, generated API code, CLI binary, SDK package, and API server. No new modules required per Constitution Principle VII (Simplicity).

## Phase 0: Outline & Research

### Research Tasks

1. **Interactive Terminal Libraries for Go**
   - **Unknown**: Which library best fits Grid's needs (multi-select, CI compatibility, minimal deps)?
   - **Task**: Compare `github.com/pterm/pterm` vs `github.com/manifoldco/promptui` vs `github.com/AlecAivazis/survey/v2`
   - **Criteria**: Multi-select support, --non-interactive mode, ~~Windows compatibility~~, active maintenance

2. **.grid File Format Design**
   - **Unknown**: JSON structure for directory context (fields, versioning, migration path)
   - **Task**: Design schema with forward compatibility (version field, optional extensions)
   - **Criteria**: Human-readable, git-friendly, supports logic-id change tracking

3. **Output Caching Strategy**
   - **Unknown**: Trade-offs of server-side output caching (performance vs consistency)
   - **Task**: Research pattern for caching Terraform outputs separately from state JSON
   - **Criteria**: Invalidation strategy, storage overhead, query performance improvement

4. **Concurrency Control for .grid File Writes**
   - **Unknown**: Best practice for local file locking in Go CLI tools
   - **Task**: Research file locking mechanisms (flock, atomic writes, error-on-conflict)
   - **Criteria**: Cross-platform (Linux/macOS), simple implementation

5. **Non-Interactive Mode Patterns**
   - **Unknown**: CLI UX patterns for --non-interactive in existing tools (Terraform, kubectl)
   - **Task**: Survey how other CLIs handle prompts in CI environments
   - **Criteria**: Error messages, exit codes, environment variable detection

**Output**: `research.md` documenting decisions for each area above

## Phase 1: Design & Contracts

### Data Model (`data-model.md`)

**Entities**:

1. **DirectoryContext** (client-side `.grid` file)
   ```
   Fields:
   - version: string (schema version, e.g., "1")
   - state_guid: string (immutable GUID)
   - state_logic_id: string (current logic ID)
   - logic_id_changed: bool (flag if logic ID was updated since creation)
   - server_url: string (optional, tracks which server)
   - created_at: timestamp
   - updated_at: timestamp

   Validation:
   - version must be "1"
   - state_guid must be valid UUID
   - state_logic_id non-empty

   Storage: JSON file at `$(pwd)/.grid`
   ```

2. **StateOutput** (server-side, new proto message or internal struct)
   ```
   Fields:
   - output_key: string (Terraform output name)
   - sensitive: bool (from TF state metadata)

   Notes:
   - Parsed from Terraform state JSON on demand
   - Optionally cached in separate table (outputs_cache)
   ```

3. **OutputsCacheRow** (server-side, optional optimization)
   ```
   Fields:
   - state_guid: string (FK to states table)
   - output_key: string
   - sensitive: bool
   - state_version: int (TF state serial/version for cache invalidation)
   - updated_at: timestamp

   Indexes:
   - PRIMARY KEY (state_guid, output_key)
   - INDEX on state_guid for bulk lookup
   ```

### API Contracts (`contracts/list-state-outputs.yaml`)

**New RPC Method**:
```protobuf
// ListStateOutputs returns available output keys from a state's Terraform JSON.
rpc ListStateOutputs(ListStateOutputsRequest) returns (ListStateOutputsResponse);

message ListStateOutputsRequest {
  oneof state {
    string logic_id = 1;
    string guid = 2;
  }
}

message ListStateOutputsResponse {
  string state_guid = 1;
  string state_logic_id = 2;
  repeated OutputKey outputs = 3;
}

message OutputKey {
  string key = 1;
  bool sensitive = 2;  // From TF state metadata, used for display warning
}
```

**Updated RPC Method** (GetStateConfig or new GetStateInfo):
```protobuf
// Enhance existing GetStateConfig or create GetStateInfo to include dependency/dependent/outputs info
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
  repeated DependencyEdge dependencies = 4;     // Incoming edges (states this depends on)
  repeated DependencyEdge dependents = 5;        // Outgoing edges (states that depend on this)
  repeated OutputKey outputs = 6;                // Available outputs from this state
  google.protobuf.Timestamp created_at = 7;
  google.protobuf.Timestamp updated_at = 8;
}
```

### Contract Tests (generated from contracts)

1. **Test: ListStateOutputs with valid logic-id**
   - Request: `{"logic_id": "test-state"}`
   - Assert: Response contains `state_guid`, `outputs` array
   - Assert: Each output has `key` and `sensitive` fields

2. **Test: ListStateOutputs with state that has no outputs**
   - Request: `{"logic_id": "empty-state"}`
   - Assert: Response `outputs` is empty array (not null/error)

3. **Test: ListStateOutputs with non-existent state**
   - Request: `{"logic_id": "nonexistent"}`
   - Assert: Error with code `NOT_FOUND`

4. **Test: GetStateInfo includes dependencies and outputs**
   - Setup: Create state with dependencies and outputs
   - Request: `{"logic_id": "test-state"}`
   - Assert: Response includes non-empty `dependencies`, `dependents`, `outputs` arrays

### Quickstart Scenarios (`quickstart.md`)

**Scenario 1: Create state with directory context**
```bash
# Create new directory and state
mkdir ~/my-terraform-project
cd ~/my-terraform-project
gridctl state create my-app

# Verify .grid file created
cat .grid  # Shows JSON with state GUID and logic-id

# Subsequent commands use context automatically
gridctl state get  # No need to specify logic-id
```

**Scenario 2: Interactive dependency creation**
```bash
cd ~/my-terraform-project  # Has .grid context for "my-app"

# From-state has 5 outputs: vpc_id, subnet_id, sg_id, db_endpoint, cache_endpoint
gridctl deps add --from networking

# CLI shows interactive menu:
# ✓ vpc_id
# ✓ subnet_id
#   sg_id
#   db_endpoint
#   cache_endpoint
# (Space to select, Enter to confirm)

# Creates 2 dependency edges: networking.vpc_id -> my-app, networking.subnet_id -> my-app
```

**Scenario 3: Enhanced state info display**
```bash
gridctl state get my-app

# Output:
# State: my-app (guid: 01JXXX...)
# Created: 2025-10-03 14:22:00
#
# Dependencies (consuming from):
#   networking.vpc_id -> vpc_id_input
#   networking.subnet_id -> subnet_id_input
#
# Dependents (consumed by):
#   frontend.app_subnet_id
#   backend.db_subnet_id
#
# Outputs:
#   app_url
#   app_version
#   healthcheck_endpoint
```

**Scenario 4: CI/automation with --non-interactive**
```bash
# In CI pipeline, no .grid context present
gridctl deps add --from networking --to my-app --output vpc_id --non-interactive

# Would fail if --output not specified and multiple outputs exist
# Error: Cannot prompt in non-interactive mode. Specify --output explicitly.
```

### Agent File Update

Run update script to add this feature's context:
```bash
.specify/scripts/bash/update-agent-context.sh claude
```

This will update `/Users/vincentdesmet/tcons/grid/CLAUDE.md` with:
- New `.grid` file format and usage
- Interactive prompt library choice
- Directory context resolution logic
- Updated deps add command with interactive mode

## Phase 2: Task Planning Approach
*This section describes what the /tasks command will do - DO NOT execute during /plan*

**Task Generation Strategy**:
1. Load `.specify/templates/tasks-template.md` as base
2. Generate tasks from Phase 1 artifacts:
   - Each proto message → implementation task
   - Each SDK method → SDK wrapper task
   - Each CLI command change → CLI implementation task
   - Each contract test → test implementation task [P]
   - Each quickstart scenario → integration test task

### Detailed Task Breakdown with Cross-References

#### Track 1: Proto & Code Generation (Foundation) [P]
**Tasks 1-3** | **Depends on**: contracts/*.proto | **Reference**: contracts/list-state-outputs.proto, contracts/get-state-info.proto

1. **Add ListStateOutputs RPC to proto**
   - File: `proto/state/v1/state.proto`
   - Reference: `contracts/list-state-outputs.proto` (lines 23-52)
   - Add: `message OutputKey`, `message ListStateOutputsRequest`, `message ListStateOutputsResponse`
   - Add: `rpc ListStateOutputs(...)` to StateService

2. **Add GetStateInfo RPC to proto**
   - File: `proto/state/v1/state.proto`
   - Reference: `contracts/get-state-info.proto` (lines 23-55)
   - Add: `message GetStateInfoRequest`, `message GetStateInfoResponse`
   - Reuse: `DependencyEdge`, `OutputKey`, `BackendConfig` (existing)
   - Add: `rpc GetStateInfo(...)` to StateService

3. **Generate API code from proto**
   - Command: `buf generate`
   - Output: `api/state/v1/*.go` (generated Connect + protobuf)
   - Validation: Check `api/state/v1/statev1connect/state.connect.go` has new RPC methods

#### Track 2: Database Layer [P]
**Tasks 4-7** | **Depends on**: data-model.md > StateOutputRow | **Reference**: data-model.md (lines 118-258)

4. **Create StateOutput Bun model**
   - File: `cmd/gridapi/internal/db/models/state_output.go` (NEW)
   - Reference: `data-model.md` (lines 139-150, Bun ORM Model section)
   - Fields: state_guid (pk), output_key (pk), sensitive, state_serial, created_at, updated_at
   - Tags: Follow existing bun tag patterns from `models/state.go`

5. **Create migration file**
   - File: `cmd/gridapi/internal/migrations/20251003140000_add_state_outputs_cache.go` (NEW)
   - Reference: `data-model.md` (lines 186-258, Migration section)
   - Pattern: Follow `20251002000002_add_edges_table.go` structure
   - Up: Create table, indexes (guid, key), FK constraint with CASCADE
   - Down: Drop table

6. **Define StateOutputRepository interface**
   - File: `cmd/gridapi/internal/repository/interface.go` (MODIFIED)
   - Reference: `data-model.md` (lines 153-184, Repository Interface section)
   - Add: `UpsertOutputs`, `GetOutputsByState`, `SearchOutputsByKey`, `DeleteOutputsByState`
   - Types: Add `OutputKey`, `StateOutputRef` structs

7. **Implement Bun repository**
   - File: `cmd/gridapi/internal/repository/bun_state_output_repository.go` (NEW)
   - Reference: `data-model.md` (lines 153-184)
   - Pattern: Follow `bun_state_repository.go` structure (context, transactions, error wrapping)
   - Implement: All 4 methods from interface
   - Tests: Create `bun_state_output_repository_test.go` with table-driven tests

#### Track 3: SDK Wrappers [P]
**Tasks 8-9** | **Depends on**: Track 1 (generated code) | **Reference**: contracts/*.proto

8. **Add ListStateOutputs to SDK**
   - File: `pkg/sdk/state_client.go` (MODIFIED)
   - Pattern: Follow existing `CreateState`, `GetStateConfig` wrappers
   - Add: `func (c *Client) ListStateOutputs(ctx, ref StateRef) ([]OutputKey, error)`
   - Error handling: Wrap Connect errors, check response validity

9. **Add GetStateInfo to SDK**
   - File: `pkg/sdk/state_client.go` (MODIFIED)
   - Pattern: Similar to above
   - Add: `func (c *Client) GetStateInfo(ctx, ref StateRef) (*StateInfo, error)`
   - Types: May need to add `StateInfo` struct that wraps GetStateInfoResponse

#### Track 4: Server RPC Handlers
**Tasks 10-13** | **Depends on**: Track 2 (repository), Track 3 (SDK) | **Reference**: data-model.md

10. **Implement ListStateOutputs handler**
    - File: `cmd/gridapi/internal/server/connect_handlers.go` (MODIFIED)
    - Reference: `data-model.md` (lines 153-240, query patterns)
    - Logic: Resolve state by logic_id/guid → repo.GetOutputsByState → build response
    - Error cases: State not found (NOT_FOUND), repository errors (INTERNAL)

11. **Implement GetStateInfo handler**
    - File: `cmd/gridapi/internal/server/connect_handlers.go` (MODIFIED)
    - Reference: `contracts/get-state-info.proto` (lines 70-90, implementation notes)
    - Logic: Fetch state + dependencies (ListDependencies) + dependents (ListDependents) + outputs (GetOutputsByState)
    - Optimization: Could parallelize 3 repository calls

12. **Add service layer methods**
    - File: `cmd/gridapi/internal/state/service.go` (MODIFIED)
    - Add: `GetOutputKeys(guid)` helper (calls repository)
    - Add: `GetStateInfo(ref)` orchestration (combines multiple repo calls)

13. **Hook Terraform backend for output caching**
    - File: `cmd/gridapi/internal/server/terraform_backend.go` (MODIFIED)
    - Reference: `data-model.md` (lines 265-269, invalidation strategy), `data-model.md` (lines 78-114, parsing logic)
    - Modify: `PUT /tfstate/{guid}` handler
    - Add: After state write, parse TF JSON serial → repo.UpsertOutputs(guid, serial, outputs)
    - Transaction: Wrap state write + output cache update in same transaction

#### Track 5: CLI Context I/O [P]
**Tasks 14-16** | **Depends on**: None (independent) | **Reference**: research.md, data-model.md

14. **Create .grid file I/O module**
    - File: `cmd/gridctl/cmd/state/context.go` (NEW)
    - Reference: `research.md` (lines 75-90, .grid file format)
    - Types: `DirectoryContext` struct matching JSON schema
    - Functions: `ReadGridContext() (*DirectoryContext, error)`, `WriteGridContext(ctx *DirectoryContext) error`
    - Validation: Check version == "1", valid GUID, non-empty logic_id

15. **Implement atomic write pattern**
    - File: `cmd/gridctl/cmd/state/context.go` (continued)
    - Reference: `research.md` (lines 194-234, concurrency control)
    - Pattern: Write to `.grid.tmp` → rename to `.grid` (atomic on POSIX)
    - Error handling: Detect write permissions, corruption, concurrent writes

16. **Add context resolution helper**
    - File: `cmd/gridctl/cmd/state/context.go` (continued)
    - Function: `ResolveStateRef(explicitRef, contextRef StateRef) StateRef`
    - Logic: Explicit params override context → return error if neither provided

#### Track 6: CLI Interactive Prompts [P]
**Tasks 17-18** | **Depends on**: None (independent) | **Reference**: research.md

17. **Add pterm dependency**
    - File: `cmd/gridctl/go.mod` (MODIFIED)
    - Command: `cd cmd/gridctl && go get github.com/pterm/pterm`
    - Reference: `research.md` (lines 7-20, pterm decision)

18. **Create multi-select helper for outputs**
    - File: `cmd/gridctl/cmd/deps/add.go` (MODIFIED, inline helper)
    - Reference: `research.md` (lines 22-57, implementation notes)
    - Function: `promptSelectOutputs(outputs []OutputKey, nonInteractive bool) ([]string, error)`
    - Logic: If nonInteractive → error, if 1 output → auto-select, else → pterm.MultiSelect

#### Track 7: CLI Command Integration
**Tasks 19-24** | **Depends on**: Tracks 3,4,5,6 | **Reference**: quickstart.md

19. **Modify `state create` command**
    - File: `cmd/gridctl/cmd/state/create.go` (MODIFIED)
    - Reference: `quickstart.md` (lines 30-98, scenario 1)
    - Add: `--force` flag
    - Add: After successful CreateState RPC → WriteGridContext()
    - Add: Check existing .grid, handle conflicts per FR-002
    - Add: Detect write permissions per FR-006c

20. **Add `--non-interactive` root flag**
    - File: `cmd/gridctl/cmd/root.go` (MODIFIED)
    - Reference: `research.md` (lines 233-269, non-interactive mode)
    - Add: `--non-interactive` persistent flag + `GRID_NON_INTERACTIVE` env var
    - Make: Accessible to all subcommands via context or global var

21. **Modify `deps add` command**
    - File: `cmd/gridctl/cmd/deps/add.go` (MODIFIED)
    - Reference: `quickstart.md` (lines 100-166, scenario 2)
    - Add: Make `--output` optional
    - Add: Resolve `--to` default from context per FR-008
    - Add: Call SDK.ListStateOutputs when --output not provided
    - Add: Call promptSelectOutputs helper
    - Add: Loop over selected outputs, call AddDependency for each

22. **Modify `state get` command**
    - File: `cmd/gridctl/cmd/state/get.go` (MODIFIED)
    - Reference: `quickstart.md` (lines 168-211, scenario 3)
    - Change: Call SDK.GetStateInfo instead of GetStateConfig
    - Add: Format dependencies section (list from_state.output → to_input)
    - Add: Format dependents section (list to_state consuming which output)
    - Add: Format outputs section (list output keys, mark sensitive)

23. **Update `state list` command** (optional enhancement)
    - File: `cmd/gridctl/cmd/state/list.go` (MODIFIED)
    - Enhancement: Show dependency count, output count in table
    - Reference: Could leverage new GetStateInfo for richer display

24. **Add context validation on command startup**
    - Files: All state/* and deps/* commands (MODIFIED)
    - Add: At start of RunE, attempt ReadGridContext() if no explicit params
    - Add: Handle corrupted context per FR-006b (warning + ignore)
    - Add: Handle stale context per FR-006e (verify state exists on server)

#### Track 8: Contract Tests [P]
**Tasks 25-30** | **Depends on**: Track 1 (proto) | **Reference**: contracts/*.proto

25-27. **ListStateOutputs contract tests**
    - File: `tests/contract/list_state_outputs_test.go` (NEW)
    - Reference: `contracts/list-state-outputs.proto` (lines 79-104, contract tests)
    - Test 1: Valid logic-id → assert response structure
    - Test 2: State with no outputs → assert empty array
    - Test 3: Non-existent state → assert NOT_FOUND error
    - Test 4: Sensitive flag accuracy → verify metadata parsing
    - Test 5: GUID vs logic-id → verify both work

28-30. **GetStateInfo contract tests**
    - File: `tests/contract/get_state_info_test.go` (NEW)
    - Reference: `contracts/get-state-info.proto` (lines 94-131, contract tests)
    - Test 1: State with dependencies → assert dependencies array populated
    - Test 2: State with dependents → assert dependents array populated
    - Test 3: Isolated state → assert empty dep/dependent arrays
    - Test 4: Non-existent state → assert NOT_FOUND
    - Test 5: Backend config included → assert URL structure

#### Track 9: Integration Tests
**Tasks 31-40** | **Depends on**: All tracks | **Reference**: quickstart.md

31. **Setup integration test harness**
    - File: `tests/integration/context_aware_test.go` (NEW)
    - Pattern: Follow `tests/integration/main_test.go` (TestMain setup)
    - Setup: Start server, create temp directories, unique state logic-ids per test

32-34. **Directory context tests**
    - Reference: `quickstart.md` (lines 20-82, scenario 1)
    - Test: Create state → verify .grid created
    - Test: Subsequent commands use context → verify no --logic-id needed
    - Test: Concurrent create without --force → verify error

35-37. **Interactive selection tests** (mocked I/O)
    - Reference: `quickstart.md` (lines 139-187, scenario 2)
    - Test: Multi-output state → verify prompt (mocked) → verify multi-edge creation
    - Test: Single output → verify auto-select → verify edge created
    - Test: Zero outputs → verify error or mock flow

38-39. **Enhanced display tests**
    - Reference: `quickstart.md` (lines 189-246, scenario 3)
    - Test: `state get` output includes dependencies, dependents, outputs
    - Test: Sensitive outputs marked correctly

40. **Non-interactive mode test**
    - Reference: `quickstart.md` (lines 248-264, scenario 4)
    - Test: `--non-interactive` without explicit --output → verify error
    - Test: `--non-interactive` with explicit --output → verify success

### Task Dependencies Graph
```
1,2 (Proto) → 3 (Generate) → 8,9 (SDK) → 10-13 (Server RPC)
                ↓                            ↓
              4-7 (DB) → 10-13 (Server) → 13 (Backend Hook)
                                             ↓
14-16 (Context I/O) [P] ──────────┐          ↓
17-18 (Prompts) [P] ──────────────┼→ 19-24 (CLI Integration)
                                  │     ↓
25-30 (Contract Tests) [P] ───────┤     ↓
                                  └→ 31-40 (Integration Tests)
```

**Total Tasks**: ~40 numbered, dependency-ordered tasks

**Estimated Implementation Time**:
- Proto + DB + SDK: 4-6 hours
- Server handlers: 6-8 hours
- CLI integration: 6-8 hours
- Tests: 8-10 hours
- **Total: 24-32 hours**

**IMPORTANT**: This phase is executed by the /tasks command, NOT by /plan

## Phase 3+: Future Implementation
*These phases are beyond the scope of the /plan command*

**Phase 3**: Task execution (/tasks command creates tasks.md)
**Phase 4**: Implementation (execute tasks.md following constitutional principles)
**Phase 5**: Validation (run tests, execute quickstart.md, performance validation)

## Functional Requirements Validation

### Coverage Matrix
Validating plan against all 29 functional requirements from spec.md:

**Directory Context Management (FR-001 to FR-006e)**: ✅ 11/11 covered
- FR-001: ✅ `.grid` file creation in `state create` command (context.go)
- FR-002: ✅ Conflict detection logic in `state create` (check existing .grid)
- FR-003: ✅ `--force` flag implementation in `state create`
- FR-004: ✅ Context reading logic in context.go (default parameter resolution)
- FR-005: ✅ Error messaging for missing context (command validations)
- FR-006: ✅ GUID storage in .grid schema (data-model.md)
- FR-006a: ✅ Current directory only search (research.md decision)
- FR-006b: ✅ Corrupted file handling (quickstart scenario 7, error handling)
- FR-006c: ✅ Write permission detection (quickstart scenario 8, early check)
- FR-006d: ✅ Concurrent write handling (atomic write pattern in research.md)
- FR-006e: ✅ Stale state detection (server validation on context use)

**Default Parameter Behavior (FR-007 to FR-009)**: ✅ 3/3 covered
- FR-007: ✅ Context-based defaults in all state commands (context.go integration)
- FR-008: ✅ Context-based `--to` default in deps commands (deps/add.go)
- FR-009: ✅ Explicit parameter override (Cobra flag precedence)

**Interactive Output Selection (FR-010 to FR-017)**: ✅ 8/8 covered
- FR-010: ✅ Optional `--output` flag in deps add (command modification)
- FR-011: ✅ ListStateOutputs RPC fetches outputs (contracts/list-state-outputs.proto)
- FR-012: ✅ pterm multi-select menu (deps/add.go integration)
- FR-013: ✅ Multi-select support (pterm.MultiSelect, quickstart scenario 2)
- FR-014: ✅ Single-output auto-select (quickstart scenario 3, conditional logic)
- FR-015: ✅ Zero-output mock dependency (quickstart scenario 4, --mock flag)
- FR-016: ✅ Error handling for fetch failures (contract tests)
- FR-017: ✅ `--non-interactive` flag (root command flag, research.md)

**Enhanced State Information Display (FR-018 to FR-022)**: ✅ 5/5 covered
- FR-018: ✅ Dependencies in GetStateInfo response (contracts/get-state-info.proto)
- FR-019: ✅ Dependents in GetStateInfo response (contracts/get-state-info.proto)
- FR-020: ✅ Outputs list in GetStateInfo response (keys only, contracts/get-state-info.proto)
- FR-021: ✅ Dependency display format (quickstart scenario 5, state/get.go output)
- FR-022: ✅ Dependent display format (quickstart scenario 5, state/get.go output)

**Server-Side Output Caching (FR-023 to FR-029)**: ✅ 7/7 covered
- FR-023: ✅ state_outputs table schema (data-model.md, migration)
- FR-024: ✅ Serial-based invalidation (repository UpsertOutputs method)
- FR-025: ✅ Cross-state search (SearchOutputsByKey repository method, idx_state_outputs_key index)
- FR-026: ✅ Cache-backed ListStateOutputs (GetOutputsByState in RPC handler)
- FR-027: ✅ Transactional update (terraform_backend.go PUT handler hook)
- FR-028: ✅ Idempotent invalidation (UpsertOutputs delete+insert pattern)
- FR-029: ✅ Cascade delete (FK constraint ON DELETE CASCADE in migration)

**Total Coverage**: ✅ **29/29 functional requirements addressed** (100%)

### Design Artifacts Mapping
| Requirement Category | Functional Requirements | Plan Artifacts |
|---------------------|------------------------|----------------|
| Directory Context | FR-001 to FR-006e (11) | research.md (.grid format), data-model.md (DirectoryContext), cmd/gridctl/cmd/state/context.go |
| Default Parameters | FR-007 to FR-009 (3) | context.go (resolution logic), all command integrations |
| Interactive Selection | FR-010 to FR-017 (8) | research.md (pterm), contracts/list-state-outputs.proto, cmd/gridctl/cmd/deps/add.go |
| Enhanced Display | FR-018 to FR-022 (5) | contracts/get-state-info.proto, cmd/gridctl/cmd/state/get.go |
| Output Caching | FR-023 to FR-029 (7) | data-model.md (StateOutputRow), migration, bun_state_output_repository.go |

### Validation Summary
- ✅ All 29 functional requirements have corresponding implementation components
- ✅ All edge cases from spec covered in quickstart scenarios
- ✅ All new requirements identified during planning (FR-023 to FR-029) added to spec
- ✅ No gaps identified between spec and plan

## Complexity Tracking
*Fill ONLY if Constitution Check has violations that must be justified*

No constitutional violations requiring justification. All changes fit within existing module boundaries and follow established patterns.

## Progress Tracking
*This checklist is updated during execution flow*

**Phase Status**:
- [x] Phase 0: Research complete (/plan command) → research.md created
- [x] Phase 1: Design complete (/plan command) → data-model.md, contracts/, quickstart.md, CLAUDE.md updated
- [x] Phase 2: Task planning complete (/plan command - describe approach only)
- [ ] Phase 3: Tasks generated (/tasks command)
- [ ] Phase 4: Implementation complete
- [ ] Phase 5: Validation passed

**Gate Status**:
- [x] Initial Constitution Check: PASS
- [x] Post-Design Constitution Check: PASS
- [x] All NEEDS CLARIFICATION resolved (9 clarifications documented in spec)
- [x] Complexity deviations documented (none)
- [x] Functional requirements validation: COMPLETE (29/29 requirements covered)

**Artifacts Generated**:
- [x] `/specs/003-ux-improvements-for/plan.md` (this file with functional requirements validation)
- [x] `/specs/003-ux-improvements-for/spec.md` (updated with 7 new requirements FR-023 to FR-029)
- [x] `/specs/003-ux-improvements-for/research.md` (5 research decisions, updated for Phase 1 output caching)
- [x] `/specs/003-ux-improvements-for/data-model.md` (4 entities including StateOutputRow, migration pattern)
- [x] `/specs/003-ux-improvements-for/contracts/list-state-outputs.proto` (new RPC contract spec)
- [x] `/specs/003-ux-improvements-for/contracts/get-state-info.proto` (enhanced RPC contract spec)
- [x] `/specs/003-ux-improvements-for/quickstart.md` (8 end-to-end scenarios)
- [x] `/CLAUDE.md` (updated with Go 1.24+ language context)
- [ ] `/specs/003-ux-improvements-for/tasks.md` (awaits /tasks command)

---
*Based on Constitution v2.0.0 - See `.specify/memory/constitution.md`*
