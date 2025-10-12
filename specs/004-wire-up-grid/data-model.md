# Data Model: Live Dashboard Integration

**Feature**: 004-wire-up-grid
**Date**: 2025-10-06
**Status**: Complete

## Overview

This document defines the data entities and type mappings for the live dashboard integration feature. The model maintains compatibility with the existing mockApi interface while mapping to protobuf definitions and database schema.

All entities are delivered via the new `ListAllEdges` RPC defined in `plan.md`, enabling manual refresh semantics (no background polling) while preserving the mock API shapes consumed by the React dashboard.

---

## Entity Hierarchy

```
StateInfo (root entity)
├── BackendConfig (embedded)
├── DependencyEdge[] (relationships - incoming)
├── DependencyEdge[] (relationships - outgoing)
└── OutputKey[] (collection)
```

---

## Entities

### 1. StateInfo

**Purpose**: Represents a Terraform remote state with comprehensive metadata, dependencies, and outputs.

**Source**: Protobuf payload from the new `ListAllEdges` RPC combined with the existing `StateInfo` message

**Fields**:

| Field | Type | Proto Source | Validation | Notes |
|-------|------|--------------|-----------|-------|
| `guid` | `string` | `guid` | UUIDv7 format | Immutable, client-generated |
| `logic_id` | `string` | `logic_id` | Non-empty, unique | User-friendly identifier |
| `locked` | `boolean` | N/A (future) | - | Currently not in GetStateInfo RPC |
| `created_at` | `string` (ISO 8601) | `created_at` (Timestamp) | Valid date | Converted from protobuf Timestamp |
| `updated_at` | `string` (ISO 8601) | `updated_at` (Timestamp) | Valid date | Converted from protobuf Timestamp |
| `size_bytes` | `number` | N/A (future) | >= 0 | Currently not in GetStateInfo RPC |
| `computed_status` | `string?` | N/A (from ListStates) | Enum: clean, stale, potentially-stale | Aggregate status from edges |
| `dependency_logic_ids` | `string[]` | Derived from `dependencies` | - | Unique producer logic_ids |
| `backend_config` | `BackendConfig` | `backend_config` | Required | Terraform HTTP backend URLs |
| `dependencies` | `DependencyEdge[]` | `dependencies` | - | Incoming edges (consumer) |
| `dependents` | `DependencyEdge[]` | `dependents` | - | Outgoing edges (producer) |
| `outputs` | `OutputKey[]` | `outputs` | - | Available Terraform outputs |

**Relationships**:
- **One-to-One**: `BackendConfig` (embedded)
- **One-to-Many**: `dependencies` (DependencyEdge where this state is `to_guid`)
- **One-to-Many**: `dependents` (DependencyEdge where this state is `from_guid`)
- **One-to-Many**: `outputs` (OutputKey)

**State Transitions**: N/A (read-only in dashboard)

**Validation Rules**:
- `guid` MUST be valid UUIDv7 format
- `logic_id` MUST NOT be empty
- `created_at` MUST be <= `updated_at`
- `dependency_logic_ids` MUST match unique `from_logic_id` values from `dependencies` array

---

### 2. DependencyEdge

**Purpose**: Directed relationship from producer state output to consumer state input, tracking synchronization status.

**Source**: Protobuf `DependencyEdge` message

**Fields**:

| Field | Type | Proto Source | Validation | Notes |
|-------|------|--------------|-----------|-------|
| `id` | `number` | `id` (int64) | Unique, > 0 | Database primary key |
| `from_guid` | `string` | `from_guid` | UUIDv7 | Producer state GUID |
| `from_logic_id` | `string` | `from_logic_id` | Non-empty | Producer state logic ID |
| `from_output` | `string` | `from_output` | Non-empty | Output key name |
| `to_guid` | `string` | `to_guid` | UUIDv7 | Consumer state GUID |
| `to_logic_id` | `string` | `to_logic_id` | Non-empty | Consumer state logic ID |
| `to_input_name` | `string?` | `to_input_name` | - | HCL local variable name override |
| `status` | `string` | `status` | Enum | See Status Values below |
| `in_digest` | `string?` | `in_digest` | SHA256 hash | Consumer's last observed value hash |
| `out_digest` | `string?` | `out_digest` | SHA256 hash | Producer's current value hash |
| `mock_value_json` | `string?` | `mock_value_json` | Valid JSON | Placeholder value for missing outputs |
| `last_in_at` | `string?` (ISO 8601) | `last_in_at` (Timestamp) | Valid date | Last time consumer updated |
| `last_out_at` | `string?` (ISO 8601) | `last_out_at` (Timestamp) | Valid date | Last time producer updated |
| `created_at` | `string` (ISO 8601) | `created_at` (Timestamp) | Valid date | Edge creation time |
| `updated_at` | `string` (ISO 8601) | `updated_at` (Timestamp) | Valid date | Edge last modification time |

