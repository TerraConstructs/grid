# Feature Specification: State Labels

**Feature Branch**: `005-add-state-dimensions`
**Created**: 2025-10-08
**Updated**: 2025-10-09
**Status**: Draft (Scope Reduced)
**Input**: User description: "Add state labels to extend the Grid Terraform state management framework. Support user-defined key/value labels attached to states for filtering, querying, and future RBAC controls. Users must be able to provide labels at state creation (via repeated CLI flags `--label`) and manage them afterward (new commands to update/delete labels, prefix key with - to remove). The (readonly) WebApp will include a dedicated section in the DetailView to display all labels, and all state list/info protocols must include the label metadata so SDKs ensure CLIs and WebApps can display and filter by these labels.

Label management must support validation against a lightweight server-side policy using regex patterns, allowed-value enums per key, and size/count limits. The server validates submitted key/value pairs on write and provides clear error messages. The policy can be updated via API/CLI and provides enum lists for UI pickers.

Filtering uses HashiCorp's go-bexpr grammar for in-memory evaluation, supporting compound boolean expressions. At 100-500 state scale, in-memory filtering performs adequately without requiring complex denormalization or facet projection."

## Execution Flow (main)
```
1. Parse user description from Input
   → SUCCESS: Captured need for state labels with policy-based validation and filtering
2. Extract key concepts from description
   → Actors: CLI users, API consumers, web dashboard viewers
   → Actions: Create/update labels, validate against policy, display/filter using bexpr
   → Data: State metadata, label key/value pairs (typed), label policy (enums + constraints)
   → Constraints: Read-heavy workloads, in-memory filtering for ~500 states, no EAV complexity
3. Scope reduction applied
   → Terminology: "labels" (not "tags" or "dimensions")
   → Storage: Single JSON column on states table (not EAV)
   → Validation: Lightweight policy (regex + enum maps, not JSON Schema)
   → Filtering: go-bexpr in-memory (not SQL query translation)
   → Migration: Simple (no existing deployments to migrate)
4. Define User Scenarios & Testing
   → SUCCESS: Core scenarios covering create, validate, update, display, and filter flows
5. Generate Functional Requirements
   → SUCCESS: Requirements simplified for label lifecycle, policy validation, and bexpr filtering
6. Identify Key Entities
   → SUCCESS: State, Label (typed value), Label Policy
7. Review checklist
   → All clarifications resolved via scope reduction
8. Return spec
   → SUCCESS: Document ready for implementation
```

---

## ⚡ Quick Guidelines
- ✅ Focus on WHAT users need and WHY
- ❌ Avoid HOW to implement (no tech stack, APIs, code structure)
- 👥 Written for business stakeholders, not developers

---

## Clarifications

### Session 2025-10-08 (Original)
- Q: What length limits should we enforce for label keys and values? → A: Keys ≤32 chars; values ≤256 chars.
- Q: How should we treat label values? → A: Allow strings, numeric, and boolean values.
- Q: How many labels may a single state hold? → A: Hard cap at 32 labels per state.
- Q: How should validation run when users modify labels? → A: Always synchronous on each write.

### Session 2025-10-09 (Scope Reduction)
- Q: Should we use EAV or JSON column for storage? → A: JSON column (JSONB for Postgres, TEXT for SQLite) - simpler, adequate for scale.
- Q: Should we use JSON Schema for validation? → A: No - use lightweight policy with regex + enum maps (simpler, no external deps).
- Q: How should filtering work? → A: Use go-bexpr for in-memory filtering with full boolean expression support.
- Q: Do we need facet promotion/projection? → A: No - deferred until scale exceeds 1000 states or latency requirements tighten.
- Q: What terminology should we use? → A: "labels" for the key/value map, "policy" for validation rules.
- Q: Do we need audit logs or compliance tracking? → A: Simplified - policy updates tracked, no complex compliance workflow needed yet.
- Q: Should bexpr grammar allow dots/dashes in keys? → A: Document identifier constraints; recommend underscore-only keys for bexpr compatibility.

### Session 2025-10-09 (Grammar Research)
- Q: Which special characters are safe in label keys for bexpr filtering? → A: go-bexpr default Identifier grammar (bexpr-grammar.peg:186) permits `[a-zA-Z][a-zA-Z0-9_/]*` only. Hyphens, dots, colons require quoted JSON-pointer selector syntax (bexpr-grammar.peg:182 `[\pL\pN-_.~:|]+`). To avoid quoting complexity, label keys should match `[a-z][a-z0-9_/]*` (lowercase + underscore + forward-slash). Label values have no character restrictions.

