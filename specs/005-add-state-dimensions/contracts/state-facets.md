# Contract: State Facet Management

**Service**: `gridapi.v1.StateFacetService`  
**Revision**: Draft – 2025-10-08

## RPC: `PromoteFacet`
Registers a tag key as a promoted facet and performs DDL to add a projection column.

### Request
```json
{
  "key_name": "team",
  "column_name": "team",
  "create_index": true
}
```

### Response
```json
{
  "facet_id": "b1ea4a8c-1e4d-409d-a6c6-356b7f9e0b57",
  "projection_column": "team",
  "status": "ENABLED",
  "promotion_started_at": "2025-10-08T16:07:12Z"
}
```

### Validation
- `column_name` must match `^[a-z][a-z0-9_]{0,30}$`
- Reject if key already promoted or top-facet budget (≤12) exceeded.

## RPC: `DisableFacet`
Marks facet disabled (projection column retained, projector stops populating).

Request: `{ "facet_id": "...", "reason": "deprecated key" }`  
Response includes timestamps, outstanding backfill jobs if any.

## RPC: `ListFacets`
Provides status board for CLI/UI.

Response snippet:
```json
{
  "facets": [
    {
      "facet_id": "...",
      "key_name": "env",
      "column_name": "env",
      "status": "ENABLED",
      "last_refresh_started_at": "2025-10-08T16:09:00Z",
      "last_refresh_succeeded_at": "2025-10-08T16:09:05Z"
    }
  ]
}
```

## RPC: `TriggerFacetBackfill`
Launches batched projector job (full rebuild or specific facet).

### Request
```json
{
  "facet_id": "b1ea4a8c-1e4d-409d-a6c6-356b7f9e0b57",
  "batch_size": 10000,
  "mode": "FULL"        // FULL | MISSING_ONLY
}
```

### Response
```json
{
  "job_id": "6d0c40fb-0a3d-4c35-a50c-86b07258b7cf",
  "status": "PENDING"
}
```

## RPC: `FacetBackfillStatus`
Returns job progress (rows processed, failures, ETA).

## RPC: `ListFacetCompliance`
Returns per-facet compliance/coverage stats (e.g., rows with NULL values).

## Query Contract: `ListStates`
Facet filters map to equality predicates.

### Request (excerpt)
```json
{
  "filters": [
    { "key": "env", "op": "EQ", "value": { "stringValue": "staging" } },
    { "key": "team", "op": "EQ", "value": { "stringValue": "core" } }
  ],
  "page_size": 20,
  "page_token": ""
}
```

### Response (excerpt)
```json
{
  "states": [...],
  "next_page_token": "opaque",
  "applied_facets": ["env", "team"],
  "non_facet_filters": []
}
```

Invalid filters (non-promoted keys or unsupported operators) return `INVALID_ARGUMENT` with detail `"key not promoted"`.
