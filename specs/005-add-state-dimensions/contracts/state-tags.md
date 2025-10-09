# Contract: State Label Mutations

**Service**: `gridapi.v1.StateService` (Connect RPC)
**Revision**: Draft – 2025-10-08 (Updated 2025-10-09: Terminology changed from "tags" to "labels")

## RPC: `UpdateStateLabels`
Mutates labels for an existing state (add/replace/remove) with policy validation.

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `state_id` | `string` (UUID) | ✅ | Target state identifier |
| `adds` | `map<string, LabelValue>` | ⛔ | Keys to upsert (last value wins if duplicates present) |
| `removals` | `repeated string` | ⛔ | Keys to drop (`-"key"` CLI syntax maps here) |
| `client_request_id` | `string` | ⛔ | Idempotency token |

`LabelValue` union supports:
- `string_value` (≤256 chars)
- `number_value` (IEEE 754 float64)
- `bool_value`

### Response
```json
{
  "state_id": "3f46d341-95f8-4f12-9703-25955dc2967e",
  "labels": {
    "env": { "stringValue": "staging" },
    "region": { "stringValue": "us-west" }
  },
  "policy_version": 2,
  "compliance_status": "COMPLIANT",
  "updated_at": "2025-10-08T16:04:22Z"
}
```

### Errors
- `INVALID_ARGUMENT`: policy validation failure with per-field errors.
- `FAILED_PRECONDITION`: exceeds 32 label limit or uses disallowed key characters.
- `NOT_FOUND`: state does not exist.

## RPC: `BatchUpdateStateLabels`
Allows mutating multiple states in one call (used by automation).

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `mutations` | `repeated StateLabelMutation` | ✅ | 1..100 items, each similar to `UpdateStateLabels` |

Responses return success/failure per item; failures include validation detail.

## Streaming API (Future Consideration)
`SubscribeStateLabelEvents` stream provides audit feed (label mutations + compliance transitions). Tagged for Phase 2+; contract placeholder only.
