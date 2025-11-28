package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/terraconstructs/grid/pkg/sdk"
)

// TestEdgeStatusCompositeModel tests the composite edge status model with drift × validation dimensions.
// Validates that edge status correctly reflects both drift (in_digest vs out_digest) and validation (schema compliance).
func TestEdgeStatusCompositeModel(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := newSDKClient()

	// Setup: Create producer and consumer states
	producer, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: uniqueLogicID("edge-composite-producer")})
	require.NoError(t, err)

	consumer, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: uniqueLogicID("edge-composite-consumer")})
	require.NoError(t, err)

	// Create dependency edge: consumer depends on producer's vpc_id output
	depResult, err := client.AddDependency(ctx, sdk.AddDependencyInput{
		From:       sdk.StateReference{GUID: producer.GUID},
		FromOutput: "vpc_id",
		To:         sdk.StateReference{GUID: consumer.GUID},
	})
	require.NoError(t, err)
	edgeID := depResult.Edge.ID

	// === Test 1: clean → clean-invalid (validation fails, no drift) ===
	t.Run("CleanToCleanInvalid", func(t *testing.T) {
		// Step 1: Producer uploads state with a value that will later violate schema
		tfstate1 := createTFState(1, "vpc-INVALID")
		err = uploadTerraformState(producer.GUID, tfstate1)
		require.NoError(t, err)
		time.Sleep(200 * time.Millisecond)

		// Edge should be dirty (consumer hasn't observed yet)
		edges, err := client.ListDependencies(ctx, sdk.StateReference{GUID: consumer.GUID})
		require.NoError(t, err)
		require.Len(t, edges, 1)
		t.Logf("Edge %d status after producer upload: %s", edgeID, edges[0].Status)

		// Step 2: Consumer observes producer's output → edge becomes clean
		tfstateConsumer := createTFStateEmpty(1)
		err = uploadTerraformState(consumer.GUID, tfstateConsumer)
		require.NoError(t, err)
		time.Sleep(200 * time.Millisecond)

		edges, err = client.ListDependencies(ctx, sdk.StateReference{GUID: consumer.GUID})
		require.NoError(t, err)
		assert.Equal(t, "clean", edges[0].Status, "Edge should be clean after consumer observes")

		// Step 3: Add strict schema for vpc_id
		schemaBytes, err := os.ReadFile(filepath.Join("testdata", "schema_pattern_strict.json"))
		require.NoError(t, err)
		err = client.SetOutputSchema(ctx, sdk.StateReference{GUID: producer.GUID}, "vpc_id", string(schemaBytes))
		require.NoError(t, err)

		// Step 4: Producer re-uploads SAME value (same fingerprint from consumer's perspective)
		// Serial increments but vpc_id value stays "vpc-INVALID"
		tfstate2 := createTFState(2, "vpc-INVALID")
		err = uploadTerraformState(producer.GUID, tfstate2)
		require.NoError(t, err)
		time.Sleep(200 * time.Millisecond)

		// Edge should now be clean-invalid (consumer up-to-date but value fails validation)
		edges, err = client.ListDependencies(ctx, sdk.StateReference{GUID: consumer.GUID})
		require.NoError(t, err)
		assert.Equal(t, "clean-invalid", edges[0].Status, "Edge should be clean-invalid (up-to-date but invalid)")
	})

	// === Test 2: clean-invalid → clean (validation passes) ===
	t.Run("CleanInvalidToClean", func(t *testing.T) {
		// Continuing from Test 1, edge is clean-invalid
		// Producer uploads VALID vpc_id
		tfstate3 := createTFState(3, "vpc-0123456789abcdef")
		err = uploadTerraformState(producer.GUID, tfstate3)
		require.NoError(t, err)

		// Consumer observes the new valid value
		tfstateConsumer := createTFStateEmpty(2)
		err = uploadTerraformState(consumer.GUID, tfstateConsumer)
		require.NoError(t, err)
		time.Sleep(200 * time.Millisecond)

		// Edge should be clean (validation passed)
		edges, err := client.ListDependencies(ctx, sdk.StateReference{GUID: consumer.GUID})
		require.NoError(t, err)
		assert.Equal(t, "clean", edges[0].Status, "Edge should be clean (validation passed)")
	})

	// === Test 3: dirty-invalid (both drift and validation fail) ===
	t.Run("DirtyInvalid", func(t *testing.T) {
		// Producer uploads INVALID vpc_id with different value
		tfstate4 := createTFState(4, "vpc-BADVALUE")
		err = uploadTerraformState(producer.GUID, tfstate4)
		require.NoError(t, err)
		time.Sleep(200 * time.Millisecond)

		// Edge should be dirty-invalid (stale AND invalid)
		edges, err := client.ListDependencies(ctx, sdk.StateReference{GUID: consumer.GUID})
		require.NoError(t, err)
		assert.Equal(t, "dirty-invalid", edges[0].Status, "Edge should be dirty-invalid (stale and invalid)")
	})

	// === Test 4: dirty-invalid → dirty (validation passes but still stale) ===
	t.Run("DirtyInvalidToDirty", func(t *testing.T) {
		// Producer uploads VALID vpc_id (different value, so still dirty)
		tfstate5 := createTFState(5, "vpc-fedcba9876543210")
		err = uploadTerraformState(producer.GUID, tfstate5)
		require.NoError(t, err)
		time.Sleep(200 * time.Millisecond)

		// Edge should be dirty (validation fixed but consumer still stale)
		edges, err := client.ListDependencies(ctx, sdk.StateReference{GUID: consumer.GUID})
		require.NoError(t, err)
		assert.Equal(t, "dirty", edges[0].Status, "Edge should be dirty (validation passed but still stale)")

		// Consumer observes → edge becomes clean
		tfstateConsumer := createTFStateEmpty(3)
		err = uploadTerraformState(consumer.GUID, tfstateConsumer)
		require.NoError(t, err)
		time.Sleep(200 * time.Millisecond)

		edges, err = client.ListDependencies(ctx, sdk.StateReference{GUID: consumer.GUID})
		require.NoError(t, err)
		assert.Equal(t, "clean", edges[0].Status, "Edge should be clean after consumer observes valid value")
	})
}

// createTFState creates a minimal Terraform state JSON with a vpc_id output.
func createTFState(serial int, vpcID string) []byte {
	state := map[string]interface{}{
		"version":           4,
		"terraform_version": "1.5.0",
		"serial":            serial,
		"lineage":           fmt.Sprintf("test-lineage-%d", serial),
		"outputs": map[string]interface{}{
			"vpc_id": map[string]interface{}{
				"value": vpcID,
				"type":  "string",
			},
		},
	}
	data, _ := json.Marshal(state)
	return data
}

// createTFStateEmpty creates a minimal Terraform state JSON with no outputs (for consumer).
func createTFStateEmpty(serial int) []byte {
	state := map[string]interface{}{
		"version":           4,
		"terraform_version": "1.5.0",
		"serial":            serial,
		"lineage":           fmt.Sprintf("test-lineage-consumer-%d", serial),
		"outputs":           map[string]interface{}{},
	}
	data, _ := json.Marshal(state)
	return data
}
