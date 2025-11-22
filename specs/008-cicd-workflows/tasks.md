# Tasks Index: CI/CD Workflows

Beads Issue Graph Index into the tasks and phases for this feature implementation.
This index does **not contain tasks directly**—those are fully managed through Beads CLI and MCP agent APIs.

## Feature Tracking

* **Beads Epic ID**: `grid-dc46`
* **User Stories Source**: `specs/008-cicd-workflows/spec.md`
* **Research Inputs**: `specs/008-cicd-workflows/research.md`
* **Planning Details**: `specs/008-cicd-workflows/plan.md`
* **Data Model**: N/A (CI/CD infrastructure only)
* **Contract Definitions**: N/A (declarative YAML workflows)

## Beads Query Hints

Use the `bd` CLI to query and manipulate the issue graph:

```bash
# Find all open tasks for this feature
bd list --label spec:008-cicd-workflows --status open --limit 20

# Find ready tasks to implement
bd ready --limit 5

# See full dependency tree for the epic
bd dep tree --reverse grid-dc46

# View issues by phase
bd list --label 'phase:setup' --label 'spec:008-cicd-workflows'
bd list --label 'phase:us1' --label 'spec:008-cicd-workflows'
bd list --label 'phase:us2' --label 'spec:008-cicd-workflows'

# View issues by component
bd list --label 'component:ci-cd' --label 'spec:008-cicd-workflows'
bd list --label 'component:build' --label 'spec:008-cicd-workflows'

# Show all phases (features)
bd list --type feature --label 'spec:008-cicd-workflows'

# Show progress statistics
bd stats

# Filter bd ready by label
bd ready --json | jq '.[] | select(.labels // [] | contains(["spec:008-cicd-workflows"]))'  
```

## Tasks and Phases Structure

This feature follows Beads' 2-level graph structure:

* **Epic**: grid-dc46 → CI/CD Workflows feature
* **Phases**: Beads issues of type `feature`, children of the epic
  * Setup Phase (grid-158d) - Project infrastructure
  * Foundational Phase (grid-c778) - Core CI patterns
  * User Story 1 (grid-ea16) - Automated PR Testing [P1]
  * User Story 4 (grid-a98b) - Protobuf Validation [P2]
  * User Story 2 (grid-a59f) - Multi-Platform Release Builds [P2]
  * User Story 3 (grid-93ab) - Database Migration Testing [P3]
  * Polish Phase (grid-3ab5) - Cross-cutting concerns [P2]
* **Tasks**: Issues of type `task`, children of each feature issue (phase)

## Convention Summary

| Type    | Description                      | Labels                                                    |
| ------- | -------------------------------- | --------------------------------------------------------- |
| epic    | Full feature epic                | `spec:008-cicd-workflows`, `component:ci-cd`              |
| feature | Implementation phase / story     | `phase:[name]`, `story:[US#]`                             |
| task    | Implementation task              | `component:[x]`, `requirement:[FR-###]`, `story:[US#]`    |

### Label Breakdown

**Phase Labels**:
- `phase:setup` - Initial project setup
- `phase:foundational` - Blocking prerequisites
- `phase:us1` - User Story 1 (PR Testing)
- `phase:us2` - User Story 2 (Release Builds)
- `phase:us3` - User Story 3 (Database Migration Testing)
- `phase:us4` - User Story 4 (Protobuf Validation)
- `phase:polish` - Cross-cutting concerns

**Component Labels**:
- `component:ci-cd` - CI/CD workflows
- `component:version` - Version management
- `component:build` - Build configuration
- `component:webapp` - Web application
- `component:sdk` - JavaScript SDK
- `component:cli` - Command-line tools
- `component:db` - Database
- `component:docs` - Documentation

**Requirement Labels** (mapped from spec.md):
- `requirement:FR-001` through `requirement:FR-014`

## Agent Execution Flow

MCP agents and AI workflows should:

1. **Assume `bd init` already done** by `specify init`
2. **Use `bd create`** to directly generate Beads issues
3. **Set metadata and dependencies** in the graph, not markdown
4. **Use this markdown only as a navigational anchor**

> Agents MUST NOT output tasks into this file. They MUST use Beads CLI to record all task and phase structure.

## Example Queries for Agents

