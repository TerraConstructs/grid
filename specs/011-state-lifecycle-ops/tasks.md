# Tasks Index: State Lifecycle Operations

Beads Issue Graph Index into the tasks and phases for this feature implementation.
This index does **not contain tasks directly**—those are fully managed through Beads CLI.

## Feature Tracking

* **Beads Epic ID**: `grid-asc3`
* **User Stories Source**: `specs/011-state-lifecycle-ops/spec.md`
* **Research Inputs**: `specs/011-state-lifecycle-ops/research.md`
* **Planning Details**: `specs/011-state-lifecycle-ops/plan.md`
* **Data Model**: `specs/011-state-lifecycle-ops/data-model.md`
* **Contract Definitions**: `specs/011-state-lifecycle-ops/contracts/lifecycle-additions.proto`

## Beads Query Hints

Use the `bd` CLI to query and manipulate the issue graph:

```bash
# Find all open tasks for this feature
bd list --label "spec:011-state-lifecycle-ops" --status open --limit 10

# Find ready tasks to implement (no blockers)
bd ready --label "spec:011-state-lifecycle-ops" --limit 5

# See dependency tree for the epic
bd dep tree grid-asc3 --reverse

# View issues by phase
bd list --label "phase:setup,spec:011-state-lifecycle-ops" --limit 10
bd list --label "phase:foundational,spec:011-state-lifecycle-ops" --limit 10
bd list --label "phase:us1,spec:011-state-lifecycle-ops" --limit 10

# View issues by user story
bd list --label "story:US1,spec:011-state-lifecycle-ops" --limit 10
bd list --label "story:US2,spec:011-state-lifecycle-ops" --limit 10
bd list --label "story:US3,spec:011-state-lifecycle-ops" --limit 10

# View issues by component
bd list --label "component:proto,spec:011-state-lifecycle-ops" --limit 5
bd list --label "component:repository,spec:011-state-lifecycle-ops" --limit 5
bd list --label "component:cli,spec:011-state-lifecycle-ops" --limit 5
```

## Tasks and Phases Structure

This feature follows Beads' 2-level graph structure:

* **Epic**: `grid-asc3` → State Lifecycle Operations (full feature)
* **Phases**: Beads issues of type `feature`, children of the epic
  * `grid-asc3.1` - Setup Phase (proto, model, migration, repository)
  * `grid-asc3.2` - Foundational Phase (service layer, HTTP backend blocking)
  * `grid-asc3.3` - User Story 1: Rename (P1 - MVP)
  * `grid-asc3.4` - User Story 2: Tombstone/Restore (P2)
  * `grid-asc3.5` - User Story 3: Purge (P3)
  * `grid-asc3.6` - Polish Phase (validation, docs)
* **Tasks**: Issues of type `task`, children of each feature (phase)

## Label Conventions

| Label | Purpose |
|-------|---------|
| `spec:011-state-lifecycle-ops` | All tasks in this feature |
| `phase:setup` | Setup phase tasks |
| `phase:foundational` | Foundational phase tasks |
| `phase:us1`, `phase:us2`, `phase:us3` | User story phases |
| `phase:polish` | Polish phase tasks |
| `story:US1`, `story:US2`, `story:US3` | User story traceability |
| `requirement:FR-XXX` | Functional requirement traceability |
| `component:proto`, `component:model`, etc. | Component mapping |
| `test:slow` | Tests requiring retention period wait |

---

## Phase 1: Setup (`grid-asc3.1`)

**Purpose**: Shared infrastructure - proto definitions, model updates, migrations, repository interface

**Beads Feature**: `grid-asc3.1` - Setup Phase - Lifecycle Infrastructure

```bash
bd list --label "phase:setup" --label "spec:011-state-lifecycle-ops" --status open
```

### Tasks (9 total)

