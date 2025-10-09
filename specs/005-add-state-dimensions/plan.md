
# Implementation Plan: State Labels

**Branch**: `005-add-state-dimensions` | **Date**: 2025-10-08 | **Updated**: 2025-10-09 (Constitution Alignment) | **Spec**: specs/005-add-state-dimensions/spec.md
**Input**: Feature specification captured in `specs/005-add-state-dimensions/spec.md`

## Summary
Extend Grid's state management so operators can attach validated typed labels and query using HashiCorp go-bexpr boolean expressions. The solution uses a simple JSON column (`labels`) on the `states` table, validates against a lightweight policy (regex + enum maps), and filters in-memory using go-bexpr. **EAV tables, JSON Schema validation, and facet projection all deferred** after scope reduction to minimize complexity.

**Constitution Compliance** (FR-045): Dashboard MUST route all API calls through js/sdk wrappers (not generated Connect clients directly) per Principle III. TypeScript SDK provides lightweight bexpr string utilities; GetLabelEnum RPC removed (UIs extract enums from GetLabelPolicy).

## Technical Context
**Language/Version**: Go 1.24.4 (cmd/gridapi, cmd/gridctl), TypeScript SDK consumers (read-only)
**Primary Dependencies**: Connect RPC (buf), Cobra CLI, PostgreSQL 15+, SQLite 3.45+, github.com/hashicorp/go-bexpr (filtering)
**Storage**: PostgreSQL primary (JSONB); SQLite parity (TEXT/JSON1) for future appliances
**Testing**: `make test-unit`, `make test-unit-db`, contract/integration suites under `tests/`
**Target Platform**: Linux/macOS development, containerized GridAPI deployment
**Project Type**: Multi-module Go workspace (API server + CLI + Go SDK + JS SDK)
**Performance Goals**: <50ms p99 for in-memory filtered lists at 100-500 state scale
**Constraints**: JSON column storage only, in-memory bexpr filtering, ≤32 labels/state, typed values (string/number/bool), single operator (pre-RBAC)
**Scale/Scope**: Hundreds of states (100-500); policy updates rare, reads dominant; SQL push-down deferred until state count > 1000

## Constitution Check
Reviewed Constitution v2.0.0. Plan stays within Go workspace boundaries (cmd/gridapi owns persistence, cmd/gridctl uses SDK/Connect clients), keeps contract-first development (proposed RPCs via Connect), and avoids introducing new modules or cross-module imports. No constitutional violations anticipated; only new dependency is go-bexpr which requires standard dependency hygiene in implementation.

## Project Structure

### Documentation (this feature)
```
specs/005-add-state-dimensions/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
└── contracts/
    ├── state-tags.md          # Renamed to state labels; UpdateStateLabels RPC
    ├── state-schemas.md       # DEFERRED: JSON Schema validation
    └── state-facets.md        # DEFERRED: Facet projection
```

### Source Code (repository root)
```
cmd/
├── gridapi/
│   └── internal/
└── gridctl/
pkg/
└── sdk/
api/
js/
└── sdk/
specs/
└── 005-add-state-dimensions/
tests/
├── unit/
├── integration/
└── contract/
```

**Structure Decision**: Maintain existing Go workspace separation—API server logic resides in `cmd/gridapi/internal/...`, CLI flows in `cmd/gridctl`, reusable client logic in `pkg/sdk`, and contracts stay in `api`. Documentation for this feature sits under `specs/005-add-state-dimensions/`.

## Phase 0: Outline & Research
Completed research documented in `research.md`:
1. **Evaluated storage models**: Compared EAV, JSON column, and denormalized approaches; **selected JSON column** for simplicity
2. **Performance analysis**: Determined in-memory bexpr filtering sufficient for 100-500 state scale (<50ms p99); deferred SQL push-down
3. **Filtering approach**: Selected HashiCorp go-bexpr for battle-tested boolean expression evaluation without custom SQL translation
4. **Validation approach**: Rejected JSON Schema in favor of lightweight policy (regex + enum maps) to avoid external deps
5. **Operational decisions**: Simplified policy management, deferred audit logs and complex compliance tracking

## Phase 1: Design & Contracts
Design artifacts produced:
1. `data-model.md` enumerates schema: add `labels` JSONB column to `states` table, create `label_policy` table for policy versioning
2. Contract drafts (`contracts/*.md`) cover label mutations (patch-based updates), policy management (get/set/validate/enum), and bexpr filtering with pagination
3. `quickstart.md` turns user stories into an end-to-end verification script (state create with labels, policy set, bexpr filtering, compliance check)
4. Migration plan simplified: single migration adds `labels` column and `label_policy` table

## Phase 2: Task Planning Approach
- Generate tasks from contracts/data model/quickstart using `.specify/templates/tasks-template.md`
- Organize tasks by TDD order: contract tests → JSON column persistence → bexpr filtering → policy validation → CLI wiring → SDK updates
- Flag parallelizable work (`[P]`) such as independent CLI commands and server handlers once shared migrations complete
- **Simplified scope**: ~10-12 tasks (reduced from original ~26 by removing EAV, JSON Schema, facet projection, audit logs)
- Focus areas: migrations, label CRUD, bexpr integration, policy validator, CLI/SDK updates, tests, documentation

## Phase 3+: Future Implementation
- `/tasks` will materialize tasks.md from the design artifacts.
- Implementation executes tasks, ensuring Go code lives in appropriate modules and CLI leverages SDK.
- Validation includes `make test-unit`, `make test-unit-db`, targeted integration tests, and the quickstart walkthrough.

## Complexity Tracking
| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|--------------------------------------|
| None | - | - |

## Scope Reductions from Original Plan
| Deferred Item | Original Justification | Deferral Reason |
|---------------|------------------------|-----------------|
| EAV tables (`meta_keys`, `meta_values`, `state_metadata`) | Normalized storage, cache-friendly | JSON column simpler, adequate for scale |
| JSON Schema validation | Rich schema governance | Lightweight policy (regex + enums) sufficient |
| SQL query builder (squirrel/goqu) | Complex filter translation | In-memory bexpr avoids SQL translation complexity |
| Facet projection (`state_facets` table) | Fast queries for promoted keys | In-memory filtering adequate at 100-500 state scale |
| Facet registry & backfill jobs | Projection maintenance | No projection layer needed yet |
| Audit log infrastructure | Track policy changes | Simple versioning sufficient for now |
| Compliance tracking system | Mark non-compliant states | Manual compliance report sufficient |
| CLI `facets` and `audit` subcommands | Facet/audit management | Deferred with facet/audit infrastructure |
| **GetLabelEnum RPC** (2025-10-09) | Dedicated endpoint for enum values | UIs extract enums from GetLabelPolicy; simpler API surface |

## Progress Tracking
**Phase Status**:
- [x] Phase 0: Research complete (/plan command)
- [x] Phase 1: Design complete (/plan command)
- [ ] Phase 2: Task planning complete (/plan command - describe approach only)
- [ ] Phase 3: Tasks generated (/tasks command)
- [ ] Phase 4: Implementation complete
- [ ] Phase 5: Validation passed

**Gate Status**:
- [x] Initial Constitution Check: PASS
- [x] Post-Design Constitution Check: PASS
- [x] All NEEDS CLARIFICATION resolved
- [x] Complexity deviations documented

---
*Based on Constitution v2.0.0 - See `.specify/memory/constitution.md`*