### Pending Clarifications
- None.

### Working Assumptions
- Policy validation runs synchronously during label write operations for immediate feedback.
- Until RBAC is implemented, any user may update the label policy.
- In-memory bexpr filtering is acceptable for 100-500 state deployments (<50ms p99).
- No existing Grid deployments exist, so migration complexity is minimal (single migration creates all needed schema).

---

## User Scenarios & Testing *(mandatory)*

### Primary User Story
As an infrastructure operator managing Terraform states with Grid, I need to attach and maintain descriptive labels on each state so I can organize environments, filter using boolean expressions, and prepare for future access controls.

### Acceptance Scenarios

1. **Given** I run `gridctl state create` with multiple `--label key=value` flags, **When** the state is created successfully, **Then** the system stores each label and returns them in the creation response and subsequent list/detail views.
2. **Given** a label policy defines allowed values for the "env" key, **When** I submit a label update that violates the policy (e.g., `env=invalid`), **Then** the system blocks the change and presents clear validation errors listing allowed values.
3. **Given** an existing state has labels, **When** I run `gridctl state set` with both additions (`--label foo=bar`) and removals (`--label -legacy`), **Then** the system applies the requested changes atomically and shows the resulting label set.
4. **Given** I view a state in the web dashboard detail pane, **When** the page loads, **Then** I see a dedicated labels section listing all current labels in a readable, copy-friendly format.
5. **Given** I request a filtered state list using a bexpr filter `env in ["staging","prod"] && team == "platform"`, **When** the query executes, **Then** the system returns matching states using in-memory filtering.
6. **Given** I request a filtered state list using simple equality `--label env=prod`, **When** the query executes, **Then** the CLI converts this to a bexpr filter and returns matching states.
7. **Given** I update the label policy to add a new allowed key or extend an enum, **When** I retrieve the policy, **Then** I see the updated constraints and can use them in subsequent label operations.
8. **Given** a policy change leaves some states as non-compliant, **When** I may run the compliance report subcommand, **Then** I receive a list of affected states with the rule violations that require manual remediation.

### Edge Cases
- Submitting duplicate `--label` flags for the same key within a single command must resolve deterministically (last value wins) and inform the user.
- Removing a label that does not exist must be treated as a no-op without error.
- States with no labels must display an empty labels section without error.
- States created before a label policy exists must accept arbitrary tag pairs matching basic format constraints (`[a-z][a-z0-9_/]*`); compliance validation only runs when label policy is set.
- Label keys containing hyphens, dots, or colons must be rejected during validation (not compatible with go-bexpr default Identifier grammar); error message must reference the allowed pattern `[a-z][a-z0-9_/]*`.
- Invalid bexpr filter syntax must be rejected with a clear parse error referencing the go-bexpr grammar.
- Filtering with a valid bexpr that references non-existent label keys must return empty results (labels default to empty map).
- Policy must prevent exceeding 32 labels per state and 256 characters per value with clear error messages.
- Typed label values (string/number/bool) must serialize correctly to JSON and evaluate correctly in bexpr expressions.

---

## Requirements

### Label Model & State Lifecycle
- **FR-001**: System MUST support zero or more user-defined labels per state, each represented as a key/value pair with typed values (string, number, or boolean).
- **FR-002**: Label keys MUST be unique within a single state; submitting an existing key MUST overwrite the prior value.
- **FR-003**: Label values MUST persist across state lifecycle operations (create, update, remote backend writes) unless explicitly changed or removed.
- **FR-003a**: States created before any policy is defined MUST accept arbitrary label key/value pairs matching basic format constraints.
- **FR-004**: Label updates MUST support atomic add, replace, and remove operations so partial failures do not leave states in inconsistent states.
- **FR-005**: System MUST preserve label metadata when other state attributes change (e.g., logic-id updates or dependency recalculations).
- **FR-006**: System MUST return the complete label map in all state retrieval responses, including list, detail, and export views (optional projection control supported).
- **FR-007**: System MUST ensure label ordering is deterministic (e.g., alphabetical by key) to support consistent CLI and UI displays.
- **FR-008**: Label keys MUST match the pattern `[a-z][a-z0-9_/]{0,31}` (lowercase alphanumeric starting with letter, permitting underscore and forward-slash, up to 32 characters total) to ensure compatibility with go-bexpr default Identifier grammar without requiring quoted selectors; submissions outside this pattern or violating reserved namespaces (e.g., `grid.io/`) MUST be rejected, and label values MUST not exceed 256 characters.
- **FR-008a**: Label values MUST support string, numeric (JSON number → float64), and boolean types; submissions with unsupported types MUST be rejected with explicit validation errors.
- **FR-008b**: Each state MUST enforce a hard cap of 32 labels; attempts to exceed the cap MUST fail with clear guidance.
- **FR-009**: System MUST record updated_at timestamp for states when labels change to enable traceability; actor attribution will be added once authentication/authorization is available.
- **FR-009a**: Label metadata MUST persist in a single JSON column (`labels`) on the `states` table (JSONB for Postgres, TEXT for SQLite) for simplicity and portability.
- **FR-009b**: REMOVED (EAV-specific; not applicable to JSON column storage).