| ID | Title | Component | Dependencies |
|----|-------|-----------|--------------|
| `grid-asc3.1.1` | Add lifecycle RPCs and messages to state.proto | proto | - |
| `grid-asc3.1.2` | Regenerate Go and TypeScript code from proto | codegen | grid-asc3.1.1 |
| `grid-asc3.1.3` | Add lifecycle action constants to auth/actions.go | auth | - |
| `grid-asc3.1.4` | Extend State model with lifecycle fields | model | - |
| `grid-asc3.1.5` | Create migration for lifecycle columns | migration | grid-asc3.1.4 |
| `grid-asc3.1.6` | Add lifecycle methods to StateRepository interface | repository | grid-asc3.1.4 |
| `grid-asc3.1.7` | Implement lifecycle methods in BunStateRepository | repository | grid-asc3.1.5, grid-asc3.1.6 |
| `grid-v3sy` | Add --retention-days config flag to gridapi serve | config | grid-asc3.1 |
| `grid-di2z` | Add lifecycle actions to product-engineer seed policy | auth | grid-asc3.1.3 |

**Parallel Opportunities**: T1, T3, T4 can run in parallel. T2 waits on T1. T5, T6 wait on T4. T7 waits on T5, T6. Config and policy tasks can run after their dependencies.

**Checkpoint**: Setup complete when all 9 tasks closed. Foundational phase can begin.

---

## Phase 2: Foundational (`grid-asc3.2`)

**Purpose**: Core service layer and HTTP backend blocking - MUST complete before any user story

**Beads Feature**: `grid-asc3.2` - Foundational Phase - Core Lifecycle Service

```bash
bd list --label "phase:foundational" --label "spec:011-state-lifecycle-ops" --status open
```

### Tasks (3 total)

| ID | Title | Component | Dependencies |
|----|-------|-----------|--------------|
| `grid-asc3.2.1` | Add tombstone check to HTTP backend middleware | middleware | Phase 1 |
| `grid-asc3.2.2` | Add lifecycle methods to StateService | service | Phase 1 |
| `grid-asc3.2.3` | Update ListStates to support tombstone filtering | service | Phase 1 |

**Parallel Opportunities**: All 3 tasks can run in parallel after Phase 1 completes.

**Checkpoint**: Foundational complete. All user stories can now begin independently.

---

## Phase 3: User Story 1 - Rename (`grid-asc3.3`) - MVP

**Goal**: Rename state logic_id without disrupting Terraform workflows

**Priority**: P1 (MVP - implement first)

**Independent Test**: Create state, rename, verify GUID unchanged, Terraform continues working

**Beads Feature**: `grid-asc3.3` - User Story 1: Rename State Logic ID

```bash
bd list --label "story:US1" --label "spec:011-state-lifecycle-ops" --status open
```

### Tasks (10 total)

| ID | Title | Component | Dependencies |
|----|-------|-----------|--------------|
| `grid-asc3.3.1` | Add RenameState case to authz interceptor | middleware | Phase 2 |
| `grid-asc3.3.2` | Implement RenameState Connect handler | handler | grid-asc3.3.1 |
| `grid-asc3.3.3` | Add RenameState method to Go SDK | sdk | grid-asc3.3.2 |
| `grid-asc3.3.4` | Implement gridctl state rename command | cli | grid-asc3.3.3 |
| `grid-asc3.3.5` | Integration test: Rename state happy path | test | grid-asc3.3.4 |
| `grid-asc3.3.6` | Integration test: Rename to existing name fails | test | grid-asc3.3.5 |
| `grid-asc3.3.7` | Integration test: Rename locked state fails | test | grid-asc3.3.5 |
| `grid-z6k7` | Integration test: Concurrent rename returns ABORTED | test | grid-asc3.3.5 |
| `grid-9bm0` | Integration test: Lookup by old logic_id returns NOT_FOUND | test | grid-asc3.3.5 |
| `grid-ukbi` | Integration test: Rename tombstoned state to free logic_id | test | grid-asc3.4.10 |

**Note**: `grid-ukbi` depends on US2 (tombstone) but tests rename functionality.

**Checkpoint**: US1 complete. Rename functionality fully working and tested.

---

## Phase 4: User Story 2 - Tombstone/Restore (`grid-asc3.4`)

**Goal**: Soft delete states with recovery option within retention period

