# Tasks: Live Dashboard Integration

**Input**: Design documents from `/specs/004-wire-up-grid/`
**Prerequisites**: plan.md (required), research.md, data-model.md, contracts/, quickstart.md

## Execution Flow (main)
```
1. Load plan.md to confirm tech stack (Go 1.24, Connect RPC, React/Vite) and repo layout
2. Read research.md for decisions on Connect testing, dual ESM/CJS builds, and adapter architecture
3. Extract entities from data-model.md (StateInfo, DependencyEdge, OutputKey, BackendConfig) for model tasks
4. Parse contracts/list-all-edges-rpc.md to drive contract + handler test coverage
5. Review cmd/gridapi/cmd/serve.go (currently plain http.Server + ListenAndServe) before refactoring for h2c
6. Expand quickstart scenarios (1-7) into parallel integration tests covering UI behaviours
7. Generate ordered tasks honoring TDD: Setup → Tests → Models → Services → Endpoints → Integration → Polish
8. Validate tasks cover proto, backend, SDK, and webapp changes without missing dependencies
9. Document TypeScript test placement rules (js/sdk under test/ and code under src/; webapp tests in component-level `__tests__`)
10. Return SUCCESS when tasks.md captures executable plan
```

## Format: `[ID] [P?] Description`
- **[P]**: Task can run in parallel (different files, no blocking deps)
- Every task lists concrete repository paths

## Phase 3.1: Setup
- [x] T001 Update `webapp/package.json` to add dev dependencies: `vitest`, `@testing-library/react`, `@testing-library/user-event`, and `@testing-library/jest-dom`; add a `pnpm test` script so UI specs can execute
- [x] T002 Create `webapp/vitest.config.ts` with jsdom + React plugin config and register `test/setup.ts`; add `webapp/test/setup.ts` to install `@testing-library/jest-dom`
- [x] T003 Scaffold `webapp/test/gridTestUtils.tsx` providing a `renderWithGrid` helper that builds a `createRouterTransport` mock + `GridProvider` wrapper for scenario tests

## Phase 3.2: Tests First (TDD) ⚠️ write before implementation
> **Exception**: `js/sdk` and webapp lack baseline failing tests, so T009 and T010–T016 will be authored immediately after the corresponding implementation tasks in Phase 3.3 while keeping their scopes intact.

- [ ] T004 [P] Add contract test `tests/contract/state_service_list_all_edges_test.go` asserting `ListAllEdges` returns sorted edges with status enums and timestamps
- [ ] T005 [P] Add repository test `cmd/gridapi/internal/repository/list_all_edges_test.go` covering `ListAllEdges` query (uses test DB helpers) and verifies ordering + JSON fields
- [ ] T006 [P] Add service test `cmd/gridapi/internal/dependency/service_list_all_edges_test.go` ensuring new service method maps repository edges to domain structs
- [ ] T007 [P] Add handler test `cmd/gridapi/internal/server/connect_handlers_list_all_edges_test.go` using httptest to exercise Connect RPC and assert proto payload
- [ ] T008 [P] Add SDK client test `pkg/sdk/state_client_list_all_edges_test.go` verifying `Client.ListAllEdges` invokes RPC and normalizes proto edges
- [ ] T009 [P] Add adapter test `js/sdk/test/adapter.test.ts` using `createRouterTransport` to ensure `GridApiAdapter.getAllEdges()` converts bigints + timestamps correctly (author immediately after adapter implementation per exception)
- [ ] T010 [P] Scenario 1 test `webapp/src/__tests__/dashboard_initial_load.test.tsx` validating initial load flow displays live states/edges from adapter (author after implementation per exception)
- [ ] T011 [P] Scenario 2 test `webapp/src/__tests__/dashboard_list_view.test.tsx` verifying list view tables render counts, status badges, and detail drawer trigger (author after implementation per exception)
- [ ] T012 [P] Scenario 3 test `webapp/src/__tests__/dashboard_detail_view.test.tsx` checking detail drawer shows dependencies, outputs, navigation flows, and sensitive-output redaction (author after implementation per exception)
- [ ] T013 [P] Scenario 4 test `webapp/src/__tests__/dashboard_manual_refresh.test.tsx` covering manual refresh button behaviour (no polling) and preserving the selected state (author after implementation per exception)
- [ ] T014 [P] Scenario 5 test `webapp/src/__tests__/dashboard_error_handling.test.tsx` asserting error toasts + fallback UI when adapter throws (author after implementation per exception)
- [ ] T015 [P] Scenario 6 test `webapp/src/__tests__/dashboard_edge_status.test.tsx` ensuring graph/list reflect edge status colors + legends (author after implementation per exception)
- [ ] T016 [P] Scenario 7 test `webapp/src/__tests__/dashboard_empty_state.test.tsx` validating empty + single-state cases render helper text without crashes (author after implementation per exception)

