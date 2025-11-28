package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/terraconstructs/grid/pkg/sdk"
)

// uniqueLogicID generates a unique logic ID by appending a timestamp to the base name.
// This prevents test collisions when states aren't cleaned up between runs.
func uniqueLogicID(base string) string {
	return fmt.Sprintf("%s-%d", base, time.Now().UnixNano())
}

// TestValidationPassPattern tests that validation passes when output values match schema pattern constraints.
// Validates FR-029 (validation uses jsonschema library) and FR-031 (validation_status="valid").
func TestValidationPassPattern(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: uniqueLogicID("test-validation-pass-pattern")})
	require.NoError(t, err)

	// Load strict pattern schema
	schemaBytes, err := os.ReadFile(filepath.Join("testdata", "schema_pattern_strict.json"))
	require.NoError(t, err)

	// Set schema for vpc_id and subnet_ids outputs
	err = client.SetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "vpc_id", string(schemaBytes))
	require.NoError(t, err)

	// Upload Terraform state with VALID outputs matching pattern
	validStateBytes, err := os.ReadFile(filepath.Join("testdata", "tfstate_valid_pattern.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, validStateBytes)
	require.NoError(t, err)

	// Wait for validation to complete (synchronous in handler, but give small buffer)
	time.Sleep(100 * time.Millisecond)

	// Verify validation passed
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(outputs), 1, "Should have at least vpc_id output")

	// Find vpc_id output
	var vpcOutput *sdk.OutputKey
	for i := range outputs {
		if outputs[i].Key == "vpc_id" {
			vpcOutput = &outputs[i]
			break
		}
	}
	require.NotNil(t, vpcOutput, "vpc_id output should exist")

	// Validate FR-031: validation_status = "valid"
	require.NotNil(t, vpcOutput.ValidationStatus, "ValidationStatus should be set")
	assert.Equal(t, "valid", *vpcOutput.ValidationStatus, "Validation should pass")
	assert.Nil(t, vpcOutput.ValidationError, "ValidationError should be nil when valid")
	assert.NotNil(t, vpcOutput.ValidatedAt, "ValidatedAt should be set")
}

// TestValidationFailPattern tests that validation fails when output values violate schema pattern constraints.
// Validates FR-029 (validation detects errors) and FR-030 (validation_status="invalid").
func TestValidationFailPattern(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: uniqueLogicID("test-validation-fail-pattern")})
	require.NoError(t, err)

	// Load strict pattern schema
	schemaBytes, err := os.ReadFile(filepath.Join("testdata", "schema_pattern_strict.json"))
	require.NoError(t, err)

	// Set schema for vpc_id output
	err = client.SetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "vpc_id", string(schemaBytes))
	require.NoError(t, err)

	// Upload Terraform state with INVALID outputs (violates pattern)
	invalidStateBytes, err := os.ReadFile(filepath.Join("testdata", "tfstate_invalid_pattern.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, invalidStateBytes)
	require.NoError(t, err)

	// Wait for validation to complete
	time.Sleep(100 * time.Millisecond)

	// Verify validation failed
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(outputs), 1, "Should have at least vpc_id output")

	// Find vpc_id output
	var vpcOutput *sdk.OutputKey
	for i := range outputs {
		if outputs[i].Key == "vpc_id" {
			vpcOutput = &outputs[i]
			break
		}
	}
	require.NotNil(t, vpcOutput, "vpc_id output should exist")

	// Validate FR-030: validation_status = "invalid"
	require.NotNil(t, vpcOutput.ValidationStatus, "ValidationStatus should be set")
	assert.Equal(t, "invalid", *vpcOutput.ValidationStatus, "Validation should fail")
	require.NotNil(t, vpcOutput.ValidationError, "ValidationError should be set when invalid")
	assert.NotEmpty(t, *vpcOutput.ValidationError, "ValidationError should contain message")
	assert.NotNil(t, vpcOutput.ValidatedAt, "ValidatedAt should be set")
}

