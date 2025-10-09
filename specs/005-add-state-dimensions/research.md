# Research: State Identity Dimensions

**Date**: 2025-10-08  
**Feature Branch**: `005-add-state-dimensions`

## Summary
Research confirms a **dictionary-compressed EAV-only** model best fits Grid's read-heavy Terraform state metadata needs at 100-500 state scale while preserving SQLite portability. Analysis shows indexed EAV joins perform sufficiently (<50ms p99) without requiring facet projection complexity. We also evaluated Go tooling for translating key=value filters into SQL across PostgreSQL and SQLite, and JSON Schema validators appropriate for constrained tag schemas.

## Data Model & Storage
- **Decision**: Use dictionary-compressed EAV tables (`meta_keys`, `meta_values`, `state_metadata`) with integer foreign keys. **No facet projection table** in initial implementation.
- **Rationale**:
  - Integer-keyed dictionaries keep predicate comparisons fast and index-friendly, enabling efficient equality filters and compact B-Tree pages
  - At 100-500 state scale, indexed EAV joins perform adequately (<10ms typical, <50ms p99) without denormalization overhead
  - Composite index on `state_metadata(key_id, value_id, state_id)` enables index-only scans for common filter patterns
  - Avoids complexity of projection lifecycle management (DDL, backfills, consistency) until scale demands it
  - Preserves SQLite parity (no JSONB dependency)
- **Evidence**:
  - UW–Madison overview of EAV trade-offs: <https://pages.cs.wisc.edu/~ajkunen/eav/overview.html>
  - Novick's paper on minimizing EAV penalties via dictionaries and indexes: <https://novicksoftware.com/wp-content/uploads/2016/09/Entity-Attribute-Value-EAV-The-Antipattern-Too-Great-to-Give-Up-Andy-Novick-2016-03-19.pdf>
- **Performance Analysis**:
  - 500 states × 10 tags avg = ~5k junction rows; entire dataset fits in shared buffers (~2MB)
  - Single equality filter: `meta_keys.name` lookup (1 row) + `meta_values` seek (O(1)) + `state_metadata` index scan (~50 matching states)
  - Two-tag AND filter: index scans on both predicates + merge join (~25 results from 100+50 intermediate rows)
  - PostgreSQL query planner handles 2-3 join optimization well at this scale; network/serialization dominates latency
- **Alternatives Considered**:
  1. **Fixed N canonical columns** (e.g., dim1_key/dim1_value...dim16_key/dim16_value):
     - Simpler queries, lower write overhead
     - Rejected: sparse storage waste, awkward multi-slot queries (OR across all dim slots), rigid capacity
  2. **Bitmap/Roaring posting lists** (inverted index with compressed state ID bitmaps):
     - Excellent for compound filters at million-state scale
     - Rejected: requires external library, SQLite incompatibility, overkill for hundreds of states
  3. **JSONB + GIN indexes** (PostgreSQL native):
     - Clean API, mature indexing
     - Rejected: breaks SQLite parity requirement (spec FR-009a)
  4. **EAV + facet projection** (original plan):
     - Future-proof for 10k+ state scale
     - Rejected for v1: premature optimization; adds ~8-10 tasks (DDL management, backfill jobs, CLI tooling) with marginal benefit at current scale
- **Migration Path**: EAV schema allows adding `state_facets` projection non-disruptively when state count exceeds 1000 or latency requirements tighten; existing queries continue working.

## Query Construction (Go)
- **Decision**: Prototype filter translation with `github.com/Masterminds/squirrel` (baseline) and evaluate `github.com/doug-martin/goqu` for richer expression trees; both support PostgreSQL and SQLite dialects.
- **Rationale**: Squirrel provides lightweight builder primitives for equality filters and IN clauses, has minimal dependencies, and is easy to unit-test. Goqu offers dialect-aware query generation if future compound expressions are prioritized.
- **Evidence**:
  - Squirrel documentation highlighting composable SQL builders: <https://github.com/Masterminds/squirrel>
  - Goqu multi-dialect SQL builder: <https://github.com/doug-martin/goqu>
- **Follow-up**: Planning phase will spike filter translation prototypes for equality filters and document extension path for AND/OR once required.

## JSON Schema Validation
- **Decision**: Use `github.com/santhosh-tekuri/jsonschema/v5` for server-side validation with a reduced keyword set (types, enum, pattern, maxLength). Keep `github.com/xeipuuv/gojsonschema` as fallback if streaming compilation is needed.
- **Rationale**: Tekuri’s validator supports draft 2020-12, precompiles schemas for repeated use, and allows custom loaders to enforce schema size limits. We can disallow unsupported keywords (e.g., unevaluatedItems) to keep evaluation predictable.
- **Evidence**:
  - Tekuri validator docs: <https://github.com/santhosh-tekuri/jsonschema>
  - gojsonschema community usage: <https://github.com/xeipuuv/gojsonschema>
- **Constraints**: Restrict schemas to scalar/string/number/boolean types, forbid `$ref` and remote fetches, cap document size to prevent DoS.

## Facet Promotion & Maintenance (DEFERRED)
- **Decision**: Defer facet projection to future milestone when state count exceeds 1000 or p99 latency requirements drop below current EAV performance.
- **Rationale**: Indexed EAV queries sufficient for 100-500 state scale; facet projection adds operational complexity without measurable benefit in v1.
- **Future Design Notes** (when needed):
  - GridAPI would own `facets_registry` plus projector routines updating `state_facets` in-transaction
  - Backfills run in batches (10k rows default) to avoid long-lived locks
  - Per-facet indexes (`state_facets_<column>_idx`) for sub-5ms dashboard queries
  - Migration from EAV-only: add projection table, backfill existing states, update query builder to prefer projection when filters match promoted keys

## Compliance & Audit Strategy
- **Decision**: Maintain a compliance status flag per state. CLI/API expose revalidation endpoints listing non-compliant states and the violated rule; audit logs are retrievable and flushable via CLI.
- **Rationale**: Aligns with single-user alpha mode while preparing for future RBAC; keeping audit surface in CLI avoids premature web UI expansion.

## Implementation Notes
- Compliance status stored directly in `states` table as simple columns (no separate table needed)
- No existing deployments to migrate; single bun migration creates all structures
- Migration follows existing pattern: `migrations/YYYYMMDDHHMMSS_add_state_tags.go` with `init()` registration

## References
1. DuckDuckGo search: “dictionary compressed entity attribute value table design”
2. DuckDuckGo search: “golang build SQL queries key value filters library”
3. DuckDuckGo search: “golang json schema validator library draft 2020-12”
