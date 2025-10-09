# Contract: State Schema Governance

**Service**: `gridapi.v1.StateSchemaService`  
**Revision**: Draft – 2025-10-08

## RPC: `SetActiveSchema`
Registers a new JSON Schema and makes it active immediately.

### Request
```json
{
  "schema_document": "{...json...}",
  "change_reason": "Baseline schema",
  "client_request_id": "uuid-optional"
}
```

| Constraint | Value |
|------------|-------|
| Document size | ≤ 64 KiB |
| Allowed keywords | `type`, `enum`, `const`, `pattern`, `maxLength`, `description`, `properties`, `required`, `additionalProperties` (boolean), `oneOf` |
| `$ref` support | Disabled |

### Response
```json
{
  "version": 3,
  "activated_at": "2025-10-08T16:05:41Z"
}
```

### Side Effects
- Writes to `dimension_schemas`.
- Appends audit entry with diff vs prior version.
- Marks non-compliant states for follow-up remediation.

## RPC: `ListSchemas`
Paginated list (newest first).

Query params:
- `page_size` (default 20, max 100)
- `page_token`

Response includes `schemas[]` (id, version, created_at, author) and `next_page_token`.

## RPC: `GetSchemaAudit`
Returns audit entries with optional time/version filters. Response payload includes diff summary and links for full diff download.

## RPC: `RevalidateStates`
Triggers server-side revalidation of all or selected states.

### Request
```json
{
  "state_ids": [],          // empty => revalidate all
  "schema_version": 3       // optional; defaults to latest
}
```
### Response
```json
{
  "job_id": "3ebc0c24-82c5-4861-bf1a-3aeea2f24e17",
  "non_compliant": [
    {
      "state_id": "3f46d341-95f8-4f12-9703-25955dc2967e",
      "violations": [
        { "key": "env", "rule": "enum", "expected": ["staging","prod"], "actual": "qa" }
      ]
    }
  ]
}
```

## RPC: `FlushSchemaAudit`
Downloads and truncates historical audit entries.

### Request Parameters
- `before_timestamp` (RFC3339, required)
- `destination` (enum: `DOWNLOAD`, `ARCHIVE_BUCKET`)

### Response
- Provides signed download URL when `DOWNLOAD`.
- Counts deleted rows.
