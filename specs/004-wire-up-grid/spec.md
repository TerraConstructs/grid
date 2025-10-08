# Feature Specification: Live Dashboard Integration

**Feature Branch**: `004-wire-up-grid`
**Created**: 2025-10-06
**Status**: Draft
**Input**: User description: "Wire up Grid dashboard PoC (under ./webapp) to live `gridapi` data so operators can browse real Terraform state topology instead of mock fixtures. Implement read-only facades in js/sdk that wrap the protobuf RPCs for fetching live data, listing states and edges for tabular views, retrieving individual state details (dependencies, dependents, outputs, and mock edges info), and searching by state identifier or output key; each facade should enforce the API contracts, map enums/status badges to the dashboard vocabulary, and expose loading/error signals for the UI. Update the /webapp React app to replace mockApi usage with these facades, centralize data refresh (initial load, manual refresh button, polling hook), and ensure the graph view, list view, and detail drawer all render live values, propagate status colors, and handle empty or failure states gracefully. The spec should cover how the dashboard consumes topological ordering, aggregates cleanliness indicators, highlights dirty/potentially-stale/mock edges, and surfaces reconciliation guidance exactly as defined by the API, guaranteeing the PoC visuals now reflect production truth while remaining read-only."

## Execution Flow (main)
```
1. Parse user description from Input
   → SUCCESS: Feature is dashboard integration with live API data
2. Extract key concepts from description
   → Actors: Operators viewing dashboard
   → Actions: Browse topology, view state details, refresh data
   → Data: States, edges, outputs, status indicators
   → Constraints: Read-only, live API data, existing dashboard UI
3. For each unclear aspect:
   → [Marked in requirements below]
4. Fill User Scenarios & Testing section
   → SUCCESS: Clear user flows identified
5. Generate Functional Requirements
   → SUCCESS: Requirements are testable
6. Identify Key Entities
   → SUCCESS: Entities identified
7. Run Review Checklist
   → WARN: Spec has uncertainties (see NEEDS CLARIFICATION markers)
8. Return: SUCCESS (spec ready for planning with clarifications needed)
```

---

## ⚡ Quick Guidelines
- ✅ Focus on WHAT users need and WHY
- ❌ Avoid HOW to implement (no tech stack, APIs, code structure)
- 👥 Written for business stakeholders, not developers

---

## Clarifications

### Session 2025-10-06
- Q: Looking at the existing webapp code, the graph view already uses specific edge status colors (green=clean, orange=dirty/stale, blue=pending). For consistency, what colors should "mock" and "missing-output" edge statuses use? → A: Purple for "mock", Red for "missing-output"
- Q: What happens when topological ordering cannot be computed (cycles exist)? → A: Database has cycle detection to reject edge creations that would create cycles, so this should not happen
- Q: What happens during a refresh if some API calls succeed and others fail? → A: Graceful fallback behavior
- Q: The webapp currently has no automatic refresh. For this PoC focused on YAGNI/KISS, should automatic polling be included? → A: Deferred (manual refresh only for PoC)
- Q: The existing webapp PoC has no search UI implemented. Given YAGNI/KISS focus, should search capabilities be included? → A: Deferred (not needed for initial PoC)
- Q: The existing webapp calls mockApi.getAllEdges() to fetch all edges at once. The real Grid API doesn't have a "list all edges" RPC. Should the dashboard aggregate edges client-side, hide global edge list, or request new API RPC? → A: Request new "ListAllEdges" RPC be added to API
- Q: Should the dashboard call GetStateStatus RPC for each state to get live status computation, or rely on computed_status field from ListStates? → A: Use computed_status from ListStates (single call)
- Q: The API server uses Chi router with Connect RPC handlers. For this alpha/PoC running on localhost, should it use h2c (HTTP/2 Cleartext), HTTP/1.1, or TLS + HTTP/2? → A: h2c (HTTP/2 Cleartext)

---

## User Scenarios & Testing *(mandatory)*

### Primary User Story
Operators need to monitor and understand the dependency topology of Terraform states in real-time. They access the Grid dashboard to visualize state relationships, identify stale or dirty dependencies, and understand which states need reconciliation. The dashboard displays live data from the running Grid API server, replacing previous mock fixtures with actual production state.

