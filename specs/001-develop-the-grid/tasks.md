# Tasks: Terraform State Management Framework

**Feature**: 001-develop-the-grid
**Branch**: `001-develop-the-grid`
**Input**: Design documents from `/Users/vincentdesmet/tcons/grid/specs/001-develop-the-grid/`

## Tech Stack Summary
- **Languages**: Go 1.24 (workspace-based), TypeScript 5.x (Node.js 22 LTS)
- **Dependencies (Go)**: Bun ORM, Chi router, Cobra CLI, Connect RPC, google/uuid v1.6+
- **Dependencies (Node.js)**: `@connectrpc/connect`, `@connectrpc/connect-node`, Vitest
- **Database**: PostgreSQL 17-alpine (Docker Compose)
- **Testing**: Go testing (table-driven), Vitest (Node.js SDK), Terraform & OpenTofu CLI integration
- **Build**: Makefile orchestration, binaries in `bin/` (gitignored), npm scripts for JS SDK

## Format: `[ID] [P?] Description`
- **[P]**: Can run in parallel (different files, no dependencies)
- All file paths are absolute from repository root

## Phase 0: Infrastructure Setup

- [X] **T001** Create Go workspace with `go.work` file declaring modules: api, pkg/sdk, cmd/gridapi, cmd/gridctl, js/sdk
- [X] **T002** Create `docker-compose.yml` with postgres:17-alpine, health checks, and persistent volumes
- [X] **T003** Create `Makefile` with required targets (build, db-up/down/reset, test, clean, help)
- [X] **T004** [P] Create `buf.yaml` (Go + TypeScript plugins) and `buf.gen.yaml`
- [X] **T005** [P] Create `.gitignore` for Go/Node projects (vendor/, node_modules/, *.test, coverage, bin/)
- [X] **T006** [P] Bootstrap `js/sdk` workspace (`package.json`, `tsconfig.json`, npm scripts for build/test/lint, dependencies)


### Makefile Specification (T003)
```makefile
# Required targets:
# - build: Build gridapi and gridctl to bin/ directory
# - db-up: Start PostgreSQL via docker compose up -d
# - db-down: Stop PostgreSQL via docker compose down
# - db-reset: docker compose down -v && docker compose up -d (fresh database)
# - test: Run all tests (unit, contract, integration)
# - clean: Remove bin/ directory and test artifacts
# - help: Display available targets
```

## Phase 1: Contract Tests (TDD - MUST FAIL BEFORE IMPLEMENTATION)

- [X] **T007** [P] Connect RPC contract test for CreateState in `tests/contract/state_service_create_test.go`
- [X] **T008** [P] Connect RPC contract test for ListStates in `tests/contract/state_service_list_test.go`
- [X] **T009** [P] Connect RPC contract test for GetStateConfig in `tests/contract/state_service_config_test.go`
- [X] **T010** [P] Connect RPC contract test for GetStateLock in `tests/contract/state_service_lock_test.go`
- [X] **T011** [P] Connect RPC contract test for UnlockState in `tests/contract/state_service_unlock_test.go`
- [X] **T012** [P] Terraform HTTP Backend REST contract test GET in `tests/contract/terraform_backend_get_test.go`
- [X] **T013** [P] Terraform HTTP Backend REST contract test POST in `tests/contract/terraform_backend_post_test.go`
- [X] **T014** [P] Terraform HTTP Backend REST contract test LOCK in `tests/contract/terraform_backend_lock_test.go` (assert both custom `LOCK` and fallback `PUT` are accepted)
- [X] **T015** [P] Terraform HTTP Backend REST contract test UNLOCK in `tests/contract/terraform_backend_unlock_test.go` (assert both custom `UNLOCK` and fallback `PUT` are accepted)

## Phase 2: Protobuf & Code Generation

- [X] **T016** Define `proto/state/v1/state.proto` with StateService (CreateState, ListStates, GetStateConfig, GetStateLock, UnlockState)
- [X] **T017** Run `buf generate` to create Go code in `api/state/v1/` and TypeScript clients in `js/sdk/gen/`
- [X] **T018** Create `api/go.mod` module (generated code only) with module path `github.com/terraconstructs/grid/api`
- [X] **T019** [P] Create `js/sdk/gen/README.md` documenting generated artifacts (no manual edits)

## Phase 3: Server Data Layer & Configuration