```bash
# Get all tasks in tree structure for the feature
bd dep tree --reverse grid-dc46

# Get all tasks by user story
bd list --label spec:008-cicd-workflows --label story:US1
bd list --label spec:008-cicd-workflows --label story:US2

# Add a new task to User Story 1
bd create "Add E2E test for PR workflow" -t task --deps "parent-child:grid-ea16" --labels "spec:008-cicd-workflows,story:US1,component:ci-cd"

# Add context to a task via comments
bd comments add grid-d1f2 "Consider using reusable workflow for shared setup steps"

# Mark task as completed
bd update grid-d1f2 --status closed --notes "Workflow created with all required triggers"
```

### Exploring comments for context

Use `bd comments` to add notes, research findings, or any relevant context to a task:

```bash
# View comments on a task
bd comments grid-d1f2

# Add a comment
bd comments add grid-d1f2 "GitHub Actions cache hit rate improved to 75% after optimization"
```

## Status Tracking

Status is tracked only in Beads:

* **Open** → default
* **In Progress** → task being worked on
* **Blocked** → dependency unresolved
* **Closed** → complete

Use `bd ready`, `bd blocked`, `bd stats` to query progress.

---

## Phase 1: Setup (Shared Infrastructure)

**Feature ID**: `grid-158d`
**Feature Title**: Setup Phase: Project Infrastructure
**Purpose**: Initial project setup and configuration files for CI/CD infrastructure

**Query**:
```bash
bd list --label 'phase:setup' --label 'spec:008-cicd-workflows' --status open
bd show grid-158d
```

**Tasks Created**:
- **grid-bf2b**: Create version.go files for gridapi and gridctl
- **grid-4032**: Create .github/workflows directory
- **grid-e8b8**: Create release-please configuration files
- **grid-d23e**: Create .goreleaser.yml configuration

**Dependencies**: None - can start immediately

**Status**: Open

---

## Phase 2: Foundational (Blocking Prerequisites)

**Feature ID**: `grid-c778`
**Feature Title**: Foundational Phase: Core CI Infrastructure
**Purpose**: Core CI infrastructure patterns that all workflows depend on

**Query**:
```bash
bd list --label 'phase:foundational' --label 'spec:008-cicd-workflows' --status open
bd show grid-c778
```

**Tasks Created**:
- **grid-e99e**: Document GitHub Actions workflow patterns

**Dependencies**: Blocks all user story phases
**⚠️ CRITICAL**: No user story work can begin until this phase is complete

**Status**: Open

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Automated PR Testing (Priority: P1) 🎯 MVP

**Feature ID**: `grid-ea16`
**Feature Title**: User Story 1: Automated PR Testing
**Goal**: Automatically test all PRs (unit, integration, frontend, linting) providing fast feedback before merge

**Independent Test**: Open a test PR with valid changes → all test jobs execute and pass. Open a PR with breaking changes → specific failing tests are clearly visible.

**Query**:
```bash
bd list --label 'story:US1' --label 'spec:008-cicd-workflows' --status open
bd show grid-ea16
```

**Tasks Created**:
- **grid-d1f2**: Create PR tests workflow scaffolding (.github/workflows/pr-tests.yml)
- **grid-f6a7**: Add unit tests job to PR workflow
- **grid-45f0**: Add integration tests job (mode2) to PR workflow
- **grid-128f**: Add frontend tests job (webapp) to PR workflow
- **grid-35ba**: Add JS SDK tests job to PR workflow
- **grid-e0fe**: Add Go linting job to PR workflow
- **grid-a322**: Create PR title validation workflow (.github/workflows/pr-title-check.yml)

**Workflows Created**:
- pr-tests.yml (main PR testing workflow with multiple jobs)
- pr-title-check.yml (conventional commit validation)

**Acceptance**:
- All test suites execute automatically on PR creation
- Test failures show clear error messages
- Green status enables merge eligibility
- New commits trigger automatic re-runs

**Dependencies**: Requires Foundational Phase (grid-c778)

**Status**: Open

**Checkpoint**: At this point, PR testing is fully functional - contributors receive automated feedback

---

## Phase 4: User Story 4 - Protobuf and Code Generation Validation (Priority: P2)

**Feature ID**: `grid-a98b`
**Feature Title**: User Story 4: Protobuf and Code Generation Validation
**Goal**: Validate that protobuf-generated code is up-to-date and properly formatted in CI

**Independent Test**: Modify a .proto file without regenerating code → CI detects and fails. Modify .proto and regenerate code → CI passes.

**Query**:
```bash
bd list --label 'story:US4' --label 'spec:008-cicd-workflows' --status open
bd show grid-a98b
```