**Priority**: P2

**Independent Test**: Create state, tombstone, verify hidden from listings, Terraform blocked with 410, restore works

**Beads Feature**: `grid-asc3.4` - User Story 2: Soft Delete (Tombstone) State

```bash
bd list --label "story:US2" --label "spec:011-state-lifecycle-ops" --status open
```

### Tasks (14 total)

| ID | Title | Component | Dependencies |
|----|-------|-----------|--------------|
| `grid-asc3.4.1` | Add TombstoneState case to authz interceptor | middleware | Phase 2 |
| `grid-asc3.4.2` | Implement TombstoneState Connect handler | handler | grid-asc3.4.1 |
| `grid-asc3.4.3` | Add RestoreState case to authz interceptor | middleware | Phase 2 |
| `grid-asc3.4.4` | Implement RestoreState Connect handler | handler | grid-asc3.4.3 |
| `grid-asc3.4.5` | Add TombstoneState method to Go SDK | sdk | grid-asc3.4.2 |
| `grid-asc3.4.6` | Add RestoreState method to Go SDK | sdk | grid-asc3.4.4 |
| `grid-asc3.4.7` | Implement gridctl state delete command | cli | grid-asc3.4.5 |
| `grid-asc3.4.8` | Implement gridctl state restore command | cli | grid-asc3.4.6 |
| `grid-asc3.4.9` | Add --all flag to gridctl state list | cli | grid-asc3.2.3 |
| `grid-asc3.4.10` | Integration test: Tombstone state happy path | test | grid-asc3.4.7 |
| `grid-asc3.4.11` | Integration test: Tombstone state with dependents fails | test | grid-asc3.4.10 |
| `grid-asc3.4.12` | Integration test: Restore tombstoned state | test | grid-asc3.4.8 |
| `grid-asc3.4.14` | Integration test: Rename to tombstoned name fails | test | grid-asc3.4.10 |
| `grid-asc3.4.15` | Integration test: Add dependency to tombstoned state fails | test | grid-asc3.4.10 |

**Note**: `grid-asc3.4.13` closed (obsolete) - policy changed to ALLOW renaming tombstoned states. See `grid-ukbi` for that test.

**Parallel Opportunities**: Tombstone track (T1→T2→T5→T7) and Restore track (T3→T4→T6→T8) can run in parallel. T9 depends only on Phase 2.

**Checkpoint**: US2 complete. Tombstone and restore fully working and tested.

---

## Phase 5: User Story 3 - Purge (`grid-asc3.5`)

**Goal**: Permanently delete tombstoned states after retention period

**Priority**: P3

**Independent Test**: Tombstone state, purge (with force or after retention), verify completely removed, logic_id reusable

**Beads Feature**: `grid-asc3.5` - User Story 3: Purge Tombstoned State

```bash
bd list --label "story:US3" --label "spec:011-state-lifecycle-ops" --status open
```

### Tasks (9 total)

| ID | Title | Component | Dependencies |
|----|-------|-----------|--------------|
| `grid-asc3.5.1` | Add PurgeState case to authz interceptor | middleware | Phase 2 |
| `grid-asc3.5.2` | Implement PurgeState Connect handler | handler | grid-asc3.5.1 |
| `grid-asc3.5.3` | Add PurgeState method to Go SDK | sdk | grid-asc3.5.2 |
| `grid-asc3.5.4` | Add --purge flag to gridctl state delete command | cli | grid-asc3.5.3, grid-asc3.4.7 |
| `grid-asc3.5.5` | Integration test: Purge active state fails | test | grid-asc3.5.4 |
| `grid-asc3.5.6` | Integration test: Purge within retention period fails | test | grid-asc3.5.5 |
| `grid-asc3.5.7` | Integration test: Force purge within retention succeeds | test | grid-asc3.5.6 |
| `grid-asc3.5.8` | Integration test: Purge past retention (slow) | test | grid-asc3.5.7 |
| `grid-asc3.5.9` | Integration test: Restore past retention fails (slow) | test | grid-asc3.5.8 |