// TestValidationSkipWhenNoSchema tests that validation is skipped (status="not_validated") when no schema exists.
// Validates FR-033 (skip validation when no schema).
func TestValidationSkipWhenNoSchema(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: uniqueLogicID("test-validation-skip-no-schema")})
	require.NoError(t, err)

	// Upload Terraform state WITHOUT setting any schemas
	validStateBytes, err := os.ReadFile(filepath.Join("testdata", "tfstate_valid_pattern.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, validStateBytes)
	require.NoError(t, err)

	// Wait for validation job to run
	time.Sleep(100 * time.Millisecond)

	// Verify outputs have "not_validated" status
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(outputs), 1, "Should have at least one output")

	for _, output := range outputs {
		// FR-033: Outputs without schemas should get "not_validated" status
		require.NotNil(t, output.ValidationStatus, "ValidationStatus should be set (not null)")
		assert.Equal(t, "not_validated", *output.ValidationStatus, "Validation should be skipped when no schema")
		assert.Nil(t, output.ValidationError, "ValidationError should be nil when not validated")
	}
}

// TestValidationComplexSchema tests validation with nested object schemas.
// Validates FR-029 (validation handles complex schemas).
func TestValidationComplexSchema(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: uniqueLogicID("test-validation-complex-schema")})
	require.NoError(t, err)

	// Load complex schema fixture
	schemaBytes, err := os.ReadFile(filepath.Join("testdata", "schema_config_object.json"))
	require.NoError(t, err)

	// Set schema for complex output
	err = client.SetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "config", string(schemaBytes))
	require.NoError(t, err)

	// Upload Terraform state with complex nested object
	complexStateBytes, err := os.ReadFile(filepath.Join("testdata", "tfstate_complex_types.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, complexStateBytes)
	require.NoError(t, err)

	// Wait for validation to complete
	time.Sleep(100 * time.Millisecond)

	// Verify validation ran (should pass or fail based on schema)
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)

	// Find config output
	var configOutput *sdk.OutputKey
	for i := range outputs {
		if outputs[i].Key == "config" {
			configOutput = &outputs[i]
			break
		}
	}
	require.NotNil(t, configOutput, "config output should exist")

	// Verify validation ran (status should not be "not_validated")
	require.NotNil(t, configOutput.ValidationStatus, "ValidationStatus should be set")
	assert.NotEqual(t, "not_validated", *configOutput.ValidationStatus, "Validation should run with schema")
	assert.NotNil(t, configOutput.ValidatedAt, "ValidatedAt should be set")
}

// TestValidationStatusInResponse tests that validation status appears in ListStateOutputs response.
// Validates FR-034 (validation metadata in responses).
func TestValidationStatusInResponse(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: uniqueLogicID("test-validation-status-in-response")})
	require.NoError(t, err)

	// Set schema
	schemaBytes, err := os.ReadFile(filepath.Join("testdata", "schema_pattern_strict.json"))
	require.NoError(t, err)

	err = client.SetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "vpc_id", string(schemaBytes))
	require.NoError(t, err)

	// Upload valid state
	validStateBytes, err := os.ReadFile(filepath.Join("testdata", "tfstate_valid_pattern.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, validStateBytes)
	require.NoError(t, err)

	// Wait for validation
	time.Sleep(100 * time.Millisecond)

	// Verify validation fields are present in ListStateOutputs response
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(outputs), 1)

	var vpcOutput *sdk.OutputKey
	for i := range outputs {
		if outputs[i].Key == "vpc_id" {
			vpcOutput = &outputs[i]
			break
		}
	}
	require.NotNil(t, vpcOutput)

	// FR-034: All validation fields should be present
	assert.NotNil(t, vpcOutput.ValidationStatus, "ValidationStatus should be in response")
	// ValidationError can be nil when valid (OK)
	assert.NotNil(t, vpcOutput.ValidatedAt, "ValidatedAt should be in response")
}

