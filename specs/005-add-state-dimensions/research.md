# Research: State Labels

**Date**: 2025-10-08
**Updated**: 2025-10-09 (Scope Reduction + Constitution Alignment)
**Feature Branch**: `005-add-state-dimensions`

## Summary
Research confirms a **JSON column with in-memory bexpr filtering** approach best fits Grid's read-heavy Terraform state metadata needs at 100-500 state scale while minimizing implementation complexity. Analysis shows in-memory filtering performs sufficiently (<50ms p99) without requiring complex SQL translation, EAV normalization, or facet projection. We selected HashiCorp go-bexpr for filtering and a lightweight policy validator (regex + enums) over JSON Schema to avoid external dependencies.

**Constitution Compliance** (2025-10-09): FR-045 mandates webapp → js/sdk → api dependency flow per Constitution Principle III. Dashboard components MUST consume TypeScript SDK wrappers, not generated Connect clients directly. Go SDK provides rich label-aware builders; TypeScript SDK provides lightweight bexpr string utilities.

## Data Model & Storage
- **Decision (UPDATED 2025-10-09)**: Use a single JSON column (`labels`) on the `states` table. JSONB for PostgreSQL, TEXT (JSON1 extension) for SQLite. **No EAV tables, no facet projection.**
- **Rationale**:
  - JSON column is simpler to implement and maintain (single ALTER TABLE, no join complexity)
  - At 100-500 state scale, fetching all states and filtering in-memory is acceptable (<50ms p99)
  - Avoids complexity of EAV normalization, dictionary management, and query building
  - Preserves SQLite parity (JSON1 extension widely available)
  - Easy migration path to add GIN index (Postgres) or expression indexes if SQL push-down becomes necessary
- **Performance Analysis**:
  - 500 states with ~1KB labels each = ~500KB total data (easily fits in memory)
  - Fetching 500 rows: ~5-10ms database time
  - In-memory bexpr evaluation: ~10-20μs per state × 500 = ~10ms
  - Total latency: ~15-25ms typical, <50ms p99
  - Network/serialization dominates; in-memory filtering is negligible overhead
- **Alternatives Considered**:
  1. **Dictionary-compressed EAV** (original plan):
     - Normalized storage, cache-friendly for SQL filtering
     - Rejected: adds significant complexity (3 tables, migrations, join logic) with no measurable benefit at current scale
  2. **SQL query builder with filter translation**:
     - Push filtering to database, reduce memory usage
     - Rejected: requires complex and potentially unsafe SQL generation from user expressions; deferred until scale demands it
  3. **Facet projection** (original plan):
     - Denormalized columns for hot keys
     - Rejected: premature optimization; adds DDL management, backfill jobs, and maintenance overhead
- **Migration Path**: JSON column allows adding GIN index (Postgres) or migrating to EAV when state count exceeds 1000; bexpr API contract remains unchanged.

## Filtering & Query Evaluation
- **Decision (UPDATED 2025-10-09)**: Use `github.com/hashicorp/go-bexpr` for in-memory boolean expression evaluation. **No SQL query builder needed.**
- **Rationale**:
  - HashiCorp go-bexpr provides battle-tested grammar and evaluator used in Consul, Nomad, and Vault
  - Eliminates need for unsafe SQL translation from user expressions
  - Supports full boolean logic (AND, OR, NOT, parentheses, in, ==, !=, >=, <=, etc.) without implementation complexity
  - Zero additional dependencies beyond standard library
  - Clean evaluation API: compile expression once, evaluate against map[string]any repeatedly
- **Evidence**:
  - go-bexpr grammar: <https://github.com/hashicorp/go-bexpr/blob/main/grammar/grammar.peg>
  - go-bexpr documentation: <https://github.com/hashicorp/go-bexpr>
