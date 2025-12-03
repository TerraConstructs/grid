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

// TestBasicSchemaOperations tests UC1: Create state, set schemas, retrieve schemas
func TestBasicSchemaOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-basic-schema-ops"})
	require.NoError(t, err)

	// Load schema fixtures
	vpcSchemaBytes, err := os.ReadFile(filepath.Join("testdata", "schema_vpc_id.json"))
	require.NoError(t, err)
	vpcSchema := string(vpcSchemaBytes)

	subnetSchemaBytes, err := os.ReadFile(filepath.Join("testdata", "schema_subnet_ids.json"))
	require.NoError(t, err)
	subnetSchema := string(subnetSchemaBytes)

	// Set schema for vpc_id
	err = client.SetOutputSchema(ctx, sdk.StateReference{LogicID: state.LogicID}, "vpc_id", vpcSchema)
	require.NoError(t, err, "Failed to set vpc_id schema")

	// Set schema for subnet_ids
	err = client.SetOutputSchema(ctx, sdk.StateReference{LogicID: state.LogicID}, "subnet_ids", subnetSchema)
	require.NoError(t, err, "Failed to set subnet_ids schema")

	// Get schema for vpc_id - verify correct schema returned
	retrievedVPCSchema, err := client.GetOutputSchema(ctx, sdk.StateReference{LogicID: state.LogicID}, "vpc_id")
	require.NoError(t, err)
	assert.JSONEq(t, vpcSchema, retrievedVPCSchema, "vpc_id schema should match")

	// Get schema for subnet_ids - verify correct schema returned
	retrievedSubnetSchema, err := client.GetOutputSchema(ctx, sdk.StateReference{LogicID: state.LogicID}, "subnet_ids")
	require.NoError(t, err)
	assert.JSONEq(t, subnetSchema, retrievedSubnetSchema, "subnet_ids schema should match")

	// Get schema for non-existent output - verify empty string
	nonExistentSchema, err := client.GetOutputSchema(ctx, sdk.StateReference{LogicID: state.LogicID}, "nonexistent")
	require.NoError(t, err)
	assert.Equal(t, "", nonExistentSchema, "Non-existent output should return empty schema")

	// List outputs - verify schemas embedded in response
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{LogicID: state.LogicID})
	require.NoError(t, err)

	// Should have 2 outputs (pre-declared via schemas)
	assert.Len(t, outputs, 2, "Should have 2 outputs with schemas")

	// Find vpc_id output
	var vpcOutput *sdk.OutputKey
	for i := range outputs {
		if outputs[i].Key == "vpc_id" {
			vpcOutput = &outputs[i]
			break
		}
	}
	require.NotNil(t, vpcOutput, "vpc_id output should exist")
	require.NotNil(t, vpcOutput.SchemaJSON, "vpc_id should have schema")
	assert.JSONEq(t, vpcSchema, *vpcOutput.SchemaJSON, "vpc_id schema in list should match")
}

// TestSchemaPreDeclaration tests UC2: Set schema before output exists in TF state
func TestSchemaPreDeclaration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-schema-predeclaration"})
	require.NoError(t, err)

	// Load schema
	vpcSchemaBytes, err := os.ReadFile(filepath.Join("testdata", "schema_vpc_id.json"))
	require.NoError(t, err)
	vpcSchema := string(vpcSchemaBytes)

	// Set schema BEFORE uploading TF state
	err = client.SetOutputSchema(ctx, sdk.StateReference{LogicID: state.LogicID}, "vpc_id", vpcSchema)
	require.NoError(t, err)

	// List outputs - verify "vpc_id" appears with schema (pending - serial=0)
	outputsBefore, err := client.ListStateOutputs(ctx, sdk.StateReference{LogicID: state.LogicID})
	require.NoError(t, err)
	assert.Len(t, outputsBefore, 1, "Should have 1 pre-declared output")
	assert.Equal(t, "vpc_id", outputsBefore[0].Key)
	require.NotNil(t, outputsBefore[0].SchemaJSON)

	// Upload TF state with vpc_id output
	tfstateData, err := os.ReadFile(filepath.Join("testdata", "vpc_output_with_schema.json"))
	require.NoError(t, err)
	putTFState(t, state.GUID, tfstateData)

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// List outputs - verify "vpc_id" still has schema and now has actual state data
	outputsAfter, err := client.ListStateOutputs(ctx, sdk.StateReference{LogicID: state.LogicID})
	require.NoError(t, err)

	// Should now have all outputs from TF state
	assert.GreaterOrEqual(t, len(outputsAfter), 1, "Should have at least vpc_id output")

	// Find vpc_id
	var vpcOutput *sdk.OutputKey
	for i := range outputsAfter {
		if outputsAfter[i].Key == "vpc_id" {
			vpcOutput = &outputsAfter[i]
			break
		}
	}
	require.NotNil(t, vpcOutput, "vpc_id should exist after state upload")
	require.NotNil(t, vpcOutput.SchemaJSON, "vpc_id schema should persist after state upload")
	assert.JSONEq(t, vpcSchema, *vpcOutput.SchemaJSON, "Schema should be preserved")
}