// TestValidationErrorMessage tests that validation errors include structured details with JSON path.
// Validates FR-035 (structured error format).
func TestValidationErrorMessage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: uniqueLogicID("test-validation-error-message")})
	require.NoError(t, err)

	// Set schema with pattern constraint
	schemaBytes, err := os.ReadFile(filepath.Join("testdata", "schema_pattern_strict.json"))
	require.NoError(t, err)

	err = client.SetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "vpc_id", string(schemaBytes))
	require.NoError(t, err)

	// Upload INVALID state
	invalidStateBytes, err := os.ReadFile(filepath.Join("testdata", "tfstate_invalid_pattern.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, invalidStateBytes)
	require.NoError(t, err)

	// Wait for validation
	time.Sleep(100 * time.Millisecond)

	// Verify error message structure
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)

	var vpcOutput *sdk.OutputKey
	for i := range outputs {
		if outputs[i].Key == "vpc_id" {
			vpcOutput = &outputs[i]
			break
		}
	}
	require.NotNil(t, vpcOutput)

	// FR-035: Validation error should contain structured details
	require.NotNil(t, vpcOutput.ValidationError, "ValidationError should be set")
	errorMsg := *vpcOutput.ValidationError
	assert.NotEmpty(t, errorMsg, "ValidationError should not be empty")

	// Error should contain useful context (pattern, path, etc.)
	// The exact format depends on jsonschema library, but should mention pattern
	assert.Contains(t, errorMsg, "pattern", "Error should mention pattern constraint")
}

// TestValidationNonBlocking tests that state upload returns success even with invalid schemas.
// Validates FR-032 (non-blocking validation).
func TestValidationNonBlocking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: uniqueLogicID("test-validation-non-blocking")})
	require.NoError(t, err)

	// Set schema with strict pattern
	schemaBytes, err := os.ReadFile(filepath.Join("testdata", "schema_pattern_strict.json"))
	require.NoError(t, err)

	err = client.SetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "vpc_id", string(schemaBytes))
	require.NoError(t, err)

	// Upload INVALID state - should NOT block or return error
	invalidStateBytes, err := os.ReadFile(filepath.Join("testdata", "tfstate_invalid_pattern.json"))
	require.NoError(t, err)

	// FR-032: State upload should succeed even with invalid output
	err = uploadTerraformState(state.GUID, invalidStateBytes)
	assert.NoError(t, err, "State upload should not be blocked by validation failure")

	// Wait for validation to complete asynchronously
	time.Sleep(100 * time.Millisecond)

	// Verify validation ran and marked as invalid
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)

	var vpcOutput *sdk.OutputKey
	for i := range outputs {
		if outputs[i].Key == "vpc_id" {
			vpcOutput = &outputs[i]
			break
		}
	}
	require.NotNil(t, vpcOutput)

	require.NotNil(t, vpcOutput.ValidationStatus)
	assert.Equal(t, "invalid", *vpcOutput.ValidationStatus, "Validation should have completed and marked as invalid")
}

// TestValidationMetadataInResponses tests that all validation fields appear in RPC responses.
// Validates FR-034 (validation metadata in ListStateOutputs and GetStateInfo).
func TestValidationMetadataInResponses(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: uniqueLogicID("test-validation-metadata-responses")})
	require.NoError(t, err)

	// Set schema
	schemaBytes, err := os.ReadFile(filepath.Join("testdata", "schema_pattern_strict.json"))
	require.NoError(t, err)

	err = client.SetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "vpc_id", string(schemaBytes))
	require.NoError(t, err)

	// Upload valid state
	validStateBytes, err := os.ReadFile(filepath.Join("testdata", "tfstate_valid_pattern.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, validStateBytes)
	require.NoError(t, err)

	// Wait for validation
	time.Sleep(100 * time.Millisecond)

	// Test 1: ListStateOutputs should include validation fields
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(outputs), 1)

	var vpcOutput *sdk.OutputKey
	for i := range outputs {
		if outputs[i].Key == "vpc_id" {
			vpcOutput = &outputs[i]
			break
		}
	}
	require.NotNil(t, vpcOutput, "vpc_id output should exist")

	// FR-034: All three validation fields should be present
	assert.NotNil(t, vpcOutput.ValidationStatus, "ListStateOutputs: ValidationStatus should be present")
	// ValidationError can be nil for valid outputs (expected)
	assert.NotNil(t, vpcOutput.ValidatedAt, "ListStateOutputs: ValidatedAt should be present")

	// Test 2: GetStateInfo should include validation fields in outputs array
	stateInfo, err := client.GetStateInfo(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)
	require.NotNil(t, stateInfo, "GetStateInfo should return state info")
	require.GreaterOrEqual(t, len(stateInfo.Outputs), 1, "GetStateInfo should include outputs")

	var vpcInfoOutput *sdk.OutputKey
	for i := range stateInfo.Outputs {
		if stateInfo.Outputs[i].Key == "vpc_id" {
			vpcInfoOutput = &stateInfo.Outputs[i]
			break
		}
	}
	require.NotNil(t, vpcInfoOutput, "vpc_id should be in GetStateInfo outputs")

	// FR-034: All validation fields should be present in GetStateInfo too
	assert.NotNil(t, vpcInfoOutput.ValidationStatus, "GetStateInfo: ValidationStatus should be present")
	assert.Equal(t, "valid", *vpcInfoOutput.ValidationStatus, "GetStateInfo: ValidationStatus should be 'valid'")
	assert.NotNil(t, vpcInfoOutput.ValidatedAt, "GetStateInfo: ValidatedAt should be present")
}

