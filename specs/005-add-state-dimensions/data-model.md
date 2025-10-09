# Data Model: State Identity Dimensions

**Date**: 2025-10-08  
**Feature Branch**: `005-add-state-dimensions`

## Entity Overview

### State (extended)
- **Description**: Existing Terraform/OpenTofu state record extended with compliance tracking columns.
- **Existing Fields** (from `models.State`):
  - `guid` (UUID, PK) - note: lowercase in DB, GUID in Go model
  - `logic_id` (string, unique)
  - `state_content`, `locked`, `lock_info`, `created_at`, `updated_at`
- **New Fields** (added by migration):
  - `compliance_status` (varchar, default 'unknown')
  - `compliance_checked_at` (timestamptz, nullable)
- **Relationships**:
  - 1-to-many with `StateMetadataRow`

### MetaKey (`meta_keys`)
- **Purpose**: Canonical dictionary of tag keys.
- **Fields**:
  - `id` (serial, PK)
  - `name` (varchar(64), unique, lowercase)
  - `created_at` (timestamptz, default current_timestamp)
- **Indexes**: unique index on `name`
- **Note**: `created_by` deferred until AuthN implemented

### MetaValue (`meta_values`)
- **Purpose**: Dictionary of values scoped by key.
- **Fields**:
  - `id` (serial, PK)
  - `key_id` (integer FK → `meta_keys.id`, on delete cascade)
  - `value` (varchar(256))
  - `created_at` (timestamptz, default current_timestamp)
- **Indexes**: unique composite on (`key_id`, `value`)

### StateMetadataRow (`state_metadata`)
- **Purpose**: Normalized EAV junction table mapping states to tag key/value pairs.
- **Fields**:
  - `state_id` (uuid FK → `states.guid`, on delete cascade)
  - `key_id` (integer FK → `meta_keys.id`, on delete cascade)
  - `value_id` (integer FK → `meta_values.id`, on delete cascade)
  - `updated_at` (timestamptz, default current_timestamp)
- **Constraints**:
  - PK: (`state_id`, `key_id`) - one value per key per state
  - Foreign keys enforce referential integrity
- **Indexes**:
  - `CREATE INDEX state_metadata_kv_idx ON state_metadata(key_id, value_id, state_id)` - critical for tag filtering queries

### DimensionSchemaVersion (`dimension_schemas`)
- **Purpose**: Stores versioned JSON Schema documents.
- **Fields**:
  - `id` (uuid, PK, default gen_random_uuid())
  - `version` (serial, unique, monotonic)
  - `schema_doc` (text) - JSON stored as text for SQLite parity
  - `created_at` (timestamptz, default current_timestamp)
  - `change_reason` (text, nullable)
- **Indexes**: unique on `version`
- **Note**: `created_by` deferred until AuthN implemented

### SchemaAuditEntry (`schema_audit`)
- **Purpose**: Append-only audit log capturing schema changes.
- **Fields**:
  - `id` (uuid, PK, default gen_random_uuid())
  - `schema_version` (integer FK → `dimension_schemas.version`)
  - `timestamp` (timestamptz, default current_timestamp)
  - `diff_json` (text) - JSON diff between versions
- **Indexes**: index on `timestamp DESC` for recent queries
- **Retention**: CLI `audit flush` downloads + trims older entries
- **Note**: `actor` field deferred until AuthN implemented

## Deferred Entities (Future Milestone)

The following entities support facet projection optimization and are deferred until state count exceeds 1000 or latency requirements demand sub-10ms query performance:

### FacetDefinition (`facets_registry`) - DEFERRED
- **Purpose**: Tracks promoted tag keys and projection metadata.
- **Trigger for implementation**: Dashboard queries exceed 50ms p99 or state count > 1000

### StateFacetsProjection (`state_facets`) - DEFERRED
- **Purpose**: Denormalized row used for fast facet read queries.
- **Trigger for implementation**: Same as FacetDefinition

### FacetRefreshJob (`facet_refresh_jobs`) - DEFERRED
- **Purpose**: Tracks backfill/reindex operations.
- **Trigger for implementation**: Same as FacetDefinition

### ComplianceReportEntry (`compliance_reports`) - OPTIONAL
- **Purpose**: Optional table for storing compliance report snapshots (may defer to ephemeral CLI output).
- **Fields**:
  - `id` (uuid, PK)
  - `state_id` (uuid FK → `states.guid`)
  - `schema_version` (integer FK → `dimension_schemas.version`)
  - `violations` (text) - JSON array of violation details
  - `reported_at` (timestamptz, default current_timestamp)
- **Indexes**: (`reported_at DESC`), (`schema_version`, `state_id`)
- **Note**: May implement compliance report as ephemeral query result instead of persistent table

## Relationships Diagram (Textual)
- State 1..* StateMetadataRow
- MetaKey 1..* MetaValue
- MetaKey 1..* StateMetadataRow (via key_id)
- MetaValue 1..* StateMetadataRow
- DimensionSchemaVersion 1..* SchemaAuditEntry
- State 1..* ComplianceReportEntry

**Deferred relationships** (facet projection):
- MetaKey 1..* FacetDefinition
- FacetDefinition 1..1 StateFacetsProjection column (enforced by DDL)
- State 1..1 StateFacetsProjection

## Validation Rules
- Tag keys: lowercase alphanumeric plus `- _ / : .`, ≤64 chars (matching `meta_keys.name` column)
- Tag values: ≤256 chars (matching `meta_values.value` column)
- Maximum 32 tags per state (enforced in application layer before insert)
- Schema submissions limited to whitelisted JSON Schema keywords (type, enum, pattern, maxLength, description)
- No reserved key prefixes enforcement in v1 (consider `grid:` prefix for future system tags)

## Migration Notes
- Single migration creates all tag-related tables following existing bun migration pattern (`migrations/YYYYMMDDHHMMSS_add_state_tags.go`)
- Tables to create:
  - `meta_keys` - tag key dictionary
  - `meta_values` - tag value dictionary
  - `state_metadata` - EAV junction table with composite PK (`state_id`, `key_id`)
  - `dimension_schemas` - JSON Schema versioning
  - `schema_audit` - audit log
- Critical index: `CREATE INDEX state_metadata_kv_idx ON state_metadata(key_id, value_id, state_id)`
- Add compliance columns to existing `states` table:
  - `compliance_status` VARCHAR DEFAULT 'unknown'
  - `compliance_checked_at` TIMESTAMPTZ NULL
- Migration uses Bun ORM models registered in `init()` following existing pattern in `migrations/main.go`

## Design Decisions
- **Compliance status in `states` table**: Keeps queries simple; rare updates acceptable overhead
- **No facet projection in v1**: Performance analysis shows indexed EAV adequate for 100-500 states (<50ms p99)
- **Dictionary compression mandatory**: Reduces storage and enables integer-based index scans
- **Single composite index strategy**: `state_metadata(key_id, value_id, state_id)` covers most query patterns without index bloat
