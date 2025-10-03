# Feature Specification: State Dependency Management

**Feature Branch**: `002-add-state-dependency`
**Created**: 2025-10-02
**Status**: Draft
**Input**: User description: "Add state dependency management to Grid so users can wire states together on specific Terraform output keys and understand impact when upstream values change. In this feature, a dependency is a directed labeled edge of the form `from_state.output_key` -> `to_state`. Multiple edges between the same two states are allowed (one per output key), but the overall graph must remain a Directed Acyclic Multigraph (no cycles permitted at any time). The system should parse stored tfstate to discover available outputs, let users declare edges by referencing states (via logic-id or GUID) and an output key, and immediately reject any edge that would create a cycle. Each edge carries a status that communicates reconciliation needs: clean (the consumer has observed the current producer value), dirty (the producer's output changed since the consumer last observed it), potentially-stale (the consumer is transitively downstream of at least one dirty edge), and mock (the dependency is declared before the producer output exists, using a provided mock value). When a producer state's output changes, directly connected edges become dirty and all transitive downstream consumers become potentially-stale until they reconcile. "Observed" for a consumer is defined as the moment its state is successfully written via the remote backend after it has planned/applied using the producer outputs in effect; observation is matched value-wise using a fingerprint of each producer output at that time. The feature should support declaring mock outputs so graphs can be assembled ahead of time; once the real output appears in the producer's tfstate, the system replaces the mock and recomputes statuses (transitioning to dirty or clean depending on whether the consumer has observed the real value). Users should be able to: (1) declare and remove dependencies, with cycle prevention on create; (2) list a state's dependencies (what it consumes) and dependents (who consumes it), including edge status and timestamps; (3) search for states by available output keys and find where a given key is consumed; (4) view a topological ordering (layered view) of the graph rooted at any state to understand recommended reconciliation order; and (5) init should be updated to generate/sync the terraform config as a managed locals block (in a file in the consumer state) that materializes referenced upstream outputs under local names (prefixed by state logic_id) for easy use in Terraform, clearly marked as managed to discourage manual edits and idempotent on re-run. The state list view should show quick indicators for each state (e.g., clean if all incoming edges are clean, stale if any incoming edge is dirty, potentially-stale if only transitive upstream is dirty) and a comma separate list of dependency states (set of logic id, without output keys)."

## Execution Flow (main)
```
1. Parse user description from Input
   → If empty: ERROR "No feature description provided"
2. Extract key concepts from description
   → Identify: actors, actions, data, constraints
3. For each unclear aspect:
   → Mark with [NEEDS CLARIFICATION: specific question]
4. Fill User Scenarios & Testing section
   → If no clear user flow: ERROR "Cannot determine user scenarios"
5. Generate Functional Requirements
   → Each requirement must be testable
   → Mark ambiguous requirements
6. Identify Key Entities (if data involved)
7. Run Review Checklist
   → If any [NEEDS CLARIFICATION]: WARN "Spec has uncertainties"
   → If implementation details found: ERROR "Remove tech details"
8. Return: SUCCESS (spec ready for planning)
```

---

## ⚡ Quick Guidelines
- ✅ Focus on WHAT users need and WHY
- ❌ Avoid HOW to implement (no tech stack, APIs, code structure)
- 👥 Written for business stakeholders, not developers

### Section Requirements
- **Mandatory sections**: Must be completed for every feature
- **Optional sections**: Include only when relevant to the feature
- When a section doesn't apply, remove it entirely (don't leave as "N/A")

### For AI Generation
When creating this spec from a user prompt:
1. **Mark all ambiguities**: Use [NEEDS CLARIFICATION: specific question] for any assumption you'd need to make
2. **Don't guess**: If the prompt doesn't specify something (e.g., "login system" without auth method), mark it
3. **Think like a tester**: Every vague requirement should fail the "testable and unambiguous" checklist item
4. **Common underspecified areas**:
   - User types and permissions
   - Data retention/deletion policies
   - Performance targets and scale
   - Error handling behaviors
   - Integration requirements
   - Security/compliance needs

---

## Clarifications

