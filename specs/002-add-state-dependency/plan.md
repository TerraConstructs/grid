
# Implementation Plan: State Dependency Management

**Branch**: `002-add-state-dependency` | **Date**: 2025-10-02 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/Users/vincentdesmet/tcons/grid/specs/002-add-state-dependency/spec.md`

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

**IMPORTANT**: The /plan command STOPS at step 7. Phases 2-4 are executed by other commands:
- Phase 2: /tasks command creates tasks.md
- Phase 3-4: Implementation execution (manual or via tools)

## Summary
Add state dependency management to Grid, enabling users to wire Terraform states together via directed edges from producer output keys to consumer states. The system maintains a Directed Acyclic Multigraph with cycle prevention, tracks edge statuses (clean/dirty/potentially-stale/mock/missing-output) using output fingerprinting from parsed tfstate, supports mock outputs for ahead-of-time graph assembly, and provides topological ordering for reconciliation. The CLI gains a `deps` command group for declaring/removing dependencies and a `deps sync` subcommand that generates managed HCL locals blocks materializing upstream outputs. Technical approach: edge table schema with DB-level cycle prevention trigger (Postgres recursive CTEs), in-memory graph validation using Gonum/graph for toposort, automatic edge status updates via background job on tfstate writes, and on-demand state status computation.

## Technical Context
**Language/Version**: Go 1.24.4
**Primary Dependencies**: Connect RPC v1.19.0, Bun ORM v1.2.15 (PostgreSQL), chi router v5.2.3, google/uuid v1.6.0, Cobra CLI v1.10.1, protobuf v1.36.9
**Storage**: PostgreSQL with Bun ORM (existing states table, new edges table with cycle prevention trigger)
**Testing**: Go standard testing, testify assertions, repository tests with real PostgreSQL (existing pattern), integration tests with TestMain server setup
**Target Platform**: Linux/macOS server (gridapi), cross-platform CLI (gridctl)
**Project Type**: single (Go workspace monorepo with 5 modules: api, cmd/gridapi, cmd/gridctl, pkg/sdk, tests)
**Performance Goals**: Handle graphs with 1000+ states and 5000+ edges in memory for Phase 1; optimize queries with indexes on (from_state), (to_state), (from_state, from_output, to_state)
**Constraints**: No pagination in Phase 1 (return full graphs); state-level status computed on demand (not persisted); idempotent managed HCL block generation; UTC timestamps (ISO 8601); no breaking proto changes (alpha contract, acceptable to modify existing v1 services)
**Scale/Scope**: New DependencyService Connect RPC service, 8-10 RPC methods (AddDependency, RemoveDependency, ListDependencies, ListDependents, SearchByOutput, GetTopologicalOrder, GetStateStatus, SyncDependencies), 1 new database table (edges), 1 new migration, 5-7 new CLI commands under `gridctl deps`, managed HCL template generation using embed.FS pattern from existing `backend.tf` template

## Constitution Check
*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Principle I: Go Workspace Architecture
- ✅ **PASS**: Feature operates within existing workspace structure (api, cmd/gridapi, cmd/gridctl, pkg/sdk modules)
- ✅ **PASS**: New proto definitions go to `proto/state/v1/state.proto` (extend existing service, no new module)
- ✅ **PASS**: Generated code updates `./api` module via `buf generate`
- ✅ **PASS**: No new Go modules required (reusing existing StateService or creating DependencyService within api module)

### Principle II: Contract-Centric SDKs
- ✅ **PASS**: All dependency operations defined in protobuf first (`proto/state/v1/state.proto`)
- ✅ **PASS**: `cmd/gridapi` implements handlers in `internal/server/connect_handlers.go` (server-side logic)
- ✅ **PASS**: `pkg/sdk` and future `js/sdk` wrap generated Connect RPC clients with ergonomic interfaces
- ✅ **PASS**: `cmd/gridctl` consumes `pkg/sdk`, not Connect clients directly
- ✅ **PASS**: Repository layer stays in `cmd/gridapi/internal/repository/*` (server-only persistence)
- ⚠️ **CRITICAL SEPARATION**: tfstate parsing, fingerprinting, and edge status computation MUST NOT be exposed via SDK; these are server-only internal packages (`cmd/gridapi/internal/{tfstate,graph}`). SDKs only wrap RPC calls defined in proto.

### Principle III: Dependency Flow Discipline
- ✅ **PASS**: Unidirectional flow maintained:
  ```
  cmd/gridapi → cmd/gridapi/internal/{repository, ...}, api (Connect server)
  cmd/gridctl → pkg/sdk → api (Connect clients)
  pkg/sdk     → api (Connect clients only)
  ```
- ✅ **PASS**: No circular dependencies introduced
- ✅ **PASS**: `./api` module remains autogenerated-only (no hand-written code)

### Principle IV: Cross-Language Parity via Connect RPC
- ✅ **PASS**: All RPC methods defined in `.proto` files first (DependencyService or extend StateService)
- ✅ **PASS**: Generated code committed to `./api` after `buf generate`
- ⚠️ **ACCEPTABLE**: Modifying existing `proto/state/v1/state.proto` without version bump (alpha contract, breaking changes acceptable per user requirement)
- ✅ **PASS**: CI will verify `buf generate` idempotency
- ✅ **PASS**: Terraform HTTP Backend exception does NOT apply (dependency management uses Connect RPC, not REST)

### Principle V: Test Strategy
- ✅ **PASS**: Repository tests with real PostgreSQL (existing pattern in `bun_state_repository_test.go`)
- ✅ **PASS**: Contract tests for Connect RPC handlers (new: `dependency_handlers_test.go`)
- ✅ **PASS**: Integration tests in `tests/integration/` with TestMain server setup
- ✅ **PASS**: TDD requirement: contract tests written first, must fail before implementation
- ✅ **PASS**: Table-driven tests for Go (multiple edge scenarios: cycle detection, status computation)

### Principle VI: Versioning & Releases
- ✅ **PASS**: No new modules, using existing module versions
- ✅ **PASS**: Proto changes in `v1` directory (acceptable breaking changes in alpha)
- ✅ **PASS**: SDK updates coordinated with proto changes

### Principle VII: Simplicity & Pragmatism
- ✅ **PASS**: Repository pattern already exists (reusing for edge table, no new abstraction)
- ✅ **PASS**: Bun ORM already in use (no new dependency for edges table)
- ⚠️ **JUSTIFICATION REQUIRED**: Adding Gonum/graph dependency for in-memory toposort
  - **Why needed**: Topological sort and cycle detection on complex graphs (FR-032, FR-033, FR-045)
  - **Simpler alternative rejected**: Hand-rolled toposort is error-prone; Gonum is battle-tested and standard in Go ecosystem
  - **Bundle impact**: Server-side only (gridapi), not in CLI or SDKs
- ⚠️ **JUSTIFICATION REQUIRED**: DB trigger for cycle prevention
  - **Why needed**: Enforce DAG invariant at database boundary (FR-003, FR-004, FR-045)
  - **Simpler alternative rejected**: Application-level checks alone can race; trigger is defensive correctness layer
  - **Operational impact**: One-time migration, documented in CLAUDE.md

**GATE STATUS**: ✅ **PASS** (with documented complexity additions in Complexity Tracking below)

## Project Structure

### Documentation (this feature)
```
specs/002-add-state-dependency/
├── plan.md              # This file (/plan command output)
├── research.md          # Phase 0 output (/plan command)
├── data-model.md        # Phase 1 output (/plan command)
├── quickstart.md        # Phase 1 output (/plan command)
├── contracts/           # Phase 1 output (/plan command)
│   └── dependency.proto # Updated proto definitions for DependencyService
└── tasks.md             # Phase 2 output (/tasks command - NOT created by /plan)
```

### Source Code (repository root)
```
# Go Workspace Monorepo Structure
proto/state/v1/
└── state.proto          # Extended with DependencyService RPC methods

api/                     # Generated Connect RPC code (buf generate output)
└── state/v1/
    ├── statev1connect/  # Generated Connect server/client stubs
    └── ...

cmd/gridapi/
├── internal/
│   ├── db/
│   │   └── models/
│   │       ├── state.go       # Existing
│   │       └── edge.go        # NEW: Edge model for dependency edges
│   ├── migrations/
│   │   └── YYYYMMDDHHMMSS_add_edges_table.go  # NEW: Migration for edges + trigger
│   ├── repository/
│   │   ├── interface.go                       # Updated with edge methods
│   │   ├── bun_state_repository.go            # Existing
│   │   └── bun_edge_repository.go             # NEW: Edge persistence
│   ├── tfstate/
│   │   ├── parser.go          # NEW: Parse tfstate JSON for outputs
│   │   └── fingerprint.go     # NEW: Compute canonical output fingerprints
│   ├── graph/
│   │   ├── builder.go         # NEW: Build in-memory graph from edges
│   │   ├── toposort.go        # NEW: Topological ordering using Gonum
│   │   └── status.go          # NEW: Compute edge/state statuses
│   └── server/
│       ├── connect_handlers.go           # Updated with DependencyService handlers
│       └── dependency_handlers_test.go   # NEW: Handler contract tests
└── cmd/
    └── serve.go         # Updated to register UpdateEdges background job hook

cmd/gridctl/
└── cmd/
    ├── state/           # Existing state commands (update state list display to include columns for dependencies, dependents and status)
    └── deps/            # NEW: Dependency command group
        ├── templates/   # embed.FS for grid_dependencies.tf.tmpl
        ├── add.go       # deps add
        ├── remove.go    # deps remove
        ├── list.go      # deps list (fixed width listing in terminal)
        ├── search.go    # deps search
        ├── status.go    # deps status
        ├── topo.go      # deps topo
        └── sync.go      # deps sync (generates grid_dependencies.tf)

pkg/sdk/
└── dependency.go        # NEW: Ergonomic wrappers for DependencyService RPC

tests/integration/
├── state_test.go        # Existing
└── dependency_test.go   # NEW: E2E tests for dependency graph workflows
```

**Structure Decision**: Go workspace monorepo (Option 1: Single project). All code organized within existing 5-module workspace (api, cmd/gridapi, cmd/gridctl, pkg/sdk, tests). New functionality integrated into existing modules with clear internal package separation (db/models, repository, tfstate parser, graph algorithms, handlers, CLI commands).

**Key Design Decisions**:
1. **to_input_name default generation**: Application layer (service) - compute default `slugify(from_logic_id) + "_" + slugify(from_output)` if not provided, then store as NOT NULL in database (ensures consistency, enables uniqueness constraint)
2. **EdgeUpdateJob background job**: Internal wiring in `TerraformHandlers.UpdateState`, NOT exposed as hook (fire-and-forget goroutine with per-state mutex, best-effort semantics with error logging)

## Phase 0: Outline & Research ✅ COMPLETE

**Research Topics**:
1. ✅ RDBMS patterns for directed acyclic multigraphs (DAGs)
   - Decision: Edge table (adjacency list) + cycle prevention trigger
   - Rationale: Simple, flexible, fits DAGs and multigraphs with Postgres recursive CTEs
   - Alternatives: Closure table (write amplification), ltree (tree-only)

2. ✅ Go graph libraries for in-memory validation and toposort
   - Decision: gonum.org/v1/gonum/graph
   - Rationale: Battle-tested, canonical Go graph library with native toposort + cycle detection
   - Alternatives: dominikbraun/graph (less mature), hand-rolled DFS (error-prone)

3. ✅ Terraform state parsing for outputs
   - Decision: Custom JSON parser with canonical fingerprinting (stdlib only)
   - Rationale: Zero dependencies, deterministic SHA-256 over canonical JSON
   - Implementation: ParseOutputs + ComputeFingerprint helpers

4. ✅ State status computation (on-demand derivation)
   - Decision: Compute from edges, never persist
   - Rationale: Single source of truth, avoids stale data
   - Algorithm: BFS propagation of dirty/stale status from incoming edges

5. ✅ Managed HCL generation (grid_dependencies.tf)
   - Decision: Template-based generation with managed block markers
   - Rationale: Idempotent, clear ownership, follows existing CLI pattern
   - Pattern: Header/footer comments, embed.FS template

**Output**: ✅ `research.md` created with all decisions documented

## Phase 1: Design & Contracts ✅ COMPLETE

**Artifacts Generated**:

1. ✅ **data-model.md** - Entity definitions and relationships:
   - Edge entity (new): Directed dependency edges with status tracking
   - State entity (existing): No schema changes, outputs parsed from tfstate
   - StateStatus (derived): Computed on-demand from edges
   - ObservationRecord (implicit): Embedded in Edge via out_digest + last_out_at
   - TFOutput (not persisted): Parsed from tfstate JSON on-demand
   - Repository interfaces: EdgeRepository (new), StateRepository (existing, no changes)
   - Database migration: edges table + cycle prevention trigger + indexes

2. ✅ **contracts/state.proto** - Extended protobuf service definition:
   - Extended StateService with 8 new RPC methods:
     * AddDependency: Declare edge with optional mock value and to_input_name
     * RemoveDependency: Delete edge by ID
     * ListDependencies: Fetch incoming edges for consumer state
     * ListDependents: Fetch outgoing edges for producer state
     * SearchByOutput: Find edges by output key name
     * GetTopologicalOrder: Compute layered ordering with upstream/downstream direction
     * GetStateStatus: On-demand status computation with incoming edge details
     * GetDependencyGraph: Fetch graph data for HCL generation
   - New messages: DependencyEdge, StateStatus, IncomingEdgeView, Layer, ProducerState
   - Updated StateInfo with computed_status and dependency_logic_ids fields

3. ✅ **quickstart.md** - Integration test scenarios (10 scenarios):
   - Scenario 1: Basic dependency declaration (FR-001, FR-002)
   - Scenario 2: Cycle prevention (FR-003, FR-004, FR-045)
   - Scenario 3: Edge status tracking lifecycle (FR-017 to FR-027)
   - Scenario 4: Mock dependencies with transition to real outputs (FR-006, FR-014, FR-015)
   - Scenario 5: Topological ordering upstream/downstream (FR-032, FR-033)
   - Scenario 6: Managed HCL generation with idempotent overwrite (FR-037 to FR-044)
   - Scenario 7: Dependency listing and state status indicators (FR-028, FR-029, FR-034, FR-035)
   - Scenario 8: Search by output key (FR-030, FR-031)
   - Scenario 9: to_input_name override with uniqueness enforcement (FR-008 to FR-011)
   - Scenario 10: Dependency removal and status recomputation (FR-005)

4. ✅ **CLAUDE.md updated** - Agent context file incremental update:
   - Added new tech stack entries (Gonum/graph for toposort)
   - Preserved existing manual sections
   - Documented new internal packages (tfstate, graph)
   - Referenced new commands (gridctl deps command group)

**Output**: ✅ All Phase 1 artifacts complete and validated against spec

## Phase 2: Task Planning Approach
*This section describes what the /tasks command will do - DO NOT execute during /plan*

**Task Generation Strategy**:
The `/tasks` command will load `.specify/templates/tasks-template.md` and generate ordered tasks from Phase 1 design artifacts:

1. **Database Layer Tasks** (Foundation):
   - Migration: Create edges table + cycle prevention trigger + indexes [P]
     - *Design ref*: research.md section 1 (Edge table schema, cycle prevention trigger)
   - Model: Create Edge model with validation (cmd/gridapi/internal/db/models/edge.go) [P]
     - *Design ref*: data-model.md section 2 (Edge entity, validation rules, to_input_name always non-null)
   - Repository: Implement EdgeRepository interface with Bun (bun_edge_repository.go)
     - *Design ref*: data-model.md section "Repository Interface Extensions"
   - Repository Tests: Write table-driven tests for edge CRUD + cycle queries

2. **Protobuf Contract Tasks** (API Surface):
   - Update proto/state/v1/state.proto with 8 new RPC methods + StateInfo extensions
     - *Design ref*: contracts/state.proto (DependencyService methods, StateInfo.computed_status field)
   - Run `buf generate` to update api/ module
   - Verify buf lint and buf breaking checks pass

3. **Server-Side Logic Tasks** (Business Layer):
   - TFState Parser: ParseOutputs + ComputeFingerprint (internal/tfstate/) [P]
     - *Design ref*: research.md section 3 (Terraform state parsing, canonical fingerprinting)
   - Graph Builder: In-memory graph construction from edges (internal/graph/builder.go) [P]
     - *Design ref*: research.md section 2 (Gonum graph library usage)
   - Toposort: Implement topological ordering using Gonum (internal/graph/toposort.go) [P]
     - *Design ref*: research.md section 2 (Topological sort algorithm)
   - Status Computation: Derive state status from edges (internal/graph/status.go) [P]
     - *Design ref*: data-model.md section 3 (StateStatus derivation algorithm, BFS propagation), research.md section 4
   - Dependency Service: AddDependency with to_input_name default generation logic (internal/dependency/service.go)
     - *Design ref*: data-model.md "to_input_name Default Generation", research.md section 5
   - EdgeUpdateJob: Background edge status update with per-state mutex (internal/server/update_edges.go)
     - *Design ref*: data-model.md section 5 (EdgeUpdateJob background processing), research.md section 6
   - Wire EdgeUpdateJob: Inject EdgeUpdateJob into TerraformHandlers, call in UpdateState handler

4. **Connect RPC Handler Tasks** (TDD):
   - Contract Tests: Write failing tests for all 8 RPC handlers (dependency_handlers_test.go)
   - Handlers: Implement AddDependency, RemoveDependency, ListDependencies, ListDependents
   - Handlers: Implement SearchByOutput, GetTopologicalOrder, GetStateStatus, GetDependencyGraph
   - Handler: Update ListStates to compute and populate StateInfo.computed_status and dependency_logic_ids
     - *Design ref*: contracts/state.proto (StateInfo extensions), data-model.md section 3 (StateStatus computation)
   - Wire handlers into server registration (cmd/serve.go)

5. **SDK Wrapper Tasks**:
   - Go SDK: Ergonomic wrappers for DependencyService (pkg/sdk/dependency.go)
   - SDK Tests: Contract tests against mocked transport

6. **CLI Command Tasks**:
   - Command Group: Create gridctl deps subcommand structure (cmd/gridctl/cmd/deps/)
   - Commands: Implement add, remove, list, search, status, topo [P for each]
   - Command: Implement sync with HCL template generation (embed.FS pattern)
     - *Design ref*: research.md section 7 (HCL managed block format, template generation)
   - HCL Template: Create grid_dependencies.tf.tmpl in embeds/
   - Command: Update `gridctl state list` to display computed_status and dependency_logic_ids columns
     - *Design ref*: quickstart.md Scenario 7 (state list output format), contracts/state.proto StateInfo

7. **Integration Test Tasks** (Validate End-to-End):
   - Integration Tests: Implement 10 quickstart scenarios (tests/integration/dependency_test.go)
     - *Design ref*: quickstart.md (all 10 scenarios)
   - TestMain: Update to register UpdateEdges job hook
   - Fixtures: Create test tfstate JSON files for scenarios

8. **Documentation Tasks**:
   - Update CLAUDE.md with new commands and internal packages (already done)
   - Update CLI help text for new deps commands

**Ordering Strategy**:
- **Dependency Order**: Database → Proto → Server Logic → Handlers → SDK → CLI → Tests
- **TDD Order**: Contract tests before handler implementation
- **Parallel Markers [P]**: Independent files within same layer can be built concurrently
- **Critical Path**: Migration → Proto → Repository → Handlers → CLI → Integration Tests

**Estimated Breakdown**:
- Database: 4 tasks
- Protobuf: 3 tasks
- Server Logic: 7 tasks
- Handlers: 5 tasks
- SDK: 2 tasks
- CLI: 9 tasks
- Integration Tests: 3 tasks
- Documentation: 2 tasks

**Total**: ~35-38 tasks in dependency-ordered, TDD-driven sequence

**IMPORTANT**: This phase is executed by the `/tasks` command, NOT by `/plan`. The above describes the *approach* for task generation, not the execution.

## Phase 3+: Future Implementation
*These phases are beyond the scope of the /plan command*

**Phase 3**: Task execution (/tasks command creates tasks.md)  
**Phase 4**: Implementation (execute tasks.md following constitutional principles)  
**Phase 5**: Validation (run tests, execute quickstart.md, performance validation)

## Complexity Tracking
*Fill ONLY if Constitution Check has violations that must be justified*

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| New dependency: gonum.org/v1/gonum/graph | Topological sort and cycle detection for DAG validation (FR-032, FR-033, FR-045) | Hand-rolled toposort is error-prone for complex graphs; Gonum is battle-tested, standard in Go ecosystem, and provides multigraph support. Server-side only (gridapi), zero client impact. |
| PostgreSQL trigger for cycle prevention | Enforce DAG invariant at database boundary (FR-003, FR-004, FR-045) | Application-level checks alone can race under concurrent edge creation; DB trigger is defensive correctness layer ensuring no cycles survive commit. One-time migration cost, documented in CLAUDE.md. |


## Progress Tracking
*This checklist is updated during execution flow*

**Phase Status**:
- [x] Phase 0: Research complete (/plan command) - ✅ research.md created
- [x] Phase 1: Design complete (/plan command) - ✅ data-model.md, contracts/state.proto, quickstart.md, CLAUDE.md updated
- [x] Phase 2: Task planning complete (/plan command - describe approach only) - ✅ Approach documented above
- [ ] Phase 3: Tasks generated (/tasks command) - READY for /tasks execution
- [ ] Phase 4: Implementation complete
- [ ] Phase 5: Validation passed

**Gate Status**:
- [x] Initial Constitution Check: PASS (with documented complexity additions)
- [x] Post-Design Constitution Check: PASS (no new violations introduced)
- [x] All NEEDS CLARIFICATION resolved (no unknowns in Technical Context)
- [x] Complexity deviations documented (Gonum/graph dependency, DB cycle trigger)

**Execution Summary**:
- ✅ All /plan command phases (0-2) complete
- ✅ Constitution-compliant design (Principles I-VII validated)
- ✅ Ready for /tasks command to generate tasks.md
- ✅ Branch: 002-add-state-dependency
- ✅ Artifacts: plan.md, research.md, data-model.md, contracts/state.proto, quickstart.md, CLAUDE.md

---
*Based on Constitution v2.0.0 - See `.specify/memory/constitution.md`*