### Acceptance Scenarios

1. **Given** the Grid API server is running with multiple states and dependencies, **When** an operator opens the dashboard, **Then** the system displays current states, edges, and their statuses from live API data

2. **Given** a state has dirty dependency edges, **When** an operator views the graph visualization, **Then** those edges are highlighted with appropriate visual indicators (color/style) showing they need attention

3. **Given** the operator is viewing state topology, **When** they click on a specific state node, **Then** the system displays detailed information including dependencies, dependents, and available outputs

4. **Given** the operator wants current data, **When** they trigger a manual refresh, **Then** the system fetches latest state from the API and updates all views accordingly

5. **Given** the operator views the list of all states, **When** states are sorted by topological order, **Then** states appear in dependency-aware layers (foundations first, consumers last)

6. **Given** the operator views state cleanliness indicators, **When** aggregating edge statuses, **Then** the system shows counts of clean, dirty, pending, and mock edges for each state

7. (deferred) ~~**Given** a state has mock dependency values, **When** viewing edge details, **Then** the system clearly indicates mock status and displays reconciliation guidance~~

### Edge Cases

- What happens when the API server is unreachable or returns errors?
  - System MUST display error state gracefully, retain last known good data if available, and provide retry mechanism

- How does system handle empty states (no dependencies/dependents)?
  - System MUST display empty state indicators clearly (e.g., "No dependencies" message, not blank areas)

- What happens when a state has no outputs available yet (new state)?
  - System MUST show empty outputs list with explanatory message (e.g., "No Terraform outputs available")

- How does system handle sensitive outputs?
  - System MUST visually mark sensitive outputs (flag/icon) but NOT display their values

- What happens when topological ordering cannot be computed (cycles exist)?
  - System can rely on API validation; database has cycle detection that rejects edge creations, so cycles should not occur in production data

- How should system handle states that are locked during viewing?
  - System MUST display lock status indicator showing state is currently in use

- deferred in PoC ~~ What happens during a refresh if some API calls succeed and others fail?~~
  - ~~System MUST use graceful fallback behavior, displaying partial data with warning indicators for failed components and allowing retry of failed requests~~

## Requirements *(mandatory)*

### Functional Requirements

**Data Integration**
- **FR-001**: System MUST fetch state listing from live API server, replacing all mock data fixtures
- **FR-002**: System MUST retrieve all dependency edges from live API for global list and graph views
- **FR-003**: System MUST fetch individual state details including dependencies, dependents, outputs, and backend configuration

Note: Search capabilities (by output key, by state identifier) are deferred for this PoC

**API Dependencies**: This feature requires a new `ListAllEdges` RPC to be added to the Grid API for retrieving all dependency edges in a single call (used by list view and graph view). For this alpha/PoC deployment, the API server uses h2c (HTTP/2 Cleartext) for localhost without TLS complexity

**Status Visualization**
- **FR-006**: System MUST display edge status using consistent visual vocabulary (colors/styles): green for clean, orange/yellow for dirty, blue for pending, orange/yellow for potentially-stale, purple for mock, red for missing-output
- **FR-007**: System MUST aggregate and display cleanliness indicators showing counts of clean, dirty, pending edges per state
- **FR-008**: System MUST visually distinguish mock edges that use placeholder values from real dependency edges
- **FR-009**: System MUST indicate missing outputs on edges where producer output doesn't exist yet (or has been deleted)
- **FR-010**: System MUST show computed state status (clean, stale, potentially-stale) using the computed_status field from ListStates response

**Topology Display**
- **FR-011**: System MUST render topological ordering with states organized in dependency layers
- **FR-012**: System MUST display dependency direction clearly showing producer→consumer relationships
- **FR-013**: System MUST support graph view visualization showing states as nodes and dependencies as directed edges
- **FR-014**: System MUST support list view showing all states and edges in tabular format

