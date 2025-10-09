# Feature Specification: State Identity Dimensions

**Feature Branch**: `005-add-state-dimensions`
**Created**: 2025-10-08
**Status**: Draft
**Input**: User description: "Add state dimensions to extend the Grid Terraform state management framework. Support user-defined key/value tags (“state identity dimensions”) attached to states for filtering, querying, and future RBAC controls. Users must be able to provide state key/value pairs at state creation (via repeated CLI flags for example `--set`) and manage them afterward (new "state set" command to update, delete, prefix key with - to remove (for example set foo=bar to add and set -foo to remove). The (readonly) WebApp will include a dedicated section in the DetailView to display all tags, and all state list/info protocols must include the tag metadata so SDKs ensure CLIs and WebApps can display and filter by these tags.

Tag management must support validation against a server-side schema supplied by users in a JSON-Schema format. The server-side will provide a way to store this schema, and use it to validate submitted key/value pairs against it. Protocols must include updates to the schema with an audit log, for example showing diff between json-schema versions (to add new keys, extend allowed value lists, or mark keys as “accept any text”).

Given the expected low write and high read frequency, the system should provide a “facets” concept: future admin users (altho currently single user and no RBAC) can designate certain tag keys as promoted facets for fast queries with filtering for dashboards. The API will expose admin endpoints to define/enable/disable facet keys, trigger backfill/refresh operations that populate facet views, and manage indexing/refresh jobs. Reads should only allow filtering on facet views for promoted keys. This design must avoid exposing database internals while ensuring that deployments can benefit from efficient read paths without requiring users to perform manual database operations."

## Execution Flow (main)
```
1. Parse user description from Input
   → SUCCESS: Captured need for state identity dimensions, schema validation, and facets
2. Extract key concepts from description
   → Actors: CLI users, API consumers, web dashboard viewers, future admins
   → Actions: Create/update tags, validate against schema, display/filter, manage facets
   → Data: State metadata, tag key/value pairs, JSON Schema versions, facet definitions, audit log
   → Constraints: Read-heavy workloads on states, schema-governed validation, no direct DB exposure
3. Identify uncertainties
   → Marked with [NEEDS CLARIFICATION] in Clarifications and Requirements
4. Define User Scenarios & Testing
   → SUCCESS: Five acceptance scenarios covering create, validate, update, display, and query flows
5. Generate Functional Requirements
   → SUCCESS: Requirements categorized for lifecycle, tooling, schema governance, and facets
6. Identify Key Entities
   → SUCCESS: State, Tag Dimension, Dimension Schema, Facet Definition, Facet Refresh Job, Audit Entry
7. Review checklist
   → WARN: Outstanding clarifications remain; see checklist section
8. Return spec
   → SUCCESS: Document ready for review pending clarifications
```

---

## ⚡ Quick Guidelines
- ✅ Focus on WHAT users need and WHY
- ❌ Avoid HOW to implement (no tech stack, APIs, code structure)
- 👥 Written for business stakeholders, not developers

---

## Clarifications

### Session 2025-10-08
- Q: What length limits should we enforce for tag keys and tag values? → A: Keys ≤32 chars; values ≤256 chars.
- Q: How should we treat tag values in this milestone? → A: Allow strings plus numeric and boolean values per schema.
- Q: How many tag dimensions may a single state hold before we reject new additions? → A: Hard cap at 32 tags per state.
- Q: How should schema validation run when users modify tags? → A: Always synchronous on each tag write.
- Q: In multi-operator environments, who may publish new schemas or facet definitions? → A: Without RBAC, any user may publish them.
- Q: When an updated schema invalidates existing tags, how should the system respond? → A: Allow the schema update, mark affected states as non-compliant, and surface them via a dedicated revalidation endpoint for manual remediation.
- Q: How should the tag schema audit log surface to users and what retention applies? → A: Provide a dedicated `gridctl` subcommand group (schema, facets, compliance, audit, flush) that can download logs and trim storage as needed.
- Q: Should facet filtering support compound expressions this milestone? → A: Keep filters to simple equality and defer compound support to the upcoming research phase on expression-to-query engines.

### Pending Clarifications
- None.

### Working Assumptions
- Schema validation runs synchronously during tag write operations so users receive immediate feedback.
- Until RBAC is implemented, any user may publish new schemas or facet definitions; future milestones will introduce tighter controls.
- Facet projection updates occur in-transaction for regular state metadata writes, with user (or cron) initiated jobs reserved for bulk rebuild operations.