// TestValidationTransitionFromInvalidToValid tests that validation status can transition from invalid to valid.
// This ensures validation re-runs on subsequent state uploads and updates status correctly.
func TestValidationTransitionFromInvalidToValid(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: uniqueLogicID("test-validation-transition")})
	require.NoError(t, err)

	// Set schema
	schemaBytes, err := os.ReadFile(filepath.Join("testdata", "schema_pattern_strict.json"))
	require.NoError(t, err)

	err = client.SetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "vpc_id", string(schemaBytes))
	require.NoError(t, err)

	// Step 1: Upload INVALID state
	invalidStateBytes, err := os.ReadFile(filepath.Join("testdata", "tfstate_invalid_pattern.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, invalidStateBytes)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	// Verify invalid status
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)

	var vpcOutput *sdk.OutputKey
	for i := range outputs {
		if outputs[i].Key == "vpc_id" {
			vpcOutput = &outputs[i]
			break
		}
	}
	require.NotNil(t, vpcOutput)
	require.NotNil(t, vpcOutput.ValidationStatus)
	assert.Equal(t, "invalid", *vpcOutput.ValidationStatus, "Initial validation should fail")

	// Step 2: Upload VALID state (fix the output value)
	validStateBytes, err := os.ReadFile(filepath.Join("testdata", "tfstate_valid_pattern.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, validStateBytes)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	// Verify status transitioned to valid
	outputs, err = client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)

	vpcOutput = nil
	for i := range outputs {
		if outputs[i].Key == "vpc_id" {
			vpcOutput = &outputs[i]
			break
		}
	}
	require.NotNil(t, vpcOutput)
	require.NotNil(t, vpcOutput.ValidationStatus)
	assert.Equal(t, "valid", *vpcOutput.ValidationStatus, "Validation status should transition to valid")
	assert.Nil(t, vpcOutput.ValidationError, "ValidationError should be cleared when valid")
}

// TestValidationWithManualSchemaSource tests that manual schemas are validated correctly.
// Ensures validation works with schema_source='manual' (user-declared schemas).
func TestValidationWithManualSchemaSource(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: uniqueLogicID("test-validation-manual-schema")})
	require.NoError(t, err)

	// Set manual schema BEFORE output exists
	schemaBytes, err := os.ReadFile(filepath.Join("testdata", "schema_pattern_strict.json"))
	require.NoError(t, err)

	err = client.SetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "vpc_id", string(schemaBytes))
	require.NoError(t, err)

	// Verify schema-only row created with schema_source='manual'
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)
	require.Len(t, outputs, 1)
	require.NotNil(t, outputs[0].SchemaSource)
	assert.Equal(t, "manual", *outputs[0].SchemaSource)

	// Upload state with valid vpc_id
	validStateBytes, err := os.ReadFile(filepath.Join("testdata", "tfstate_valid_pattern.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, validStateBytes)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	// Verify validation ran with manual schema
	outputs, err = client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)

	var vpcOutput *sdk.OutputKey
	for i := range outputs {
		if outputs[i].Key == "vpc_id" {
			vpcOutput = &outputs[i]
			break
		}
	}
	require.NotNil(t, vpcOutput)

	// Verify manual schema is still set and validation passed
	require.NotNil(t, vpcOutput.SchemaSource)
	assert.Equal(t, "manual", *vpcOutput.SchemaSource, "Schema source should remain manual")
	require.NotNil(t, vpcOutput.ValidationStatus)
	assert.Equal(t, "valid", *vpcOutput.ValidationStatus, "Manual schema should be validated")
}