- [X] **T020** Create `cmd/gridapi/internal/config/config.go` (flag/env parsing for DSN, pool sizing, telemetry toggles)
- [X] **T021** Implement Bun provider in `cmd/gridapi/internal/db/bunx/provider.go` (pgdriver connector + pgdialect, returns `*bun.DB`)
- [X] **T022** Define Bun model in `cmd/gridapi/internal/db/models/state.go` (shared between migrations and repository)
- [X] **T023** Create Go migration `cmd/gridapi/internal/db/migrations/20250930000001_create_states_table.go`
- [X] **T024** Register migrations in `cmd/gridapi/internal/db/migrations/registry.go` with embed FS loader
- [X] **T025** Define repository interface in `cmd/gridapi/internal/db/repository/interface.go`
- [X] **T026** Implement Bun repository in `cmd/gridapi/internal/db/repository/bun_state_repository.go`
- [X] **T027** [P] Repository unit tests in `cmd/gridapi/internal/db/repository/bun_state_repository_test.go` (lock atomicity, size warning logic)
- [X] **T028** Implement internal state service in `cmd/gridapi/internal/state/service.go` (validation + repository orchestration + lock helpers)
- [ ] **T029** [P] Service unit tests in `cmd/gridapi/internal/state/service_test.go`

## Phase 4: SDK Implementation – Go

- [X] **T030** Create `pkg/sdk/go.mod` (module `github.com/terraconstructs/grid/pkg/sdk`) and configure toolchain
- [X] **T031** Ensure Go SDK surfaces generated proto types directly (no custom DTO duplication)
- [X] **T032** Implement Go SDK client in `pkg/sdk/state_client.go` (wrap generated client, expose lock inspection/unlock helpers)
- [X] **T033** [P] Go SDK unit tests in `pkg/sdk/state_client_test.go` (table-driven with mocked Connect client)

## Phase 5: SDK Implementation – Node.js

- [X] **T034** Implement Node.js SDK surface in `js/sdk/src/index.ts` (createState, listStates, getStateConfig, getStateLock, unlockState)
- [X] **T035** [P] Configure build output in `js/sdk/tsconfig.build.json` and npm `build` script (emit ESM+CJS bundles)
- [X] **T036** [P] Node.js SDK contract tests in `js/sdk/test/state.test.ts` (Vitest with mocked transport including lock/unlock)

## Phase 6: API Server – Connect RPC

- [X] **T037** Create `cmd/gridapi/go.mod` (module `github.com/terraconstructs/grid/cmd/gridapi`) with Chi, Bun, Connect dependencies
- [X] **T038** Implement StateService Connect handlers in `cmd/gridapi/internal/server/connect_handlers.go` (invoke state service for create/list/config/lock/unlock)
- [X] **T039** [P] Mount Connect RPC handlers on Chi router in `cmd/gridapi/internal/server/connect.go`
- [X] **T040** [P] Unit test Connect handlers in `cmd/gridapi/internal/server/connect_handlers_test.go`

## Phase 7: API Server – Terraform HTTP Backend

- [X] **T041** Implement GET /tfstate/{guid} in `cmd/gridapi/internal/server/tfstate_handlers.go`
- [X] **T042** Implement POST /tfstate/{guid} in `cmd/gridapi/internal/server/tfstate_handlers.go` (persist state, warn on size)
- [X] **T043** Implement LOCK-compatible /tfstate/{guid}/lock in `cmd/gridapi/internal/server/tfstate_handlers.go` (support method whitelist including `LOCK` and `PUT`)
- [X] **T044** Implement UNLOCK-compatible /tfstate/{guid}/unlock in `cmd/gridapi/internal/server/tfstate_handlers.go` (support method whitelist including `UNLOCK` and `PUT`, validate lock ID)
- [X] **T045** [P] Mount Terraform handlers with method whitelist in `cmd/gridapi/internal/server/tfstate.go`
- [X] **T046** [P] Unit tests for Terraform handlers in `cmd/gridapi/internal/server/tfstate_handlers_test.go`

## Phase 8: API Server – Cobra Commands

- [X] **T047** Create Cobra root command in `cmd/gridapi/cmd/root.go` (flags: --port, --db-url)
- [X] **T048** Create `serve` command in `cmd/gridapi/cmd/serve.go` (start Chi server, mount handlers)
- [X] **T049** Create `db` command group in `cmd/gridapi/cmd/db.go` (migrate up/down/status)
- [X] **T050** Wire Bun migrations in `cmd/gridapi/cmd/db.go` (register migrations, connect to Postgres via bun provider)
- [X] **T051** Create `cmd/gridapi/main.go` entry point executing root command

## Phase 9: CLI Client – SDK Integration

- [X] **T052** Create `cmd/gridctl/go.mod` (module `github.com/terraconstructs/grid/cmd/gridctl`) with Cobra, Connect, google/uuid dependencies
- [X] **T053** Embed HCL template in `cmd/gridctl/internal/templates/backend.hcl.tmpl`
- [X] **T054** Create Cobra root command in `cmd/gridctl/cmd/root.go` (flags: --server)
- [X] **T055** Implement `state create` in `cmd/gridctl/cmd/state.go` (UUIDv7, call Go SDK)
- [X] **T056** Implement `state list` in `cmd/gridctl/cmd/state.go` (tab-delimited output)
- [X] **T057** Implement `state init` in `cmd/gridctl/cmd/state.go` (lookup by logic_id, render template with overwrite prompt)
- [X] **T058** Wire state command group in `cmd/gridctl/cmd/state.go`
- [X] **T059** Create `cmd/gridctl/main.go` entry point executing root command