**Status Values** (enum):
- `pending`: Edge created, no digest values yet
- `clean`: `in_digest === out_digest` (in sync)
- `dirty`: `in_digest !== out_digest` (out of sync)
- `potentially-stale`: Producer updated recently, consumer not checked
- `mock`: Using `mock_value_json`, real output not available
- `missing-output`: Producer doesn't have required output key

**Visual Mapping** (from clarifications):
- `clean` → Green
- `dirty` → Orange/Yellow
- `pending` → Blue
- `potentially-stale` → Orange/Yellow
- `mock` → Purple
- `missing-output` → Red

**Relationships**:
- **Many-to-One**: `from_guid` → StateInfo (producer)
- **Many-to-One**: `to_guid` → StateInfo (consumer)

**Validation Rules**:
- `from_guid` MUST NOT equal `to_guid` (no self-references)
- If `status === 'mock'`, `mock_value_json` MUST be present
- If `status === 'clean'`, `in_digest === out_digest`
- If `status === 'dirty'`, `in_digest !== out_digest` AND both present

---

### 3. OutputKey

**Purpose**: Metadata about a Terraform output available from a state (key name + sensitivity flag).

**Source**: Protobuf `OutputKey` message

**Fields**:

| Field | Type | Proto Source | Validation | Notes |
|-------|------|--------------|-----------|-------|
| `key` | `string` | `key` | Non-empty | Output name from Terraform state JSON |
| `sensitive` | `boolean` | `sensitive` | - | Whether output marked sensitive in Terraform |

**Notes**:
- Output **values** are NOT exposed (security/size concerns)
- Only keys and sensitivity flags are available
- Used for dependency creation UI and reconciliation guidance

**Validation Rules**:
- `key` MUST NOT be empty
- `key` MUST match pattern `/^[a-zA-Z_][a-zA-Z0-9_]*$/` (valid Terraform identifier)

---

### 4. BackendConfig

**Purpose**: Terraform HTTP backend configuration URLs for a state.

**Source**: Protobuf `BackendConfig` message

**Fields**:

| Field | Type | Proto Source | Validation | Notes |
|-------|------|--------------|-----------|-------|
| `address` | `string` | `address` | Valid URL | Main state endpoint (`/tfstate/{guid}`) |
| `lock_address` | `string` | `lock_address` | Valid URL | Lock endpoint (`/tfstate/{guid}/lock`) |
| `unlock_address` | `string` | `unlock_address` | Valid URL | Unlock endpoint (`/tfstate/{guid}/unlock`) |

**Validation Rules**:
- All URLs MUST have same base URL (API server)
- All URLs MUST use HTTPS in production
- URLs MUST match pattern: `{baseURL}/tfstate/{guid}[/lock|/unlock]`

---

## Type Conversion Mapping

### Protobuf → TypeScript Adapter Types

**Timestamp Conversion**:
```typescript
function timestampToISO(ts: Timestamp | undefined): string {
  if (!ts) return new Date().toISOString();
  return new Date(Number(ts.seconds) * 1000 + ts.nanos / 1000000).toISOString();
}
```

**int64 → number**:
```typescript
// Protobuf int64 → TypeScript number
// Safe for edge IDs (< Number.MAX_SAFE_INTEGER = 2^53-1)
id: Number(protoEdge.id)
```

**Optional Fields**:
- Proto `optional string` → TypeScript `string | undefined`
- Proto `repeated` → TypeScript `Array<T>`

---

## Database Schema Reference

**Existing Tables** (not modified in this feature):

