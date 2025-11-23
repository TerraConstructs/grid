# Output Schema Support Implementation

This document summarizes the implementation of JSON Schema support for State outputs in Grid.

## Overview

This feature adds the ability to publish and retrieve JSON Schema definitions for Terraform/OpenTofu state outputs. This allows:
- Clients to declare expected output types before outputs exist in state
- TypeScript and other clients to generate type-safe code from schemas
- Validation of actual output values against declared schemas (future enhancement)

## Changes Made

### 1. Protobuf Definitions (`proto/state/v1/state.proto`)

**New RPC Methods:**
- `SetOutputSchema`: Publishes or updates a JSON Schema for a specific state output
- `GetOutputSchema`: Retrieves the JSON Schema for a specific state output

**New Messages:**
- `SetOutputSchemaRequest`: Request to set schema (with state reference oneof, output_key, schema_json)
- `SetOutputSchemaResponse`: Response confirming schema was set
- `GetOutputSchemaRequest`: Request to get schema (with state reference oneof, output_key)
- `GetOutputSchemaResponse`: Response with schema JSON

**Updated Messages:**
- `OutputKey`: Added optional `schema_json` field for embedding schemas in output listings

### 2. Authorization (`cmd/gridapi/internal/auth/`)

**New Actions (actions.go):**
- `StateOutputSchemaWrite = "state-output:schema-write"`
- `StateOutputSchemaRead = "state-output:schema-read"`

Both actions added to:
- `ValidateAction()` function
- `ExpandWildcard()` for `StateOutputWildcard`
- Migration seed data for `product-engineer` role (`20251013140501_seed_auth_data.go`)

**Middleware (middleware/authz_interceptor.go):**
- Added authorization cases for `SetOutputSchemaProcedure` and `GetOutputSchemaProcedure`
- Both follow standard pattern: resolve state ID, load state for labels, check permissions

### 3. Database Layer

**Models (`cmd/gridapi/internal/db/models/state_output.go`):**
- Added `SchemaJSON *string` field to `StateOutput` model
- Field is nullable (TEXT type) to allow schemas to be optional

**Migration (`cmd/gridapi/internal/migrations/20251123000001_add_output_schemas.go`):**
- Adds `schema_json TEXT` column to `state_outputs` table
- Includes both `up` and `down` migrations

### 4. Repository Layer (`cmd/gridapi/internal/repository/`)

**Interface (`interface.go`):**
- Updated `OutputKey` struct to include `SchemaJSON *string` field
- Added `SetOutputSchema(ctx, stateGUID, outputKey, schemaJSON)` method
- Added `GetOutputSchema(ctx, stateGUID, outputKey) (string, error)` method

**Implementation (`bun_state_output_repository.go`):**
- `SetOutputSchema`: Upserts schema using `ON CONFLICT DO UPDATE`, creates output record if needed
- `GetOutputSchema`: Returns schema or empty string if not set (not an error)
- `GetOutputsByState`: Updated to include `SchemaJSON` in returned `OutputKey` structs
- `UpsertOutputs`: Preserves existing schemas when Terraform state is uploaded (schemas managed separately)

### 5. Service Layer (`cmd/gridapi/internal/services/state/service.go`)

**New Methods:**
- `SetOutputSchema(ctx, guid, outputKey, schemaJSON)`: Validates state exists and schema is non-empty, delegates to repository
- `GetOutputSchema(ctx, guid, outputKey)`: Validates state exists, delegates to repository

Both methods check if `outputRepo` is configured and return appropriate errors.

### 6. Connect RPC Handlers (`cmd/gridapi/internal/server/connect_handlers_deps.go`)

**New Handlers:**
- `SetOutputSchema`: Resolves state reference (logic_id or guid), calls service, returns confirmation
- `GetOutputSchema`: Resolves state reference, calls service, returns schema JSON

**Updated Handlers:**
- `ListStateOutputs`: Now includes `schema_json` in `OutputKey` proto responses when available

### 7. SDK (`pkg/sdk/`)