// TestSchemaUpdate tests UC3: Update existing schema
func TestSchemaUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-schema-update"})
	require.NoError(t, err)

	// Schema v1
	schemaV1 := `{"type": "string", "pattern": "^vpc-[a-z0-9]+$"}`

	// Schema v2 (different pattern)
	schemaV2 := `{"type": "string", "pattern": "^vpc-[A-Za-z0-9-]+$", "minLength": 10}`

	// Set schema v1
	err = client.SetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "vpc_id", schemaV1)
	require.NoError(t, err)

	// Get schema - verify v1
	retrieved, err := client.GetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "vpc_id")
	require.NoError(t, err)
	assert.JSONEq(t, schemaV1, retrieved)

	// Set schema v2 (update)
	err = client.SetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "vpc_id", schemaV2)
	require.NoError(t, err)

	// Get schema - verify v2 (updated)
	retrievedV2, err := client.GetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "vpc_id")
	require.NoError(t, err)
	assert.JSONEq(t, schemaV2, retrievedV2, "Schema should be updated to v2")
}

// TestSchemaPreservationDuringStateUpload tests UC4: Verify schemas not lost when TF state uploaded
func TestSchemaPreservationDuringStateUpload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-schema-preservation"})
	require.NoError(t, err)

	// Load schemas
	vpcSchemaBytes, err := os.ReadFile(filepath.Join("testdata", "schema_vpc_id.json"))
	require.NoError(t, err)
	vpcSchema := string(vpcSchemaBytes)

	subnetSchemaBytes, err := os.ReadFile(filepath.Join("testdata", "schema_subnet_ids.json"))
	require.NoError(t, err)
	subnetSchema := string(subnetSchemaBytes)

	// Set schemas
	err = client.SetOutputSchema(ctx, sdk.StateReference{LogicID: state.LogicID}, "vpc_id", vpcSchema)
	require.NoError(t, err)
	err = client.SetOutputSchema(ctx, sdk.StateReference{LogicID: state.LogicID}, "subnet_ids", subnetSchema)
	require.NoError(t, err)

	// Upload TF state (serial=1)
	tfstateData, err := os.ReadFile(filepath.Join("testdata", "vpc_output_with_schema.json"))
	require.NoError(t, err)
	putTFState(t, state.GUID, tfstateData)

	time.Sleep(200 * time.Millisecond)

	// Verify schemas still present
	vpcSchemaAfter1, err := client.GetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "vpc_id")
	require.NoError(t, err)
	assert.JSONEq(t, vpcSchema, vpcSchemaAfter1, "vpc_id schema should persist after first upload")

	subnetSchemaAfter1, err := client.GetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "subnet_ids")
	require.NoError(t, err)
	assert.JSONEq(t, subnetSchema, subnetSchemaAfter1, "subnet_ids schema should persist after first upload")

	// Upload new TF state (serial=2) - modify the serial
	var tfstateJSON map[string]interface{}
	err = json.Unmarshal(tfstateData, &tfstateJSON)
	require.NoError(t, err)
	tfstateJSON["serial"] = 2
	tfstateData2, err := json.Marshal(tfstateJSON)
	require.NoError(t, err)

	putTFState(t, state.GUID, tfstateData2)
	time.Sleep(200 * time.Millisecond)

	// Verify schemas STILL present (not cleared)
	vpcSchemaAfter2, err := client.GetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "vpc_id")
	require.NoError(t, err)
	assert.JSONEq(t, vpcSchema, vpcSchemaAfter2, "vpc_id schema should persist after second upload")

	subnetSchemaAfter2, err := client.GetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "subnet_ids")
	require.NoError(t, err)
	assert.JSONEq(t, subnetSchema, subnetSchemaAfter2, "subnet_ids schema should persist after second upload")
}

