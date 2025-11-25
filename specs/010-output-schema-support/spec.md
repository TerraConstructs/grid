# Feature Specification: Comprehensive JSON Schema Support for Terraform State Outputs

**Feature Branch**: `010-output-schema-support`
**Created**: 2025-11-25
**Status**: Partially Implemented
**Input**: User description: "Add comprehensive JSON Schema support for Terraform/OpenTofu state outputs, enabling clients to declare and retrieve expected output types. specifically per-output JSON Schema RPCs with embedded schema metadata for ease of use. This means: • Clients (especially the TypeScript library) will convert their output interface to JSON Schema and call something like SetOutputSchema for each output (or in bulk). • The server will store these schemas (likely just as strings or JSONB in the database, keyed by state and output name for simplicity). • When new state JSON is uploaded, the backend's processing job can validate outputs against any stored schema and flag mismatches (e.g., marking the dependency edge or output as "schema_mismatch" or logging a warning). - review db models for dependency edge • Clients can retrieve schema info either via GetOutputSchema for a single output or by calling GetStateInfo/ListStateOutputs to see all outputs with their schemas."

## Implementation Status

### ✅ Phase 1: Schema Declaration & Storage (COMPLETED)
- Core schema CRUD operations via RPC methods
- Database persistence with schema preservation during state uploads
- SDK and CLI support for schema management
- Authorization and RBAC integration
- Comprehensive integration testing (8 test functions, 9 fixture files)

### ⏳ Phase 2: Schema Validation (PENDING)
- Output value validation against declared schemas
- Validation status tracking and error reporting
- Dependency edge status updates for schema mismatches
- Background job integration for validation

### ⏳ Phase 3: Webapp UI (PENDING)
- Schema display in web interface
- Validation status visualization
- Schema editing capabilities

## User Scenarios & Testing

### User Story 1 - Pre-declare Expected Output Types (Priority: P1) ✅ COMPLETED

Infrastructure engineers may declare the expected structure and types of Terraform outputs before the infrastructure is provisioned. This enables downstream teams to develop against known contracts while infrastructure is still being built.

**Why this priority**: Establishes the foundational schema declaration capability. Without this, no other schema-related features can function. Enables contract-driven development where API consumers can code against expected output structures before infrastructure exists.

**Independent Test**: Can be fully tested by creating a state, setting output schemas via SDK/CLI, retrieving schemas, and verifying schema persistence without any actual Terraform state upload. Delivers immediate value by documenting expected output contracts.

**Acceptance Scenarios**:

1. **Given** a newly created Terraform state with no outputs yet, **When** an engineer calls SetOutputSchema with a JSON Schema for output "vpc_id", **Then** the schema is stored and retrievable via GetOutputSchema
2. **Given** multiple expected outputs (vpc_id, subnet_ids, security_group_id), **When** engineer sets schemas for each output, **Then** all schemas are stored independently and queryable
3. **Given** a state with pre-declared schemas, **When** engineer calls ListStateOutputs, **Then** all outputs are returned with their associated schemas (even if no actual output values exist yet)
4. **Given** authorized access to a state, **When** engineer attempts to set an output schema, **Then** the operation succeeds if they have "state-output:schema-write" permission
5. **Given** insufficient permissions, **When** engineer attempts to set an output schema, **Then** the operation fails with authorization error

---

### User Story 2 - Update and Manage Schemas Over Time (Priority: P1) ✅ COMPLETED

As infrastructure contracts evolve, engineers need to update output schemas to reflect new requirements without losing existing data or breaking dependent systems.

**Why this priority**: Real-world infrastructure evolves continuously. Schema management must support updates, not just initial declaration. Critical for maintaining accurate contracts as infrastructure requirements change.

**Independent Test**: Can be tested by setting initial schemas, updating them with modified JSON Schema definitions, and verifying that updates replace old schemas atomically without affecting actual output values.

**Acceptance Scenarios**:

1. **Given** an existing output schema for "vpc_id", **When** engineer updates the schema with additional validation rules (e.g., pattern matching for VPC ID format), **Then** the new schema replaces the old one
2. **Given** a state with both schemas and actual output values, **When** engineer updates a schema, **Then** the output values remain unchanged (schemas are metadata, not data)
3. **Given** concurrent schema update requests for the same output, **When** multiple engineers attempt updates simultaneously, **Then** updates are applied atomically without data corruption
4. **Given** a schema update that changes validation rules, **When** engineer retrieves the schema after update, **Then** the returned schema matches the latest version exactly

---

### User Story 3 - Preserve Schemas During State Uploads (Priority: P1) ✅ COMPLETED

When Terraform applies infrastructure changes and uploads new state, previously declared schemas must persist. Engineers should not need to re-declare schemas after every Terraform apply.

**Why this priority**: Without schema preservation, the entire schema declaration system becomes unusable in practice. Schemas would be lost on every Terraform run, requiring manual re-declaration. This is foundational for schema utility.

