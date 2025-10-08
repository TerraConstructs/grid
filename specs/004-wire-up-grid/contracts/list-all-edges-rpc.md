# API Contract: ListAllEdges RPC

**Feature**: 004-wire-up-grid
**RPC Service**: `state.v1.StateService`
**Method**: `ListAllEdges`
**Status**: NEW - To be added to proto

---

## Overview

The `ListAllEdges` RPC retrieves all dependency edges in the Grid system. This provides an efficient alternative to aggregating edges from individual state queries, supporting the dashboard's global graph and list views.

---

## Protobuf Definition

```protobuf
// proto/state/v1/state.proto

service StateService {
  // ... existing RPCs

  // ListAllEdges returns all dependency edges in the system.
  // Used by dashboards and monitoring tools to visualize complete topology.
  rpc ListAllEdges(ListAllEdgesRequest) returns (ListAllEdgesResponse);
}

// ListAllEdgesRequest currently has no parameters.
// Future: Add filtering, pagination, sorting options.
message ListAllEdgesRequest {
  // No fields for PoC (returns all edges)

  // Future enhancements (not in PoC):
  // optional string status_filter = 1;  // e.g., "dirty", "clean"
  // optional int32 page_size = 2;
  // optional string page_token = 3;
}

// ListAllEdgesResponse contains all dependency edges.
message ListAllEdgesResponse {
  repeated DependencyEdge edges = 1;

  // Future enhancements (not in PoC):
  // optional string next_page_token = 2;
  // optional int32 total_count = 3;
}
```

---

## Request

### Fields
*None in PoC version*

**Future Fields** (deferred):
- `status_filter` (string, optional): Filter by edge status ("clean", "dirty", "pending", etc.)
- `page_size` (int32, optional): Number of edges per page
- `page_token` (string, optional): Pagination token from previous response

### Validation
*No validation required for empty request*

### Examples

**Empty Request** (PoC):
```json
{}
```

**Future Filtered Request** (not in PoC):
```json
{
  "status_filter": "dirty"
}
```

---

## Response

### Fields

| Field | Type | Cardinality | Description |
|-------|------|-------------|-------------|
| `edges` | `DependencyEdge` | repeated | All dependency edges in system |

**DependencyEdge Structure** (from existing proto):
| Field | Type | Description |
|-------|------|-------------|
| `id` | `int64` | Unique edge identifier |
| `from_guid` | `string` | Producer state GUID |
| `from_logic_id` | `string` | Producer state logic ID |
| `from_output` | `string` | Producer output key |
| `to_guid` | `string` | Consumer state GUID |
| `to_logic_id` | `string` | Consumer state logic ID |
| `to_input_name` | `string` (optional) | HCL variable name override |
| `status` | `string` | Edge synchronization status |
| `in_digest` | `string` (optional) | Consumer value hash (SHA256) |
| `out_digest` | `string` (optional) | Producer value hash (SHA256) |
| `mock_value_json` | `string` (optional) | Mock value for missing outputs |
| `last_in_at` | `Timestamp` (optional) | Last consumer update time |
| `last_out_at` | `Timestamp` (optional) | Last producer update time |
| `created_at` | `Timestamp` | Edge creation timestamp |
| `updated_at` | `Timestamp` | Edge last modification timestamp |

### Status Values

The `status` field contains one of:
- `pending`: Edge created, no digest values yet
- `clean`: `in_digest` equals `out_digest` (synchronized)
- `dirty`: `in_digest` differs from `out_digest` (out of sync)
- `potentially-stale`: Producer updated, consumer not re-evaluated
- `mock`: Using `mock_value_json`, real output unavailable
- `missing-output`: Producer state doesn't have required output key

### Ordering

Edges returned in ascending order by `id` (database insertion order).

### Examples

**Successful Response**:
```json
{
  "edges": [
    {
      "id": "1",
      "from_guid": "01HZXK1234567890ABCDEF0001",
      "from_logic_id": "prod/network",
      "from_output": "vpc_id",
      "to_guid": "01HZXK1234567890ABCDEF0002",
      "to_logic_id": "prod/cluster",
      "to_input_name": "vpc_id",
      "status": "dirty",
      "in_digest": "sha256:abc123",
      "out_digest": "sha256:def456",
      "created_at": {
        "seconds": "1696176000",
        "nanos": 0
      },
      "updated_at": {
        "seconds": "1696190400",
        "nanos": 0
      }
    },
    {
      "id": "2",
      "from_guid": "01HZXK1234567890ABCDEF0001",
      "from_logic_id": "prod/network",
      "from_output": "data_subnets",
      "to_guid": "01HZXK1234567890ABCDEF0003",
      "to_logic_id": "prod/db",
      "to_input_name": "data_subnets",
      "status": "clean",
      "in_digest": "sha256:stu901",
      "out_digest": "sha256:stu901",
      "created_at": {
        "seconds": "1696176000",
        "nanos": 0
      },
      "updated_at": {
        "seconds": "1696194000",
        "nanos": 0
      }
    }
  ]
}
```

**Empty Response** (no edges):
```json
{
  "edges": []
}
```

---

## Error Cases

| Error Code | Condition | Response |
|------------|-----------|----------|
| `UNAVAILABLE` (14) | Database connection failure | Service temporarily unavailable |
| `INTERNAL` (13) | Query execution error | Internal server error |
| `DEADLINE_EXCEEDED` (4) | Query timeout (>30s) | Request timeout |

### Error Response Examples

