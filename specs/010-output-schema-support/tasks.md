---
description: "Task list index for Output Schema Support - Phase 2"
---

# Tasks Index: Output Schema Support - Phase 2

Beads Issue Graph Index into the tasks and phases for this feature implementation.
This index does **not contain tasks directly**—those are fully managed through Beads CLI and MCP agent APIs.

## Feature Tracking

* **Beads Epic ID**: `grid-c64a`
* **User Stories Source**: `specs/010-output-schema-support/spec.md`
* **Research Inputs**: `specs/010-output-schema-support/research.md`
* **Planning Details**: `specs/010-output-schema-support/plan.md`
* **Data Model**: `specs/010-output-schema-support/data-model.md`
* **Contract Definitions**: `specs/010-output-schema-support/contracts/`

## Beads Query Hints

Use the `bd` CLI to query and manipulate the issue graph:

> NOTE: You MUST view comments when working on tasks, as implementation details and changes are often documented there.

```bash
# Find all open tasks for this feature
bd list --label spec:010-output-schema-support --status open --limit 20

# See full task tree for the feature
bd dep tree --reverse grid-c64a 

# Find ready tasks to implement
bd ready --limit 5

# See dependencies for a specific issue
bd dep tree grid-c64a

# View issues by component
bd list --label 'component:gridapi' --label 'spec:010-output-schema-support'

# Show all phases
bd list --type feature --label 'spec:010-output-schema-support'


# View issues by phase
bd list --label 'phase:prereq' --label 'spec:010-output-schema-support'
bd list --label 'phase:edge-status' --label 'spec:010-output-schema-support'
bd list --label 'phase:validation' --label 'spec:010-output-schema-support'
bd list --label 'phase:inference' --label 'spec:010-output-schema-support'
bd list --label 'phase:webapp' --label 'spec:010-output-schema-support'

# Show progress statistics
bd stats

# Filter bd ready tasks only by label
# In table format
bd ready --json | jq -r '.[] | select(.labels // [] | contains(["spec:010-output-schema-support"])) | select (.issue_type == "task")| [.id, .title] | @tsv'

# Full JSON details
bd ready --json --limit 5 | jq '.[] | select(.labels // [] | contains(["spec:010-output-schema-support"]))'

# See implementation details
bd show grid-d219
# you MUST view comments for any modifications to the plan
bd comments grid-d219
```

## Phases Structure

* **Epic**: `grid-c64a` (Output Schema Support - Phase 2)
* **Phase 2: Prerequisites**: `grid-5d3e` (Fix Phase 1 bugs) - ✅ **COMPLETED** (2025-11-25)
* **Phase 2A: Schema Inference**: `grid-daf8` (US5) - ✅ **COMPLETED** (2025-11-26)
* **Phase 2B: Schema Validation**: `grid-093b` (US6) - ✅ **COMPLETED** (2025-11-27)
* **Phase 2C: Edge Status Updates**: `grid-e70b` (US6) - ✅ **COMPLETED** (2025-11-27)
* **Phase 3: Webapp UI**: `grid-bfd6` (US7) - 🔄 **In Progress**

## Implementation Strategy

1. **Phase 2: Prerequisites** (P1) - ✅ **COMPLETED**
   - Fix schema preservation bug in `UpsertOutputs`
   - All Phase 1 integration tests passing

2. **Phase 2A: Schema Inference** (P2) - ✅ **COMPLETED** (2025-11-26)
   - ✅ Database migration (schema_source + validation columns)
   - ✅ Inference service using `jsonschema-infer v0.1.2`
   - ✅ Repository interface extensions (SetOutputSchemaWithSource, GetOutputsWithoutSchema)
   - ✅ State upload workflow integration (fire-and-forget async)
   - ✅ Proto/SDK updates (schema_source field)
   - ✅ 10 integration tests passing (FR-019 through FR-028)
   - **Bug Fixed**: JSON double-encoding in inferrer.go
   - **Tasks Closed**: grid-5d22, grid-9461, grid-befd, grid-3f9b, grid-1845, grid-d219, grid-aeba, grid-4ab5, grid-1049

3. **Phase 2B: Schema Validation** (P2) - ✅ **COMPLETED** (2025-11-27)
   - Implement validation using `santhosh-tekuri/jsonschema/v6`
   - Add background validation job
   - TDD approach: Write integration tests first
   - **Ready Tasks**: grid-c833, grid-bef1, grid-1c39

4. **Phase 2C: Edge Status Updates** (P2)
   - Add `schema-invalid` edge status
   - Update edges based on validation results

5. **Phase 3: Webapp UI** (P3)
   - Update `OutputKey` and `EdgeStatus` models
   - Add "Outputs" tab to DetailView
   - Display validation status and schema preview

## Status Tracking

Status is tracked only in Beads. Use `bd ready`, `bd blocked`, `bd stats` to query progress.
YOU MUST view comments on tasks for implementation details and changes.

Tip: to run integration tests use:

```bash
# MAKE SURE YOU ARE IN GRIDAPI ROOT DIRECTORY
make db-reset && make db-migrate 
make test-integration 2>&1 | tee /tmp/integration-test-output.txt | grep -E "(^=== RUN|^--- PASS|^--- FAIL|PASS:|FAIL:)"
```