---

## User Scenarios & Testing *(mandatory)*

### Primary User Story
As an infrastructure operator managing Terraform states with Grid, I need to attach and maintain descriptive tag dimensions on each state so I can organize environments, filter dashboards, and prepare for future access controls without touching database internals.

### Acceptance Scenarios

1. **Given** I run `gridctl state create` with multiple `--set key=value` flags, **When** the state is created successfully, **Then** the system stores each tag and returns them in the creation response and subsequent list/detail views.
2. **Given** a JSON Schema is active that restricts allowable tag keys and value formats, **When** I submit a tag update that violates the schema, **Then** the system blocks the change and presents clear validation errors.
3. **Given** an existing state has tags, **When** I run `gridctl state set` with both additions (`foo=bar`) and removals (`-legacy`), **Then** the system applies the requested changes atomically and shows the resulting tag set.
4. **Given** I view a state in the web dashboard detail pane, **When** the page loads, **Then** I see a dedicated tags section listing all current tag dimensions in a readable, copy-friendly format.
5. **Given** I request a filtered state list by tag via CLI or API, **When** the query executes, **Then** the system returns matching states using indexed EAV joins.
6. **Given** a new schema version is published, **When** I inspect the schema audit log, **Then** I can see who made the change, when it occurred, and a diff highlighting the JSON Schema alterations.
7. **Given** a schema update marks some states as non-compliant, **When** I run the compliance report subcommand, **Then** I receive a list of affected states with the rule violations that require manual remediation.

### Edge Cases
- Submitting duplicate `--set` flags for the same key within a single command (e.g., conflicting values) must resolve deterministically using the user-supplied flag order and inform the user of precedence.
- Removing a tag that does not exist must be treated as a no-op and return a clear message rather than an error.
- States with no tags must display empty tags section without error.
- States created without a schema must accept arbitrary tag pairs; compliance validation only runs when schema is set.
- Schema payloads that are syntactically invalid JSON or invalid JSON Schema must be rejected with precise error reporting.
- Compliance report commands must clearly list each non-compliant state and the schema rule violated to guide manual remediation.
- Filtering on non-existent tag keys must return empty results without error.
- Queries with multiple tag filters must use indexed intersection for acceptable performance at 100-500 state scale.

---

## Requirements

### Tag Model & State Lifecycle
- **FR-001**: System MUST support zero or more user-defined tag dimensions per state, each represented as a key/value pair.
- **FR-002**: Tag keys MUST be unique within a single state; submitting an existing key MUST overwrite the prior value.
- **FR-003**: Tag values MUST persist across state lifecycle operations (create, update, remote backend writes) unless explicitly changed or removed.
- **FR-003a**: States created before any schema is defined MUST accept arbitrary tag key/value pairs; compliance status remains 'unknown' until first schema validation.
- **FR-004**: Tag updates MUST support atomic add, replace, and remove operations so partial failures do not leave states in inconsistent states.
- **FR-005**: System MUST preserve tag metadata when other state attributes change (e.g., logic-id updates or dependency recalculations).
- **FR-006**: System MUST return the complete tag map in all state retrieval responses, including list, detail, and export views.
- **FR-007**: System MUST ensure tag ordering is deterministic (e.g., alphabetical by key) to support consistent CLI and UI displays.
- **FR-008**: Tag keys MUST be lowercase alphanumeric strings up to 32 characters, permitting '-', '_', '/', ':', and '.' characters; submissions outside this pattern or violating reserved namespaces MUST be rejected, and tag values MUST not exceed 256 characters.
- **FR-008a**: Tag values MUST support string, numeric, and boolean types when permitted by the active schema; submissions outside the allowed type set MUST be rejected with explicit validation errors.
- **FR-008b**: Each state MUST enforce a hard cap of 32 tag dimensions; attempts to exceed the cap MUST fail with guidance on pruning or consolidating tags.
- **FR-009**: System MUST record the timestamp for every tag mutation to enable traceability, with actor attribution added once authentication/authorization is available.
- **FR-009a**: Tag metadata MUST persist in dictionary-compressed EAV tables (`meta_keys`, `meta_values`, `state_metadata`) so keys and values are normalized for storage and reuse across states.
- **FR-009b**: `state_metadata` MUST expose integer foreign keys for `key_id`, `value_id`, and `state_id`, enabling compact composite indexes that keep equality filters cache-friendly.