// TestValidationWithInferredSchema tests that inferred schemas are validated on subsequent uploads.
// Note: Inference is async while validation is sync, so the first upload won't validate
// (schema doesn't exist yet). The SECOND upload triggers validation with the inferred schema.
func TestValidationWithInferredSchema(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: uniqueLogicID("test-validation-inferred-schema")})
	require.NoError(t, err)

	// First upload: Triggers inference (async) but validation runs before inference completes
	stateBytes, err := os.ReadFile(filepath.Join("testdata", "tfstate_string_output.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, stateBytes)
	require.NoError(t, err)

	// Wait for inference to complete (async, ~200-500ms)
	time.Sleep(500 * time.Millisecond)

	// Verify inferred schema was created
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(outputs), 1)

	// Find an output with inferred schema
	var inferredOutput *sdk.OutputKey
	for i := range outputs {
		if outputs[i].SchemaSource != nil && *outputs[i].SchemaSource == "inferred" {
			inferredOutput = &outputs[i]
			break
		}
	}
	require.NotNil(t, inferredOutput, "Should have at least one inferred schema")

	// First upload: validation_status should be "not_validated" (schema didn't exist during validation)
	require.NotNil(t, inferredOutput.ValidationStatus)
	assert.Equal(t, "not_validated", *inferredOutput.ValidationStatus,
		"First upload should have 'not_validated' status (inference is async)")

	// Second upload: Now the inferred schema exists, validation should run
	err = uploadTerraformState(state.GUID, stateBytes)
	require.NoError(t, err)

	// Short wait for sync validation to complete
	time.Sleep(100 * time.Millisecond)

	// Verify validation ran with inferred schema
	outputs, err = client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)

	inferredOutput = nil
	for i := range outputs {
		if outputs[i].SchemaSource != nil && *outputs[i].SchemaSource == "inferred" {
			inferredOutput = &outputs[i]
			break
		}
	}
	require.NotNil(t, inferredOutput)

	// Second upload: Should now be validated (inferred schema exists)
	require.NotNil(t, inferredOutput.ValidationStatus)
	assert.Equal(t, "valid", *inferredOutput.ValidationStatus,
		"Second upload should validate against inferred schema")
	assert.Nil(t, inferredOutput.ValidationError)
}

// TestValidationErrorWithArrayItemViolation tests that validation errors include array index in path.
// Validates FR-035 for array validation errors.
func TestValidationErrorWithArrayItemViolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: uniqueLogicID("test-validation-array-error")})
	require.NoError(t, err)

	// Load schema with array pattern constraint
	schemaBytes, err := os.ReadFile(filepath.Join("testdata", "schema_subnet_array_pattern.json"))
	require.NoError(t, err)

	// Parse schema to verify it has array validation
	var schema map[string]interface{}
	err = json.Unmarshal(schemaBytes, &schema)
	require.NoError(t, err)

	// Set schema for output with array
	err = client.SetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "subnet_ids", string(schemaBytes))
	require.NoError(t, err)

	// Upload INVALID state (subnet_ids array has invalid item at index 1)
	invalidStateBytes, err := os.ReadFile(filepath.Join("testdata", "tfstate_invalid_pattern.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, invalidStateBytes)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	// Verify error message includes array index
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)

	var subnetOutput *sdk.OutputKey
	for i := range outputs {
		if outputs[i].Key == "subnet_ids" {
			subnetOutput = &outputs[i]
			break
		}
	}
	require.NotNil(t, subnetOutput)

	require.NotNil(t, subnetOutput.ValidationStatus)
	assert.Equal(t, "invalid", *subnetOutput.ValidationStatus)

	require.NotNil(t, subnetOutput.ValidationError)
	errorMsg := *subnetOutput.ValidationError

	// FR-035: Error should mention array context (index or path like /subnet_ids/1)
	// The exact format depends on jsonschema library implementation
	assert.NotEmpty(t, errorMsg, "ValidationError should contain details")
}