- **Implementation Pattern**:
  1. Fetch states from database (with optional limits for pagination over-fetch)
  2. Unmarshal `labels` JSONB/TEXT to `map[string]any`
  3. Compile bexpr filter once (or cache compiled evaluator)
  4. Evaluate each state's label map; include if match
  5. Trim results to page_size after filtering
- **Future Optimization** (when scale exceeds 1000 states):
  - Translate safe bexpr subset (equality, in, simple AND/OR) to SQL WHERE clauses
  - Fall back to in-memory for complex expressions
  - Bexpr API contract remains unchanged

## Label Validation
- **Decision (UPDATED 2025-10-09)**: Use lightweight policy validator with regex + enum maps. **No JSON Schema dependency.**
- **Rationale**:
  - Simple regex pattern for key format: `^[a-z0-9][a-z0-9._:/-]{0,31}$`
  - Per-key enum maps for allowed values: `map[string]map[string]struct{}`
  - Reserved namespace prefixes (e.g., `grid.io/`) checked via `strings.HasPrefix`
  - Type checking (string/number/bool) via Go type assertion on `map[string]any`
  - No external dependencies, easy to test, predictable performance
- **Policy Structure** (stored as JSON in `label_policy` table):
  ```go
  type Policy struct {
      AllowedKeys      map[string]struct{}            // nil = any key allowed
      AllowedValues    map[string]map[string]struct{} // key -> set of allowed values
      ReservedPrefixes []string                        // e.g., ["grid.io/", "kubernetes.io/"]
      MaxKeys          int                             // e.g., 32
      MaxValueLen      int                             // e.g., 256
  }
  ```
- **Deferred**: JSON Schema support if richer validation becomes necessary (pattern matching, conditional schemas, etc.)

## Compliance & Audit Strategy
- **Decision (UPDATED 2025-10-09)**: Simple policy versioning with optional manual compliance reporting. **No automated compliance tracking or audit log infrastructure.**
- **Rationale**:
  - Store policy version number and updated_at timestamp in `label_policy` table
  - Optional CLI command to revalidate all states against current policy and report violations
  - Aligns with single-user alpha mode; deferred complex audit trail until RBAC/governance requirements emerge
- **Future Enhancement**: Add audit log table tracking policy changes with diffs when governance requirements justify the complexity

## SDK Architecture & Responsibilities
- **Decision (UPDATED 2025-10-09)**: TypeScript SDK provides lightweight bexpr string utilities; GetLabelEnum RPC removed in favor of extracting enums from GetLabelPolicy.
- **Rationale**:
  - **Constitution Principle III enforcement**: webapp → js/sdk → api (never webapp → api directly)
  - **Go SDK**: Rich builders (ConvertProtoLabels, BuildBexprFilter, SortLabels) for CLI and server-side consumers
  - **TypeScript SDK**: Simple bexpr string concatenation utilities (buildEqualityFilter, buildInFilter, combineFilters) to avoid duplicating complex builder logic in JavaScript
  - **Endpoint consolidation**: GetLabelPolicy returns full policy including enums; UI components extract enum values client-side rather than adding a dedicated GetLabelEnum RPC
  - **Benefits**: Simpler API surface (one less RPC), reduced network calls (single policy fetch), aligns with read-heavy usage pattern
- **CLI Output Format** (FR-015): State list displays labels as comma-separated key=value pairs truncated at 32 chars; info/get commands show full labels without truncation

## Implementation Notes
- No existing deployments to migrate; single bun migration adds `labels` column and `label_policy` table
- Migration follows existing pattern: `migrations/YYYYMMDDHHMMSS_add_state_labels.go` with `init()` registration
- bexpr grammar documented for users; recommend underscore-separated keys for maximum compatibility

## References
1. HashiCorp go-bexpr: <https://github.com/hashicorp/go-bexpr>
2. go-bexpr grammar (PEG): <https://github.com/hashicorp/go-bexpr/blob/main/grammar/grammar.peg>
3. Scope reduction document: `specs/005-add-state-dimensions/scope-reduction.md`