### CLI & SDK Experience
- **FR-010**: `gridctl state create` MUST accept repeated `--set key=value` flags to define tags at creation time.
- **FR-011**: CLI MUST surface validation errors for invalid tag inputs with actionable messaging referencing the offending key/value.
- **FR-012**: CLI MUST introduce a `gridctl state set` command that applies tag additions, replacements, and removals to an existing state.
- **FR-013**: `gridctl state set` MUST treat flag syntax `--set foo=bar` as an upsert and `--set -foo` as a removal.
- **FR-014**: CLI MUST support applying multiple tag mutations in a single invocation and report a summary of resulting tags.
- **FR-014a**: When duplicate `--set` flags target the same key, the CLI MUST honor the user-supplied order (last value wins) and surface the resolution in command output.
- **FR-015**: CLI state listing and info commands MUST display associated tags in a readable layout (e.g., table or key=value pairs).
- **FR-016**: CLI MUST support filtering state listings by tags using a consistent flag syntax (e.g., `--filter environment=prod`).
- **FR-017**: SDKs MUST expose tag metadata and filtering helpers so downstream tools (other CLIs, automation) can rely on consistent behavior.
- **FR-017a**: CLI MUST introduce a dedicated `gridctl tags` command group covering schema management, compliance reporting, audit log retrieval, and audit log flushing (download + trim).
- **FR-017c**: CLI MUST expose a compliance report command that revalidates existing states against the current schema and lists non-compliant entries with violated rules.

### API & Protocol Coverage
- **FR-018**: State creation APIs MUST accept optional tag payloads and validate them before persisting the state.
- **FR-019**: System MUST provide an API surface dedicated to managing tags on existing states, supporting batch updates and removals.
- **FR-020**: All state retrieval APIs (list, describe, search) MUST return the current tag set alongside other metadata.
- **FR-021**: Protocol definitions MUST expose tag metadata in existing alpha messages without introducing a new version; document the addition as an alpha extension for current clients.
- **FR-021a**: Listing RPCs MUST add pagination inputs and outputs so clients can traverse tag-filtered state sets efficiently.
- **FR-022**: API MUST support filtering by any indexed tag key using simple equality predicates.
- **FR-024**: API MUST provide pagination-safe mechanisms to retrieve states filtered by tags without requiring clients to understand database internals.

### Schema Governance & Validation
- **FR-025**: System MUST allow operators (until AuthN/AuthZ arrives) to register and update a JSON Schema that governs allowable tag keys, values, and formats.
- **FR-026**: Each schema update MUST create a new version with immutable historical records.
- **FR-027**: System MUST validate every tag mutation against the active schema before committing changes.
- **FR-028**: Validation failures MUST indicate the specific schema rule violated and the offending key/value.
- **FR-028a**: When a new schema is set that conflicts with existing tags, the schema update MUST succeed while marking affected states as non-compliant for later remediation.
- **FR-028b**: System MUST maintain compliance status metadata and expose it to the CLI compliance report command for manual cleanup.
- **FR-028c**: System MUST offer an API endpoint that revalidates states on demand, returning the same non-compliance details surfaced by the CLI.
- **FR-029**: Schema submissions MUST be validated for JSON syntax and JSON Schema compliance before activation.
- **FR-030**: System MUST record an audit entry for each schema change, capturing actor, timestamp, rationale (optional), and a diff versus the prior schema.
- **FR-031**: Users MUST be able to retrieve current and historical schema versions, including the diff information, via API and CLI.
- **FR-031a**: Audit history MUST be retrievable and flushable through the dedicated CLI commands so operators can download and trim logs according to storage policies.

### Query Performance (Deferred: Facet Promotion)
- **FR-037**: State read APIs MUST use indexed EAV joins for tag filtering to achieve acceptable performance at 100-500 state scale.
- **FR-037a**: Tag filtering MUST be limited to simple equality comparisons in this milestone; support for compound expressions is out of scope pending query language research.
- **FR-037b**: System MUST ensure composite index on `state_metadata(key_id, value_id, state_id)` exists for efficient tag filtering.

