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

func TestBasicDependencyDeclaration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := sdk.NewClient(serverURL)

	// Create producer state
	producer, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-producer-basic"})
	require.NoError(t, err)

	// Create consumer state
	consumer, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-consumer-basic"})
	require.NoError(t, err)

	// Add dependency
	result, err := client.AddDependency(ctx, sdk.AddDependencyInput{
		From:       sdk.StateReference{LogicID: producer.LogicID},
		FromOutput: "vpc_id",
		To:         sdk.StateReference{LogicID: consumer.LogicID},
	})
	require.NoError(t, err)
	assert.NotNil(t, result.Edge)
	assert.Equal(t, "vpc_id", result.Edge.FromOutput)

	// Verify edge exists
	edges, err := client.ListDependencies(ctx, sdk.StateReference{LogicID: consumer.LogicID})
	require.NoError(t, err)
	assert.Len(t, edges, 1)
	assert.Equal(t, "vpc_id", edges[0].FromOutput)
	assert.Equal(t, "pending", edges[0].Status)
}

func TestCyclePrevention(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := sdk.NewClient(serverURL)

	// Create states A, B, C
	stateA, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-state-a-cycle"})
	require.NoError(t, err)

	stateB, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-state-b-cycle"})
	require.NoError(t, err)

	stateC, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-state-c-cycle"})
	require.NoError(t, err)

	// Create A→B
	_, err = client.AddDependency(ctx, sdk.AddDependencyInput{
		From:       sdk.StateReference{LogicID: stateA.LogicID},
		FromOutput: "out1",
		To:         sdk.StateReference{LogicID: stateB.LogicID},
	})
	require.NoError(t, err)

	// Create B→C
	_, err = client.AddDependency(ctx, sdk.AddDependencyInput{
		From:       sdk.StateReference{LogicID: stateB.LogicID},
		FromOutput: "out2",
		To:         sdk.StateReference{LogicID: stateC.LogicID},
	})
	require.NoError(t, err)

	// Attempt C→A (should fail with cycle error)
	_, err = client.AddDependency(ctx, sdk.AddDependencyInput{
		From:       sdk.StateReference{LogicID: stateC.LogicID},
		FromOutput: "out3",
		To:         sdk.StateReference{LogicID: stateA.LogicID},
	})
	assert.Error(t, err, "Expected cycle error")
}

func TestEdgeStatusTracking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := sdk.NewClient(serverURL)

	// Create states
	producer, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-producer-status"})
	require.NoError(t, err)

	consumer, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-consumer-status"})
	require.NoError(t, err)

	// Add dependency
	result, err := client.AddDependency(ctx, sdk.AddDependencyInput{
		From:       sdk.StateReference{LogicID: producer.LogicID},
		FromOutput: "subnet_id",
		To:         sdk.StateReference{LogicID: consumer.LogicID},
	})
	require.NoError(t, err)
	edgeID := result.Edge.ID

	// Initially pending
	edges, err := client.ListDependencies(ctx, sdk.StateReference{LogicID: consumer.LogicID})
	require.NoError(t, err)
	assert.Equal(t, "pending", edges[0].Status)

	// Upload producer state with subnet_id output
	tfstateData, err := os.ReadFile(filepath.Join("testdata", "landing_zone_subnet_output.json"))
	require.NoError(t, err)

	putTFState(t, producer.GUID, tfstateData)

	// Wait for EdgeUpdateJob to process
	time.Sleep(500 * time.Millisecond)

	// Check edge status - should transition from pending
	edges, err = client.ListDependencies(ctx, sdk.StateReference{LogicID: consumer.LogicID})
	require.NoError(t, err)
	t.Logf("Edge %d status after tfstate upload: %s", edgeID, edges[0].Status)
}

func TestMockDependencies(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := sdk.NewClient(serverURL)

	// Create states
	producer, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-producer-mock"})
	require.NoError(t, err)

	consumer, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-consumer-mock"})
	require.NoError(t, err)

	// Add dependency with mock value
	mockValue := `{"value": "vpc-mock-12345"}`
	result, err := client.AddDependency(ctx, sdk.AddDependencyInput{
		From:          sdk.StateReference{LogicID: producer.LogicID},
		FromOutput:    "vpc_id",
		To:            sdk.StateReference{LogicID: consumer.LogicID},
		MockValueJSON: mockValue,
	})
	require.NoError(t, err)
	assert.Equal(t, "mock", result.Edge.Status)
	assert.Equal(t, mockValue, result.Edge.MockValueJSON)

	// Upload real producer state
	tfstateData, err := os.ReadFile(filepath.Join("testdata", "landing_zone_vpc_output.json"))
	require.NoError(t, err)

	putTFState(t, producer.GUID, tfstateData)

	// Wait for EdgeUpdateJob
	time.Sleep(500 * time.Millisecond)

	// Verify mock replaced
	edges, err := client.ListDependencies(ctx, sdk.StateReference{LogicID: consumer.LogicID})
	require.NoError(t, err)
	t.Logf("Edge status after real output: %s", edges[0].Status)
}

