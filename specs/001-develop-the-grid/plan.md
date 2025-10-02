
# Implementation Plan: Terraform State Management Framework

**Branch**: `001-develop-the-grid` | **Date**: 2025-09-30 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/Users/vincentdesmet/tcons/grid/specs/001-develop-the-grid/spec.md`

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
This feature implements a Terraform/OpenTofu HTTP backend for remote state management. Users can create named states via CLI (client generates UUIDv7 for optimal database performance), receive Terraform backend configuration, and use standard Terraform commands (init/plan/apply) to persist state remotely. The system handles state locking, provides GUID-to-logic-id listing, exposes lock inspection and unlock helpers through the SDK contracts, and enforces global logic-id uniqueness without authentication/authorization in this initial version. Uses Go 1.24, Postgres 17-alpine (Docker Compose), Bun ORM with Go migrations, and Connect RPC for SDK APIs. Server internals include a dedicated config + Bun provider layer to construct the PostgreSQL connection that feeds migrations and repositories. Both Go (`pkg/sdk`) and TypeScript (`js/sdk`) SDKs reuse the generated `github.com/terraconstructs/grid/api` message types (no duplicate DTOs) while business logic remains internal to the API service.

## Technical Context
**Languages/Versions**: Go 1.24 (managed via mise, workspace-based monorepo), TypeScript 5.x (Node.js 22 LTS)
**Primary Dependencies**:
- Bun ORM (https://bun.uptrace.dev/) for database access inside API server internal repository package (Go)
- Chi router for HTTP server
- Cobra for CLI command tree structure (Go)
- Connect RPC (connectrpc.com) for protobuf-based API services (Go/TypeScript clients)
- PostgreSQL 17 for state persistence (via Docker Compose)
- google/uuid v1.6+ for client-side UUIDv7 generation (Go)
- Connect RPC TypeScript client runtime (`@connectrpc/connect`, `@connectrpc/connect-node`) for Node.js SDK

**Storage**: PostgreSQL 17-alpine (Docker Compose) with native UUID type, no extensions required
**Testing**: Go standard testing, table-driven tests for Connect RPC services, Vitest/Jest for Node.js SDK, integration tests with both Terraform and OpenTofu CLI
**Target Platform**: Linux/macOS servers (API), cross-platform CLI binaries
**Module Namespace**: All Go modules live under `github.com/terraconstructs/grid/...` inside the workspace
**Project Type**: Monorepo with API server (`cmd/gridapi`), Go CLI client (`cmd/gridctl`), Go SDK (`pkg/sdk`), and Node.js SDK (`js/sdk`)
**Performance Goals**: Sub-second state retrieval for files <10MB, immediate lock conflict detection, UUIDv7 for optimal Postgres B-tree index performance
**Constraints**: No authentication/authorization in initial version, 10MB state file size warning threshold, client-side GUID generation (per specification)
**Scale/Scope**: Single-user proof of concept, unlimited states per deployment, all states globally accessible

## Constitution Check
*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**I. Go Workspace Architecture**: ✅ PASS
- Using Go workspaces with independent modules: api-server (cmd/gridapi), cli (cmd/gridctl), pkg/sdk, api (generated)
- Proto definitions in ./proto/<service>/v1/
- No constitutional violations

**II. Contract-Centric SDKs**: ✅ PASS
- API server keeps business logic within internal packages (e.g., `cmd/gridapi/internal/...`) and exposes it only via Connect RPC.
- Go (`pkg/sdk`) and Node.js (`js/sdk`) SDKs wrap generated Connect clients without embedding persistence or domain logic.
- CLI consumes the Go SDK; other clients must use published SDKs where available.
- Terraform HTTP Backend endpoint (/tfstate) remains an explicitly allowed exception for external tool integration per Principle IV.

**III. Dependency Flow Discipline**: ✅ PASS
- cmd/gridapi → `cmd/gridapi/internal/config`, `cmd/gridapi/internal/db/...` (bun provider, models, repository), `cmd/gridapi/internal/state`, api (generated clients)
- cmd/gridctl → pkg/sdk, api
- pkg/sdk → api (Connect clients)
- js/sdk → api (generated TypeScript clients)
- api ← proto (autogenerated)
- No circular dependencies; SDKs never depend on application code and the API server never imports SDK modules.

**IV. Cross-Language Parity via Connect RPC**: ✅ PASS with EXCEPTION
- Using Connect RPC for state management API services
- **Constitutional Exception Applied**: Terraform HTTP Backend at /tfstate endpoint is explicitly permitted
  - Not reflected in protobuf/SDKs
  - Consumed only by Terraform/OpenTofu CLI binaries
  - Will document in docs/terraform-backend.md
  - Will include integration tests with real Terraform CLI

**V. Test Strategy**: ✅ PASS
- Unit tests for SDK using table-driven patterns
- Contract tests for Connect RPC services following kmcd.dev/posts/connectrpc-unittests
- Integration tests with real Terraform/OpenTofu CLI
- TDD approach: tests first, implementation after

**VI. Versioning & Releases**: ✅ PASS
- Proto versioning: proto/state/v1/
- Independent module versioning
- Initial v0.1.0 releases for all modules

**VII. Simplicity & Pragmatism**: ⚠️ JUSTIFIED DEVIATION
- Using Repository pattern with Bun ORM (user-specified implementation detail)
- **Justification**: Repository pattern acceptable for initial implementation with clear persistence layer
- Using Cobra for CLI (user-specified, industry standard for Go CLIs)
- Using Chi router (user-specified, lightweight and idiomatic)

**Overall Status**: ✅ PASS - All constitutional requirements met, exceptions properly documented

## Project Structure

### Documentation (this feature)
```
specs/[###-feature]/
├── plan.md              # This file (/plan command output)
├── research.md          # Phase 0 output (/plan command)
├── data-model.md        # Phase 1 output (/plan command)
├── quickstart.md        # Phase 1 output (/plan command)
├── contracts/           # Phase 1 output (/plan command)
└── tasks.md             # Phase 2 output (/tasks command - NOT created by /plan)
```

### Source Code (repository root)
```
/
├── go.work                          # Go workspace definition
├── docker-compose.yml               # PostgreSQL 17-alpine with volumes
├── mise.toml                        # Tool versions (go 1.24, terraform, opentofu, node 22)
├── proto/
│   └── state/
│       └── v1/
│           └── state.proto          # State service protobuf definitions
│
├── api/                             # Generated Connect RPC Go code (module)
│   ├── go.mod
│   └── state/
│       └── v1/                      # Generated from proto/state/v1
│
├── pkg/
│   └── sdk/                         # Public Go SDK (module)
│       ├── go.mod
│       ├── state_client.go          # Connect client wrapper + DTOs
│       └── state_client_test.go
│
├── js/
│   └── sdk/                         # Public Node.js SDK (module)
│       ├── package.json
│       ├── src/
│       │   ├── index.ts             # Exported SDK surface mirroring Go SDK
│       │   └── clients.ts           # Generated Connect client re-export
│       ├── test/
│       │   └── state.test.ts        # Contract tests for Node.js SDK
│       └── tsconfig.json
│
├── cmd/
│   ├── gridapi/                     # API Server (module)
│   │   ├── go.mod
│   │   ├── main.go
│   │   ├── internal/
│   │   │   ├── config/                   # DSN parsing + runtime flags/env
│   │   │   │   └── config.go
│   │   │   ├── db/
│   │   │   │   ├── bunx/                 # Bun provider (pgdriver + pgdialect)
│   │   │   │   │   └── provider.go
│   │   │   │   ├── models/               # Bun models shared by repo/migrations
│   │   │   │   │   └── state.go
│   │   │   │   ├── repository/
│   │   │   │   │   ├── interface.go     # Persistence interface (internal only)
│   │   │   │   │   ├── bun_state_repository.go
│   │   │   │   │   └── bun_state_repository_test.go
│   │   │   │   └── migrations/           # Bun-powered migrations (Go files)
│   │   │   │       ├── registry.go
│   │   │   │       └── 20250930000001_create_states_table.go
│   │   │   ├── state/
│   │   │   │   ├── service.go       # Business logic orchestrating repositories
│   │   │   │   └── service_test.go
│   │   │   ├── server/
│   │   │   │   ├── chi.go           # Chi router setup
│   │   │   │   ├── connect.go       # Connect RPC handler mounting
│   │   │   │   ├── connect_handlers.go   # StateService RPC handlers (incl. lock ops)
│   │   │   │   └── tfstate.go       # Terraform HTTP Backend handlers (LOCK/UNLOCK + PUT fallbacks)
│   │   └── cmd/
│   │       ├── root.go              # Cobra root command
│   │       ├── serve.go             # Serve command group
│   │       └── db.go                # DB command group (migrations)
│   │
│   └── gridctl/                     # CLI Client (module)
│       ├── go.mod
│       ├── main.go
│       ├── internal/
│       │   ├── templates/           # Embedded HCL templates
│       │   │   └── backend.hcl.tmpl
│       │   └── ui/                  # CLI output formatting
│       └── cmd/
│           ├── root.go              # Cobra root command
│           └── state/               # State command group
│               ├── create.go
│               ├── list.go
│               └── init.go          # Generate backend config
│
├── tests/
│   ├── contract/                    # Connect RPC contract tests (shared expectations)
│   ├── integration/                 # Full system integration tests
│   │   ├── terraform/               # Terraform CLI scenarios
│   │   └── opentofu/                # OpenTofu CLI scenarios
│   └── fixtures/                    # Test data (sample .tf files)
│
└── docs/
    └── terraform-backend.md         # Terraform HTTP Backend API documentation
```

**Structure Decision**: Go workspace monorepo with constitutional architecture. Modules: API server (`cmd/gridapi`), Go SDK (`pkg/sdk`), CLI (`cmd/gridctl`), generated API clients (`api`), and Node.js SDK (`js/sdk`). API server mounts Connect RPC and Terraform HTTP Backend handlers, with Bun-backed repositories isolated under `cmd/gridapi/internal/db/repository` and hydrated by the shared Bun provider. SDK modules remain transport/client-focused, consuming generated Connect clients. Proto definitions generate shared artifacts for both SDKs.

## Phase 0: Outline & Research
1. **Extract unknowns from Technical Context** above:
   - For each NEEDS CLARIFICATION → research task
   - For each dependency → best practices task
   - For each integration → patterns task

2. **Generate and dispatch research agents**:
   ```
   For each unknown in Technical Context:
     Task: "Research {unknown} for {feature context}"
   For each technology choice:
     Task: "Find best practices for {tech} in {domain}"
   ```

3. **Consolidate findings** in `research.md` using format:
   - Decision: [what was chosen]
   - Rationale: [why chosen]
   - Alternatives considered: [what else evaluated]

**Output**: research.md with all NEEDS CLARIFICATION resolved

## Phase 1: Design & Contracts
*Prerequisites: research.md complete*

1. **Extract entities from feature spec** → `data-model.md`:
   - Entity name, fields, relationships
   - Validation rules from requirements
   - State transitions if applicable

2. **Generate API contracts** from functional requirements:
   - For each user action → endpoint
   - Use standard REST/GraphQL patterns
   - Output OpenAPI/GraphQL schema to `/contracts/`

3. **Generate contract tests** from contracts:
   - One test file per endpoint
   - Assert request/response schemas
   - Tests must fail (no implementation yet)

4. **Extract test scenarios** from user stories:
   - Each story → integration test scenario
   - Quickstart test = story validation steps

5. **Update agent file incrementally** (O(1) operation):
   - Run `.specify/scripts/bash/update-agent-context.sh claude`
     **IMPORTANT**: Execute it exactly as specified above. Do not add or remove any arguments.
   - If exists: Add only NEW tech from current plan
   - Preserve manual additions between markers
   - Update recent changes (keep last 3)
   - Keep under 150 lines for token efficiency
   - Output to repository root

**Output**: data-model.md, /contracts/*, failing tests, quickstart.md, agent-specific file

## Phase 2: Task Planning Approach
*This section describes what the /tasks command will do - DO NOT execute during /plan*

**Task Generation Strategy**:
The /tasks command will load `.specify/templates/tasks-template.md` and generate tasks from Phase 1 artifacts:

1. **Infrastructure Setup** (from project structure):
   - Initialize Go workspace (go.work)
   - Create module structure (api-server, cli, pkg/sdk, js/sdk, api)
   - Setup database (PostgreSQL schema, Bun ORM initialization)
   - Configure Buf for protobuf generation (Go + TypeScript targets)
   - Configure Node.js tooling (pnpm, package.json, tsconfig, biome(linting) /vitest (testing) scripts)
   - Makefile must expose the following targets:
     ```makefile
     build      # Build gridapi and gridctl into bin/
     db-up      # Start PostgreSQL via docker compose up -d
     db-down    # Stop PostgreSQL via docker compose down
     db-reset   # docker compose down -v && up -d (fresh database)
     test       # Run Go + JS test suites (unit + contract + integration)
     clean      # Remove bin/ directory and test artifacts
     help       # Display available targets with descriptions
     ```

2. **Contract Tests** (from contracts/):
   - Connect RPC contract tests (state-service.proto) [P]
   - Terraform HTTP Backend REST contract tests (terraform-backend-rest.yaml) [P]
   - Tests MUST fail initially (no implementation)

3. **Data Layer** (from data-model.md):
   - Config package for DSN/flag handling under `cmd/gridapi/internal/config`
   - Bun provider under `cmd/gridapi/internal/db/bunx` (pgdriver + pgdialect)
   - Models + migrations under `cmd/gridapi/internal/db/models` and `.../migrations`
   - Repository interface + Bun implementation with unit tests [P]
   - Internal service wiring under `cmd/gridapi/internal/state` exposing business operations

4. **Protobuf & Code Generation** (from contracts/state-service.proto):
   - Define proto/state/v1/state.proto covering create/list/config plus lock inspection (`GetStateLock`) and unlock (`UnlockState`)
   - Generate Connect RPC code into api/state/v1
   - Verify generated code compiles

5. **SDK Implementation – Go** (from data-model.md + contracts):
   - State management Go SDK wrapping Connect client, including lock inspection + unlock helpers
   - SDK unit tests (table-driven, Connect client mocks)
   - Ensure SDK remains persistence-agnostic

6. **SDK Implementation – Node.js** (mirrors Go SDK):
   - Generate TypeScript clients from proto using Buf
   - Implement Node.js SDK surface (`createState`, `listStates`, `getStateConfig`, `getStateLock`, `unlockState`)
   - Write contract/unit tests in Vitest/Jest using mocked transport
   - Configure build/test scripts and TypeScript compilation

7. **API Server - Connect RPC** (from contracts/state-service.proto):
   - Implement StateService Connect handlers (create/list/config/lock inspection/unlock)
   - Wire handlers to internal service consuming repository
   - Mount Connect handlers on Chi router
   - Handler unit tests

8. **API Server - Terraform Backend** (from contracts/terraform-backend-rest.yaml):
   - Implement GET /tfstate/{guid}
   - Implement POST /tfstate/{guid}
   - Implement LOCK-compatible /tfstate/{guid}/lock (method whitelist covering `LOCK` + `PUT`)
   - Implement UNLOCK-compatible /tfstate/{guid}/unlock (method whitelist covering `UNLOCK` + `PUT`)
   - Mount Terraform handlers on Chi router
   - REST handler unit tests

9. **API Server - Cobra Commands** (from research.md):
   - Cobra root command setup
   - Serve command (start HTTP server)
   - DB command group (migrate up/down/status)
   - Command tests

10. **CLI - Cobra Commands** (from research.md + quickstart.md):
   - Cobra root command setup
   - State command group (create/list/init)
   - Embed HCL template (backend.hcl.tmpl)
   - CLI output formatting (tab-delimited list)
   - Command tests

11. **Integration Tests** (from quickstart.md scenarios):
    - Terraform and OpenTofu integration test suites (real CLIs)
    - Test fixture: sample .tf files with null resources
    - End-to-end test: create → init (Terraform & OpenTofu) → apply → plan → apply
    - Lock conflict test scenario
    - Server restart resiliency test ensuring persisted state survives

12. **Documentation & Validation** (from contracts):
    - docs/terraform-backend.md (API documentation)
    - Repository root README.md (getting started)
    - Quickstart walkthrough updates covering Terraform & OpenTofu nuances

**Ordering Strategy**:
- **Phase 0**: Infrastructure & workspace setup (blocking)
- **Phase 1**: Contract tests (parallel, must fail)
- **Phase 2**: Data layer (migrations, repository)
- **Phase 3**: Protobuf generation (blocking for SDK)
- **Phase 4**: SDK implementation (depends on proto)
- **Phase 5**: API server implementation (depends on SDK)
- **Phase 6**: CLI implementation (depends on SDK)
- **Phase 7**: Integration tests (depends on all components)
- **Phase 8**: Documentation

**Dependency Graph**:
```
Workspace → Proto Gen → SDK (Go/JS) → API Server → Integration Tests
         ↘         ↘          ↘    → CLI        ↗
          Data Layer ↗
```

**Parallelization Markers** [P]:
- Contract test files (independent)
- Repository tests (independent)
- SDK unit tests (Go/Node.js) (language-isolated)
- Handler tests (independent within layer)
- CLI command implementation (after SDK ready)
- Node.js SDK build/test tasks

**TDD Enforcement**:
- All tests written before implementation
- Contract tests fail with "not implemented" errors
- Integration tests skip until components ready
- Tests drive implementation (make tests pass)

**Estimated Output**:
- ~70 numbered tasks in tasks.md (additional JS SDK + resiliency coverage)
- 12-15 parallel execution opportunities
- Clear phase boundaries for incremental delivery

**Constitutional Alignment**:
- Contract-centric SDKs: Go and Node.js SDK tasks precede application consumers while keeping business logic server-side
- Test strategy: TDD with Go + TypeScript contract/unit/integration layers
- Dependency flow: Unidirectional (no circular references)
- Connect RPC: Proto generation tasks precede implementation (Go/TS SDKs consume generated code)

**IMPORTANT**: This phase is executed by the /tasks command, NOT by /plan. The above describes the strategy; actual task generation happens in /tasks.

## Phase 3+: Future Implementation
*These phases are beyond the scope of the /plan command*

**Phase 3**: Task execution (/tasks command creates tasks.md)  
**Phase 4**: Implementation (execute tasks.md following constitutional principles)  
**Phase 5**: Validation (run tests, execute quickstart.md, performance validation)

## Complexity Tracking
*Fill ONLY if Constitution Check has violations that must be justified*

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| Repository pattern (Principle VII) | User-specified architecture for persistence layer abstraction | Acceptable for initial implementation: provides testability (mock repositories for SDK tests), supports TDD approach, aligns with SDK-first principle. Direct Bun usage in SDK would couple SDK to specific ORM. |


## Progress Tracking
*This checklist is updated during execution flow*

**Phase Status**:
- [x] Phase 0: Research complete (/plan command)
- [x] Phase 1: Design complete (/plan command)
- [x] Phase 2: Task planning complete (/plan command - describe approach only)
- [ ] Phase 3: Tasks generated (/tasks command)
- [ ] Phase 4: Implementation complete
- [ ] Phase 5: Validation passed

**Gate Status**:
- [x] Initial Constitution Check: PASS
- [x] Post-Design Constitution Check: PASS (re-evaluated, no new violations)
- [x] All NEEDS CLARIFICATION resolved
- [x] Complexity deviations documented (Repository pattern justified)

---
*Based on Constitution v2.0.0 - See `/memory/constitution.md`*