**Independent Test**: Can be tested by setting output schemas, uploading Terraform state JSON (which includes output values but not schemas), and verifying that declared schemas remain intact and queryable after the upload.

**Acceptance Scenarios**:

1. **Given** output schemas set for vpc_id and subnet_ids, **When** Terraform state JSON is uploaded containing these outputs with actual values, **Then** schemas persist and remain associated with their respective outputs
2. **Given** a state upload that introduces new outputs not covered by existing schemas, **When** the upload completes, **Then** new outputs are created without schemas while existing schemas remain unchanged
3. **Given** a state upload that removes an output that had a schema, **When** the upload completes, **Then** the output and its schema are both removed (schemas follow output lifecycle)
4. **Given** multiple state uploads over time, **When** each upload contains the same set of outputs, **Then** schemas remain consistently associated across all state versions

---

### User Story 4 - Retrieve Schemas via SDK and CLI (Priority: P1) ✅ COMPLETED

Developers consuming Terraform outputs need programmatic access to schemas through Go SDK, TypeScript SDK, and command-line tools to integrate schema information into their workflows.

**Why this priority**: Schema declaration is only useful if consumers can retrieve and use schemas. Multiple access methods (SDK, CLI) serve different use cases: SDK for automation, CLI for interactive debugging and documentation.

**Independent Test**: Can be tested by setting schemas via one method (e.g., Go SDK) and retrieving them via different methods (TypeScript SDK, CLI), verifying consistent schema content across all access patterns.

**Acceptance Scenarios**:

1. **Given** schemas stored for multiple outputs, **When** developer calls GetOutputSchema via Go SDK for a specific output, **Then** the exact JSON Schema is returned
2. **Given** the same schemas, **When** developer runs `gridctl state get-output-schema --logic-id my-state --output-key vpc_id`, **Then** the CLI displays the schema in readable JSON format
3. **Given** a state with outputs and schemas, **When** developer calls ListStateOutputs via TypeScript SDK, **Then** response includes both output values and associated schemas for each output
4. **Given** a state referenced by GUID instead of logic-id, **When** developer retrieves schema using GUID, **Then** schema is returned identically as when using logic-id
5. **Given** context saved in .grid file, **When** developer runs CLI commands without explicit state flags, **Then** commands use context-resolved state reference

---

### User Story 5 - Automatically Infer Schemas from Output Values (Priority: P2) ⏳ PENDING

When Terraform uploads state with output values but no pre-declared schema exists, the system should automatically infer a JSON Schema from the actual output data to provide baseline type safety and documentation. This should be opt-in behavior (no schema = no validation).

**Why this priority**: Provides immediate value for users who don't pre-declare schemas, creating a safety net that catches type changes in future uploads. P2 because it's an enhancement to the core declaration workflow (P1) but doesn't require explicit user action. Creates "discovered" schemas that can later be refined by engineers.

**Independent Test**: Can be tested by uploading Terraform state with various output types (strings, numbers, booleans, objects, arrays) without pre-declaring schemas, then verifying that inferred schemas are stored and retrievable. Subsequent state uploads should validate against the inferred schema.

**Acceptance Scenarios**:

1. **Given** a state with no pre-declared schemas, **When** Terraform uploads state containing output "vpc_id" with value "vpc-abc12345" (string), **Then** system infers and stores a JSON Schema with type "string"
2. **Given** no pre-declared schema for output "config", **When** state is uploaded with nested object `{"region":"us-east-1","zones":3}`, **Then** system infers schema with type "object", properties for "region" (string) and "zones" (integer), and marks both as required
3. **Given** no schema for output "subnet_ids", **When** state contains array `["subnet-123","subnet-456"]`, **Then** system infers schema with type "array" and items type "string"
4. **Given** output "timestamp" with value "2025-11-25T10:30:00Z", **When** schema is inferred, **Then** system detects ISO 8601 format and sets format "date-time"
5. **Given** an existing pre-declared schema for an output, **When** state is uploaded, **Then** system preserves the explicit schema and does NOT overwrite with inferred schema
6. **Given** multiple state uploads with the same output but varying data, **When** inference runs multiple times, **Then** schema inference only occurs on first upload (when schema is missing); subsequent uploads validate against existing schema
7. **Given** inferred schema stored for an output, **When** engineer retrieves it via GetOutputSchema, **Then** response includes metadata indicating schema was auto-inferred (e.g., schema_source: "inferred")

---

### User Story 6 - Validate Outputs Against Schemas (Priority: P2) ⏳ PENDING

When Terraform uploads new state containing output values, the system should automatically validate each output against its declared or inferred schema and report validation results.

**Why this priority**: This is the primary value proposition of schema support - catching contract violations early. However, it depends on Phase 1 (schema declaration) being complete. P2 because schemas are still useful as documentation even without validation.