### CLI & SDK Experience
- **FR-010**: `gridctl state create` MUST accept repeated `--label key=value` flags to define labels at creation time.
- **FR-011**: CLI MUST surface validation errors for invalid label inputs with actionable messaging referencing the offending key/value and policy constraints (if label policy exists).
- **FR-012**: CLI MUST introduce a `gridctl state set` command that applies label additions, replacements, and removals to an existing state.
- **FR-013**: `gridctl state set` MUST treat flag syntax `--label foo=bar` as an upsert and `--label -foo` as a removal.
- **FR-014**: CLI MUST support applying multiple label mutations in a single invocation and report a summary of resulting labels.
- **FR-014a**: When duplicate `--label` flags target the same key, the CLI MUST honor the user-supplied order (last value wins) and inform the user.
- **FR-015**: CLI state listing and info commands MUST display associated labels in a readable layout (e.g., table or key=value pairs).
- **FR-016**: CLI MUST support filtering state listings using bexpr filter expressions via `--filter` flag (e.g., `--filter 'env in ["staging","prod"]'`).
- **FR-016a**: CLI MUST support simplified filtering via repeated `--label key=value` flags that are internally converted to a bexpr AND expression.
- **FR-017**: SDKs MUST expose label metadata in state responses and provide helpers for constructing bexpr filters so downstream tools can rely on consistent behavior.
- **FR-017a**: CLI MUST introduce a `gridctl policy` command group for managing the label policy (get, set, validate dry-run).
- **FR-017b**: CLI MUST provide a `gridctl policy enum` command to retrieve allowed values for a specific key (useful for UI pickers and autocomplete).
- **FR-017c**: OPTIONAL: CLI MAY expose a compliance report command that revalidates existing states against the current policy and lists non-compliant entries (deferred if not immediately needed).

### API & Protocol Coverage
- **FR-018**: State creation APIs MUST accept optional label payloads (map of typed values) and validate them against the policy (if defined) before persisting the state.
- **FR-019**: System MUST provide an RPC for updating labels on existing states, accepting a patch with upserts and removals.
- **FR-020**: All state retrieval APIs (GetState, ListStates) MUST return the current label map alongside other metadata.
- **FR-020a**: ListStates MUST support an optional `include_labels` field (default true) to allow clients to opt out of label projection for bandwidth optimization.
- **FR-021**: Protocol definitions MUST expose label metadata using a typed `LabelValue` message (oneof string/number/bool) in existing alpha messages without introducing a new API version.
- **FR-021a**: ListStates RPC MUST add a `filter` field accepting bexpr filter expressions and pagination fields (`page_size`, `page_token`).
- **FR-022**: API MUST evaluate bexpr filters in-memory against label maps after fetching states from the database.
- **FR-023**: API MUST provide dedicated RPCs for label policy management (GetLabelPolicy, SetLabelPolicy, ValidateLabels dry-run, GetLabelEnum for key).
- **FR-024**: API pagination MUST work correctly with in-memory filtering by over-fetching from database and trimming to page_size after filter evaluation.

