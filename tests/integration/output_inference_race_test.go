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

// TestInferenceDoesNotResurrectRemovedOutput tests that async inference does not
// resurrect outputs removed by a subsequent state upload.
// This validates the fix for grid-f430 (inference serial check).
//
// Race Condition Scenario:
// 1. POST A (serial=10): Creates vpc_id output, fires inference goroutine
// 2. Wait 50ms (let inference start but not complete)
// 3. POST B (serial=11): Removes vpc_id output, purges row
// 4. Wait 200ms (let inference complete)
// 5. Verify vpc_id does NOT reappear (inference skipped due to serial check)
func TestInferenceDoesNotResurrectRemovedOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-inference-no-resurrection"})
	require.NoError(t, err)

	// POST A: Upload Terraform state WITH vpc_id output (serial=10)
	// This triggers inference goroutine in background
	stateWithVPC, err := os.ReadFile(filepath.Join("testdata", "tfstate_with_vpc_serial10.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, stateWithVPC)
	require.NoError(t, err, "Failed to upload state with vpc_id")

	// Wait 50ms to let inference START but NOT complete
	// Inference typically takes 100-200ms, so this ensures we're mid-inference
	time.Sleep(50 * time.Millisecond)

	// POST B: Upload Terraform state WITHOUT vpc_id output (serial=11)
	// This should purge the vpc_id row
	stateWithoutVPC, err := os.ReadFile(filepath.Join("testdata", "tfstate_without_vpc_serial11.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, stateWithoutVPC)
	require.NoError(t, err, "Failed to upload state without vpc_id")

	// Wait 200ms for inference from POST A to complete
	// The inference goroutine will attempt to write the schema, but should be blocked by serial check
	time.Sleep(200 * time.Millisecond)

	// Verify vpc_id does NOT exist (was NOT resurrected by late inference)
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)

	// Check that vpc_id is not in the outputs list
	for _, output := range outputs {
		assert.NotEqual(t, "vpc_id", output.Key, "vpc_id should not be resurrected by late inference")
	}

	// Double-check: try to get vpc_id schema directly (should return empty)
	vpcSchema, err := client.GetOutputSchema(ctx, sdk.StateReference{GUID: state.GUID}, "vpc_id")
	require.NoError(t, err)
	assert.Equal(t, "", vpcSchema, "vpc_id schema should not exist")
}

// TestInferenceCompletesBeforeRemoval tests the normal case where inference
// completes before the output is removed (no race condition).
func TestInferenceCompletesBeforeRemoval(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-inference-normal-case"})
	require.NoError(t, err)

	// POST A: Upload state with vpc_id (serial=10)
	stateWithVPC, err := os.ReadFile(filepath.Join("testdata", "tfstate_with_vpc_serial10.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, stateWithVPC)
	require.NoError(t, err)

	// Wait for inference to complete fully
	time.Sleep(500 * time.Millisecond)

	// Verify inferred schema was created
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)
	require.Len(t, outputs, 1, "Should have 1 output with inferred schema")

	vpcOutput := outputs[0]
	assert.Equal(t, "vpc_id", vpcOutput.Key)
	require.NotNil(t, vpcOutput.SchemaJSON, "Schema should be inferred")
	require.NotNil(t, vpcOutput.SchemaSource, "Schema source should be set")
	assert.Equal(t, "inferred", *vpcOutput.SchemaSource, "Schema source should be 'inferred'")

	// POST B: Remove vpc_id (serial=11)
	stateWithoutVPC, err := os.ReadFile(filepath.Join("testdata", "tfstate_without_vpc_serial11.json"))
	require.NoError(t, err)

	err = uploadTerraformState(state.GUID, stateWithoutVPC)
	require.NoError(t, err)

	// Verify vpc_id was purged (normal behavior)
	outputs, err = client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)
	assert.Len(t, outputs, 0, "vpc_id should be purged after removal")
}

// TestRapidStatePOSTs tests multiple rapid state updates with different serials
// to verify serial monotonicity and correct purge behavior.
func TestRapidStatePOSTs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Create state
	state, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-rapid-posts"})
	require.NoError(t, err)

	// Rapid POST sequence:
	// POST serial=10 (with vpc_id) → POST serial=11 (without vpc_id) → wait for inference

	stateWithVPC, err := os.ReadFile(filepath.Join("testdata", "tfstate_with_vpc_serial10.json"))
	require.NoError(t, err)

	stateWithoutVPC, err := os.ReadFile(filepath.Join("testdata", "tfstate_without_vpc_serial11.json"))
	require.NoError(t, err)

	// POST serial=10
	err = uploadTerraformState(state.GUID, stateWithVPC)
	require.NoError(t, err)

	// Immediately POST serial=11 (before inference completes)
	// This simulates a user rapidly running terraform apply
	time.Sleep(10 * time.Millisecond) // Minimal delay
	err = uploadTerraformState(state.GUID, stateWithoutVPC)
	require.NoError(t, err)

	// Wait for any pending inference to complete
	time.Sleep(500 * time.Millisecond)

	// Verify final state: no outputs (vpc_id should NOT be resurrected)
	outputs, err := client.ListStateOutputs(ctx, sdk.StateReference{GUID: state.GUID})
	require.NoError(t, err)
	assert.Len(t, outputs, 0, "No outputs should exist after rapid serial=10→serial=11 POSTs")
}