## Phase 3.3: Core Implementation (after tests are failing)
- [x] T017 [P] Create `js/sdk/src/models/state-info.ts` defining `StateInfo` interface per data-model (timestamps as ISO strings, nested relationships)
- [x] T018 [P] Create `js/sdk/src/models/dependency-edge.ts` defining `DependencyEdge` interface with status union and digest fields
- [x] T019 [P] Create `js/sdk/src/models/output-key.ts` defining `OutputKey` model with sensitivity flag
- [x] T020 [P] Create `js/sdk/src/models/backend-config.ts` defining `BackendConfig` model with lock URLs
- [x] T021 Update `js/sdk/src/types.ts` to re-export model interfaces and expose enums/constants needed by adapter consumers
- [x] T022 [P] Implement error normalization helpers in `js/sdk/src/errors.ts` mapping `ConnectError` codes to dashboard-ready messages
- [x] T023 Implement `GridApiAdapter` in `js/sdk/src/adapter.ts` (methods: `listStates`, `getStateInfo`, `getAllEdges`, conversions, loading/error signals)
- [x] T024 Implement transport factory + helper in `js/sdk/src/client.ts` building Connect clients from `Transport` or base URL
- [x] T025 Rewrite `js/sdk/src/index.ts` to export adapter, client factory, model types, and preserve generated service exports
- [x] T026 Add `ListAllEdges` RPC + messages to `proto/state/v1/state.proto` with comments mirroring contract requirements
- [x] T027 Regenerate API stubs by running `buf generate` so `api/state/v1`, `pkg/sdk`, and `js/sdk/gen` include the new RPC
- [x] T028 Extend repository contracts in `cmd/gridapi/internal/repository/interface.go` and implement `ListAllEdges` in `bun_edge_repository.go` (ordered query, model mapping)
- [x] T029 Add `ListAllEdges` method to `cmd/gridapi/internal/dependency/service.go` pulling repository edges and enriching logic IDs
- [x] T030 Implement `ListAllEdges` handler in `cmd/gridapi/internal/server/connect_handlers.go` converting domain edges to proto (including timestamps + optional fields)
- [x] T031 Add `ListAllEdges` method to `pkg/sdk/dependency.go` with state/reference conversions + doc comments

## Phase 3.4: Integration
- [x] T032 Refactor `cmd/gridapi/internal/server/connect.go` so `NewHTTPServer` accepts dependency service, mounts it, and wraps router with `h2c.NewHandler`
- [x] T033 Update `cmd/gridapi/cmd/serve.go` to build the new HTTP handler (h2c cleartext) and reuse chi middleware instead of raw `http.Server`
- [x] T034 Implement `GridContext` provider in `webapp/src/context/GridContext.tsx` exposing adapter + transport via React context
- [x] T035 Implement `useGridData` hook in `webapp/src/hooks/useGridData.ts` handling initial load, manual refresh (no polling), preserving selected state across refreshes, and surfacing error state
- [x] T036 Add `webapp/src/services/gridApi.ts` to instantiate `GridApiAdapter` (reading `VITE_GRID_API_URL`) and expose helper factories
- [x] T037 Wrap app root with provider in `webapp/src/main.tsx`, injecting adapter + transport into React tree
- [x] T038 Refactor `webapp/src/App.tsx` to consume `useGridData`, render refresh controls, and surface loading/error signals
- [x] T039 Update `webapp/src/components/GraphView.tsx` to use new types, status color mapping from data-model, and adapter-driven callbacks
- [x] T040 Update `webapp/src/components/ListView.tsx` to source edges/states from adapter types and render status badges per contract
- [x] T041 Update `webapp/src/components/DetailView.tsx` to display full `StateInfo` data, marking sensitive outputs without showing values, and sourcing dependencies/dependents from live API
- [x] T042 Remove `webapp/src/services/mockApi.ts` and replace residual imports with adapter equivalents (including cleanup in tests)

## Phase 3.5: Polish
- [ ] T043 [P] Add `js/sdk/test/errors.test.ts` covering error normalization edge cases (NotFound, Internal, fallback)
- [ ] T044 [P] (Deferred) Add repository benchmark `cmd/gridapi/internal/repository/list_all_edges_bench_test.go` measuring query latency for 1k edges
- [ ] T045 [P] Update `webapp/README.md` with instructions for running vitest scenarios, live API setup, and expected manual QA steps

## Dependencies
- Setup (T001-T003) must finish before any tests (T004-T016)
- Contract + service tests (T004-T009) must exist before implementing proto/handlers (T026-T031)
- Quickstart scenario tests (T010-T016) block UI integration tasks (T034-T042)
- TypeScript models (T017-T021) must complete before adapter/client work (T022-T025)
- Proto update (T026) precedes code generation (T027), which unblocks Go + SDK implementation (T028-T031)
- Server refactor (T032-T033) depends on Connect handler implementation (T030)
- Hook and component updates (T035-T041) depend on adapter + client implementation (T023-T025) and tests (T010-T016)
- Polish tasks (T043-T045) run last after integration is green

## Parallel Execution Example
```
# After setup, kick off independent [P] tests in parallel
task run T004 &  # contract test skeleton
task run T005 &  # repository test skeleton
task run T009 &  # adapter test skeleton
task run T010 &  # scenario 1 UI test
wait
```

## Notes
- Keep generated code (api/, js/sdk/gen) in sync with proto changes from T026/T027
- Ensure new tasks respect existing gofmt/eslint/vitest conventions
- Confirm `serve.go` refactor preserves graceful shutdown + middleware stack while adding h2c support (review current `NewHTTPServer`/`MountConnectHandlers` before refactoring)
- Place `js/sdk` tests alongside source files for easy publish filtering, and house webapp tests in feature-level `__tests__` directories with tsconfig/Vitest updates to match