### Label Policy & Validation
- **FR-025**: System MUST allow operators to set and update a label policy defining allowed keys, value enums per key, reserved namespaces, and limits (max keys, max value length).
- **FR-026**: Policy updates MUST persist in a single `label_policy` table row with versioning (version number, updated_at, policy JSON blob).
- **FR-027**: System MUST validate every label mutation against the active policy synchronously before committing changes.
- **FR-028**: Validation failures MUST return specific errors identifying the violated policy rule (unknown key, invalid value, exceeds cap, reserved namespace).
- **FR-028a**: DEFERRED: When a policy update would invalidate existing labels, system MAY mark affected states for review (compliance tracking deferred, Manual state compliance report only).
- **FR-028b**: System provides compliance report for states to validate if label values match label policy.
- **FR-029**: Policy submissions MUST be validated for structure (valid JSON, correct schema) before activation.
- **FR-030**: DEFERRED: System MAY record audit entries for policy changes (actor, timestamp, diff) for governance needs.
- **FR-031**: Users MUST be able to retrieve the current policy via API/CLI.
- **FR-031a**: Users MUST be able to retrieve allowed value enums for a specific key to support UI pickers and autocomplete.

### Query Performance & Filtering
- **FR-037**: State list queries MUST use in-memory bexpr evaluation for filtering at 100-500 state scale.
- **FR-037a**: Label filtering MUST support full bexpr grammar (equality, in, boolean ops, parentheses) for compound expressions.

**DEFERRED** (Future milestone when scale exceeds 1000 states or latency requirements tighten):
- SQL push-down for safe bexpr subset (equality, in, AND/OR) to reduce memory pressure
- EAV projection for high-cardinality label keys
- Facet promotion/projection infrastructure

### WebApp & Visualization
- **FR-040**: Web dashboard MUST display a dedicated labels section in the state detail view presenting all current labels.
- **FR-041**: Web dashboard list and graph views MUST incorporate label metadata provided by the API without caching stale values.
- **FR-042**: Web dashboard MUST remain read-only for labels in this milestone, deferring inline editing to future phases.
- **FR-043**: Web dashboard MUST gracefully handle states with no labels by presenting an empty-state message or placeholder.
- **FR-044**: Web dashboard uses the label policy enum endpoint to display available filter options for UI pickers.

### Non-Functional Requirements
- **FR-049**: Label-filtered queries MUST complete within 50ms p99 latency for 100-500 state deployments using in-memory bexpr evaluation; load testing deferred to future phase.

### Constraints & Tradeoffs
- The feature remains in alpha; protocol adjustments avoid version bumps but require documentation so early adopters know about evolving semantics.
- JSON column storage (JSONB for Postgres, TEXT for SQLite) maintains database portability while keeping implementation simple.
- In-memory bexpr filtering is acceptable for 100-500 state scale; SQL push-down deferred to avoid complexity of safe query translation.
- HashiCorp go-bexpr provides battle-tested grammar and evaluation without additional dependencies beyond the standard library.

---

## Key Entities
- **State**: Terraform/OpenTofu state entry that now includes a typed labels map (JSON column storing key → LabelValue pairs) alongside existing metadata.
- **Label**: Typed key/value pair attached to a state, with key as string and value as one of: string, number (float64), or boolean.
- **Label Policy**: Lightweight validation rules stored as a single JSON document, defining allowed keys, value enums per key, reserved namespaces (e.g., `grid.io/`), and limits (max 32 keys, max 256 char values).
- **Label Policy Version**: Tracked via version number and updated_at timestamp in the `label_policy` table for governance.
- **Compliance Report**: Generated listing showing which states have labels that violate the current policy, used for manual remediation after policy changes.

**DEFERRED** (Future milestone):
- **SQL Push-down Filter**: Safe translation of bexpr subset to SQL WHERE clauses for >1000 state scale
- **EAV Projection**: Normalized label storage for high-cardinality scenarios
- **Facet Projection**: Denormalized columns for hot label keys

---

## Review & Acceptance Checklist

### Content Quality
- [x] No implementation details (languages, frameworks, code structures)
- [x] Focused on user value and business needs
- [x] Written for non-technical stakeholders
- [x] All mandatory sections completed

### Requirement Completeness
- [x] No [NEEDS CLARIFICATION] markers remain
- [x] Requirements are testable and unambiguous
- [x] Success criteria are measurable where defined
- [x] Scope is clearly bounded to label lifecycle, policy validation, and bexpr filtering
- [x] Dependencies and assumptions identified

**Outstanding Clarifications**: None. Performance targets deferred to future load-testing phase (acceptable for 100-500 state scale).

---

## Execution Status
- [x] User description parsed
- [x] Key concepts extracted
- [x] Ambiguities marked
- [x] User scenarios defined
- [x] Requirements generated
- [x] Entities identified
- [ ] Review checklist passed (pending clarifications)