**Independent Test**: Can be tested by setting a restrictive schema (e.g., vpc_id must match pattern "vpc-[a-f0-9]{8,17}"), uploading state with both valid and invalid output values, and verifying that validation status is correctly recorded and queryable.

**Acceptance Scenarios**:

1. **Given** a schema defining vpc_id as a string matching pattern "vpc-[a-f0-9]{8,17}", **When** Terraform state is uploaded with vpc_id="vpc-abc12345", **Then** validation passes and validation_status is "valid"
2. **Given** the same schema, **When** state is uploaded with vpc_id="invalid-format", **Then** validation fails, validation_status is "invalid", and validation_error contains descriptive message
3. **Given** an output with no declared schema, **When** state is uploaded, **Then** validation is skipped and validation_status is "not_validated"
4. **Given** a complex schema with nested objects and arrays, **When** state is uploaded with outputs matching the schema structure, **Then** validation succeeds with all nested properties validated
5. **Given** validation failures on one output, **When** other outputs in the same state are valid, **Then** each output has independent validation status

---

### User Story 6 - Track Dependency Schema Mismatches (Priority: P2) ⏳ PENDING

When a state depends on outputs from another state, and those outputs fail schema validation, the dependency edge should be marked with "schema-invalid" status to alert engineers of contract violations.

**Why this priority**: Extends validation to the dependency graph, enabling teams to detect breaking changes in upstream infrastructure. P2 because it requires both validation (US-5) and dependency tracking to be functional.

**Independent Test**: Can be tested by creating two states with a dependency relationship, setting schemas on the upstream state's outputs, uploading invalid output values, and verifying the dependency edge reflects the schema violation status.

**Acceptance Scenarios**:

1. **Given** State A depends on State B's output "vpc_id", **When** State B uploads a vpc_id that fails schema validation, **Then** the dependency edge from A to B is marked with status "schema-invalid"
2. **Given** a schema-invalid edge, **When** engineers query State A's dependencies, **Then** the edge status clearly indicates which output failed validation and why
3. **Given** State B subsequently uploads a valid vpc_id, **When** validation passes, **Then** the dependency edge status is updated to "valid" automatically
4. **Given** multiple outputs in a dependency (vpc_id valid, subnet_ids invalid), **When** validation runs, **Then** edge status reflects the most severe validation failure (invalid takes precedence)

---

### User Story 7 - View Schemas in Web Interface (Priority: P3) ⏳ PENDING

Engineers using the Grid webapp need to view output schemas, validation statuses, and schema-related errors directly in the UI without switching to CLI or SDK.

**Why this priority**: Improves user experience and accessibility for engineers who prefer graphical interfaces. P3 because CLI and SDK already provide full functionality; webapp is a convenience layer.

**Independent Test**: Can be tested by setting schemas and validation results via API, then loading the webapp and verifying that schemas, validation statuses, and errors are displayed correctly in the state detail view.

**Acceptance Scenarios**:

1. **Given** a state with outputs and schemas, **When** engineer opens the state detail modal in webapp, **Then** a new "Outputs" tab displays each output with its schema preview
2. **Given** an output with validation failure, **When** engineer views the output in webapp, **Then** validation status is shown with error message and timestamp
3. **Given** dependency edges with schema-invalid status, **When** engineer views the graph visualization, **Then** invalid edges are visually distinguished (e.g., red dashed lines)
4. **Given** a JSON Schema for an output, **When** engineer clicks to expand schema details, **Then** the full schema is displayed in readable, formatted JSON

---

### Edge Cases

#### Schema Declaration Edge Cases
- **Empty Schema**: What happens when SetOutputSchema is called with an empty JSON object `{}`? System should accept it as a valid (but permissive) schema
- **Invalid JSON Schema**: What happens when SetOutputSchema receives malformed JSON or non-schema JSON? System should validate it's valid JSON Schema syntax and reject invalid schemas with clear error messages
- **Schema Size Limits**: What happens when a schema exceeds database column limits (e.g., 1MB)? System should reject schemas above configurable size threshold with error indicating limit
- **Special Characters in Output Keys**: What happens when output keys contain special characters (dots, slashes, Unicode)? System should handle any valid Terraform output name without escaping issues
- **Concurrent Schema Operations**: What happens when multiple clients set/update schemas for the same output simultaneously? Database transactions ensure atomic updates; last write wins with no data corruption

#### Schema Preservation Edge Cases
- **State Upload Without Outputs**: What happens when Terraform state is uploaded with zero outputs but schemas exist? Schemas remain in database as "pending" until outputs appear
- **Output Removed Then Re-added**: What happens when an output is removed in one Terraform apply, then re-added in a later apply? Schema persists if output is re-added within the same state; behaves like update
- **Bulk State Upload**: What happens when a state upload contains 100+ outputs? Schema preservation operates per-output with batch database operations for efficiency
- **State Rollback**: What happens when engineers rollback Terraform state to a previous version? Schemas persist independently of state version; they represent expected contract, not historical values