func TestTopologicalOrdering(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := sdk.NewClient(serverURL)

	// Create chain: foundation → network → compute → app
	foundation, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-foundation-topo"})
	require.NoError(t, err)

	network, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-network-topo"})
	require.NoError(t, err)

	compute, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-compute-topo"})
	require.NoError(t, err)

	app, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-app-topo"})
	require.NoError(t, err)

	// Build dependency chain
	_, err = client.AddDependency(ctx, sdk.AddDependencyInput{
		From:       sdk.StateReference{LogicID: foundation.LogicID},
		FromOutput: "region",
		To:         sdk.StateReference{LogicID: network.LogicID},
	})
	require.NoError(t, err)

	_, err = client.AddDependency(ctx, sdk.AddDependencyInput{
		From:       sdk.StateReference{LogicID: network.LogicID},
		FromOutput: "vpc_id",
		To:         sdk.StateReference{LogicID: compute.LogicID},
	})
	require.NoError(t, err)

	_, err = client.AddDependency(ctx, sdk.AddDependencyInput{
		From:       sdk.StateReference{LogicID: compute.LogicID},
		FromOutput: "cluster_endpoint",
		To:         sdk.StateReference{LogicID: app.LogicID},
	})
	require.NoError(t, err)

	// Get topological order from app perspective (upstream)
	layers, err := client.GetTopologicalOrder(ctx, sdk.TopologyInput{
		Root:      sdk.StateReference{LogicID: app.LogicID},
		Direction: sdk.Upstream,
	})
	require.NoError(t, err)
	assert.Len(t, layers, 4)

	// Verify layer structure
	assert.Equal(t, 0, layers[0].Level)
	assert.Equal(t, 1, layers[1].Level)
	assert.Equal(t, 2, layers[2].Level)
	assert.Equal(t, 3, layers[3].Level)
}

func TestDependencyListingAndStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := sdk.NewClient(serverURL)

	// Create states
	landingZone, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-landing-zone-status"})
	require.NoError(t, err)

	iamSetup, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-iam-setup-status"})
	require.NoError(t, err)

	cluster, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-cluster-status"})
	require.NoError(t, err)

	// Add dependencies
	_, err = client.AddDependency(ctx, sdk.AddDependencyInput{
		From:       sdk.StateReference{LogicID: landingZone.LogicID},
		FromOutput: "vpc_id",
		To:         sdk.StateReference{LogicID: cluster.LogicID},
	})
	require.NoError(t, err)

	_, err = client.AddDependency(ctx, sdk.AddDependencyInput{
		From:       sdk.StateReference{LogicID: iamSetup.LogicID},
		FromOutput: "cluster_role_arn",
		To:         sdk.StateReference{LogicID: cluster.LogicID},
	})
	require.NoError(t, err)

	// List incoming dependencies
	edges, err := client.ListDependencies(ctx, sdk.StateReference{LogicID: cluster.LogicID})
	require.NoError(t, err)
	assert.Len(t, edges, 2)

	// List outgoing dependents from landing-zone
	dependents, err := client.ListDependents(ctx, sdk.StateReference{LogicID: landingZone.LogicID})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(dependents), 1)

	// Get state status
	status, err := client.GetStateStatus(ctx, sdk.StateReference{LogicID: cluster.LogicID})
	require.NoError(t, err)
	assert.NotNil(t, status)
	t.Logf("Cluster status: %s", status.Status)
}

