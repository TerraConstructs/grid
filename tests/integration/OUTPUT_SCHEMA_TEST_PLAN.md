# Output Schema Integration Test Plan

## Test Use Cases

### UC1: Basic Schema Operations
**Scenario**: Create state, set schemas for outputs, retrieve schemas
**Steps**:
1. Create state A
2. Set JSON Schema for output "vpc_id" (string pattern schema)
3. Set JSON Schema for output "subnet_ids" (array of strings schema)
4. Get schema for "vpc_id" - verify correct schema returned
5. Get schema for "subnet_ids" - verify correct schema returned
6. Get schema for non-existent output - verify empty string
7. List outputs - verify schemas embedded in response

**Expected**: All schemas stored and retrieved correctly

### UC2: Schema Pre-Declaration
**Scenario**: Set schema before output exists in Terraform state
**Steps**:
1. Create state A
2. Set schema for "vpc_id" BEFORE uploading TF state
3. List outputs - verify "vpc_id" appears with schema (pending)
4. Upload TF state with "vpc_id" output
5. List outputs - verify "vpc_id" still has schema and actual value exists

**Expected**: Schema persists through state upload

### UC3: Schema Update
**Scenario**: Update existing schema
**Steps**:
1. Create state A
2. Set schema v1 for "vpc_id"
3. Get schema - verify v1
4. Set schema v2 for "vpc_id" (different pattern)
5. Get schema - verify v2 (updated)

**Expected**: Schema updates correctly

### UC4: Schema Preservation During State Upload
**Scenario**: Verify schemas not lost when TF state uploaded
**Steps**:
1. Create state A
2. Set schema for "vpc_id" and "subnet_ids"
3. Upload TF state with outputs (serial=1)
4. Verify schemas still present
5. Upload new TF state (serial=2)
6. Verify schemas STILL present (not cleared)

**Expected**: Schemas preserved across state updates

### UC5: Schema with Dependencies (Happy Path)
**Scenario**: Producer has schema, consumer depends on it
**Steps**:
1. Create state A (producer) and B (consumer)
2. Set schema on A's "vpc_id" output (must be "vpc-*" pattern)
3. Add dependency: A.vpc_id -> B
4. Upload TF state to A with vpc_id="vpc-12345" (matches schema)
5. Verify edge status updates correctly

**Expected**: Dependency tracking works with schemas

### UC6: Schema Validation Context
**Scenario**: Prepare for future schema validation
**Steps**:
1. Create state A
2. Set strict schema for "vpc_id" (pattern: "^vpc-[a-z0-9]+$")
3. Upload TF state with matching value
4. Verify no errors
5. (Future) Upload mismatched value - should flag warning

**Expected**: System ready for schema validation

### UC7: Complex Schemas (Nested Objects)
**Scenario**: Test complex JSON Schema types
**Steps**:
1. Create state A
2. Set schema for "config" output (object with properties)
3. Set schema for "tags" output (object with pattern properties)
4. Retrieve and verify schemas

**Expected**: Complex schemas stored correctly

### UC8: Schema with gridctl CLI
**Scenario**: Use CLI commands to manage schemas
**Steps**:
1. Create state via CLI
2. Create schema JSON file on disk
3. Run: gridctl state set-output-schema --output-key vpc_id --schema-file schema.json
4. Run: gridctl state get-output-schema --output-key vpc_id
5. Verify schema matches file content

**Expected**: CLI commands work correctly

## Test Organization

### Test Files
- `output_schema_test.go` - Basic tests (no auth)
- `auth_mode1_output_schema_test.go` - Mode 1 OIDC tests (already exists, extend)
- `auth_mode2_output_schema_test.go` - Mode 2 internal IdP tests (create)

### Test Fixtures
Create in `tests/integration/testdata/`:
- `vpc_output_with_schema.json` - TF state with VPC outputs
- `subnet_output_with_schema.json` - TF state with subnet outputs
- `complex_output.json` - TF state with complex nested outputs
- `schema_vpc_id.json` - JSON Schema for VPC ID (string with pattern)
- `schema_subnet_ids.json` - JSON Schema for subnet IDs (array of strings)
- `schema_config_object.json` - JSON Schema for complex object
- `schema_tags.json` - JSON Schema for tags map
- `mismatched_output.json` - TF state with output that doesn't match schema

### Validation Strategy
1. Use SDK for programmatic tests
2. Use gridctl for CLI tests
3. Verify HTTP responses for edge cases
4. Test both with and without authentication
5. Verify schemas embedded in ListOutputs responses
6. Test state resolution (logic_id vs guid)