#### Schema Inference Edge Cases
- **Ambiguous Types**: What happens when inferring schema from output value "123" (could be string or number)? Inference uses actual JSON type from Terraform state; if JSON contains number 123, infers integer; if string "123", infers string
- **Null Values**: What happens when output has null value during first upload (when inference would run)? System infers a permissive schema that allows null but cannot determine specific type; engineer should manually set schema or wait for non-null value
- **Empty Arrays/Objects**: What happens when inferring from empty array `[]` or empty object `{}`? System infers type "array" or "object" but cannot determine item/property schemas; subsequent uploads with populated data do NOT update inferred schema
- **Inference Conflicts with Manual Schema**: What happens when engineer manually sets schema after system inferred one? Manual schema always takes precedence; SetOutputSchema overwrites inferred schema completely
- **Multi-Sample Inference**: What happens when engineer wants to improve inferred schema by providing multiple samples? Inference only runs once on first state upload; to use multi-sample inference, engineer must collect samples externally and manually set schema
- **Required Field Detection**: What happens when inferring from single JSON object sample? All fields present in the sample are marked as required; this may be overly strict but safer than under-constraining
- **Deeply Nested Structures**: What happens when inferring schema from complex nested JSON (5+ levels deep)? Inference handles arbitrary depth but may produce verbose schemas; no depth limit imposed
- **Inferred Schema Metadata**: What happens when engineer wants to distinguish inferred vs manually-declared schemas? Schema retrieval includes schema_source field indicating "inferred" or "manual"

#### Validation Edge Cases
- **Schema Draft Version Mismatch**: What happens when a schema uses JSON Schema Draft 2020-12 but validator supports only Draft 7? Validation fails with error indicating unsupported draft version
- **Null vs Undefined Outputs**: What happens when Terraform output exists but has null value? Validation treats null as a value; schemas can explicitly allow or disallow null
- **Validation Timeout**: What happens when schema validation takes longer than request timeout (e.g., extremely complex schemas)? Validation is performed asynchronously in background job; immediate response indicates "validating" status
- **Circular Schema References**: What happens when a schema contains $ref cycles? Validation library handles circular references per JSON Schema spec; system does not impose additional restrictions
- **Schema Changed During Validation**: What happens when a schema is updated while background validation is running? Validation completes with the schema version that existed at start of validation; next state upload uses new schema

#### Authorization Edge Cases
- **Cross-Environment Schema Access**: What happens when an engineer with env=dev scope tries to set schemas on an env=prod state? Authorization check fails; operation rejected with 403 Forbidden
- **Partial Permission**: What happens when an engineer has schema-read but not schema-write? GetOutputSchema succeeds; SetOutputSchema fails with insufficient permission error
- **Schema Retrieval on Unauthorized State**: What happens when listing all states, some with schemas, but user lacks schema-read on some states? States are returned but schema_json fields are null/omitted for unauthorized states

#### Dependency Edge Cases
- **Dependency Created Before Schema**: What happens when State A declares dependency on State B's output, but State B has no schema for that output? Dependency is valid; edge status is "valid" (absence of schema is not a failure)
- **Transitive Schema Validation**: What happens when State A → State B → State C and State C's output fails validation? Only the direct edge (B → C) is marked schema-invalid; transitive edges reflect indirect impact
- **Self-Referential State**: What happens when a state attempts to depend on its own outputs? Dependency graph prevents cycles; this is blocked at dependency creation, not schema validation

#### UI/UX Edge Cases
- **Large Schema Display**: What happens when a schema is 50KB+ and displayed in webapp? UI truncates display with "show more" expansion; full schema available via download
- **Real-Time Validation Updates**: What happens when validation status changes while engineer is viewing the state in webapp? UI polls for updates or uses WebSocket to reflect status changes live
- **Schema Diff on Update**: What happens when an engineer wants to see what changed between schema versions? System does not track schema history by default; this would require version control integration (out of scope)

## Requirements

### Functional Requirements

#### Schema Declaration & Storage (✅ Completed)

- **FR-001**: System MUST provide RPC methods `SetOutputSchema` and `GetOutputSchema` for managing per-output JSON Schemas
- **FR-002**: System MUST store output schemas in the database keyed by state identifier (GUID) and output name
- **FR-003**: System MUST support schemas for outputs that do not yet exist in Terraform state (pre-declaration)
- **FR-004**: System MUST preserve previously set schemas when Terraform state JSON is uploaded with output values
- **FR-005**: System MUST allow schema updates by upserting (update if exists, insert if new)
- **FR-006**: System MUST return schemas as part of `ListStateOutputs` RPC responses
- **FR-007**: System MUST validate that submitted schemas are syntactically valid JSON before storage (parse validation only; full JSON Schema Draft 7 compliance validation is deferred to future iterations)
- **FR-008**: System MUST support state references via both logic-id (user-readable) and GUID (immutable)