**State Details**
- **FR-015**: System MUST display comprehensive state information when user selects a state, including GUID, logic-id, creation/update timestamps
- **FR-016**: System MUST list all incoming dependencies (edges where state is consumer) for selected state
- **FR-017**: System MUST list all outgoing dependents (edges where state is producer) for selected state
- **FR-018**: System MUST display available output keys from state's Terraform configuration
- **FR-019**: System MUST mark sensitive outputs with visual indicator and NOT display their values
- **FR-020**: System MUST display backend configuration URLs (address, lock_address, unlock_address) for selected state

**Data Refresh**
- **FR-021**: System MUST load data from API on initial dashboard access
- **FR-022**: System MUST provide manual refresh capability allowing operators to fetch current data on-demand
- **FR-023**: System MUST indicate loading state during data refresh operations
- **FR-024**: System MUST preserve user's current view context during refresh (e.g., don't lose selected state)

Note: Automatic polling is deferred for this PoC (manual refresh only)

**Error Handling**
- **FR-026**: System MUST display error messages when API requests fail
- **FR-027** *(Deferred for a future milestone)*: System MUST handle network connectivity issues gracefully without crashing
- **FR-028**: System MUST allow retry of failed requests
- **FR-029**: System MUST display empty states clearly when no data exists (e.g., no states, no edges)
- **FR-030** *(Deferred for a future milestone)*: System MUST handle API timeout and partial failure scenarios with graceful fallback, displaying partial data with warning indicators for failed components

**Reconciliation Guidance** *(Deferred until UX is defined)*
- **FR-031** *(Deferred)*: System MUST surface reconciliation guidance for states with dirty or stale dependencies
- **FR-032** *(Deferred)*: System MUST indicate when mock values need to be replaced with real outputs
- **FR-033** *(Deferred)*: System MUST show digest mismatches between input and output values on dirty edges
- **FR-034** *(Deferred)*: System MUST display temporal information (last_in_at, last_out_at) showing when edge values were last synchronized

**Read-Only Constraints**
- **FR-035**: System MUST NOT provide capability to create, modify, or delete states
- **FR-036**: System MUST NOT provide capability to add or remove dependency edges
- **FR-037**: System MUST NOT expose state lock/unlock operations
- **FR-038**: System MUST display all information in read-only mode (view/browse only)

**API Contract Compliance**
- **FR-039**: System MUST correctly interpret all protobuf message types from StateService API
- **FR-040**: System MUST map API status enums to dashboard vocabulary consistently
- **FR-041**: System MUST handle optional fields in API responses (e.g., mock_value_json, to_input_name)
- **FR-042**: System MUST correctly parse timestamp fields for display (in browser local time)

### Non-Functional Requirements

- Performance benchmarking is deferred for this PoC/alpha dashboard and will be defined in a later iteration.

### Key Entities

- **State**: Represents a Terraform remote state with unique GUID (immutable, client-generated UUIDv7) and logic-id (mutable, user-friendly name). Contains metadata (creation/update timestamps, size, lock status), backend configuration (HTTP URLs), computed status (clean/stale/potentially-stale), and collections of dependencies and dependents.

- **Dependency Edge**: Directed relationship from producer state output to consumer state input. Contains producer reference (from_guid, from_logic_id, from_output), consumer reference (to_guid, to_logic_id, to_input_name), status indicator (pending/clean/dirty/potentially-stale/mock/missing-output), digest values for change detection (in_digest, out_digest), optional mock value, and temporal tracking (last_in_at, last_out_at, created_at, updated_at).

- **Output Key**: Named output from Terraform state JSON with sensitivity flag. Represents available values that can be consumed by dependent states. Output values themselves are NOT exposed for security/size reasons, only the key names and sensitive indicators.

- **Topological Layer**: Grouping of states at same dependency depth in the graph. Layer 0 contains root states with no dependencies, each subsequent layer contains states that depend on previous layers. Used for visual organization and understanding dependency order.

- **Status Indicator**: Computed cleanliness metric for state based on incoming edges. Aggregates counts of clean (in-sync), dirty (out-of-sync), pending (not yet synced), and unknown edges. Determines overall state status for operator decision-making.

- **Backend Configuration**: Set of HTTP endpoints for Terraform HTTP backend protocol, including main state address, lock address, and unlock address. Mounted on API server base URL under `/tfstate/{guid}` pattern.

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

---

