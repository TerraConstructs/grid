package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/terraconstructs/grid/pkg/sdk"
)

// TestDirectoryContextCreation tests .grid file creation and usage
func TestDirectoryContextCreation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create temporary directory for test isolation
	tempDir := t.TempDir()
	logicID := fmt.Sprintf("test-ctx-%d", time.Now().UnixNano())

	// Get gridctl path before changing directory
	gridctlPath := getGridctlPath(t)

	// Change to temp directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalDir) }()

	err = os.Chdir(tempDir)
	require.NoError(t, err)

	// 1. Create state - should create .grid file
	createCmd := exec.CommandContext(ctx, gridctlPath, "state", "create", logicID, "--server", serverURL)
	output, err := createCmd.CombinedOutput()
	require.NoError(t, err, "Failed to create state: %s", string(output))

	t.Logf("Create output: %s", string(output))

	// Verify .grid file was created
	gridPath := filepath.Join(tempDir, ".grid")
	require.FileExists(t, gridPath, ".grid file should be created")

	// Parse .grid file
	gridData, err := os.ReadFile(gridPath)
	require.NoError(t, err)

	var gridCtx struct {
		Version      string    `json:"version"`
		StateGUID    string    `json:"state_guid"`
		StateLogicID string    `json:"state_logic_id"`
		ServerURL    string    `json:"server_url"`
		CreatedAt    time.Time `json:"created_at"`
		UpdatedAt    time.Time `json:"updated_at"`
	}
	err = json.Unmarshal(gridData, &gridCtx)
	require.NoError(t, err, "Failed to parse .grid file")

	assert.Equal(t, "1", gridCtx.Version)
	assert.Equal(t, logicID, gridCtx.StateLogicID)
	assert.Equal(t, serverURL, gridCtx.ServerURL)
	assert.NotEmpty(t, gridCtx.StateGUID)

	t.Logf(".grid context: GUID=%s, LogicID=%s", gridCtx.StateGUID, gridCtx.StateLogicID)

	// 2. Use state get without arguments - should use .grid context
	getCmd := exec.CommandContext(ctx, gridctlPath, "state", "get", "--server", serverURL)
	output, err = getCmd.CombinedOutput()
	require.NoError(t, err, "Failed to get state using context: %s", string(output))

	// Verify output contains the logic ID
	assert.Contains(t, string(output), logicID, "Get output should contain logic ID from context")
	assert.Contains(t, string(output), gridCtx.StateGUID, "Get output should contain GUID from context")

	t.Logf("Get output (using context): %s", string(output))

	// 3. Try creating another state without --force - should error
	anotherLogicID := fmt.Sprintf("another-%d", time.Now().UnixNano())
	createCmd2 := exec.CommandContext(ctx, gridctlPath, "state", "create", anotherLogicID, "--server", serverURL)
	output, err = createCmd2.CombinedOutput()
	assert.Error(t, err, "Creating another state without --force should fail")
	assert.Contains(t, string(output), ".grid exists", "Error should mention existing .grid file")

	t.Logf("Expected error output: %s", string(output))

	// 4. Use --force flag to overwrite
	createCmd3 := exec.CommandContext(ctx, gridctlPath, "state", "create", anotherLogicID, "--server", serverURL, "--force")
	output, err = createCmd3.CombinedOutput()
	require.NoError(t, err, "Creating with --force should succeed: %s", string(output))

	// Verify .grid was updated
	gridData, err = os.ReadFile(gridPath)
	require.NoError(t, err)
	err = json.Unmarshal(gridData, &gridCtx)
	require.NoError(t, err)
	assert.Equal(t, anotherLogicID, gridCtx.StateLogicID, ".grid should be updated with new logic ID")

	t.Logf("Updated .grid context: GUID=%s, LogicID=%s", gridCtx.StateGUID, gridCtx.StateLogicID)
}