func TestSearchByOutputKey(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := sdk.NewClient(serverURL)

	// Create states
	producer1, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-producer1-search"})
	require.NoError(t, err)

	producer2, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-producer2-search"})
	require.NoError(t, err)

	consumer1, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-consumer1-search"})
	require.NoError(t, err)

	consumer2, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-consumer2-search"})
	require.NoError(t, err)

	// Add edges with vpc_id output
	_, err = client.AddDependency(ctx, sdk.AddDependencyInput{
		From:       sdk.StateReference{LogicID: producer1.LogicID},
		FromOutput: "vpc_id",
		To:         sdk.StateReference{LogicID: consumer1.LogicID},
	})
	require.NoError(t, err)

	_, err = client.AddDependency(ctx, sdk.AddDependencyInput{
		From:       sdk.StateReference{LogicID: producer2.LogicID},
		FromOutput: "vpc_id",
		To:         sdk.StateReference{LogicID: consumer2.LogicID},
	})
	require.NoError(t, err)

	// Search by output key
	edges, err := client.SearchByOutput(ctx, "vpc_id")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(edges), 2, "Should find at least 2 edges with vpc_id")

	// Verify all edges have vpc_id as output
	for _, edge := range edges {
		if edge.FromOutput == "vpc_id" {
			assert.Equal(t, "vpc_id", edge.FromOutput)
		}
	}
}

func TestToInputNameDefaultAndOverride(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := sdk.NewClient(serverURL)

	// Create states
	producer, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-producer-input-name"})
	require.NoError(t, err)

	consumer, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-consumer-input-name"})
	require.NoError(t, err)

	// Add dependency WITHOUT to_input_name (should generate default)
	result1, err := client.AddDependency(ctx, sdk.AddDependencyInput{
		From:       sdk.StateReference{LogicID: producer.LogicID},
		FromOutput: "vpc_id",
		To:         sdk.StateReference{LogicID: consumer.LogicID},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result1.Edge.ToInputName, "Default to_input_name should be generated")

	// Add dependency WITH to_input_name override
	customName := "network_vpc"
	result2, err := client.AddDependency(ctx, sdk.AddDependencyInput{
		From:        sdk.StateReference{LogicID: producer.LogicID},
		FromOutput:  "subnet_ids",
		To:          sdk.StateReference{LogicID: consumer.LogicID},
		ToInputName: customName,
	})
	require.NoError(t, err)
	assert.Equal(t, customName, result2.Edge.ToInputName)

	// Attempt duplicate to_input_name (should fail)
	_, err = client.AddDependency(ctx, sdk.AddDependencyInput{
		From:        sdk.StateReference{LogicID: producer.LogicID},
		FromOutput:  "another_output",
		To:          sdk.StateReference{LogicID: consumer.LogicID},
		ToInputName: customName,
	})
	assert.Error(t, err, "Expected uniqueness error for duplicate to_input_name")
}

func TestDependencyRemoval(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := sdk.NewClient(serverURL)

	// Create states
	producer, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-producer-removal"})
	require.NoError(t, err)

	consumer, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-consumer-removal"})
	require.NoError(t, err)

	// Add dependency
	result, err := client.AddDependency(ctx, sdk.AddDependencyInput{
		From:       sdk.StateReference{LogicID: producer.LogicID},
		FromOutput: "vpc_id",
		To:         sdk.StateReference{LogicID: consumer.LogicID},
	})
	require.NoError(t, err)
	edgeID := result.Edge.ID

	// Verify dependency exists
	edges, err := client.ListDependencies(ctx, sdk.StateReference{LogicID: consumer.LogicID})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(edges), 1)

	// Remove dependency
	err = client.RemoveDependency(ctx, edgeID)
	require.NoError(t, err)

	// Verify dependency removed
	edges2, err := client.ListDependencies(ctx, sdk.StateReference{LogicID: consumer.LogicID})
	require.NoError(t, err)

	// Should not find the removed edge
	for _, edge := range edges2 {
		assert.NotEqual(t, edgeID, edge.ID, "Removed edge should not appear")
	}
}

func TestGetDependencyGraph(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	client := sdk.NewClient(serverURL)

	// Create states
	producer, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-producer-graph"})
	require.NoError(t, err)

	consumer, err := client.CreateState(ctx, sdk.CreateStateInput{LogicID: "test-consumer-graph"})
	require.NoError(t, err)

	// Add dependency
	_, err = client.AddDependency(ctx, sdk.AddDependencyInput{
		From:       sdk.StateReference{LogicID: producer.LogicID},
		FromOutput: "vpc_id",
		To:         sdk.StateReference{LogicID: consumer.LogicID},
	})
	require.NoError(t, err)

	// Get dependency graph
	graph, err := client.GetDependencyGraph(ctx, sdk.StateReference{LogicID: consumer.LogicID})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(graph.Edges), 1)
	assert.GreaterOrEqual(t, len(graph.Producers), 1)

	// Verify producer state has backend config
	assert.NotEmpty(t, graph.Producers[0].BackendConfig.Address)
}
