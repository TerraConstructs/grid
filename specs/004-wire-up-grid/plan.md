
# Implementation Plan: [FEATURE]

**Branch**: `004-wire-up-grid` | **Date**: 2025-10-06 | **Spec**: [link](./spec.md)
**Input**: Feature specification from `/specs/004-wire-up-grid/spec.md`

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

Wire the React dashboard PoC to the live Grid API so operators can browse real Terraform state topology instead of mock fixtures. Work centers on adding the `ListAllEdges` RPC, updating SDK facades, and swapping the webapp to manual refresh driven data loads (no background polling in this milestone).

## Technical Context
**Language/Version**: gridapi Go 1.24+, js/sdk TypeScript 5.5+
**Primary Dependencies**:
- webapp React 18+, Vite 5.4+
- pnpm 10.15 workspaces
- Connect RPC client (@connectrpc/connect-web, @bufbuild/protobuf)
**Storage**: not applicable for webapp, PostgreSQL 17+ for gridapi
**Testing**: gp test for all golang code, vitest for all TypeScript code
**Target Platform**: Static Host (S3, CDN) for webapp
**Project Type**: web/desktop
**Performance Goals**: Deferred for PoC/alpha focus on functional wiring
**Constraints**: Manual refresh only (no polling); reuse h2c cleartext server wiring
**Scale/Scope**: Operator-focused PoC against local Grid API instances

### Testing & Code Organization Decisions
- Colocate new `js/sdk` tests next to their source files (e.g., `src/adapter.test.ts`) so publishing filters remain simple.
- In the webapp, place tests in feature-level `__tests__` folders and update Vitest/tsconfig includes accordingly.
- Evaluate `@testing-library/jest-dom` versus `happy-dom` before locking PoC testing dependencies.

## Constitution Check
*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

[Gates determined based on constitution file]

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
proto/state/v1/
└── state.proto          # Add RPC method(s) (for example ListAllEdges, ...)

api/state/v1/            # Generated from proto (buf generate)
└── [generated files]

pkg/sdk/
├── state_client.go      # MODIFIED: Add new RPC Wrappers and types (follow existing patterns)
└── state_client_test.go # MODIFIED: Test for new features such as ListAllEdges

js/sdk/
├── gen         # Generated from proto (buf generate)
├── src
│   └── index.ts      # MODIFIED: Add thin adapter facade and types (follow pkg/sdk thin wrappers patterns)
└── test
    └── state.test.ts # MODIFIED: Add thin adapter facade tests (vitest)

cmd/gridapi/internal/
├── server/
│   └── connect_handlers.go     # MODIFIED: Implement ListAllEdges, ... RPC handlers
├── dependency/
│   └── service.go              # MODIFIED: Add ListAllEdges method
└── repository/
    ├── interface.go            # MODIFIED: Add ListAllEdges interface
    └── bun_edge_repository.go  # MODIFIED: Bun implementation for ListAllEdges