### Session 2025-10-02
- Q: How should state-level statuses be computed and persisted? → A: State Status MUST always be computed on demand from current edges, producer output fingerprints, and edge state (observed, dirty, ...). Do NOT persist state-level statuses in database in Phase 1. Derived function: any incoming dirty → stale; only transitive dirty → potentially-stale; else clean.
- Q: How should dependency removal be handled? → A: Allow immediate removal and recompute state statuses on demand.
- Q: What happens when a producer output key is removed? → A: Mark the edge as "missing output" and next locals generation will re-render without that edge (mocked edges should be flagged similarly but are included in locals block).
- Q: How should duplicate dependency declarations be handled? → A: Treat as idempotent: surface the existing edge, do not change edge state.
- Q: How should transitive potentially-stale statuses be cleared? → A: Automated on best effort: each state write event (Terraform PUT, gridctl tfstate push) triggers background job "UpdateEdges" to recompute statuses.
- Q: What happens when init is run multiple times with changing dependencies? → A: Deterministically overwrite only the managed locals block. gridctl owns this block; manual edits are reverted (idempotent overwrite of clearly marked managed section).
- Q: How should very large graphs be handled in Phase 1? → A: Return in full with no pagination or slicing. Future phases will add filtering and pagination.
- Q: What happens when a state with active dependencies is deleted? → A: State deletion not supported in Phase 1. Future: allow deletion only if state contents empty and no edges connected (edges must be deleted first).
- Q: What timezone should be used for timestamps? → A: UTC (ISO 8601 format with timezone marker).
- Q: What marking convention should be used for managed locals block? → A: HCL comment header: `# BEGIN GRID MANAGED BLOCK - DO NOT EDIT` and footer: `# END GRID MANAGED BLOCK`.
- Q: What file name and location for managed locals block? → A: File named `grid_dependencies.tf` in the consumer state directory root.
- Q: How should users control generated local variable names in grid_dependencies.tf? → A: Support optional `to_input_name` field on dependency edges. Default naming: `<state_slug>_<output_slug>`. With override: use `to_input_name` directly. Uniqueness constraint: composite key (to_state, to_input_name (if provided, default is from_state.logic_id + from_output)) must be unique across all edges to that consumer state.
- Q: How should edge to_input_name field be managed if from_state logic_id is changed? → A: It remains unchanged until user explicitly updates the to_input_name or deletes/recreates the edge.

---

## User Scenarios & Testing *(mandatory)*

### Primary User Story
As a Grid user managing multiple Terraform states, I need to wire states together so that outputs from one state can be consumed by others, while understanding when upstream changes require downstream states to be reconciled. The system should prevent circular dependencies, track the synchronization status of each dependency relationship, and provide visibility into the impact of changes throughout the dependency graph.

### Acceptance Scenarios
1. **Given** I have two states (A and B) where A produces an output "vpc_id", **When** I declare a dependency from A.vpc_id to B, **Then** the system creates the dependency edge and B can reference the vpc_id value in its Terraform configuration
2. **Given** I have states A, B, and C with dependencies A→B and B→C, **When** I attempt to create a dependency C→A, **Then** the system rejects the operation because it would create a cycle
3. **Given** I have a dependency from A.subnet_id to B that is currently clean, **When** the subnet_id output value in state A changes, **Then** the dependency edge is marked as dirty and state B is marked as needing reconciliation
4. **Given** I want to build a dependency graph before states are fully configured, **When** I declare a dependency on an output that doesn't exist yet and provide a mock value, **Then** the system creates the edge with mock status and state B can use the mock value until the real output appears
5. **Given** I have a complex dependency graph, **When** I request a topological ordering rooted at any state, **Then** the system shows me a layered view indicating the recommended reconciliation order
6. **Given** I have dependencies declared for a state, **When** I run init on that state, **Then** the system generates a managed locals block containing all upstream output values prefixed by their state logic_id
7. **Given** I want to understand my state's dependencies, **When** I list my state's information, **Then** I see all incoming dependencies (what I consume), outgoing dependents (who consumes my outputs), their statuses, and timestamps
8. **Given** I need to find where a specific output is being used, **When** I search for states consuming a particular output key, **Then** the system shows me all dependency edges referencing that key
9. **Given** I have a dependency from A.output_x to B with a mock value, **When** state A is applied and output_x appears in its tfstate, **Then** the system replaces the mock with the real value and updates the edge status based on whether B has observed the real value
10. **Given** I have multiple dependencies from landing_zone state to cluster state, **When** I declare a dependency with `to_input_name` override (e.g., landing_zone.vpc_id → cluster with to_input_name="network_vpc_id"), **Then** the generated `grid_dependencies.tf` uses `network_vpc_id` as the local variable name instead of the default `landing_zone_vpc_id`
11. **Given** I have an existing dependency with a `to_input_name` override, **When** I run deps sync to regenerate the managed locals block, **Then** the system validates uniqueness of `to_input_name` across all edges to the consumer state and rejects conflicts