**Database Unavailable**:
```json
{
  "code": "unavailable",
  "message": "database connection failed"
}
```

**Query Timeout**:
```json
{
  "code": "deadline_exceeded",
  "message": "query execution exceeded deadline"
}
```

---

## Performance Characteristics

### Expected Performance

- **Typical Response Time**: <100ms for 100-200 edges
- **PoC Scale**: Up to 500 edges, <200ms p95
- **Production Scale**: Up to 10,000 edges, <500ms p95
- **Database Impact**: Simple `SELECT * FROM dependency_edges ORDER BY id` (indexed)

### Optimization Notes

- No JOINs required (edges contain denormalized state references)
- Ordered by primary key (efficient index scan)
- Future: Add pagination if edge count exceeds 10,000

---

## Implementation Checklist

### Backend (Go)

- [ ] Add RPC definition to `proto/state/v1/state.proto`
- [ ] Run `buf generate` to regenerate `./api` module
- [ ] Add `ListAllEdges(ctx) ([]*models.DependencyEdge, error)` to repository interface
- [ ] Implement Bun query in `bun_state_repository.go`
- [ ] Add service method in `cmd/gridapi/internal/state/service.go`
- [ ] Wire up Connect handler in `cmd/gridapi/internal/server/connect_handlers.go`
- [ ] Write handler test in `connect_handlers_test.go`
- [ ] Write repository test in `bun_state_repository_test.go`

### SDK (TypeScript)

- [ ] Run `buf generate` to update `js/sdk/gen/` with new RPC
- [ ] Add `listAllEdges()` method to GridApiAdapter
- [ ] Map protobuf `DependencyEdge[]` → plain TypeScript types
- [ ] Write adapter tests using `createRouterTransport()`

### Webapp

- [ ] Update `useGridData` hook to call `api.getAllEdges()`
- [ ] Replace mockApi.getAllEdges() with SDK call
- [ ] Verify graph view renders live edges
- [ ] Verify list view displays all edges

---

## Testing Strategy

### Contract Tests

**Scenario 1: Empty System**
```
Given: No states or edges exist
When: ListAllEdges called
Then: Returns empty array
```

**Scenario 2: Multiple Edges**
```
Given: 3 states with 8 edges
When: ListAllEdges called
Then: Returns 8 edges in ascending ID order
```

**Scenario 3: Edge Status Variety**
```
Given: Edges with different statuses (clean, dirty, pending, mock)
When: ListAllEdges called
Then: Returns all edges with correct status values
```

### Integration Tests

**Test Case**: Graph View Uses Live Edges
```typescript
test('graph view renders all edges from API', async () => {
  const mockTransport = createRouterTransport(({ service }) => {
    service(StateService, {
      listAllEdges: () => ({
        edges: [
          { id: 1n, from_guid: '...', /* ... */ },
          { id: 2n, from_guid: '...', /* ... */ }
        ]
      })
    });
  });

  const api = new GridApiAdapter(mockTransport);
  const edges = await api.getAllEdges();

  expect(edges).toHaveLength(2);
  expect(edges[0].id).toBe(1);
});
```

---

## Migration Notes

### Before (mockApi Aggregation)

```typescript
// Inefficient: N+1 queries
async getAllEdges(): Promise<DependencyEdge[]> {
  const states = await this.listStates();  // 1 query
  const edges = new Map();

  for (const state of states) {
    const info = await this.getStateInfo(state.logic_id);  // N queries
    info.dependencies.forEach(e => edges.set(e.id, e));
    info.dependents.forEach(e => edges.set(e.id, e));
  }

  return Array.from(edges.values());
}
```

### After (Direct RPC)

```typescript
// Efficient: Single query
async getAllEdges(): Promise<DependencyEdge[]> {
  const response = await this.client.listAllEdges({});
  return response.edges.map(convertProtoDependencyEdge);
}
```

**Performance Improvement**: O(N+1) → O(1) queries

---

## Security Considerations

### Authorization
- **PoC**: No authentication/authorization (read-only, internal tool)
- **Future**: Verify user has permission to view all edges

### Data Exposure
- Edge metadata is visible (GUIDs, logic IDs, output keys)
- Output **values** are NOT exposed (only digest hashes)
- Sensitive outputs marked but values not returned

### Rate Limiting
- **PoC**: No rate limiting
- **Future**: Limit to 10 requests/minute per client

---

## Future Enhancements

**Not in PoC Scope**:

1. **Filtering**:
   ```protobuf
   message ListAllEdgesRequest {
     optional string status_filter = 1;      // "dirty", "clean", etc.
     optional string from_logic_id = 2;      // Producer state filter
     optional string to_logic_id = 3;        // Consumer state filter
   }
   ```

2. **Pagination**:
   ```protobuf
   message ListAllEdgesRequest {
     optional int32 page_size = 4;
     optional string page_token = 5;
   }

   message ListAllEdgesResponse {
     repeated DependencyEdge edges = 1;
     optional string next_page_token = 2;
     optional int32 total_count = 3;
   }
   ```

3. **Sorting**:
   ```protobuf
   message ListAllEdgesRequest {
     optional string order_by = 6;  // "created_at", "updated_at", "status"
     optional bool desc = 7;
   }
   ```

4. **Field Masks** (reduce response size):
   ```protobuf
   import "google/protobuf/field_mask.proto";

   message ListAllEdgesRequest {
     optional google.protobuf.FieldMask field_mask = 8;
   }
   ```

---

**Status**: ✅ **Contract Defined**
**Next**: Implement in backend, regenerate clients, update SDK