// TestContextAwareDepsAdd tests deps add command using .grid context for --to
func TestContextAwareDepsAdd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client := sdk.NewClient(serverURL)

	// Create temporary directory for consumer state
	consumerDir := t.TempDir()
	consumerLogicID := fmt.Sprintf("consumer-%d", time.Now().UnixNano())
	producerLogicID := fmt.Sprintf("producer-%d", time.Now().UnixNano())

	// Create producer state with outputs
	producerState, err := client.CreateState(ctx, sdk.CreateStateInput{
		GUID:    uuid.Must(uuid.NewV7()).String(),
		LogicID: producerLogicID,
	})
	require.NoError(t, err)

	// Upload Terraform state with outputs for producer
	tfState := map[string]interface{}{
		"version":           4,
		"terraform_version": "1.6.0",
		"serial":            1,
		"outputs": map[string]interface{}{
			"vpc_id": map[string]interface{}{
				"value":     "vpc-abc123",
				"type":      "string",
				"sensitive": false,
			},
			"subnet_id": map[string]interface{}{
				"value":     "subnet-xyz789",
				"type":      "string",
				"sensitive": false,
			},
			"db_password": map[string]interface{}{
				"value":     "secret123",
				"type":      "string",
				"sensitive": true,
			},
		},
	}

	tfStateJSON, err := json.Marshal(tfState)
	require.NoError(t, err)

	url := fmt.Sprintf("%s/tfstate/%s", serverURL, producerState.GUID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(tfStateJSON))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	t.Logf("Uploaded Terraform state to producer %s", producerLogicID)

	// Wait for output caching to complete
	time.Sleep(1 * time.Second)

	// Get gridctl path before changing directory
	gridctlPath := getGridctlPath(t)

	// Change to consumer directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalDir) }()

	err = os.Chdir(consumerDir)
	require.NoError(t, err)

	// Create consumer state (creates .grid file)
	createCmd := exec.CommandContext(ctx, gridctlPath, "state", "create", consumerLogicID, "--server", serverURL)
	output, err := createCmd.CombinedOutput()
	require.NoError(t, err, "Failed to create consumer state: %s", string(output))

	t.Logf("Created consumer state: %s", consumerLogicID)

	// Add dependency using context-aware --to (should use .grid)
	// Note: This test uses --output explicitly to avoid interactive prompt in tests
	addCmd := exec.CommandContext(ctx, gridctlPath, "deps", "add",
		"--from", producerLogicID,
		"--output", "vpc_id",
		"--server", serverURL,
		"--non-interactive")
	output, err = addCmd.CombinedOutput()
	require.NoError(t, err, "Failed to add dependency: %s", string(output))

	assert.Contains(t, string(output), "Dependency added")
	assert.Contains(t, string(output), producerLogicID)
	assert.Contains(t, string(output), consumerLogicID)

	t.Logf("Dependency added: %s", string(output))

	// Verify dependency was created
	listCmd := exec.CommandContext(ctx, gridctlPath, "deps", "list", "--server", serverURL)
	output, err = listCmd.CombinedOutput()
	require.NoError(t, err, "Failed to list dependencies: %s", string(output))

	assert.Contains(t, string(output), producerLogicID)
	assert.Contains(t, string(output), consumerLogicID)
	assert.Contains(t, string(output), "vpc_id")

	t.Logf("Dependencies list: %s", string(output))
}