#### Authorization & Access Control (✅ Completed)

- **FR-009**: System MUST enforce authorization for schema operations using two distinct actions: `state-output:schema-write` and `state-output:schema-read`
- **FR-010**: System MUST scope schema access by state labels (e.g., env=dev, env=prod) consistent with state access policies
- **FR-011**: System MUST reject schema write operations for users lacking `state-output:schema-write` permission on the target state
- **FR-012**: System MUST reject schema read operations for users lacking `state-output:schema-read` permission on the target state

#### SDK & CLI Support (✅ Completed)

- **FR-013**: Go SDK MUST provide `SetOutputSchema` and `GetOutputSchema` client methods
- **FR-014**: TypeScript SDK MUST provide generated type-safe methods for schema operations via Connect-Web
- **FR-015**: CLI MUST provide `gridctl state set-output-schema` command accepting `--output-key` and `--schema-file` flags
- **FR-016**: CLI MUST provide `gridctl state get-output-schema` command to retrieve and display schemas
- **FR-017**: CLI commands MUST support state resolution via .grid context files
- **FR-018**: CLI commands MUST support explicit state reference via `--logic-id` or `--guid` flags

#### Schema Inference (⏳ Pending)

- **FR-019**: System MUST automatically infer JSON Schema from output values when state is uploaded and no schema exists for that output
- **FR-020**: System MUST use JSON Schema Draft 7 format for inferred schemas to maintain consistency with manually declared schemas
- **FR-021**: System MUST detect and infer basic JSON types: string, number, integer, boolean, object, array, null
- **FR-022**: System MUST infer nested object properties recursively, creating schemas for all nested levels
- **FR-023**: System MUST infer schemas that accurately represent common Terraform output types (implementation MAY include heuristics such as ISO 8601 date-time format detection)
- **FR-024**: System SHOULD mark object fields as required based on inference heuristics; fields with null values SHOULD NOT be marked required (see Scope Limitations for configurable inference modes)
- **FR-025**: System MUST NOT overwrite existing schemas (manually declared or previously inferred) during state uploads
- **FR-026**: System MUST store metadata indicating schema source (manual vs inferred) with each schema
- **FR-027**: System MUST run inference only once per output (on first state upload when no schema exists)
- **FR-028**: System MUST include schema_source field in GetOutputSchema and ListStateOutputs responses

#### Schema Validation (⏳ Pending)

- **FR-029**: System MUST validate output values against declared schemas when Terraform state is uploaded
- **FR-030**: System MUST record validation results in database with fields: validation_status (valid/invalid/not_validated), validation_error (text), validated_at (timestamp)
- **FR-031**: System MUST use JSON Schema Draft 7 specification for validation
- **FR-032**: System MUST perform validation asynchronously in background jobs to avoid blocking state uploads
- **FR-033**: System MUST skip validation for outputs with no declared schema (validation_status = "not_validated")
- **FR-034**: System MUST include validation status in `GetOutputSchema` and `ListStateOutputs` responses
- **FR-035**: System MUST store validation errors as structured JSON containing at minimum: `path` (JSON path to failing property), `expected` (expected type/constraint), `actual` (actual value, truncated), `message` (human-readable description)

#### Dependency Edge Status (⏳ Pending)

- **FR-036**: System MUST update dependency edge status to "schema-invalid" when a depended-upon output fails schema validation
- **FR-037**: System MUST update edge status atomically with validation status to prevent race conditions
- **FR-038**: System MUST clear "schema-invalid" status when subsequent validation passes
- **FR-039**: System MUST expose edge validation status in dependency query responses

#### Webapp UI (⏳ Pending)

- **FR-040**: Webapp MUST display output schemas when viewing state details (specific UI/UX design deferred to implementation; see specs/010-output-schema-support/webapp-output-schema-design.md for detailed design documentation)

### Key Entities

- **StateOutput**: Represents a single output from a Terraform/OpenTofu state. Key attributes: state GUID, output key (name), output value (JSON), schema JSON (optional), schema source (manual/inferred), validation status, validation error, validated timestamp, serial number (for output lifecycle tracking)

- **OutputSchema**: Logical concept (embedded in StateOutput, not a separate table). Represents a JSON Schema document declaring the expected structure and constraints for an output value. Key attributes: JSON Schema content, association with specific state and output key, schema source indicator (manual vs inferred)

- **DependencyEdge**: Represents a dependency relationship between two states, specifically tracking which output(s) from one state are consumed by another. Key attributes: source state GUID, target state GUID, output key(s), edge status (valid/schema-invalid/other), validation-related metadata

- **ValidationResult**: Logical concept (embedded in StateOutput). Represents the outcome of validating an output value against its schema. Key attributes: validation status (enum: valid/invalid/not_validated), validation error message, validated timestamp