**DEFERRED** (Future milestone when state count exceeds 1000 or latency requirements tighten):
- Facet promotion registry allowing operators to designate hot keys for projection columns
- `state_facets` denormalized projection table with per-column indexes
- Backfill jobs and CLI tooling for projection maintenance
- Online DDL management for adding/removing facet columns
- Facet status indicators and refresh job tracking

### WebApp & Visualization
- **FR-040**: Web dashboard MUST display a dedicated tags section in the state detail view presenting all current tag dimensions.
- **FR-041**: Web dashboard MUST render facet-enabled tags with visual emphasis (e.g., badges) to highlight available filters.
- **FR-042**: Web dashboard list and graph views MUST incorporate tag metadata provided by the API without caching stale values.
- **FR-043**: Web dashboard MUST remain read-only for tags in this milestone, deferring inline editing to future phases.
- **FR-044**: Web dashboard MUST gracefully handle states with no tags by presenting an empty-state message or placeholder.

### Non-Functional Requirements
- **FR-049**: Tag-filtered queries MUST complete within 50ms p99 latency for 100-500 state deployments using indexed EAV joins; load testing deferred to future phase.
- **FR-050**: The design MUST avoid requiring operators to run manual database operations; all necessary management MUST occur through documented APIs or tooling.
- **FR-051**: System MUST maintain migration path to add facet projection layer in future without breaking schema changes or data loss.

### Constraints & Tradeoffs
- Research during planning MUST evaluate expression-to-query translation options in the Go backend to support future compound tag filters without exposing raw SQL.
- The feature remains in alpha; protocol adjustments avoid version bumps but require documentation so early adopters know about evolving semantics.
- Dictionary-compressed EAV storage delivers the portability and cache-friendly indexes highlighted in external references such as UW–Madison's EAV overview and Novick's EAV antipattern paper; JSONB storage is explicitly avoided to maintain SQLite parity.
- Planning MUST compare Go SQL builder libraries (e.g., `huandu/go-sqlbuilder`, `Masterminds/squirrel`) and JSON Schema validators (`santhosh-tekuri/jsonschema`, `xeipuuv/gojsonschema`) surfaced during research to ensure Postgres and SQLite compatibility with constrained schema features.
- Facet projection layer explicitly deferred to reduce implementation complexity; EAV query performance sufficient for 100-500 state scale per performance analysis.

---

## Key Entities
- **State**: Terraform/OpenTofu state entry that now includes a collection of tag dimensions alongside existing metadata and a compliance status reflecting the latest schema validation pass.
- **Meta Key**: Dictionary record (`meta_keys`) mapping each canonical tag key to an integer identifier used across states.
- **Meta Value**: Dictionary record (`meta_values`) storing allowable values per key, referenced by integer identifiers to compress storage.
- **State Metadata Row**: Junction entry (`state_metadata`) tying a state, key id, and value id together for the normalized EAV model.
- **Dimension Schema**: Versioned JSON Schema document governing allowable tag keys, structures, and value constraints.
- **Audit Entry**: Log item capturing schema changes or tag mutations for traceability, retrievable and flushable via the dedicated CLI tools.
- **Compliance Report**: Generated listing of states marked non-compliant after schema updates, used by operators to prioritize manual remediation.

**DEFERRED** (Future milestone):
- **Facet Definition**: Registry mapping promoted tag keys to projection columns
- **State Facets Projection**: Denormalized table with per-facet columns and indexes
- **Facet Refresh Job**: Backfill operation tracking for projection maintenance

---

## Review & Acceptance Checklist

### Content Quality
- [x] No implementation details (languages, frameworks, code structures)
- [x] Focused on user value and business needs
- [x] Written for non-technical stakeholders
- [x] All mandatory sections completed

### Requirement Completeness
- [ ] No [NEEDS CLARIFICATION] markers remain
- [x] Requirements are testable and unambiguous (aside from noted clarifications)
- [x] Success criteria are measurable where defined
- [x] Scope is clearly bounded to tagging, schema governance, and facet promotion
- [x] Dependencies and assumptions identified

**Outstanding Clarifications**: Performance targets deferred to future load-testing phase.

---

## Execution Status
- [x] User description parsed
- [x] Key concepts extracted
- [x] Ambiguities marked
- [x] User scenarios defined
- [x] Requirements generated
- [x] Entities identified
- [ ] Review checklist passed (pending clarifications)
