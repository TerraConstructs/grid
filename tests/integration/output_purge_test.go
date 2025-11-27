package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/terraconstructs/grid/pkg/sdk"
)

// TestManualSchemaOnlyRowSurvivesPurge tests that schema-only rows (schema_source=manual, state_serial=0)
// survive purge when output not in Terraform state.
// This validates the fix for grid-1e1f (purge logic bug).
func TestManualSchemaOnlyRowSurvivesPurge(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-manual-schema-survives-purge"})
	require.NoError(t, err)

	// Load schema fixture
	vpcSchemaBytes, err := os.ReadFile(filepath.Join("testdata", "schema_vpc_id.json"))
	require.NoError(t, err)
	vpcSchema := string(vpcSchemaBytes)

	// Pre-declare schema BEFORE output exists in Terraform state
	err = client.SetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "vpc_id", vpcSchema)
	require.NoError(t, err, "Failed to set vpc_id schema")

	// Verify schema-only row was created
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)
	require.Len(t, outputs, 1, "Should have 1 schema-only output")

	vpcOutput := outputs[0]
	require.NotNil(t, vpcOutput.SchemaJSON, "Schema should be set")
	require.NotNil(t, vpcOutput.SchemaSource, "Schema source should be set")
	assert.Equal(t, "manual", *vpcOutput.SchemaSource, "Schema source should be 'manual'")

	// Upload Terraform state WITHOUT vpc_id output (should trigger purge logic)
	// This state has no outputs, so purge logic will run
	emptyStateBytes, err := os.ReadFile(filepath.Join("testdata", "empty_state.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, emptyStateBytes)
	require.NoError(t, err, "Failed to upload empty Terraform state")

	// Verify manual schema still exists (NOT purged)
	outputs, err = client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)
	require.Len(t, outputs, 1, "Manual schema should survive purge")

	// Verify schema details
	vpcOutput = outputs[0]
	assert.Equal(t, "vpc_id", vpcOutput.Key, "Output key should be vpc_id")
	require.NotNil(t, vpcOutput.SchemaJSON, "Schema should still be set")
	require.NotNil(t, vpcOutput.SchemaSource, "Schema source should still be set")
	assert.Equal(t, "manual", *vpcOutput.SchemaSource, "Schema source should still be 'manual'")
	assert.JSONEq(t, vpcSchema, *vpcOutput.SchemaJSON, "Schema should match original")

	// Upload another state without vpc_id to verify schema survives multiple purges
	err = uploadTerraformState(state.GUID, emptyStateBytes)
	require.NoError(t, err)

	outputs, err = client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)
	require.Len(t, outputs, 1, "Manual schema should survive multiple purges")
	assert.Equal(t, "vpc_id", outputs[0].Key, "Output key should still be vpc_id")
}

// TestInferredSchemaPurgedWhenOutputRemoved tests that inferred schemas are purged
// when the output is removed from Terraform state.
// This validates the fix for grid-1e1f (purge logic should NOT retain inferred schemas).
func TestInferredSchemaPurgedWhenOutputRemoved(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-inferred-schema-purged"})
	require.NoError(t, err)

	// Upload Terraform state WITH vpc_id output (triggers inference)
	stateWithVPC, err := os.ReadFile(filepath.Join("testdata", "tfstate_string_output.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, stateWithVPC)
	require.NoError(t, err)

	// Wait for inference to complete (async processing)
	time.Sleep(500 * time.Millisecond)

	// Verify inferred schema was created
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)
	require.Len(t, outputs, 1, "Should have 1 output with inferred schema")

	vpcOutput := outputs[0]
	require.NotNil(t, vpcOutput.SchemaJSON, "Schema should be inferred")
	require.NotNil(t, vpcOutput.SchemaSource, "Schema source should be set")
	assert.Equal(t, "inferred", *vpcOutput.SchemaSource, "Schema source should be 'inferred'")

	// Upload Terraform state WITHOUT vpc_id output (should purge inferred schema)
	emptyStateBytes, err := os.ReadFile(filepath.Join("testdata", "empty_state.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, emptyStateBytes)
	require.NoError(t, err)

	// Verify inferred schema was PURGED
	outputs, err = client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)
	assert.Len(t, outputs, 0, "Inferred schema should be purged when output removed")
}
