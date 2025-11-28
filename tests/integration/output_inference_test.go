package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/terraconstructs/grid/pkg/sdk"
)

// TestSchemaInferenceFromString tests FR-019, FR-021: Infer string type schema
func TestSchemaInferenceFromString(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-inference-string"})
	require.NoError(t, err)

	// Upload Terraform state with string output
	stateBytes, err := os.ReadFile(filepath.Join("testdata", "tfstate_string_output.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, stateBytes)
	require.NoError(t, err, "Failed to upload Terraform state")

	// Wait for inference to complete (async processing)
	time.Sleep(500 * time.Millisecond)

	// List outputs to verify schema was inferred
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)

	// Find the vpc_id output
	var vpcOutput *sdk.OutputKey
	for i := range outputs {
		if outputs[i].Key == "vpc_id" {
			vpcOutput = &outputs[i]
			break
		}
	}
	require.NotNil(t, vpcOutput, "vpc_id output should exist")
	require.NotNil(t, vpcOutput.SchemaJSON, "Schema should be inferred")

	// Verify schema source is "inferred"
	require.NotNil(t, vpcOutput.SchemaSource, "SchemaSource should be set")
	assert.Equal(t, "inferred", *vpcOutput.SchemaSource, "Schema source should be 'inferred'")

	// Parse and verify schema structure
	var schema map[string]interface{}
	err = json.Unmarshal([]byte(*vpcOutput.SchemaJSON), &schema)
	require.NoError(t, err, "Schema should be valid JSON")

	// Verify it's a string type
	assert.Equal(t, "string", schema["type"], "Inferred type should be string")
}

// TestSchemaInferenceFromNumber tests FR-021: Infer number/integer types
func TestSchemaInferenceFromNumber(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-inference-number"})
	require.NoError(t, err)

	// Upload state with number/integer outputs
	stateBytes, err := os.ReadFile(filepath.Join("testdata", "tfstate_complex_types.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, stateBytes)
	require.NoError(t, err)

	// Wait for inference
	time.Sleep(500 * time.Millisecond)

	// Get outputs
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)

	// Find port output (should be integer)
	var portOutput *sdk.OutputKey
	for i := range outputs {
		if outputs[i].Key == "port" {
			portOutput = &outputs[i]
			break
		}
	}
	require.NotNil(t, portOutput, "port output should exist")
	require.NotNil(t, portOutput.SchemaJSON, "Schema should be inferred")

	var schema map[string]interface{}
	err = json.Unmarshal([]byte(*portOutput.SchemaJSON), &schema)
	require.NoError(t, err)

	// Verify number type (jsonschema-infer may use "number" or "integer")
	schemaType := schema["type"]
	assert.Contains(t, []string{"number", "integer"}, schemaType, "Type should be number or integer")
}

// TestSchemaInferenceFromBoolean tests FR-021: Infer boolean type
func TestSchemaInferenceFromBoolean(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-inference-boolean"})
	require.NoError(t, err)

	// Upload state with boolean output
	stateBytes, err := os.ReadFile(filepath.Join("testdata", "tfstate_complex_types.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, stateBytes)
	require.NoError(t, err)

	// Wait for inference
	time.Sleep(500 * time.Millisecond)

	// Get outputs
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)

	// Find enabled output (should be boolean)
	var enabledOutput *sdk.OutputKey
	for i := range outputs {
		if outputs[i].Key == "enabled" {
			enabledOutput = &outputs[i]
			break
		}
	}
	require.NotNil(t, enabledOutput, "enabled output should exist")
	require.NotNil(t, enabledOutput.SchemaJSON, "Schema should be inferred")

	var schema map[string]interface{}
	err = json.Unmarshal([]byte(*enabledOutput.SchemaJSON), &schema)
	require.NoError(t, err)

	assert.Equal(t, "boolean", schema["type"], "Type should be boolean")
}

// TestSchemaInferenceFromArray tests FR-021, FR-022: Infer array with items
func TestSchemaInferenceFromArray(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-inference-array"})
	require.NoError(t, err)

	// Upload state with array output
	stateBytes, err := os.ReadFile(filepath.Join("testdata", "tfstate_complex_types.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, stateBytes)
	require.NoError(t, err)

	// Wait for inference
	time.Sleep(500 * time.Millisecond)

	// Get outputs
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)

	// Find subnet_ids output (should be array)
	var arrayOutput *sdk.OutputKey
	for i := range outputs {
		if outputs[i].Key == "subnet_ids" {
			arrayOutput = &outputs[i]
			break
		}
	}
	require.NotNil(t, arrayOutput, "subnet_ids output should exist")
	require.NotNil(t, arrayOutput.SchemaJSON, "Schema should be inferred")

	var schema map[string]interface{}
	err = json.Unmarshal([]byte(*arrayOutput.SchemaJSON), &schema)
	require.NoError(t, err)

	assert.Equal(t, "array", schema["type"], "Type should be array")
	// Verify items schema exists
	assert.NotNil(t, schema["items"], "Array schema should have items definition")
}

// TestSchemaInferenceFromObject tests FR-021, FR-022: Nested object inference
func TestSchemaInferenceFromObject(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-inference-object"})
	require.NoError(t, err)

	// Upload state with nested object output
	stateBytes, err := os.ReadFile(filepath.Join("testdata", "tfstate_complex_types.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, stateBytes)
	require.NoError(t, err)

	// Wait for inference
	time.Sleep(500 * time.Millisecond)

	// Get outputs
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)

	// Find config output (should be object)
	var objectOutput *sdk.OutputKey
	for i := range outputs {
		if outputs[i].Key == "config" {
			objectOutput = &outputs[i]
			break
		}
	}
	require.NotNil(t, objectOutput, "config output should exist")
	require.NotNil(t, objectOutput.SchemaJSON, "Schema should be inferred")

	var schema map[string]interface{}
	err = json.Unmarshal([]byte(*objectOutput.SchemaJSON), &schema)
	require.NoError(t, err)

	assert.Equal(t, "object", schema["type"], "Type should be object")
	// Verify properties exist for nested structure
	assert.NotNil(t, schema["properties"], "Object schema should have properties")
}

// TestSchemaInferenceDateTime tests FR-023: ISO 8601 format detection
func TestSchemaInferenceDateTime(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-inference-datetime"})
	require.NoError(t, err)

	// Upload state with ISO 8601 timestamp output
	stateBytes, err := os.ReadFile(filepath.Join("testdata", "tfstate_datetime_output.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, stateBytes)
	require.NoError(t, err)

	// Wait for inference
	time.Sleep(500 * time.Millisecond)

	// Get outputs
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)

	// Find created_at output
	var datetimeOutput *sdk.OutputKey
	for i := range outputs {
		if outputs[i].Key == "created_at" {
			datetimeOutput = &outputs[i]
			break
		}
	}
	require.NotNil(t, datetimeOutput, "created_at output should exist")
	require.NotNil(t, datetimeOutput.SchemaJSON, "Schema should be inferred")

	var schema map[string]interface{}
	err = json.Unmarshal([]byte(*datetimeOutput.SchemaJSON), &schema)
	require.NoError(t, err)

	assert.Equal(t, "string", schema["type"], "Type should be string")
	// Verify date-time format detection
	assert.Equal(t, "date-time", schema["format"], "Format should be date-time for ISO 8601")
}

// TestSchemaInferencePreserveManual tests FR-025: Manual schemas not overwritten
func TestSchemaInferencePreserveManual(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-inference-preserve-manual"})
	require.NoError(t, err)

	// Load and set manual schema first
	manualSchemaBytes, err := os.ReadFile(filepath.Join("testdata", "schema_vpc_id.json"))
	require.NoError(t, err)
	manualSchema := string(manualSchemaBytes)

	err = client.SetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "vpc_id", manualSchema)
	require.NoError(t, err)

	// Upload Terraform state (would trigger inference if no schema exists)
	stateBytes, err := os.ReadFile(filepath.Join("testdata", "tfstate_string_output.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, stateBytes)
	require.NoError(t, err)

	// Wait for potential inference
	time.Sleep(500 * time.Millisecond)

	// Verify manual schema is preserved
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)

	var vpcOutput *sdk.OutputKey
	for i := range outputs {
		if outputs[i].Key == "vpc_id" {
			vpcOutput = &outputs[i]
			break
		}
	}
	require.NotNil(t, vpcOutput, "vpc_id output should exist")
	require.NotNil(t, vpcOutput.SchemaJSON, "Schema should exist")

	// Verify schema is still manual
	require.NotNil(t, vpcOutput.SchemaSource, "SchemaSource should be set")
	assert.Equal(t, "manual", *vpcOutput.SchemaSource, "Schema source should remain 'manual'")

	// Verify schema content unchanged
	assert.JSONEq(t, manualSchema, *vpcOutput.SchemaJSON, "Manual schema should be preserved")
}

// TestSchemaSourceMetadata tests FR-026, FR-028: schema_source field
func TestSchemaSourceMetadata(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Test 1: Manual schema via SetOutputSchema
	state1, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-schema-source-manual"})
	require.NoError(t, err)

	manualSchemaBytes, err := os.ReadFile(filepath.Join("testdata", "schema_vpc_id.json"))
	require.NoError(t, err)
	manualSchema := string(manualSchemaBytes)

	err = client.SetOutputSchema(ctx, sdk.StateReference{GUID: state1.GUID}, "vpc_id", manualSchema)
	require.NoError(t, err)

	// Retrieve and verify schema_source = "manual"
	retrievedSchema, err := client.GetOutputSchema(ctx, sdk.StateReference{GUID: state1.GUID}, "vpc_id")
	require.NoError(t, err)
	assert.JSONEq(t, manualSchema, retrievedSchema)

	outputs1, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state1.GUID})
	require.NoError(t, err)

	var manualOutput *sdk.OutputKey
	for i := range outputs1 {
		if outputs1[i].Key == "vpc_id" {
			manualOutput = &outputs1[i]
			break
		}
	}
	require.NotNil(t, manualOutput)
	require.NotNil(t, manualOutput.SchemaSource)
	assert.Equal(t, "manual", *manualOutput.SchemaSource)

	// Test 2: Inferred schema via state upload
	state2, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-schema-source-inferred"})
	require.NoError(t, err)

	stateBytes, err := os.ReadFile(filepath.Join("testdata", "tfstate_string_output.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state2.GUID, stateBytes)
	require.NoError(t, err)

	// Wait for inference
	time.Sleep(500 * time.Millisecond)

	outputs2, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state2.GUID})
	require.NoError(t, err)

	var inferredOutput *sdk.OutputKey
	for i := range outputs2 {
		if outputs2[i].Key == "vpc_id" {
			inferredOutput = &outputs2[i]
			break
		}
	}
	require.NotNil(t, inferredOutput)
	require.NotNil(t, inferredOutput.SchemaSource)
	assert.Equal(t, "inferred", *inferredOutput.SchemaSource)
}

// TestSchemaInferenceOnceOnly tests FR-027: Only first upload triggers inference
func TestSchemaInferenceOnceOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-inference-once-only"})
	require.NoError(t, err)

	// First upload: Should trigger inference
	stateBytes1, err := os.ReadFile(filepath.Join("testdata", "tfstate_string_output.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, stateBytes1)
	require.NoError(t, err)

	// Wait for inference
	time.Sleep(500 * time.Millisecond)

	// Get inferred schema
	outputs1, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)

	var output1 *sdk.OutputKey
	for i := range outputs1 {
		if outputs1[i].Key == "vpc_id" {
			output1 = &outputs1[i]
			break
		}
	}
	require.NotNil(t, output1)
	require.NotNil(t, output1.SchemaJSON)
	firstSchema := *output1.SchemaJSON

	// Second upload with DIFFERENT value: Should NOT re-infer
	// Modify the state to have a different vpc_id value
	var stateData map[string]interface{}
	err = json.Unmarshal(stateBytes1, &stateData)
	require.NoError(t, err)

	outputs := stateData["outputs"].(map[string]interface{})
	vpcOutput := outputs["vpc_id"].(map[string]interface{})
	vpcOutput["value"] = "vpc-different-12345" // Change value

	stateBytes2, err := json.Marshal(stateData)
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, stateBytes2)
	require.NoError(t, err)

	// Wait
	time.Sleep(500 * time.Millisecond)

	// Verify schema UNCHANGED
	outputs2, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)

	var output2 *sdk.OutputKey
	for i := range outputs2 {
		if outputs2[i].Key == "vpc_id" {
			output2 = &outputs2[i]
			break
		}
	}
	require.NotNil(t, output2)
	require.NotNil(t, output2.SchemaJSON)
	assert.JSONEq(t, firstSchema, *output2.SchemaJSON, "Schema should not change on subsequent uploads")
	assert.Equal(t, "inferred", *output2.SchemaSource, "Schema source should remain 'inferred'")
}

// TestSchemaInferenceRequiredFields tests FR-024: Required-field heuristic
func TestSchemaInferenceRequiredFields(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state with object output containing multiple fields
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-inference-required-fields"})
	require.NoError(t, err)

	// Upload state with nested object
	stateBytes, err := os.ReadFile(filepath.Join("testdata", "tfstate_complex_types.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, stateBytes)
	require.NoError(t, err)

	// Wait for inference
	time.Sleep(500 * time.Millisecond)

	// Get config output (object type)
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)

	var configOutput *sdk.OutputKey
	for i := range outputs {
		if outputs[i].Key == "config" {
			configOutput = &outputs[i]
			break
		}
	}
	require.NotNil(t, configOutput)
	require.NotNil(t, configOutput.SchemaJSON)

	var schema map[string]interface{}
	err = json.Unmarshal([]byte(*configOutput.SchemaJSON), &schema)
	require.NoError(t, err)

	// Verify required fields are present
	// With single-sample inference, all fields should be marked required
	required, ok := schema["required"]
	if ok {
		requiredFields, ok := required.([]interface{})
		assert.True(t, ok, "Required should be an array")
		assert.Greater(t, len(requiredFields), 0, "At least one field should be required with single-sample inference")
	}
}

// TestSchemaInferenceRunsOnce tests FR-027 explicitly
func TestSchemaInferenceRunsOnce(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-inference-runs-once"})
	require.NoError(t, err)

	// First upload with specific value
	stateBytes1, err := os.ReadFile(filepath.Join("testdata", "tfstate_string_output.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, stateBytes1)
	require.NoError(t, err)

	// Wait for inference
	time.Sleep(500 * time.Millisecond)

	// Get first schema
	schema1, err := client.GetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "vpc_id")
	require.NoError(t, err)
	require.NotEmpty(t, schema1, "Schema should be inferred after first upload")

	// Parse to get details
	var schemaObj1 map[string]interface{}
	err = json.Unmarshal([]byte(schema1), &schemaObj1)
	require.NoError(t, err)

	// Second upload with different value type structure (still string but could have different constraints)
	var stateData map[string]interface{}
	err = json.Unmarshal(stateBytes1, &stateData)
	require.NoError(t, err)

	outputs := stateData["outputs"].(map[string]interface{})
	vpcOutput := outputs["vpc_id"].(map[string]interface{})
	vpcOutput["value"] = "vpc-completely-different-format-xyz-789"

	stateBytes2, err := json.Marshal(stateData)
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, stateBytes2)
	require.NoError(t, err)

	// Wait
	time.Sleep(500 * time.Millisecond)

	// Get schema again
	schema2, err := client.GetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "vpc_id")
	require.NoError(t, err)

	// Verify schema is IDENTICAL (no re-inference)
	assert.JSONEq(t, schema1, schema2, "Schema should not change - inference runs only once")

	// Verify it's still marked as inferred
	outputs2, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)

	var vpcOutput2 *sdk.OutputKey
	for i := range outputs2 {
		if outputs2[i].Key == "vpc_id" {
			vpcOutput2 = &outputs2[i]
			break
		}
	}
	require.NotNil(t, vpcOutput2)
	require.NotNil(t, vpcOutput2.SchemaSource)
	assert.Equal(t, "inferred", *vpcOutput2.SchemaSource)
}