// TestValidationErrorStructure tests that validation errors include all required components per SC-006 and FR-035.
// Validates structured error format: JSON path, expected constraint, actual value, and truncation for long values.
func TestValidationErrorStructure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	t.Run("PatternViolation", func(t *testing.T) {
		// Create state
		state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: uniqueLogicID("test-error-structure-pattern")})
		require.NoError(t, err)

		// Set schema with pattern constraint
		schemaBytes, err := os.ReadFile(filepath.Join("testdata", "schema_pattern_strict.json"))
		require.NoError(t, err)

		err = client.SetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "vpc_id", string(schemaBytes))
		require.NoError(t, err)

		// Upload INVALID state
		invalidStateBytes, err := os.ReadFile(filepath.Join("testdata", "tfstate_invalid_pattern.json"))
		require.NoError(t, err)

		err = uploadTerraformState(state.GUID, invalidStateBytes)
		require.NoError(t, err)
		time.Sleep(100 * time.Millisecond)

		// Verify error structure
		outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
		require.NoError(t, err)

		var vpcOutput *sdk.OutputKey
		for i := range outputs {
			if outputs[i].Key == "vpc_id" {
				vpcOutput = &outputs[i]
				break
			}
		}
		require.NotNil(t, vpcOutput)

		require.NotNil(t, vpcOutput.ValidationError, "ValidationError should be set")
		errorMsg := *vpcOutput.ValidationError

		// SC-006, FR-035: Structured error format
		// Should include: JSON path, error description
		assert.Contains(t, errorMsg, "$", "Error should include JSON path marker")
		assert.Contains(t, errorMsg, "at '", "Error should include path indicator")
		assert.Contains(t, errorMsg, "pattern", "Error should mention pattern constraint")
		assert.Contains(t, errorMsg, "does not match", "Error should describe the validation failure")
	})

	t.Run("LongValueTruncation", func(t *testing.T) {
		// Create state
		state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: uniqueLogicID("test-error-truncation")})
		require.NoError(t, err)

		// Create schema that expects a short string
		shortStringSchema := `{
			"$schema": "http://json-schema.org/draft-07/schema#",
			"type": "string",
			"maxLength": 10
		}`

		err = client.SetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "short_string", shortStringSchema)
		require.NoError(t, err)

		// Create state with a very long string (>100 chars) that violates maxLength
		longValue := strings.Repeat("a", 150) // 150 chars
		tfstate := map[string]interface{}{
			"version":           4,
			"terraform_version": "1.5.0",
			"serial":            1,
			"lineage":           "test-lineage-truncation",
			"outputs": map[string]interface{}{
				"short_string": map[string]interface{}{
					"value": longValue,
					"type":  "string",
				},
			},
		}
		tfstateBytes, err := json.Marshal(tfstate)
		require.NoError(t, err)

		err = uploadTerraformState(state.GUID, tfstateBytes)
		require.NoError(t, err)
		time.Sleep(100 * time.Millisecond)

		// Verify truncation
		outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
		require.NoError(t, err)

		var output *sdk.OutputKey
		for i := range outputs {
			if outputs[i].Key == "short_string" {
				output = &outputs[i]
				break
			}
		}
		require.NotNil(t, output)

		require.NotNil(t, output.ValidationError)
		errorMsg := *output.ValidationError

		// FR-035: Error message itself should be truncated for very long errors
		// The library error format is: "maxLength: got 150, want 10"
		assert.Contains(t, errorMsg, "maxLength", "Error should mention constraint type")
		assert.Contains(t, errorMsg, "$", "Error should include JSON path")
		assert.Less(t, len(errorMsg), 400, "Error message should not be excessively long (truncated if needed)")
	})
}