// TestEdgeCreationAfterProducerHasOutputs tests the scenario where:
// 1. Producer state is created and has outputs
// 2. Consumer state is created
// 3. Edge is added (producer already has outputs)
// 4. Consumer posts state (observes producer outputs)
// Expected: Edge should be "clean" (not "pending")
func TestEdgeCreationAfterProducerHasOutputs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client := sdk.NewClient(serverURL)

	// 1. Create producer state
	producerLogicID := fmt.Sprintf("provider-%d", time.Now().UnixNano())
	producerState, err := client.CreateState(ctx, sdk.CreateStateInput{
		GUID:    uuid.Must(uuid.NewV7()).String(),
		LogicID: producerLogicID,
	})
	require.NoError(t, err)
	t.Logf("Created producer state: %s (GUID: %s)", producerLogicID, producerState.GUID)

	// 2. POST provider tfstate with outputs (BEFORE creating edge)
	tfState1 := map[string]interface{}{
		"version":           4,
		"terraform_version": "1.6.0",
		"serial":            1,
		"outputs": map[string]interface{}{
			"subnet_id": map[string]interface{}{
				"value":     "subnet-initial",
				"type":      "string",
				"sensitive": false,
			},
		},
	}

	tfStateJSON1, err := json.Marshal(tfState1)
	require.NoError(t, err)

	producerURL := fmt.Sprintf("%s/tfstate/%s", serverURL, producerState.GUID)
	req1, err := http.NewRequestWithContext(ctx, http.MethodPost, producerURL, bytes.NewReader(tfStateJSON1))
	require.NoError(t, err)
	req1.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp1, err := httpClient.Do(req1)
	require.NoError(t, err)
	defer resp1.Body.Close()
	require.Equal(t, http.StatusOK, resp1.StatusCode)

	t.Logf("Uploaded producer Terraform state (producer already has outputs)")

	// Wait for output caching to complete
	time.Sleep(2 * time.Second)

	// 3. Create consumer state
	consumerLogicID := fmt.Sprintf("consumer-%d", time.Now().UnixNano())
	consumerState, err := client.CreateState(ctx, sdk.CreateStateInput{
		GUID:    uuid.Must(uuid.NewV7()).String(),
		LogicID: consumerLogicID,
	})
	require.NoError(t, err)
	t.Logf("Created consumer state: %s (GUID: %s)", consumerLogicID, consumerState.GUID)

	// 4. Add dependency edge (producer ALREADY has outputs)
	dep1, err := client.AddDependency(ctx, sdk.AddDependencyInput{
		From:        sdk.StateReference{LogicID: producerLogicID},
		FromOutput:  "subnet_id",
		To:          sdk.StateReference{LogicID: consumerLogicID},
		ToInputName: "subnet_id",
	})
	require.NoError(t, err)
	t.Logf("Created dependency edge ID: %d, status: %s, InDigest: %s, OutDigest: %s",
		dep1.Edge.ID, dep1.Edge.Status, dep1.Edge.InDigest, dep1.Edge.OutDigest)

	// BUG: Edge should have InDigest set because producer already has outputs
	// Expected: status = "dirty", InDigest = non-empty, OutDigest = empty
	// Actual: status = "pending", InDigest = empty, OutDigest = empty
	assert.NotEmpty(t, dep1.Edge.InDigest, "Edge should have InDigest set when created (producer already has outputs)")
	assert.Equal(t, "dirty", dep1.Edge.Status, "Edge should be dirty when created (producer has outputs, consumer hasn't observed)")

	// 5. POST consumer tfstate
	consumerTfState := map[string]interface{}{
		"version":           4,
		"terraform_version": "1.6.0",
		"serial":            1,
		"outputs":           map[string]interface{}{},
	}
	consumerTfStateJSON, err := json.Marshal(consumerTfState)
	require.NoError(t, err)

	consumerURL := fmt.Sprintf("%s/tfstate/%s", serverURL, consumerState.GUID)
	consumerReq, err := http.NewRequestWithContext(ctx, http.MethodPost, consumerURL, bytes.NewReader(consumerTfStateJSON))
	require.NoError(t, err)
	consumerReq.Header.Set("Content-Type", "application/json")

	consumerResp, err := httpClient.Do(consumerReq)
	require.NoError(t, err)
	defer consumerResp.Body.Close()
	require.Equal(t, http.StatusOK, consumerResp.StatusCode)

	t.Logf("Uploaded consumer Terraform state (consumer observes producer output)")

	// Wait for edge update job
	time.Sleep(2 * time.Second)

	// Check edge status - should be "clean"
	edges, err := client.ListDependencies(ctx, sdk.StateReference{LogicID: consumerLogicID})
	require.NoError(t, err)

	var foundEdge *sdk.DependencyEdge
	for _, edge := range edges {
		if edge.ID == dep1.Edge.ID {
			foundEdge = &edge
			break
		}
	}
	require.NotNil(t, foundEdge, "Should find the edge")
	t.Logf("Edge after consumer observes: ID=%d, status=%s, InDigest=%s, OutDigest=%s",
		foundEdge.ID, foundEdge.Status, foundEdge.InDigest, foundEdge.OutDigest)

	// THIS IS THE BUG: Edge should be "clean" but is "pending"
	assert.Equal(t, "clean", foundEdge.Status, "Edge should be clean after consumer observes")
	assert.NotEmpty(t, foundEdge.InDigest, "InDigest should be set")
	assert.Equal(t, foundEdge.InDigest, foundEdge.OutDigest, "OutDigest should equal InDigest when clean")
}