### Edge Cases
- **Dependency removal**: System allows immediate removal and recomputes statuses on demand
- **State with mixed incoming edge statuses**: State status is computed at runtime from edges (any incoming dirty → stale; only transitive dirty → potentially-stale; else clean)
- **Missing producer output key**: Edge marked as "missing output" and blocks locals generation for that edge
- **Multiple mock dependencies transitioning**: Each edge transitions independently when its corresponding real output appears
- **Duplicate dependency declaration**: Treated as idempotent operation; system surfaces existing edge without changing state
- **Clearing transitive stale statuses**: Automated via background "UpdateEdges" job triggered on each state write event (Terraform PUT, gridctl tfstate push)
- **Repeated init with changing dependencies**: Managed locals block in `grid_dependencies.tf` deterministically overwritten; manual edits reverted
- **Very large dependency graphs**: Phase 1 returns full graph without pagination; future phases will add filtering/pagination
- **State deletion with dependencies**: Not supported in Phase 1; future requires empty state contents and zero connected edges
- **to_input_name conflicts**: System rejects dependency declaration if `to_input_name` conflicts with existing edge's `to_input_name` for the same consumer state (composite uniqueness on to_state (guid) + to_input_name)
- **to_input_name validation**: System validates `to_input_name` follows slug rules (lowercase, [a-z0-9_-]) when provided

## Requirements *(mandatory)*

### Functional Requirements

#### Dependency Declaration and Management
- **FR-001**: System MUST allow users to declare a dependency from a producer state's output key to a consumer state, referencing states by either logic-id or GUID
- **FR-002**: System MUST allow multiple dependency edges between the same two states, one per distinct output key
- **FR-003**: System MUST validate that adding a new dependency edge will not create a cycle in the dependency graph
- **FR-004**: System MUST reject any dependency declaration that would create a cycle
- **FR-005**: System MUST allow users to remove existing dependency edges immediately and recompute statuses on demand
- **FR-006**: System MUST allow users to declare mock dependencies with a user-provided mock value when the producer output does not yet exist
- **FR-007**: System MUST treat duplicate dependency declarations as idempotent operations, surfacing the existing edge without changing its state
- **FR-008**: System MUST allow users to optionally specify a `to_input_name` when declaring a dependency to control the generated local variable name in `grid_dependencies.tf`; if not provided, default is generated using from_state.logic_id + from_output and converted to slug format
- **FR-009**: System MUST validate that `to_input_name` follows slug rules (lowercase, [a-z0-9_-] with collapsed repeated underscores/hyphens) and enforce uniqueness within a consumer state (composite key: to_state + to_input_name), rejecting dependency declarations that conflict with existing edges

#### Output Discovery and Parsing
- **FR-012**: System MUST parse stored Terraform state files to discover available output keys and their current values
- **FR-013**: System MUST extract output values from tfstate to enable fingerprinting for observation tracking
- **FR-014**: System MUST detect when an output key appears in a producer state's tfstate that was previously declared as a mock dependency
- **FR-015**: System MUST replace mock values with real values once the output appears in the producer's tfstate, transitioning edge status independently per edge
- **FR-016**: System MUST mark an edge as "missing-output" when a producer output key referenced in a dependency is removed from the tfstate; the edge MUST be retained (not deleted) and locals generation MUST skip that edge

#### Edge Status Tracking
- **FR-017**: System MUST track the status of each dependency edge using one of six states: pending (initial state after creation, producer output not yet observed by consumer), clean (consumer observed current producer value), dirty (producer value changed after consumer observation), potentially-stale (transitive downstream of dirty edge), mock (producer output not yet created, using mock value), or missing-output (producer output removed from tfstate)
- **FR-018**: System MUST mark a dependency edge as clean when the consumer has observed the current producer output value
- **FR-019**: System MUST mark a dependency edge as dirty when the producer's output value changes after the consumer last observed it
- **FR-020**: System MUST mark a dependency edge as potentially-stale when the consumer is transitively downstream of at least one dirty edge (but its direct dependencies are clean)
- **FR-021**: System MUST mark a dependency edge as mock when it references a producer output that does not yet exist and is using a provided mock value
- **FR-022**: System MUST record timestamps for each edge status change in UTC (ISO 8601 format with timezone marker)
- **FR-023**: System MUST compute a fingerprint of each producer output value to track when consumers have observed specific values
- **FR-024**: System MUST update edge statuses automatically via background EdgeUpdateJob triggered on each state write event (Terraform PUT, gridctl tfstate push); the job operates on best-effort basis, logging errors without propagating failures to the state write operation
- **FR-025**: System MUST define "observed" as the moment when the consumer state is successfully written via the remote backend after planning/applying with the producer outputs in effect
- **FR-026**: System MUST match observation value-wise using the fingerprint of each producer output at observation time
- **FR-027**: System MUST compute state-level status on demand from current edges, producer output fingerprints, and observation records (do NOT persist in database); state status derivation: any incoming dirty edge → stale, only transitive upstream dirty (propagated via clean intermediate states) → potentially-stale, else → clean