**Slow Tests**: T8 and T9 tagged with `test:slow` build tag - excluded from CI, run manually.

**Checkpoint**: US3 complete. Purge fully working and tested.

---

## Phase 6: Polish (`grid-asc3.6`)

**Purpose**: Cross-cutting validation and documentation

**Beads Feature**: `grid-asc3.6` - Polish Phase - Documentation and Validation

```bash
bd list --label "phase:polish" --label "spec:011-state-lifecycle-ops" --status open
```

### Tasks (2 total)

| ID | Title | Component | Dependencies |
|----|-------|-----------|--------------|
| `grid-asc3.6.1` | Validate authorization test matrix | test | US1, US2, US3 |
| `grid-asc3.6.2` | Validate quickstart.md scenarios end-to-end | docs | US1, US2, US3 |

**Checkpoint**: Feature complete. All functionality validated.

---

## Dependencies & Execution Order

### Phase Dependencies

```
Phase 1 (Setup)
    ↓
Phase 2 (Foundational) ─── BLOCKS ALL USER STORIES
    ↓
┌───┴───┐
↓       ↓
US1    US2           ← US1 and US2 can run in parallel after Phase 2
(P1)   (P2)
        ↓
       US3           ← US3 depends on US2 (purge requires tombstone)
       (P3)
        ↓
┌───────┴───────┐
↓               ↓
Phase 6 (Polish)
```

**Note**: US3 (Purge) depends on US2 (Tombstone) because:
- `gridctl state delete --purge` extends the delete command from US2
- Purge operation requires state to be tombstoned first

### Recommended Execution Order

1. **MVP (US1 only)**: Setup → Foundational → US1 → Validate
2. **Full Feature**: Setup → Foundational → (US1 || US2) → US3 → Polish

### Within Each User Story

- Authz interceptor → Handler → SDK → CLI → Tests
- Tests run after implementation (not TDD - tests not explicitly requested)

---

## Implementation Strategy

### MVP Scope (Recommended First Delivery)

**User Story 1: Rename** is the suggested MVP:
- Most common day-2 operation
- Simplest lifecycle operation (no status change)
- Fully independent of tombstone/purge
- Delivers immediate value

```bash
# MVP tasks only
bd list --label "phase:setup" --label "spec:011-state-lifecycle-ops"
bd list --label "phase:foundational" --label "spec:011-state-lifecycle-ops"
bd list --label "story:US1" --label "spec:011-state-lifecycle-ops"
```

### Incremental Delivery

| Increment | Scope | Value Delivered |
|-----------|-------|-----------------|
| MVP | US1 (Rename) | Users can reorganize state names |
| +US2 | Tombstone/Restore | Safe deletion with recovery |
| +US3 | Purge | Compliance, resource cleanup |
| +Polish | Full validation | Production-ready |

---

## Summary

| Metric | Count |
|--------|-------|
| Total Issues | 54 |
| Epic | 1 |
| Features (Phases) | 6 |
| Tasks | 47 |
| P1 (MVP) Tasks | 21 (+2 rename tests, +2 setup: config, seed policy) |
| P2 Tasks | 15 (+1 tombstone rename test, -1 obsolete) |
| P3 Tasks | 11 |
| Slow Tests | 2 |

### Story Testability

| Story | Independently Testable | Test Criteria |
|-------|------------------------|---------------|
| US1 (Rename) | Yes | Create → Rename → Verify GUID unchanged |
| US2 (Tombstone) | Yes | Create → Tombstone → Verify 410 → Restore → Verify active |
| US3 (Purge) | Yes | Tombstone → Force Purge → Verify deleted → Reuse logic_id |

### Parallel Opportunities

- **Phase 1**: 3 independent tracks (proto, auth, model)
- **Phase 2**: All 3 tasks can run in parallel
- **User Stories**: US1 and US2 can run in parallel after Phase 2; US3 depends on US2
- **Within US2**: Tombstone and Restore tracks can parallelize

---

> This file is intentionally index-only. Implementation data lives in Beads. Update this file only to point humans and agents to canonical query paths and feature references.