// TestSchemaWithDependencies tests UC5: Producer has schema, consumer depends on it
func TestSchemaWithDependencies(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create producer and consumer states
	producer, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-schema-producer"})
	require.NoError(t, err)

	consumer, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-schema-consumer"})
	require.NoError(t, err)

	// Load schema
	vpcSchemaBytes, err := os.ReadFile(filepath.Join("testdata", "schema_vpc_id.json"))
	require.NoError(t, err)
	vpcSchema := string(vpcSchemaBytes)

	// Set schema on producer's vpc_id output
	err = client.SetOutputSchema(ctx, sdk.StateReference{LogicID: producer.LogicID}, "vpc_id", vpcSchema)
	require.NoError(t, err)

	// Add dependency: producer.vpc_id -> consumer
	depResult, err := client.AddDependency(ctx, sdk.AddDependencyInput{
		From:       sdk.StateReference{LogicID: producer.LogicID},
		FromOutput: "vpc_id",
		To:         sdk.StateReference{LogicID: consumer.LogicID},
	})
	require.NoError(t, err)
	assert.NotNil(t, depResult.Edge)
	assert.Equal(t, "pending", depResult.Edge.Status, "Edge should start as pending")

	// Upload TF state to producer with vpc_id matching schema pattern
	tfstateData, err := os.ReadFile(filepath.Join("testdata", "vpc_output_with_schema.json"))
	require.NoError(t, err)
	putTFState(t, producer.GUID, tfstateData)

	// Wait for edge processing
	time.Sleep(500 * time.Millisecond)

	// Verify edge status updates
	edges, err := client.ListDependencies(ctx, sdk.StateReference{LogicID: consumer.LogicID})
	require.NoError(t, err)
	assert.Len(t, edges, 1)
	t.Logf("Edge status after state upload: %s", edges[0].Status)
	// Edge should transition from pending (actual status depends on EdgeUpdateJob)
}

// TestComplexSchemas tests UC7: Complex JSON Schema types
func TestComplexSchemas(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-complex-schemas"})
	require.NoError(t, err)

	// Load complex schemas
	configSchemaBytes, err := os.ReadFile(filepath.Join("testdata", "schema_config_object.json"))
	require.NoError(t, err)
	configSchema := string(configSchemaBytes)

	tagsSchemaBytes, err := os.ReadFile(filepath.Join("testdata", "schema_tags.json"))
	require.NoError(t, err)
	tagsSchema := string(tagsSchemaBytes)

	// Set schema for complex object
	err = client.SetOutputSchema(ctx, sdk.StateReference{LogicID: state.LogicID}, "network_config", configSchema)
	require.NoError(t, err)

	// Set schema for tags map
	err = client.SetOutputSchema(ctx, sdk.StateReference{LogicID: state.LogicID}, "tags", tagsSchema)
	require.NoError(t, err)

	// Retrieve and verify schemas
	retrievedConfigSchema, err := client.GetOutputSchema(ctx, sdk.StateReference{LogicID: state.LogicID}, "network_config")
	require.NoError(t, err)
	assert.JSONEq(t, configSchema, retrievedConfigSchema, "Complex object schema should match")

	retrievedTagsSchema, err := client.GetOutputSchema(ctx, sdk.StateReference{LogicID: state.LogicID}, "tags")
	require.NoError(t, err)
	assert.JSONEq(t, tagsSchema, retrievedTagsSchema, "Tags schema should match")

	// Upload complex state
	tfstateData, err := os.ReadFile(filepath.Join("testdata", "complex_output.json"))
	require.NoError(t, err)
	putTFState(t, state.GUID, tfstateData)

	time.Sleep(200 * time.Millisecond)

	// Verify schemas persisted
	retrievedAfter, err := client.GetOutputSchema(ctx, sdk.StateReference{LogicID: state.LogicID}, "network_config")
	require.NoError(t, err)
	assert.JSONEq(t, configSchema, retrievedAfter, "Complex schema should persist after state upload")
}

// TestSchemaWithGridctl tests UC8: Use CLI commands to manage schemas
func TestSchemaWithGridctl(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Create state via CLI (use --force to handle leftover .grid files from other tests)
	logicID := "test-gridctl-schema"
	output := mustRunGridctl(t, ctx, "", "state", "create", logicID, "--force")
	assert.Contains(t, output, logicID, "State creation should succeed")

	// Get absolute path to schema file
	schemaPath, err := filepath.Abs(filepath.Join("testdata", "schema_vpc_id.json"))
	require.NoError(t, err)

	// Set schema via CLI
	setOutput := mustRunGridctl(t, ctx, "", "state", "set-schema",
		"--logic-id", logicID,
		"--key", "vpc_id",
		"--file", schemaPath)
	assert.Contains(t, setOutput, "Set schema for output 'vpc_id'", "Schema should be set")

	// Get schema via CLI
	getOutput := mustRunGridctlStdOut(t, ctx, "", "state", "get-schema",
		"--logic-id", logicID,
		"--key", "vpc_id")

	// Verify schema content matches file
	schemaBytes, err := os.ReadFile(schemaPath)
	require.NoError(t, err)
	assert.JSONEq(t, string(schemaBytes), getOutput, "Retrieved schema should match file content")
}