```sql
-- states table (from previous features)
CREATE TABLE states (
  guid UUID PRIMARY KEY,
  logic_id TEXT UNIQUE NOT NULL,
  locked BOOLEAN DEFAULT false,
  lock_metadata JSONB,
  state_json JSONB,
  size_bytes BIGINT,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- dependency_edges table (from previous features)
CREATE TABLE dependency_edges (
  id BIGSERIAL PRIMARY KEY,
  from_guid UUID NOT NULL REFERENCES states(guid) ON DELETE CASCADE,
  from_logic_id TEXT NOT NULL,
  from_output TEXT NOT NULL,
  to_guid UUID NOT NULL REFERENCES states(guid) ON DELETE CASCADE,
  to_logic_id TEXT NOT NULL,
  to_input_name TEXT,
  status TEXT NOT NULL,
  in_digest TEXT,
  out_digest TEXT,
  mock_value_json TEXT,
  last_in_at TIMESTAMPTZ,
  last_out_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW(),
  UNIQUE(from_guid, from_output, to_guid, to_input_name)
);
```

**Repository Query for ListAllEdges** (new):
```go
// cmd/gridapi/internal/repository/interface.go
type StateRepository interface {
    // ... existing methods
    ListAllEdges(ctx context.Context) ([]*models.DependencyEdge, error)
}

// cmd/gridapi/internal/repository/bun_state_repository.go
func (r *BunStateRepository) ListAllEdges(ctx context.Context) ([]*models.DependencyEdge, error) {
    var edges []*models.DependencyEdge
    err := r.db.NewSelect().
        Model(&edges).
        Order("id ASC").
        Scan(ctx)
    return edges, err
}
```

---

## API Contract Mapping

### RPC Calls → Data Entities

1. **ListStates** → `StateInfo[]` (partial - no dependencies/outputs)
   - Returns: `guid`, `logic_id`, `locked`, `created_at`, `updated_at`, `size_bytes`, `computed_status`, `dependency_logic_ids`

2. **GetStateInfo** → `StateInfo` (complete)
   - Returns: Full StateInfo with `dependencies`, `dependents`, `outputs`, `backend_config`

3. **ListAllEdges** (NEW) → `DependencyEdge[]`
   - Returns: All dependency edges in system

### Adapter Responsibilities

The `GridApiAdapter` layer:
1. Calls proto RPCs via Connect clients
2. Converts protobuf types → plain TypeScript types
3. Handles Timestamp → ISO 8601 string conversion
4. Normalizes error responses (gRPC Code → user messages)
5. Maintains interface compatibility with mockApi

**Example Conversion**:
```typescript
function convertProtoDependencyEdge(proto: ProtoDependencyEdge): DependencyEdge {
  return {
    id: Number(proto.id),
    from_guid: proto.fromGuid,
    from_logic_id: proto.fromLogicId,
    from_output: proto.fromOutput,
    to_guid: proto.toGuid,
    to_logic_id: proto.toLogicId,
    to_input_name: proto.toInputName,
    status: proto.status,
    in_digest: proto.inDigest,
    out_digest: proto.outDigest,
    mock_value_json: proto.mockValueJson,
    last_in_at: proto.lastInAt ? timestampToISO(proto.lastInAt) : undefined,
    last_out_at: proto.lastOutAt ? timestampToISO(proto.lastOutAt) : undefined,
    created_at: timestampToISO(proto.createdAt),
    updated_at: timestampToISO(proto.updatedAt)
  };
}
```

---

## Validation Summary

### Required Validations

**Server-Side** (already implemented):
- GUID format (UUIDv7)
- Logic ID uniqueness
- Foreign key constraints (from_guid, to_guid → states)
- Status enum values
- Cycle detection (prevent dependency cycles)

**Client-Side** (adapter layer):
- Timestamp conversion edge cases (null/undefined)
- int64 overflow checks (edge IDs)
- Optional field handling (undefined vs. null)

**UI Layer** (components):
- Empty state handling (no states, no edges)
- Loading states during async operations
- Error display (using normalized messages)

---

## Future Enhancements

**Not in PoC Scope** (deferred):
1. Add `locked`, `size_bytes`, `computed_status` to `GetStateInfo` RPC
2. Batch operations for efficient state list retrieval
3. Pagination for large edge lists
4. Filtering/sorting options on ListAllEdges
5. Subscription/streaming for real-time updates

---

**Status**: ✅ **COMPLETE**
**Next**: Create API contract documentation