// TestEdgeUpdateAndStatusComputation tests edge update logic and state status computation
func TestEdgeUpdateAndStatusComputation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client := sdk.NewClient(serverURL)

	// Create producer state
	producerLogicID := fmt.Sprintf("edge-producer-%d", time.Now().UnixNano())
	producerState, err := client.CreateState(ctx, sdk.CreateStateInput{
		GUID:    uuid.Must(uuid.NewV7()).String(),
		LogicID: producerLogicID,
	})
	require.NoError(t, err)
	t.Logf("Created producer state: %s (GUID: %s)", producerLogicID, producerState.GUID)

	// Create consumer state
	consumerLogicID := fmt.Sprintf("edge-consumer-%d", time.Now().UnixNano())
	consumerState, err := client.CreateState(ctx, sdk.CreateStateInput{
		GUID:    uuid.Must(uuid.NewV7()).String(),
		LogicID: consumerLogicID,
	})
	require.NoError(t, err)
	t.Logf("Created consumer state: %s (GUID: %s)", consumerLogicID, consumerState.GUID)

	// Add dependency before producer has any outputs
	dep1, err := client.AddDependency(ctx, sdk.AddDependencyInput{
		From:        sdk.StateReference{LogicID: producerLogicID},
		FromOutput:  "vpc_id",
		To:          sdk.StateReference{LogicID: consumerLogicID},
		ToInputName: "vpc_id",
	})
	require.NoError(t, err)
	t.Logf("Created dependency edge ID: %d, status: %s", dep1.Edge.ID, dep1.Edge.Status)

	// Edge should be in "pending" status since producer has no outputs yet
	assert.Equal(t, "pending", dep1.Edge.Status, "Edge should be pending when producer has no outputs")

	// Upload Terraform state with outputs for producer (serial 1)
	tfState1 := map[string]interface{}{
		"version":           4,
		"terraform_version": "1.6.0",
		"serial":            1,
		"outputs": map[string]interface{}{
			"vpc_id": map[string]interface{}{
				"value":     "vpc-initial",
				"type":      "string",
				"sensitive": false,
			},
		},
	}

	tfStateJSON1, err := json.Marshal(tfState1)
	require.NoError(t, err)

	url1 := fmt.Sprintf("%s/tfstate/%s", serverURL, producerState.GUID)
	req1, err := http.NewRequestWithContext(ctx, http.MethodPost, url1, bytes.NewReader(tfStateJSON1))
	require.NoError(t, err)
	req1.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp1, err := httpClient.Do(req1)
	require.NoError(t, err)
	defer resp1.Body.Close()
	require.Equal(t, http.StatusOK, resp1.StatusCode)

	t.Logf("Uploaded initial Terraform state (serial 1) to producer")

	// Wait for edge update job to process
	time.Sleep(2 * time.Second)

	// Check edge status - should now be "ready"
	// List dependencies by consumer state
	edges, err := client.ListDependencies(ctx, sdk.StateReference{LogicID: consumerLogicID})
	require.NoError(t, err)

	var foundEdge *sdk.DependencyEdge
	for _, edge := range edges {
		if edge.ID == dep1.Edge.ID {
			foundEdge = &edge
			break
		}
	}
	require.NotNil(t, foundEdge, "Should find the created edge")
	t.Logf("Edge after producer state upload: ID=%d, status=%s, InDigest=%s, OutDigest=%s", foundEdge.ID, foundEdge.Status, foundEdge.InDigest, foundEdge.OutDigest)

	assert.Equal(t, "dirty", foundEdge.Status, "Edge should be dirty after producer has output (consumer hasn't observed yet)")
	assert.NotEmpty(t, foundEdge.InDigest, "Edge should have InDigest set from producer output")
	assert.Empty(t, foundEdge.OutDigest, "Edge OutDigest should be empty until consumer observes")
	firstInDigest := foundEdge.InDigest

	// Upload consumer state (this should trigger dirty → clean transition)
	consumerTfState1 := map[string]interface{}{
		"version":           4,
		"terraform_version": "1.6.0",
		"serial":            1,
		"outputs":           map[string]interface{}{},
	}
	consumerTfStateJSON1, err := json.Marshal(consumerTfState1)
	require.NoError(t, err)

	consumerURL := fmt.Sprintf("%s/tfstate/%s", serverURL, consumerState.GUID)
	consumerReq1, err := http.NewRequestWithContext(ctx, http.MethodPost, consumerURL, bytes.NewReader(consumerTfStateJSON1))
	require.NoError(t, err)
	consumerReq1.Header.Set("Content-Type", "application/json")

	consumerResp1, err := httpClient.Do(consumerReq1)
	require.NoError(t, err)
	defer consumerResp1.Body.Close()
	require.Equal(t, http.StatusOK, consumerResp1.StatusCode)

	t.Logf("Uploaded consumer Terraform state (consumer observes producer output)")

	// Wait for edge update job to process
	time.Sleep(2 * time.Second)

	// Check edge status - should now be "clean"
	edgesAfterConsumer, err := client.ListDependencies(ctx, sdk.StateReference{LogicID: consumerLogicID})
	require.NoError(t, err)

	var foundEdgeAfterConsumer *sdk.DependencyEdge
	for _, edge := range edgesAfterConsumer {
		if edge.ID == dep1.Edge.ID {
			foundEdgeAfterConsumer = &edge
			break
		}
	}
	require.NotNil(t, foundEdgeAfterConsumer, "Should find the edge after consumer update")
	t.Logf("Edge after consumer observes: ID=%d, status=%s, InDigest=%s, OutDigest=%s", foundEdgeAfterConsumer.ID, foundEdgeAfterConsumer.Status, foundEdgeAfterConsumer.InDigest, foundEdgeAfterConsumer.OutDigest)

	// THIS IS THE REAL BUG: Edge should be "clean" but stays "dirty"
	assert.Equal(t, "clean", foundEdgeAfterConsumer.Status, "Edge should be clean after consumer observes producer output")
	assert.Equal(t, firstInDigest, foundEdgeAfterConsumer.OutDigest, "OutDigest should equal InDigest after consumer observes")
	assert.Equal(t, foundEdgeAfterConsumer.InDigest, foundEdgeAfterConsumer.OutDigest, "InDigest and OutDigest should match when clean")

	// Update producer state with new output value (serial 2)
	tfState2 := map[string]interface{}{
		"version":           4,
		"terraform_version": "1.6.0",
		"serial":            2,
		"outputs": map[string]interface{}{
			"vpc_id": map[string]interface{}{
				"value":     "vpc-updated",
				"type":      "string",
				"sensitive": false,
			},
		},
	}

	tfStateJSON2, err := json.Marshal(tfState2)
	require.NoError(t, err)

	req2, err := http.NewRequestWithContext(ctx, http.MethodPost, url1, bytes.NewReader(tfStateJSON2))
	require.NoError(t, err)
	req2.Header.Set("Content-Type", "application/json")

	resp2, err := httpClient.Do(req2)
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusOK, resp2.StatusCode)

	t.Logf("Uploaded updated Terraform state (serial 2) to producer")

	// Wait for edge update job to process
	time.Sleep(2 * time.Second)

	// Check edge status again - OutDigest should change, status should remain "ready"
	edges2, err := client.ListDependencies(ctx, sdk.StateReference{LogicID: consumerLogicID})
	require.NoError(t, err)

	var foundEdge2 *sdk.DependencyEdge
	for _, edge := range edges2 {
		if edge.ID == dep1.Edge.ID {
			foundEdge2 = &edge
			break
		}
	}
	require.NotNil(t, foundEdge2, "Should find the edge after producer update")
	t.Logf("Edge after producer updates output: ID=%d, status=%s, InDigest=%s, OutDigest=%s", foundEdge2.ID, foundEdge2.Status, foundEdge2.InDigest, foundEdge2.OutDigest)

	// Edge should go back to dirty because producer output changed
	assert.NotEqual(t, firstInDigest, foundEdge2.InDigest, "InDigest should change when producer output value changes")
	assert.Equal(t, "dirty", foundEdge2.Status, "Edge should be dirty after producer output changes (consumer hasn't observed new value)")
	assert.Equal(t, firstInDigest, foundEdge2.OutDigest, "OutDigest should still be the old value consumer observed")

	// Upload consumer state again (consumer observes new producer output)
	consumerTfState2 := map[string]interface{}{
		"version":           4,
		"terraform_version": "1.6.0",
		"serial":            2,
		"outputs":           map[string]interface{}{},
	}
	consumerTfStateJSON2, err := json.Marshal(consumerTfState2)
	require.NoError(t, err)

	consumerReq2, err := http.NewRequestWithContext(ctx, http.MethodPost, consumerURL, bytes.NewReader(consumerTfStateJSON2))
	require.NoError(t, err)
	consumerReq2.Header.Set("Content-Type", "application/json")

	consumerResp2, err := httpClient.Do(consumerReq2)
	require.NoError(t, err)
	defer consumerResp2.Body.Close()
	require.Equal(t, http.StatusOK, consumerResp2.StatusCode)

	t.Logf("Uploaded consumer state again (observes new producer output)")

	// Wait for edge update job
	time.Sleep(2 * time.Second)

	// Check edge status - should be clean again
	edgesAfterConsumer2, err := client.ListDependencies(ctx, sdk.StateReference{LogicID: consumerLogicID})
	require.NoError(t, err)

	var foundEdgeCleanAgain *sdk.DependencyEdge
	for _, edge := range edgesAfterConsumer2 {
		if edge.ID == dep1.Edge.ID {
			foundEdgeCleanAgain = &edge
			break
		}
	}
	require.NotNil(t, foundEdgeCleanAgain, "Should find the edge after second consumer update")
	t.Logf("Edge after consumer observes updated output: ID=%d, status=%s, InDigest=%s, OutDigest=%s", foundEdgeCleanAgain.ID, foundEdgeCleanAgain.Status, foundEdgeCleanAgain.InDigest, foundEdgeCleanAgain.OutDigest)

	assert.Equal(t, "clean", foundEdgeCleanAgain.Status, "Edge should be clean again after consumer observes new output")
	assert.Equal(t, foundEdgeCleanAgain.InDigest, foundEdgeCleanAgain.OutDigest, "InDigest and OutDigest should match")
	assert.NotEqual(t, firstInDigest, foundEdgeCleanAgain.OutDigest, "OutDigest should now be the new value")

	// Remove the output from producer state (serial 3)
	tfState3 := map[string]interface{}{
		"version":           4,
		"terraform_version": "1.6.0",
		"serial":            3,
		"outputs":           map[string]interface{}{}, // No outputs
	}

	tfStateJSON3, err := json.Marshal(tfState3)
	require.NoError(t, err)

	req3, err := http.NewRequestWithContext(ctx, http.MethodPost, url1, bytes.NewReader(tfStateJSON3))
	require.NoError(t, err)
	req3.Header.Set("Content-Type", "application/json")

	resp3, err := httpClient.Do(req3)
	require.NoError(t, err)
	defer resp3.Body.Close()
	require.Equal(t, http.StatusOK, resp3.StatusCode)

	t.Logf("Uploaded state with no outputs (serial 3) to producer")

	// Wait for edge update job to process
	time.Sleep(2 * time.Second)

	// Check edge status - should go back to "pending"
	edges3, err := client.ListDependencies(ctx, sdk.StateReference{LogicID: consumerLogicID})
	require.NoError(t, err)

	var foundEdge3 *sdk.DependencyEdge
	for _, edge := range edges3 {
		if edge.ID == dep1.Edge.ID {
			foundEdge3 = &edge
			break
		}
	}
	require.NotNil(t, foundEdge3, "Should find the edge after output removal")
	t.Logf("Edge after output removal: ID=%d, status=%s, InDigest=%s, OutDigest=%s", foundEdge3.ID, foundEdge3.Status, foundEdge3.InDigest, foundEdge3.OutDigest)

	// When output is removed, edge should be "missing-output" (not "pending")
	assert.Equal(t, "missing-output", foundEdge3.Status, "Edge should be missing-output when producer output key is removed")
	// OutDigest and InDigest behavior when output is missing may vary - check actual implementation
}