webapp
├── eslint.config.js
├── index.html
├── postcss.config.js
├── README.md
├── src
│   ├── App.tsx
│   ├── components
│   │   ├── DetailView.tsx
│   │   ├── GraphView.tsx
│   │   └── ListView.tsx
│   ├── index.css
│   ├── main.tsx
│   ├── services
│   │   ├── gridApi.ts   # NEW: Real API client using js/sdk
│   │   └── mockApi.ts   # REMOVED
│   └── vite-env.d.ts
└── vite.config.ts
```

**Structure Decision**: Go workspace monorepo (single project structure). All changes fit
within existing modules: proto definitions, generated API code, CLI binary, SDK package, JS/SDK package, API server and webapp PoC.
No new modules required per Constitution Principle VII (Simplicity).

## Phase 0: Outline & Research

✅ **COMPLETE** - See `research.md`

**Completed Research Topics**:
1. ✅ Connect RPC Web Testing Patterns (`createRouterTransport()`, React Context API)
2. ✅ TypeScript SDK Build Configuration (dual ESM/CJS, declaration paths)
3. ✅ Vite Tree-Shaking Requirements (named exports, ESM format)
4. ✅ Error Normalization Pattern (Code enum → user messages, complete mapping)
5. ✅ Minimal API Surface Design (GridApiAdapter, type conversions)

**Key Decisions**:
- Use React Context API for transport injection (testing + production)
- Dual compilation: `lib/{esm,cjs,types}/` with package.json markers
- Error adapter layer maps all 16 Connect error codes to user-friendly messages
- Adapter interface matches mockApi for zero component refactoring
- New RPC required: `ListAllEdges` for performance (O(N+1) → O(1) queries)

**Output**: ✅ `research.md` with all unknowns resolved

## Phase 1: Design & Contracts

✅ **COMPLETE** - See design artifacts

**Completed Deliverables**:
1. ✅ **data-model.md**: 4 entities (StateInfo, DependencyEdge, OutputKey, BackendConfig)
   - Proto → TypeScript type mappings
   - Validation rules and relationships
   - 6 edge status enum values with color mappings

2. ✅ **contracts/list-all-edges-rpc.md**: New RPC specification
   - Request/response schema (protobuf)
   - Error cases and codes
   - Performance characteristics
   - Implementation checklist
   - Migration notes (N+1 → single query)

3. ✅ **quickstart.md**: 7 end-to-end test scenarios
   - Dashboard initial load
   - List view display
   - Detail view with dependencies
   - Manual refresh
   - Error handling
   - Edge status visualization
   - Empty state handling
   - Performance validation metrics

4. ✅ **CLAUDE.md**: Updated with new technologies
   - Added Connect RPC patterns
   - TypeScript SDK build setup
   - Dashboard integration context

**Key Design Decisions**:
- Proto extension adds `ListAllEdges` RPC (backward compatible)
- Repository adds single method: `ListAllEdges(ctx) ([]*models.DependencyEdge, error)`
- Adapter layer provides mockApi-compatible interface (zero component refactoring)
- Type conversion helpers centralized (Timestamp → ISO 8601, int64 → number)

**Output**: ✅ All Phase 1 artifacts generated

## Phase 2: Task Planning Approach
*This section describes what the /tasks command will do - DO NOT execute during /plan*

### Task Generation Strategy

**From Contracts** (`contracts/list-all-edges-rpc.md`):
1. Add `ListAllEdges` RPC to `proto/state/v1/state.proto`
2. Run `buf generate` to regenerate Go + TypeScript clients
3. Add repository interface method + Bun implementation
4. Add service method
5. Wire up Connect RPC handler
6. Write handler tests (table-driven, httptest)
7. Write repository tests (Bun + real PostgreSQL)

**From Data Model** (`data-model.md`):
1. Update `js/sdk/package.json` with dual build config
2. Add TypeScript configs (tsconfig.esm.json, tsconfig.cjs.json)
3. Create adapter types (`js/sdk/src/types.ts`)
4. Create error normalizer (`js/sdk/src/errors.ts`)
5. Create GridApiAdapter (`js/sdk/src/adapter.ts`)
6. Create transport factory (`js/sdk/src/client.ts`)
7. Update main export (`js/sdk/src/index.ts`)
8. Write adapter tests (Vitest + `createRouterTransport()`)

**From Quickstart** (`quickstart.md`):
1. Create GridContext provider (`webapp/src/context/GridContext.tsx`)
2. Create useGridData hook (`webapp/src/hooks/useGridData.ts`)
3. Update App.tsx to use GridProvider
4. Update GraphView to use SDK types
5. Update ListView to use SDK types
6. Update DetailView to use SDK types
7. Delete mockApi.ts
8. Add error toast/notification system
9. Build webapp and verify bundle size
10. Run all 7 quickstart scenarios
11. Preserve the currently selected state during manual refresh interactions
12. Skip background polling in line with FR-022 deferral

### Ordering Strategy

**TDD Order** (tests before implementation):
- Proto contract tests → RPC handler implementation
- Adapter interface tests → adapter implementation
- Component integration tests → component updates

**Exception**: Because `js/sdk` and the webapp lack baseline failing tests, author the new adapter/UI tests immediately after their implementation tasks while still keeping their scopes focused.

**Dependency Order**:
1. **Backend Layer**: Proto → API generation → Repository → Service → Handlers
2. **SDK Layer**: Build config → Types → Error handler → Adapter → Tests
3. **UI Layer**: Context → Hooks → Component updates → Integration tests

**Parallel Execution** ([P] marker):
- Backend repository + handler tests can run in parallel
- SDK adapter tests can run in parallel with build config setup
- Multiple component updates can be done in parallel

### Task Breakdown Structure

**Group 1: Backend (Proto + API Server)** - 8 tasks
- Proto definition
- Code generation
- Repository layer
- Service layer
- Handler layer
- Tests

**Group 2: SDK (TypeScript)** - 10 tasks
- Build configuration
- Type definitions
- Error handling
- Adapter implementation
- Transport factory
- Tests

**Group 3: Webapp Integration** - 8 tasks
- Context provider
- Hooks
- Component updates
- Error UX
- Build verification
- Quickstart validation

**Estimated Output**: 26 numbered tasks in dependency order with [P] markers

### Validation Gates

After each group:
1. **Backend Gate**: All handler tests pass, `buf lint` passes, repository coverage ≥80%
2. **SDK Gate**: Adapter tests pass, build produces all outputs (esm/cjs/types), exports valid
3. **Webapp Gate**: All components render with live data, bundle size <500KB, quickstart scenarios pass

**IMPORTANT**: This phase is executed by the /tasks command, NOT by /plan

## Phase 3+: Future Implementation
*These phases are beyond the scope of the /plan command*

**Phase 3**: Task execution (/tasks command creates tasks.md)  
**Phase 4**: Implementation (execute tasks.md following constitutional principles)  
**Phase 5**: Validation (run tests, execute quickstart.md, performance validation)

## Complexity Tracking
*Fill ONLY if Constitution Check has violations that must be justified*

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |


## Progress Tracking
*This checklist is updated during execution flow*

**Phase Status**:
- [x] Phase 0: Research complete (/plan command) ✅
- [x] Phase 1: Design complete (/plan command) ✅
- [x] Phase 2: Task planning complete (/plan command - describe approach only) ✅
- [x] Phase 3: Tasks generated (/tasks command)
- [ ] Phase 4: Implementation complete
- [ ] Phase 5: Validation passed

**Gate Status**:
- [x] Initial Constitution Check: PASS ✅
- [x] Post-Design Constitution Check: PASS ✅ (re-evaluated, no new violations)
- [x] All NEEDS CLARIFICATION resolved ✅ (via /clarify session 2025-10-06)
- [x] Complexity deviations documented ✅ (none required)

---
*Based on Constitution v2.1.1 - See `/memory/constitution.md`*