**Types (`state_types.go`):**
- Updated `OutputKey` struct to include `SchemaJSON *string` field
- Updated `outputKeyFromProto()` to populate `SchemaJSON` from proto

**Client (`state_client.go`):**
- `SetOutputSchema(ctx, ref, outputKey, schemaJSON)`: Validates inputs, builds request, calls RPC
- `GetOutputSchema(ctx, ref, outputKey)`: Validates inputs, builds request, calls RPC, returns schema

Both methods support `StateReference` (logic_id or guid).

### 8. CLI (`cmd/gridctl/cmd/state/`)

**New Commands:**
- `set-output-schema`: Sets schema from a JSON file on disk
  - Flags: `--output-key`, `--schema-file` (required), `--logic-id`, `--guid` (optional)
  - Supports `.grid` context for state resolution
  - Usage: `gridctl state set-output-schema --output-key vpc_id --schema-file vpc_schema.json my-state`

- `get-output-schema`: Retrieves and displays schema
  - Flags: `--output-key` (required), `--logic-id`, `--guid` (optional)
  - Supports `.grid` context for state resolution
  - Usage: `gridctl state get-output-schema --output-key vpc_id my-state`

Both commands registered in `state.go` init function.

## Implementation Notes

### Schema Storage Strategy

Schemas are stored in the existing `state_outputs` table rather than a separate table because:
1. Schemas are fundamentally metadata about specific outputs
2. The primary key (state_guid, output_key) already provides the needed uniqueness
3. Simplifies queries - no joins needed
4. Allows "pending" outputs - schema can be set before output exists in Terraform state

### Schema Lifecycle

1. **Pre-declaration**: Client can call `SetOutputSchema` before running `terraform apply`
   - Creates output record with `state_serial=0`, `sensitive=false`, `schema_json=<schema>`
   - Allows downstream consumers to see expected type structure

2. **State Upload**: When Terraform uploads state via HTTP backend
   - `UpsertOutputs` updates `state_serial` and `sensitive` but **preserves** `schema_json`
   - Schemas are managed independently of actual output values

3. **Schema Updates**: Client can call `SetOutputSchema` at any time to update schema
   - Uses `ON CONFLICT DO UPDATE` to handle existing records gracefully

### Code Generation Requirement

**IMPORTANT**: This implementation requires running `buf generate` to generate protobuf code:
```bash
buf generate
```

This will generate:
- Go code in `api/state/v1/` (Connect RPC stubs)
- TypeScript code in `js/sdk/gen/` (for web clients)

The code will not compile until this step is completed in your development environment.

## Testing Strategy

Integration tests should cover:
1. Setting schema for non-existent output (creates output record)
2. Setting schema for existing output (updates schema field only)
3. Getting schema returns correct JSON
4. Getting schema for output with no schema returns empty string
5. `ListStateOutputs` includes schemas when available
6. `UpsertOutputs` preserves existing schemas
7. Authorization checks for both RPCs

## Future Enhancements

1. **Schema Validation**: Validate that `schema_json` is valid JSON Schema before storing
2. **Output Validation**: When state is uploaded, validate actual outputs against schemas
3. **Schema Versioning**: Track schema changes over time
4. **Dependency Validation**: Validate that consumed outputs match expected schemas
5. **TypeScript Code Generation**: Automatically generate TypeScript types from schemas

## Usage Example

```bash
# Create state
gridctl state create my-vpc --label env=dev

# Set schema for vpc_id output (before running terraform apply)
cat > vpc_id_schema.json <<EOF
{
  "type": "string",
  "pattern": "^vpc-[a-z0-9]+$",
  "description": "AWS VPC ID"
}
EOF

gridctl state set-output-schema --output-key vpc_id --schema-file vpc_id_schema.json my-vpc

# Get schema
gridctl state get-output-schema --output-key vpc_id my-vpc

# Now run terraform apply - schema will be preserved
cd terraform/vpc
terraform init
terraform apply

# List outputs (will show schema alongside output keys)
gridctl state get my-vpc
```