// TestCorruptedGridFile tests handling of corrupted .grid file
func TestCorruptedGridFile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	logicID := fmt.Sprintf("test-corrupt-%d", time.Now().UnixNano())

	// Get gridctl path before changing directory
	gridctlPath := getGridctlPath(t)

	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalDir) }()

	err = os.Chdir(tempDir)
	require.NoError(t, err)

	// Create corrupted .grid file
	gridPath := filepath.Join(tempDir, ".grid")
	err = os.WriteFile(gridPath, []byte("not valid json{{{"), 0644)
	require.NoError(t, err)

	// Try to use state get - should warn but not crash
	getCmd := exec.CommandContext(ctx, gridctlPath, "state", "get", "--server", serverURL)
	output, err := getCmd.CombinedOutput()

	// Should fail because no state identifier is provided and context is corrupted
	assert.Error(t, err)
	outputStr := string(output)

	// Should contain warning about corrupted file
	assert.True(t,
		strings.Contains(outputStr, "corrupted") ||
			strings.Contains(outputStr, "invalid") ||
			strings.Contains(outputStr, "Warning"),
		"Output should warn about corrupted .grid file")

	// Should require explicit identifier
	assert.Contains(t, outputStr, "required", "Should require explicit state identifier")

	t.Logf("Corrupted .grid handling: %s", outputStr)

	// Should work with explicit logic-id
	// First create the state
	createCmd := exec.CommandContext(ctx, gridctlPath, "state", "create", logicID, "--server", serverURL, "--force")
	output, err = createCmd.CombinedOutput()
	require.NoError(t, err, "Failed to create state: %s", string(output))

	getCmd2 := exec.CommandContext(ctx, gridctlPath, "state", "get", logicID, "--server", serverURL)
	output, err = getCmd2.CombinedOutput()
	require.NoError(t, err, "Should work with explicit logic-id: %s", string(output))
	assert.Contains(t, string(output), logicID)
}