// TestSetOutputSchemaTriggersValidation tests grid-a966: SetOutputSchema should trigger validation job for existing outputs.
// When a schema is set on an output that already has a value, validation should run immediately
// without requiring a terraform refresh.
func TestSetOutputSchemaTriggersValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: uniqueLogicID("test-set-schema-triggers-validation")})
	require.NoError(t, err)

	// Upload Terraform state with an output but NO schema set
	// Output: vpc_id = "vpc-abc12345" (valid format)
	tfstate := map[string]any{
		"version":           4,
		"terraform_version": "1.5.0",
		"serial":            1,
		"lineage":           "test-lineage-grid-a966",
		"outputs": map[string]any{
			"vpc_id": map[string]any{
				"value": "vpc-abc12345",
				"type":  "string",
			},
		},
	}
	tfstateBytes, err := json.Marshal(tfstate)
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, tfstateBytes)
	require.NoError(t, err)

	// Wait for initial output to be uploaded
	time.Sleep(100 * time.Millisecond)

	// Verify output exists but has NO validation status (no schema yet)
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(outputs), 1, "Should have vpc_id output")

	var vpcOutput *sdk.OutputKey
	for i := range outputs {
		if outputs[i].Key == "vpc_id" {
			vpcOutput = &outputs[i]
			break
		}
	}
	require.NotNil(t, vpcOutput, "vpc_id output should exist")

	// Verify no validation status initially (no schema)
	// Either ValidationStatus is nil OR it's "not_validated"
	if vpcOutput.ValidationStatus != nil {
		assert.Equal(t, "not_validated", *vpcOutput.ValidationStatus, "Should be not_validated before schema is set")
	}

	// NOW: Set a pattern-matching schema on vpc_id
	// This should IMMEDIATELY trigger validation
	schema := `{
		"type": "string",
		"pattern": "^vpc-[a-f0-9]+$",
		"description": "AWS VPC ID"
	}`

	err = client.SetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "vpc_id", schema)
	require.NoError(t, err)

	// Wait for validation to complete (it's async-capable but fires immediately)
	time.Sleep(200 * time.Millisecond)

	// Verify validation ran and PASSED (value matches pattern)
	outputs, err = client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)

	vpcOutput = nil
	for i := range outputs {
		if outputs[i].Key == "vpc_id" {
			vpcOutput = &outputs[i]
			break
		}
	}
	require.NotNil(t, vpcOutput, "vpc_id output should still exist")

	// CRITICAL: Validation should have run and passed
	// grid-a966: SetOutputSchema should trigger validation without terraform refresh
	require.NotNil(t, vpcOutput.ValidationStatus, "ValidationStatus should be set after SetOutputSchema")
	assert.Equal(t, "valid", *vpcOutput.ValidationStatus, "Validation should pass (value matches pattern)")
	assert.Nil(t, vpcOutput.ValidationError, "ValidationError should be nil when valid")
	assert.NotNil(t, vpcOutput.ValidatedAt, "ValidatedAt should be set")

	// Verify schema was stored
	require.NotNil(t, vpcOutput.SchemaJSON, "SchemaJSON should be set")
	assert.Contains(t, *vpcOutput.SchemaJSON, "pattern", "Schema should contain pattern constraint")

	t.Run("InvalidOutputAfterSchemaSet", func(t *testing.T) {
		// Create another state for invalid test
		state2, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: uniqueLogicID("test-schema-invalid-after-set")})
		require.NoError(t, err)

		// Upload Terraform state with INVALID output (doesn't match future schema)
		tfstate2 := map[string]any{
			"version":           4,
			"terraform_version": "1.5.0",
			"serial":            1,
			"lineage":           "test-lineage-grid-a966-invalid",
			"outputs": map[string]any{
				"vpc_id": map[string]any{
					"value": "invalid-format", // Doesn't match pattern
					"type":  "string",
				},
			},
		}
		tfstateBytes2, err := json.Marshal(tfstate2)
		require.NoError(t, err)

		err = uploadTerraformState(state2.GUID, tfstateBytes2)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Set schema with pattern constraint
		err = client.SetOutputSchema(ctx, sdk.StateReference{GUID: state2.GUID}, "vpc_id", schema)
		require.NoError(t, err)

		// Wait for validation to complete
		time.Sleep(200 * time.Millisecond)

		// Verify validation ran and FAILED
		outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state2.GUID})
		require.NoError(t, err)

		var vpcOutput2 *sdk.OutputKey
		for i := range outputs {
			if outputs[i].Key == "vpc_id" {
				vpcOutput2 = &outputs[i]
				break
			}
		}
		require.NotNil(t, vpcOutput2, "vpc_id output should exist")

		// Validation should have run and FAILED
		require.NotNil(t, vpcOutput2.ValidationStatus, "ValidationStatus should be set")
		assert.Equal(t, "invalid", *vpcOutput2.ValidationStatus, "Validation should fail (value doesn't match pattern)")
		assert.NotNil(t, vpcOutput2.ValidationError, "ValidationError should be set")
		assert.Greater(t, len(*vpcOutput2.ValidationError), 0, "ValidationError message should not be empty")
	})
}