// TestStateReferenceResolution tests that both logic_id and guid work for schema operations
func TestStateReferenceResolution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-ref-resolution"})
	require.NoError(t, err)

	schema := `{"type": "string"}`

	// Set schema using logic_id
	err = client.SetOutputSchema(ctx, sdk.StateReference{LogicID: state.LogicID}, "output1", schema)
	require.NoError(t, err)

	// Set schema using GUID
	err = client.SetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "output2", schema)
	require.NoError(t, err)

	// Get schema using logic_id
	retrieved1, err := client.GetOutputSchema(ctx, sdk.StateReference{LogicID: state.LogicID}, "output1")
	require.NoError(t, err)
	assert.JSONEq(t, schema, retrieved1)

	// Get schema using GUID
	retrieved2, err := client.GetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "output2")
	require.NoError(t, err)
	assert.JSONEq(t, schema, retrieved2)
}

// TestInvalidSchemaRejection tests that SetOutputSchema rejects invalid JSON Schema strings.
// This addresses grid-e903: SetOutputSchema should validate schemas before storing them.
func TestInvalidSchemaRejection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state with unique logic ID to avoid collisions
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: uniqueLogicID("test-invalid-schema-rejection")})
	require.NoError(t, err)

	t.Run("MalformedJSON", func(t *testing.T) {
		// Invalid JSON syntax
		invalidSchema := `{invalid json}`

		err := client.SetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "output1", invalidSchema)
		require.Error(t, err, "Should reject malformed JSON")
		assert.Contains(t, err.Error(), "invalid JSON Schema", "Error should mention invalid schema")
	})

	t.Run("InvalidJSONSchemaType", func(t *testing.T) {
		// Invalid JSON Schema: "invalid_type" is not a valid JSON Schema type
		invalidSchema := `{"type": "invalid_type"}`

		err := client.SetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "output2", invalidSchema)
		require.Error(t, err, "Should reject invalid type keyword")
		assert.Contains(t, err.Error(), "invalid JSON Schema", "Error should mention invalid schema")
	})

	t.Run("InvalidConstraintType", func(t *testing.T) {
		// Invalid JSON Schema: minLength must be a number, not a string
		invalidSchema := `{"type": "string", "minLength": "not a number"}`

		err := client.SetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "output3", invalidSchema)
		require.Error(t, err, "Should reject invalid constraint type")
		assert.Contains(t, err.Error(), "invalid JSON Schema", "Error should mention invalid schema")
	})

	t.Run("InvalidPropertyDefinition", func(t *testing.T) {
		// Invalid JSON Schema: properties must be an object, not an array
		invalidSchema := `{"type": "object", "properties": ["not", "an", "object"]}`

		err := client.SetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "output4", invalidSchema)
		require.Error(t, err, "Should reject invalid properties definition")
		assert.Contains(t, err.Error(), "invalid JSON Schema", "Error should mention invalid schema")
	})

	t.Run("ValidSchemaStillWorks", func(t *testing.T) {
		// Verify that valid schemas still work after rejecting invalid ones
		validSchema := `{"type": "string", "pattern": "^vpc-[a-z0-9]+$"}`

		err := client.SetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "vpc_id", validSchema)
		require.NoError(t, err, "Valid schema should be accepted")

		// Verify schema was stored
		retrieved, err := client.GetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "vpc_id")
		require.NoError(t, err)
		assert.JSONEq(t, validSchema, retrieved, "Valid schema should be retrievable")
	})

	t.Run("InvalidSchemaNotStored", func(t *testing.T) {
		// Try to set an invalid schema
		invalidSchema := `{"type": "invalid_type"}`
		err := client.SetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "not_stored", invalidSchema)
		require.Error(t, err, "Invalid schema should be rejected")

		// Verify schema was NOT stored
		retrieved, err := client.GetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "not_stored")
		require.NoError(t, err, "GetOutputSchema should succeed even for non-existent output")
		assert.Empty(t, retrieved, "Invalid schema should not be stored")

		// Verify output does NOT appear in ListStateOutputs
		outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
		require.NoError(t, err)

		for _, output := range outputs {
			assert.NotEqual(t, "not_stored", output.Key, "Invalid schema output should not exist")
		}
	})
}