## Phase 10: Integration Tests

- [X] **T060** Create fixture in `tests/fixtures/null_resources.tf`
- [X] **T061** Integration test quickstart in `tests/integration/quickstart_test.go` (Terraform + OpenTofu init/apply/plan, verify state persistence)
- [X] **T062** [P] Integration test lock conflict in `tests/integration/lock_conflict_test.go`
- [X] **T063** [P] Integration test non-existent state access in `tests/integration/not_found_test.go`
- [X] **T064** [P] Integration test state size warning in `tests/integration/size_warning_test.go`
- [X] **T065** [P] Integration test duplicate logic_id in `tests/integration/duplicate_logic_id_test.go`
- [X] **T066** [P] Integration test server restart durability in `tests/integration/restart_persistence_test.go`

## Phase 11: SDK Packaging & Validation

<!-- DEFERRED: Packaging and publishing will be handled in future sprint -->

- [ ] **T067** [P] Configure `js/sdk` package exports (`exports`, `types`, `files`) - _DEFERRED_
- [ ] **T068** [P] Add npm `lint` script (eslint or ts-standard) and baseline config in `js/sdk` - _DEFERRED_
- [ ] **T069** Run `buf lint` on proto files and ensure generated code committed - _DEFERRED_
- [ ] **T070** Create `docs/terraform-backend.md` documenting REST + Connect surfaces - _DEFERRED_
- [ ] **T071** Create repository root `README.md` covering Terraform & OpenTofu quickstart - _DEFERRED_
- [ ] **T072** Create GitHub Actions workflow `.github/workflows/ci.yml` (Go build/test, Node build/test, buf lint) - _DEFERRED_
- [ ] **T073** Verify all Go contract tests now pass (T007-T015) - _DEFERRED_
- [ ] **T074** Run Node.js SDK test suite (Vitest) and ensure parity with Go SDK - _DEFERRED_
- [ ] **T075** Execute `quickstart.md` manually for Terraform & OpenTofu flows (document results) - _DEFERRED_

## Dependencies

**Blocking Chains**:
1. T001 → T016 → T017 (workspace before proto before generation)
2. T020 → T021 → T026 → T028 → T038 (config/provider before repository, service, and Connect handlers)
3. T017 → T030 → T032 → T038 (generated clients before Go SDK before server consumption)
4. T017 → T034 → T036 → T074 (generated clients before Node SDK before Node testing)
5. T037 → T041 → T045 → T061 (API server module before Terraform handlers before integration tests)
6. T047-T051 → T061 (server commands ready before quickstart integration)
7. T052-T059 → T061 (CLI ready before quickstart integration)
8. T002 → T003 → T061 (infrastructure before integration tests)

**Parallel Opportunities**:
- T004, T005, T006
- Contract tests T007-T015
- Repository tests T027 + service tests T029
- SDK unit tests (T033, T036)
- Handler tests (T040, T046)
- Integration scenarios (T062-T066)
- Packaging tasks (T067-T072)

## Makefile Usage

```bash
# First time setup
make db-up
make build

# Node.js SDK
npm install --workspace js/sdk
npm run build --workspace js/sdk

# Testing
make test
npm test --workspace js/sdk

# Cleanup
make db-down
make clean
```

## Task Execution Notes

1. TDD Enforcement: Complete T007-T015 before any implementation tasks; keep failing tests committed.
2. Contract-Centric SDKs: SDKs (Go/Node.js) must stay persistence-free; all server logic lives under cmd/gridapi/internal.
3. Database Provider: Use `cmd/gridapi/internal/config` and `internal/db/bunx` everywhere to source `*bun.DB`; repositories/migrations must not instantiate their own connections.
4. OpenTofu Support: Integration quickstart (T061) must exercise both Terraform and OpenTofu CLIs.
5. Durability Validation: Restart test (T066) ensures FR-022 (state survives server restarts).
6. Publish Readiness: Packaging tasks (T067-T068) prepare js/sdk for release (proper exports, type declarations).
7. Manual Validation: Record outcomes of quickstart execution (T075) for stakeholders.

## Validation Checklist

- [ ] All proto-defined RPCs have contract tests (T007-T011)
- [ ] Node.js SDK includes automated tests (T036, T074)
- [ ] OpenTofu compatibility validated via integration (T061)
- [ ] Bun provider + config tested through repository/service suites (T027, T029)

———

Total Tasks: 75
Estimated Parallel Batches: ~15
Critical Path: T001 → T016 → T017 → T020 → T021 → T026 → T028 → T038 → T041 → T061 → T075