- **InferredSchema**: Logical concept (OutputSchema with schema_source="inferred"). Represents a JSON Schema automatically generated from output value inspection rather than explicitly declared by engineers. Key attributes: same as OutputSchema but created through inference process, marked with inferred source

## Success Criteria

### Measurable Outcomes

- **SC-001**: Engineers can declare output schemas for 100% of expected outputs before Terraform infrastructure is provisioned, enabling contract-driven development
- **SC-002**: Schema declarations persist across unlimited Terraform apply cycles without manual re-declaration
- **SC-003**: 95% of schema validation operations complete within 2 seconds for schemas up to 10KB and outputs up to 100KB
- **SC-004**: Zero data loss or corruption occurs during concurrent schema updates by multiple engineers
- **SC-005**: Engineers can retrieve output schemas via any access method (Go SDK, TypeScript SDK, CLI) with consistent results in under 500ms
- **SC-006**: Validation error messages MUST include: (a) output key, (b) expected type/constraint, (c) actual value (truncated to 100 chars), (d) JSON path to failure point
- **SC-007**: Dependency edges reflect schema validation status within 5 seconds of state upload completion
- **SC-008**: Webapp displays schema information and validation status with zero additional API calls beyond existing state detail queries (data included in batch responses)
- **SC-009**: For outputs with non-null values and no pre-declared schema, system attempts best-effort schema inference on first state upload; inference failures are logged but do not block state upload
- **SC-010**: Inferred schemas accurately represent output structure with 95% precision for common Terraform output types (strings, numbers, objects, arrays)

### Previous Work

No directly related previous features identified in Beads issue tracker. This is a new capability for the Grid system.

Related foundational work:
- **001-develop-the-grid**: Established core state management, database schema, and RPC framework
- **002-add-state-dependency**: Created dependency edge model and graph relationships (extends with schema-invalid status in this feature)
- **006-authz-authn-rbac**: Implemented authorization framework reused for schema-write and schema-read actions
- **007-webapp-auth**: Created webapp authentication flow extended by schema display UI in Phase 3

## Assumptions

1. **JSON Schema Version**: Validation uses JSON Schema Draft 7 (widely supported, stable specification). Future support for newer drafts (2019-09, 2020-12) may be added based on library support
2. **Schema Size**: Individual schemas are limited to 1MB to prevent database performance issues. Complex schemas beyond this size should be refactored or split
3. **Validation Performance**: Validation library can process typical schemas (under 10KB) against typical outputs (under 100KB) in under 2 seconds. Complex schemas may require optimization or async processing
4. **Schema Storage Format**: Schemas are stored as TEXT (not JSONB) to avoid validation overhead on storage and to simplify future SQLite adoption. Schema source metadata (manual/inferred) stored in additional column. Note: JSONB migration for query capabilities is deferred (see Future Considerations)
5. **Validation Library**: `github.com/santhosh-tekuri/jsonschema/v6` provides sufficient Draft 7 support with acceptable performance. Alternative libraries may be considered if performance issues arise
6. **Inference Library**: `github.com/JLugagne/jsonschema-infer` provides Draft 7 schema inference from JSON samples with support for type detection, nested objects, arrays, and ISO 8601 date-time format detection
7. **Background Jobs**: Existing background job infrastructure (or new job queue system) is available for asynchronous validation processing
8. **Dependency Edge Model**: Existing `DependencyEdge` database model can be extended with validation status fields without major schema migration issues
9. **Webapp Technology**: React webapp with @connectrpc/connect-web can display JSON data with syntax highlighting using standard React libraries
10. **Schema Evolution**: Schema versions are not tracked historically. Engineers update schemas in place; old schema versions are not retained (unless external version control is used)
11. **Circular Dependencies**: Dependency graph already prevents circular state dependencies; schema validation does not introduce new circular reference concerns beyond what JSON Schema spec handles
12. **Inference Accuracy**: Single-sample inference may produce overly strict schemas (all fields marked required). Multi-sample inference improves accuracy but requires manual schema management by engineers

## Scope Limitations

This section documents known limitations of the current design that are intentionally deferred to maintain Alpha/PoC simplicity per Constitution Principle VII (Simplicity & Pragmatism).

### Schema Reusability Across Environments

**Limitation**: Schemas are stored per-state-per-output. Identical infrastructure components deployed across environments (dev/staging/prod) require duplicate schema declarations.

