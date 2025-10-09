
# Implementation Plan: State Identity Dimensions

**Branch**: `005-add-state-dimensions` | **Date**: 2025-10-08 | **Spec**: specs/005-add-state-dimensions/spec.md
**Input**: Feature specification captured in `specs/005-add-state-dimensions/spec.md`

## Summary
Extend Grid's state management so operators can attach validated tag dimensions and query through indexed EAV joins while maintaining auditability and compliance surfacing. The solution uses dictionary-compressed EAV tables (`meta_keys`, `meta_values`, `state_metadata`) as the complete storage model, adds CLI/SDK tooling for schema management and compliance reporting, and preserves SQLite parity for future embedded deployments. **Facet projection deferred** to future milestone after performance analysis showed indexed EAV adequate for 100-500 state scale.

## Technical Context
**Language/Version**: Go 1.24.4 (cmd/gridapi, cmd/gridctl), TypeScript SDK consumers (read-only)  
**Primary Dependencies**: Connect RPC (buf), Cobra CLI, PostgreSQL 15+, SQLite 3.45+, santhosh-tekuri/jsonschema (planned), Masterminds/squirrel (planned)  
**Storage**: PostgreSQL primary; SQLite parity required for future appliances  
**Testing**: `make test-unit`, `make test-unit-db`, contract/integration suites under `tests/`  
**Target Platform**: Linux/macOS development, containerized GridAPI deployment  
**Project Type**: Multi-module Go workspace (API server + CLI + Go SDK + JS SDK)  
**Performance Goals**: Read latency benchmarks deferred to future load-testing phase  
**Constraints**: Dictionary-compressed EAV only, equality tag filters only, ≤32 tags/state, no JSONB or external search, single authenticated operator (pre-RBAC)
**Scale/Scope**: Hundreds of states (100-500); schema updates rare, reads dominant; facet projection deferred until state count > 1000

## Constitution Check
Reviewed Constitution v2.0.0. Plan stays within Go workspace boundaries (cmd/gridapi owns persistence, cmd/gridctl uses SDK/Connect clients), keeps contract-first development (proposed RPCs via Connect), and avoids introducing new modules or cross-module imports. No constitutional violations anticipated; any new dependency (squirrel/jsonschema) will require standard dependency hygiene in implementation.

## Project Structure

### Documentation (this feature)
```
specs/005-add-state-dimensions/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
└── contracts/
    ├── state-tags.md
    ├── state-schemas.md
    └── state-facets.md
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
1. **Evaluated storage models**: Compared dictionary-compressed EAV, fixed canonical columns, bitmap posting lists, and JSONB approaches
2. **Performance analysis**: Determined indexed EAV sufficient for 100-500 state scale (<50ms p99); deferred facet projection
3. **SQL builders**: Evaluated `Masterminds/squirrel` and `doug-martin/goqu` for Postgres/SQLite support; selected squirrel as baseline
4. **JSON Schema validators**: Evaluated `santhosh-tekuri/jsonschema/v5` and `xeipuuv/gojsonschema`; selected tekuri with keyword restrictions
5. **Operational decisions**: Defined CLI tooling for schema/audit management and compliance surfacing (no facet promotion in v1)

## Phase 1: Design & Contracts
Design artifacts produced:
1. `data-model.md` enumerates dictionary tables, schema versioning, compliance entities, and constraints (≤32 tags, composite index strategy); facet entities marked as deferred
2. Contract drafts (`contracts/*.md`) cover tag mutations, schema governance (set/list/audit/revalidate/flush), and pagination semantics (facet management contracts deferred)
3. `quickstart.md` turns user stories into an end-to-end verification script (state create, schema upload, tag filtering, compliance report)
4. Migration plan simplified since no existing Grid deployments exist - single migration creates all tables following bun pattern

## Phase 2: Task Planning Approach
- Generate tasks from contracts/data model/quickstart using `.specify/templates/tasks-template.md`
- Organize tasks by TDD order: contract tests → EAV data persistence → query builder → CLI wiring → SDK updates
- Flag parallelizable work (`[P]`) such as independent CLI commands and server handlers once shared migrations complete
- **Simplified scope**: ~18-20 tasks (reduced from original ~26 by removing facet promotion/backfill/registry tasks)
- Focus areas: migrations, EAV repository, query builder with squirrel, schema validation, CLI/SDK updates, audit tooling, tests, documentation

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
| Facet projection (`state_facets` table) | Fast queries for promoted keys | Indexed EAV adequate at 100-500 state scale |
| Facet registry (`facets_registry`) | Track promoted key → column mappings | No projection table needed yet |
| Backfill jobs (`facet_refresh_jobs`) | Populate projection after promotion | No projection maintenance needed |
| CLI `facets` subcommand group | Promote/disable/reindex operations | Deferred with projection layer |
| Online DDL management | Add/remove projection columns | Deferred with projection layer |
| Per-facet indexes | Sub-5ms dashboard queries | Composite index on EAV sufficient |

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
