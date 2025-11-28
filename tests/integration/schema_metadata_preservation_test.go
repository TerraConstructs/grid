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

// TestSchemaMetadataPreservedOnRefresh tests bug fix for grid-58bb
// Reproduces: Schema metadata (schema_source, validation_status, etc.) lost on terraform refresh
func TestSchemaMetadataPreservedOnRefresh(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-metadata-preservation"})
	require.NoError(t, err)

	// First upload: Triggers inference
	stateBytes, err := os.ReadFile(filepath.Join("testdata", "tfstate_string_output.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, stateBytes)
	require.NoError(t, err, "Failed to upload Terraform state")

	// Wait for inference to complete
	time.Sleep(500 * time.Millisecond)

	// Verify initial state: schema inferred and validated
	outputs1, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)

	var vpcOutput1 *sdk.OutputKey
	for i := range outputs1 {
		if outputs1[i].Key == "vpc_id" {
			vpcOutput1 = &outputs1[i]
			break
		}
	}
	require.NotNil(t, vpcOutput1, "vpc_id output should exist")
	require.NotNil(t, vpcOutput1.SchemaJSON, "Schema should be inferred")
	require.NotNil(t, vpcOutput1.SchemaSource, "SchemaSource should be set")
	assert.Equal(t, "inferred", *vpcOutput1.SchemaSource, "Schema source should be 'inferred'")

	// Store initial metadata for comparison
	initialSchemaJSON := *vpcOutput1.SchemaJSON
	initialSchemaSource := *vpcOutput1.SchemaSource

	// Capture validation metadata if present
	var initialValidationStatus *string
	var initialValidatedAt *time.Time
	if vpcOutput1.ValidationStatus != nil {
		initialValidationStatus = vpcOutput1.ValidationStatus
	}
	if vpcOutput1.ValidatedAt != nil {
		initialValidatedAt = vpcOutput1.ValidatedAt
	}

	// Second upload: Simulate terraform refresh (same state, same serial)
	// This triggers UpsertOutputs with ON CONFLICT DO UPDATE
	err = uploadTerraformState(state.GUID, stateBytes)
	require.NoError(t, err, "Failed to re-upload Terraform state")

	// Small delay for processing
	time.Sleep(200 * time.Millisecond)

	// Verify schema metadata is PRESERVED (this fails with bug grid-58bb)
	outputs2, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)

	var vpcOutput2 *sdk.OutputKey
	for i := range outputs2 {
		if outputs2[i].Key == "vpc_id" {
			vpcOutput2 = &outputs2[i]
			break
		}
	}
	require.NotNil(t, vpcOutput2, "vpc_id output should still exist")

	// Core assertions: schema_json and schema_source MUST be preserved
	require.NotNil(t, vpcOutput2.SchemaJSON, "Schema JSON should be preserved")
	assert.JSONEq(t, initialSchemaJSON, *vpcOutput2.SchemaJSON, "Schema JSON should be unchanged")

	require.NotNil(t, vpcOutput2.SchemaSource, "SchemaSource should be preserved (BUG grid-58bb: was returning nil)")
	assert.Equal(t, initialSchemaSource, *vpcOutput2.SchemaSource, "SchemaSource should remain 'inferred'")

	// Note: Validation fields may change because validation job runs on every upload
	// This is expected behavior - we just verify they're still populated
	if initialValidationStatus != nil {
		require.NotNil(t, vpcOutput2.ValidationStatus, "ValidationStatus should still exist")
		// Status may change from "not_validated" to "valid" - that's OK
	}

	if initialValidatedAt != nil {
		require.NotNil(t, vpcOutput2.ValidatedAt, "ValidatedAt should still exist")
		// Timestamp may update when validation re-runs - that's OK
	}
}

// TestSchemaMetadataPreservedWithValidation tests metadata preservation after validation runs
func TestSchemaMetadataPreservedWithValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-metadata-validation-preservation"})
	require.NoError(t, err)

	// Set manual schema first (so we have schema_source="manual")
	manualSchemaBytes, err := os.ReadFile(filepath.Join("testdata", "schema_vpc_id.json"))
	require.NoError(t, err)
	manualSchema := string(manualSchemaBytes)

	err = client.SetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "vpc_id", manualSchema)
	require.NoError(t, err)

	// Upload state (triggers validation)
	stateBytes, err := os.ReadFile(filepath.Join("testdata", "tfstate_string_output.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, stateBytes)
	require.NoError(t, err)

	// Wait for validation to complete
	time.Sleep(1 * time.Second)

	// Get initial state with validation metadata
	outputs1, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)

	var vpcOutput1 *sdk.OutputKey
	for i := range outputs1 {
		if outputs1[i].Key == "vpc_id" {
			vpcOutput1 = &outputs1[i]
			break
		}
	}
	require.NotNil(t, vpcOutput1)
	require.NotNil(t, vpcOutput1.SchemaSource, "SchemaSource should be 'manual'")
	assert.Equal(t, "manual", *vpcOutput1.SchemaSource)

	// Should have validation metadata
	require.NotNil(t, vpcOutput1.ValidationStatus, "ValidationStatus should be set after validation")

	// Re-upload (terraform refresh scenario)
	err = uploadTerraformState(state.GUID, stateBytes)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	// Verify ALL metadata preserved
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

	// Core assertion: schema_source MUST remain "manual"
	require.NotNil(t, vpcOutput2.SchemaSource, "SchemaSource should be preserved")
	assert.Equal(t, "manual", *vpcOutput2.SchemaSource, "SchemaSource should remain 'manual'")

	// Validation fields should still exist (may be updated by validation job)
	require.NotNil(t, vpcOutput2.ValidationStatus, "ValidationStatus should still exist")
	require.NotNil(t, vpcOutput2.ValidatedAt, "ValidatedAt should still exist")
	// Note: Values may change when validation re-runs - that's expected behavior
}

// TestSchemaMetadataPreservedOnSerialChange tests metadata preservation across serial increments
func TestSchemaMetadataPreservedOnSerialChange(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-metadata-serial-change"})
	require.NoError(t, err)

	// Upload initial state (serial 1)
	stateBytes1, err := os.ReadFile(filepath.Join("testdata", "tfstate_string_output.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, stateBytes1)
	require.NoError(t, err)

	// Wait for inference
	time.Sleep(500 * time.Millisecond)

	outputs1, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)

	var vpcOutput1 *sdk.OutputKey
	for i := range outputs1 {
		if outputs1[i].Key == "vpc_id" {
			vpcOutput1 = &outputs1[i]
			break
		}
	}
	require.NotNil(t, vpcOutput1)
	require.NotNil(t, vpcOutput1.SchemaSource)
	assert.Equal(t, "inferred", *vpcOutput1.SchemaSource)

	// Modify state to increment serial
	var stateData map[string]any
	err = json.Unmarshal(stateBytes1, &stateData)
	require.NoError(t, err)

	stateData["serial"] = float64(2) // Increment serial

	stateBytes2, err := json.Marshal(stateData)
	require.NoError(t, err)

	// Upload with new serial
	err = uploadTerraformState(state.GUID, stateBytes2)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	// Verify metadata preserved across serial change
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
	require.NotNil(t, vpcOutput2.SchemaSource, "SchemaSource should be preserved across serial change")
	assert.Equal(t, "inferred", *vpcOutput2.SchemaSource, "SchemaSource should remain 'inferred'")
}