**Tasks Created**:
- **grid-b53f**: Add buf lint job to PR workflow
- **grid-aea9**: Add buf breaking change detection job to PR workflow
- **grid-5121**: Add generated code freshness check job to PR workflow

**Validations Added to pr-tests.yml**:
- buf lint (protobuf style violations)
- buf breaking (backward compatibility)
- Generated code freshness check (detects stale generated code)

**Acceptance**:
- buf generate executes in CI
- Generated code drift detected and fails build
- Breaking changes flagged with clear messages
- Linting passes for protobuf files

**Dependencies**: Requires Foundational Phase (grid-c778)

**Status**: Open

**Checkpoint**: Protobuf validation ensures API contract consistency

---

## Phase 5: User Story 2 - Multi-Platform Release Builds (Priority: P2)

**Feature ID**: `grid-a59f`
**Feature Title**: User Story 2: Multi-Platform Release Builds
**Goal**: Automatically build and publish release artifacts when changes are merged to main

**Independent Test**: Merge PR to main → build workflows trigger automatically. Builds complete → artifacts produced for all platforms and available on release page.

**Query**:
```bash
bd list --label 'story:US2' --label 'spec:008-cicd-workflows' --status open
bd show grid-a59f
```

**Tasks Created**:
- **grid-25ef**: Create release-please workflow (.github/workflows/release-please.yml)
- **grid-fd31**: Create goreleaser build workflow (.github/workflows/release-build.yml)
- **grid-318b**: Create npm publish workflow (.github/workflows/release-npm.yml)
- **grid-30b0**: Wire version variables to Cobra version commands (gridapi and gridctl)

**Workflows Created**:
- release-please.yml (version management and Release PR automation)
- release-build.yml (goreleaser + webapp bundle on tag push)
- release-npm.yml (npm package publishing)

**Artifacts Produced**:
- gridapi binaries (2: linux/amd64, linux/arm64)
- gridctl binaries (4: linux+darwin for amd64+arm64)
- webapp bundle (1: .tar.gz)
- npm package (1: @tcons/grid)

**Acceptance**:
- Release PR created automatically on releasable commits
- Tag and GitHub Release created on Release PR merge
- All binaries built and uploaded to GitHub Release
- npm package published with correct version
- Zero manual steps required

**Dependencies**: Requires Setup Phase (grid-158d) and Foundational Phase (grid-c778)

**Status**: Open

**Checkpoint**: Full release automation is operational - zero-manual-step releases achieved

---

## Phase 6: User Story 3 - Database Migration Testing (Priority: P3)

**Feature ID**: `grid-93ab`
**Feature Title**: User Story 3: Database Migration Testing
**Goal**: Test database schema migrations against real PostgreSQL instance in CI

**Independent Test**: Create PR with migration changes → CI spins up PostgreSQL, runs migrations, validates schema.

**Query**:
```bash
bd list --label 'story:US3' --label 'spec:008-cicd-workflows' --status open
bd show grid-93ab
```

**Tasks Created**:
- **grid-dd03**: Add database repository tests job to PR workflow
- **grid-1ed4**: Add explicit migration validation test script

**Validations Added**:
- Database repository tests job (validates migrations and database interactions)
- Explicit migration validation script (tests up/down paths)

**Note**: Integration tests (US1) already partially cover this by running against PostgreSQL. This story adds explicit migration-focused validation.

**Acceptance**:
- PostgreSQL container starts in CI
- Migrations execute in sequence
- Schema validation confirms expected structure
- Both upgrade and rollback paths validated (if implemented)

**Dependencies**: Requires Foundational Phase (grid-c778)

**Status**: Open

**Checkpoint**: Database migrations validated before merge - prevents production failures

---

## Phase 7: Polish & Cross-Cutting Concerns (Priority: P2)

**Feature ID**: `grid-3ab5`
**Feature Title**: Polish Phase: Cross-Cutting Concerns and Integration
**Purpose**: Final integration, documentation, and automation enhancements

**Query**:
```bash
bd list --label 'phase:polish' --label 'spec:008-cicd-workflows' --status open
bd show grid-3ab5
```

**Tasks Created**:
- **grid-b182**: Create Dependabot configuration (.github/dependabot.yml)
- **grid-a7d1**: Document branch protection rules
- **grid-6c76**: Update project README with CI/CD badges and release info
- **grid-016f**: Add workflow dispatch triggers for manual runs
- **grid-e513**: Create end-to-end release flow test plan
- **grid-e38b**: Document required GitHub secrets