#### Querying and Visibility
- **FR-028**: System MUST allow users to list all dependencies for a given state (what it consumes), including edge status and timestamps
- **FR-029**: System MUST allow users to list all dependents for a given state (who consumes its outputs), including edge status and timestamps
- **FR-030**: System MUST allow users to search for states by available output keys
- **FR-031**: System MUST allow users to find all dependency edges that consume a given output key
- **FR-032**: System MUST provide a topological ordering (layered view) of the dependency graph rooted at any state
- **FR-033**: System MUST indicate the recommended reconciliation order based on the topological ordering
- **FR-034**: System MUST display quick status indicators for each state in the state list view: clean if all incoming edges are clean, stale if any incoming edge is dirty, potentially-stale if only transitive upstream edges are dirty
- **FR-035**: System MUST display a comma-separated list of dependency states (showing unique logic_id values without output keys) in the state list view
- **FR-036**: System MUST return full dependency graphs in Phase 1 without pagination or slicing (future phases will add filtering and pagination)

#### Integration with Init Command
- **FR-037**: System MUST update the init command to generate a managed locals block in the consumer state's Terraform configuration
- **FR-038**: System MUST materialize referenced upstream outputs in the locals block using local names derived from `to_input_name` if specified, otherwise using pattern `<state_slug>_<output_slug>` (generated at dependency declaration time per FR-009 slug rules)
- **FR-039**: (REMOVED - slug generation rules consolidated in FR-009)
- **FR-040**: System MUST mark the generated locals block with HCL comment header `# BEGIN GRID MANAGED BLOCK - DO NOT EDIT` and footer `# END GRID MANAGED BLOCK`
- **FR-041**: System MUST make the init command idempotent, deterministically overwriting only the managed locals block on re-run and reverting any manual edits
- **FR-042**: System MUST write the managed locals block to a file named `grid_dependencies.tf` in the consumer state directory root; if template rendering fails or directory is not writable, the CLI MUST display the failure cause and leave any existing file unchanged
- **FR-043**: System MUST skip locals generation for edges marked as "missing-output" (but include mock edges)
- **FR-044**: System MUST generate `terraform_remote_state` data sources for each unique producer state, including Grid backend configuration with immutable GUID-based endpoints

#### Graph Constraints
- **FR-045**: System MUST maintain the dependency graph as a Directed Acyclic Multigraph (DAG) at all times
- **FR-046**: System MUST support multiple edges between the same pair of states as long as they reference different output keys
- **FR-047**: System MUST prevent cycles at the graph level, not just at the edge level (A→B and B→A with different keys is still a cycle)

#### State Lifecycle
- **FR-048**: System MUST NOT support state deletion in Phase 1
- **FR-049**: Future state deletion MUST only be allowed when state contents are empty and no dependency edges are connected (edges must be deleted first)

### Key Entities *(include if feature involves data)*
- **Dependency Edge**: A directed relationship from a producer state's specific output key to a consumer state. Contains: producer state reference (logic-id or GUID), output key name, consumer state reference, optional `to_input_name` for overriding generated local variable name (must be unique per consumer state), status (clean/dirty/potentially-stale/mock/missing-output), timestamps in UTC ISO 8601 format, and optionally a mock value and observation fingerprint.
- **State Output**: An output value declared in a Terraform state file. Contains: the output key name, current value, and value fingerprint for tracking observation.
- **State Status**: The overall synchronization health of a state based on its incoming dependency edges. Always computed on demand (never persisted). Derivation: any incoming dirty → stale; only transitive dirty → potentially-stale; else clean.
- **Observation Record**: A record of when a consumer state has observed a specific value of a producer output. Contains: consumer state reference, producer state reference, output key, observed value fingerprint, and observation timestamp in UTC.
- **Mock Value**: A placeholder value provided when declaring a dependency on an output that does not yet exist in the producer state. Transitions independently per edge when real output appears.
- **EdgeUpdateJob**: A background job triggered on each state write event (Terraform PUT, gridctl tfstate push) to recompute edge statuses and handle mock-to-real transitions on best-effort basis.

---

## Review & Acceptance Checklist
*GATE: Automated checks run during main() execution*

### Content Quality
- [x] No implementation details (languages, frameworks, APIs)
- [x] Focused on user value and business needs
- [x] Written for non-technical stakeholders
- [x] All mandatory sections completed

### Requirement Completeness
- [x] No [NEEDS CLARIFICATION] markers remain
- [x] Requirements are testable and unambiguous
- [x] Success criteria are measurable
- [x] Scope is clearly bounded
- [x] Dependencies and assumptions identified

---

## Execution Status
*Updated by main() during processing*

- [x] User description parsed
- [x] Key concepts extracted
- [x] Ambiguities marked
- [x] User scenarios defined
- [x] Requirements generated
- [x] Entities identified
- [x] Review checklist passed
- [x] All clarifications resolved (Session 2025-10-02)

---