**IaC Reality**: Terraform modules are designed for reuse. Environment promotion (dev→staging→prod) is the standard deployment model. Tools like [fogg](https://github.com/chanzuckerberg/fogg) with [gridops](https://github.com/vincenthsh/fogg/tree/main/cmd/gridops) generate boilerplate for multiple environments from shared module definitions. When schemas are managed via CDKTF→TypeScript→JSON Schema pipelines, the same schema applies across all instances of a module.

**Current Workaround**: Use CLI scripts or SDK automation to batch-set identical schemas across states sharing the same component type. Example pattern:
```bash
# Set schema for all network components
for env in dev staging prod; do
  gridctl state set-output-schema --logic-id "network-${env}" \
    --output-key vpc_id --schema-file schemas/network/vpc_id.json
done
```

**Future**: Schema Templates (see Future Considerations) will allow defining schemas once and associating with multiple states via labels or component identifiers.

Or `gridctl state create --output-schema-from <state-ref>` to copy all output schemas from another state.

### Schema Ownership Model

**Limitation**: Schema updates follow last-write-wins semantics. No distinction between module authors, platform teams, and consumers.

**IaC Reality**: Different stakeholders have different schema needs:
- Module authors define baseline contracts
- Platform teams enforce organization-wide validation patterns
- Consumer teams may need stricter validation for their use case

**Current Workaround**: Establish team conventions for who owns schema definitions. Use RBAC labels (e.g., `schema-owner=platform-team`) to control write access.

**Future**: Schema ownership tiers or layered schemas may be considered if the single-owner model proves insufficient.

### Required Field Inference Accuracy

**Limitation**: Single-sample inference marks all present fields as required, which may be overly strict for outputs with optional/conditional fields.

**IaC Reality**: Terraform outputs often include optional fields:
```hcl
output "nat_gateway_id" {
  value = var.enable_nat ? aws_nat_gateway.main[0].id : null
}
```

Inference from a sample where `enable_nat=true` creates a schema requiring `nat_gateway_id`, causing validation failures in environments where `enable_nat=false`.

**Current Workaround**: For outputs with conditional fields, pre-declare schemas manually instead of relying on inference. Use `"type": ["string", "null"]` to allow null values.

**Future**: Configurable inference modes (strict/permissive) or multi-sample inference aggregation may improve accuracy.

### JSON Schema Draft Version

**Limitation**: Only JSON Schema Draft 7 is supported. Schemas using Draft 2019-09 or Draft 2020-12 features will not validate correctly.

**Current Workaround**: Convert schemas to Draft 7 syntax before uploading.

**Future**: Library upgrades may enable newer draft support based on ecosystem adoption.

## Future Considerations

These items are documented for potential future implementation but are explicitly out of scope for the current feature.

### Schema Templates / Shared Definitions

**Concept**: Define a schema template once and associate it with multiple states via labels, component identifiers, or explicit linking.

**Use Case**: Infrastructure teams maintaining network components across 10+ environments want to update the `vpc_id` schema once, not 10 times.

**Potential Implementation**:
1. New `schema_templates` table with template definitions
2. `state_outputs.schema_template_id` FK for linking
3. Template resolution at validation time (template OR inline schema)
4. CLI: `gridctl schema-template create`, `gridctl state link-schema-template`

**Complexity**: Medium. Requires new entity, migration, and template resolution logic.

### JSONB Schema Storage

**Concept**: Migrate `schema_json` column from TEXT to JSONB for PostgreSQL deployments to enable in-database schema querying.

**Use Case**: Query all states with schemas containing specific patterns (e.g., "find all outputs with type=string and pattern constraint").

**Consideration**: JSONB is PostgreSQL-specific. SQLite support (potential future database target) lacks equivalent JSONB functionality. Current TEXT storage maintains database portability.

**Research Needed**: Evaluate actual query patterns before committing to JSONB migration.

### Schema Size Limits

**Concept**: Enforce configurable schema size limits with clear error codes.

**Current State**: Terraform state size is monitored via `X-Grid-State-Size-Warning` header when exceeding 10MB threshold (see `tfstate_handlers.go:130-132`). Similar pattern could apply to schemas.

**Research Needed**: Determine appropriate schema size thresholds based on real-world usage. 1MB assumption may be too generous or too restrictive.

**Potential Implementation**: Add `X-Grid-Schema-Size-Warning` header and reject schemas exceeding configurable limit with `schema_too_large` error code.

### Edge Schema Validation Status (Orthogonal Model)

**Concept**: Separate schema validation status from data freshness status on dependency edges.

**Current Model**: `EdgeStatus` enum tracks data freshness (clean/dirty/stale/mock/missing-output). Adding `schema-invalid` conflates data state with contract compliance.

**Proposed Model**: Add separate fields to Edge model:
```go
type Edge struct {
    Status            EdgeStatus `bun:"status"` // Data freshness
    SchemaValidation  *string    `bun:"schema_validation"` // null, "valid", "invalid"
    SchemaError       *string    `bun:"schema_error,type:text"`
}
```

**Benefit**: Independent queries ("show dirty edges" vs "show schema-invalid edges") and cleaner state machine.

**Decision Point**: Evaluate during Phase 2B implementation whether orthogonal model is warranted.

### Full JSON Schema Compliance Validation

**Concept**: Validate that submitted schemas are compliant JSON Schema documents, not just valid JSON.

**Current State**: FR-007 requires JSON parse validation only. Full schema compliance (valid `$schema`, `type`, `properties` structure) is deferred.

**Potential Implementation**: Use validation library to compile schema on SetOutputSchema; reject schemas that fail compilation with descriptive errors.

**Trade-off**: Adds latency to schema write operations but catches invalid schemas early.

### Multi-Sample Schema Inference

**Concept**: Improve inference accuracy by aggregating multiple state upload samples before finalizing inferred schema.

**Use Case**: Output field present in 3 of 5 samples should be marked optional, not required.

**Complexity**: High. Requires tracking sample count, deferred schema finalization, and UI for "inference in progress" state.

**Current Position**: Single-sample inference with manual refinement is acceptable for Alpha/PoC.

## Notes

### Implementation Phases

**Phase 1 (COMPLETED)**: Schema Declaration & Storage
- 7 commits implementing core functionality
- RPC methods, database schema, SDK/CLI, authorization, integration tests
- 6,176 lines added across 46 files
- Documented in OUTPUT_SCHEMA_IMPLEMENTATION.md

**Phase 2A (PENDING)**: Schema Inference
- Automatic schema generation from output values using JLugagne/jsonschema-infer library
- Database schema extension to track schema source (manual vs inferred)
- Integration with state upload workflow to trigger inference for outputs without schemas
- Integration tests for various output types (primitives, objects, arrays, nested structures)
- Estimated 3-5 days of implementation effort

**Phase 2B (PENDING)**: Schema Validation
- Planned implementation documented in OUTPUT_VALIDATION.md
- 8-day phased rollout with library integration, validation logic, background jobs, edge updates
- Validation against both manually declared and inferred schemas
- Estimated 1,057 lines of documentation already prepared

**Phase 3 (PENDING)**: Webapp UI
- Planned design documented in specs/010-output-schema-support/webapp-output-schema-design.md
- 1,034 lines of design documentation with component specs, TypeScript interface updates, visual designs
- Schema source indicators (manual/inferred badges) in UI
- Estimated 15 hours of implementation effort (P0-P2 tasks)

### Key Design Decisions

1. **Single Table Storage**: Schemas stored in `state_outputs` table with `schema_json` column rather than separate `output_schemas` table. Simplifies queries and enables "pending" outputs (schema exists but no value yet)

2. **Dual State References**: Support both logic-id (mutable, user-friendly) and GUID (immutable, system-level) for maximum flexibility across use cases

3. **Schema Preservation**: Upsert logic in `UpsertOutputs` explicitly preserves `schema_json` when Terraform uploads state. Critical for usability

4. **Authorization Granularity**: Two separate actions (schema-write, schema-read) rather than combining with generic state permissions. Enables fine-grained RBAC policies

5. **Validation Async**: Validation runs in background jobs, not inline during state upload, to prevent blocking Terraform operations

6. **No Schema Versioning**: Schemas are updated in place without version history. Engineers should use external version control (Git) if schema history is needed

7. **Inference Trigger**: Schema inference runs automatically during state upload only when no schema exists for an output. Once-per-output inference prevents performance degradation and maintains explicit schemas as source of truth. Manual schemas always take precedence over inference

8. **Single-Sample Inference**: Inference uses only the current output value (single sample) rather than aggregating multiple state versions. Simpler implementation with acceptable accuracy for typical Terraform outputs. Engineers can manually refine schemas if multi-sample precision is needed

### Testing Strategy

**Integration Tests (Completed)**:
- 8 test functions covering UC1-UC8
- 9 fixture files (3 Terraform states, 6 JSON schemas)
- Mode 1 OIDC/RBAC tests for authorization enforcement
- All tests passing with race detector clean

**Inference Tests (Pending)**:
- Schema inference from primitive types (string, number, integer, boolean, null)
- Inference from complex types (objects, arrays, nested structures)
- ISO 8601 date-time format detection
- Required field detection from single samples
- Schema source metadata verification (manual vs inferred)
- Inference precedence tests (manual schemas not overwritten)

**Validation Tests (Pending)**:
- Schema validation library integration tests
- Complex schema validation (nested objects, arrays, pattern matching)
- Validation against inferred schemas
- Background job validation processing
- Edge status update tests

**Webapp Tests (Pending)**:
- E2E tests for schema display UI
- Schema source indicator display (manual/inferred badges)
- Visual regression tests for schema validation status indicators
- Accessibility tests for schema viewer components

### Documentation

- **OUTPUT_SCHEMA_IMPLEMENTATION.md**: Implementation details and design decisions (193 lines)
- **OUTPUT_VALIDATION.md**: Validation implementation plan (1,057 lines)
- **INTEGRATION_TESTS_SUMMARY.md**: Test coverage and execution guide (203 lines)
- **specs/010-output-schema-support/webapp-output-schema-design.md**: UI/UX design for webapp schema display (1,034 lines)