**Deliverables**:
- Dependabot configuration (automated dependency updates)
- Branch protection rules documentation
- README updates with CI/CD badges and release info
- Workflow dispatch triggers for manual runs
- End-to-end release flow test plan
- GitHub secrets documentation

**Acceptance**:
- All workflows integrated and tested end-to-end
- Documentation complete and accurate
- Dependency automation configured
- Manual testing procedures documented

**Dependencies**: Requires all user story phases (grid-ea16, grid-a98b, grid-a59f, grid-93ab)

**Status**: Open

**Checkpoint**: CI/CD infrastructure is complete, documented, and production-ready

---

## Dependencies & Execution Order

### Phase Dependencies

```
Setup (grid-158d)
    ↓
Foundational (grid-c778) ← BLOCKS all user stories
    ↓
    ├─→ User Story 1 (grid-ea16) [P1] ← MVP
    ├─→ User Story 4 (grid-a98b) [P2]
    ├─→ User Story 2 (grid-a59f) [P2]
    └─→ User Story 3 (grid-93ab) [P3]
           ↓
    Polish (grid-3ab5) [P2]
```

### Recommended Execution Order

1. **Phase 1: Setup** (grid-158d) - Creates configuration files
2. **Phase 2: Foundational** (grid-c778) - Establishes CI patterns
3. **Phase 3: User Story 1** (grid-ea16) - **MVP SCOPE** - PR testing operational
4. **Phase 4: User Story 4** (grid-a98b) - Protobuf validation (can parallel with US1)
5. **Phase 5: User Story 2** (grid-a59f) - Release automation
6. **Phase 6: User Story 3** (grid-93ab) - Migration testing (optional for MVP)
7. **Phase 7: Polish** (grid-3ab5) - Documentation and integration

### Parallelization Opportunities

After Foundational phase completes, these can proceed in parallel (if staffed):
- User Story 1, 4, and 3 (all add jobs to pr-tests.yml - coordinate to avoid conflicts)
- User Story 2 can proceed independently (separate workflow files)

### Within Each User Story

Tasks within a story follow this general pattern:
1. Workflow scaffolding first
2. Individual jobs can be added in parallel (if different files)
3. Jobs adding to same file must be sequential
4. Documentation tasks can proceed in parallel with implementation

---

## MVP Scope Recommendation

**Minimum Viable Product**: User Story 1 only

Implementing just User Story 1 (Automated PR Testing) provides immediate value:
- ✅ Automated quality gates on all PRs
- ✅ Fast feedback for contributors
- ✅ Prevents broken code from merging
- ✅ Establishes CI/CD foundation

**Next Priority**: User Story 2 (Release Builds)
- Enables automated releases
- Removes manual build/publish steps

**Lower Priority**: User Stories 3 and 4
- Nice-to-have validations
- Can be added incrementally

---

## Implementation Strategy

### MVP-First Approach (Recommended)

1. **Sprint 1**: Setup + Foundational + User Story 1
   - Deliverable: Automated PR testing operational
   - Value: Immediate quality improvement

2. **Sprint 2**: User Story 2
   - Deliverable: Automated release pipeline
   - Value: Zero-manual-step releases

3. **Sprint 3**: User Stories 3, 4 + Polish
   - Deliverable: Complete CI/CD infrastructure
   - Value: Additional validations and documentation

### Full Implementation (All User Stories)

If implementing all stories:
1. Complete Setup + Foundational phases first
2. Implement User Stories in parallel teams OR sequentially by priority
3. Polish phase after desired user stories complete
4. Test end-to-end release flow before production use

---

## Performance Goals

From spec.md success criteria:

- **PR Test Feedback**: <10 minutes (SC-001)
- **Release Build Time**: <20 minutes (SC-002)
- **Cache Effectiveness**: 40%+ reduction vs cold builds (SC-007)

Monitor these via GitHub Actions timing data and optimize caching strategies as needed.

---

## Required GitHub Configuration

**Not automated via workflows** - manual setup required:

1. **Branch Protection Rules** (see grid-a7d1 for details)
   - Require PR reviews
   - Require status checks (all PR test jobs)
   - Require branches up-to-date
   - No force pushes to main

2. **GitHub Secrets** (see grid-e38b for details)
   - `NPM_TOKEN` for npm package publishing

3. **Repository Settings**
   - Allow GitHub Actions to create PRs (for release-please)
   - Configure automatic deletion of head branches

---

> This file is intentionally light and index-only. Implementation data lives in Beads. Update this file only to point humans and agents to canonical query paths and feature references.