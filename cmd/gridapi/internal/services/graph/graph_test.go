package graph

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
)

func TestBuildGraph_Empty(t *testing.T) {
	g, guidToNodeID, err := BuildGraph([]models.Edge{})
	require.NoError(t, err)
	assert.NotNil(t, g)
	assert.Empty(t, guidToNodeID)
	assert.Equal(t, 0, g.Nodes().Len())
}

func TestBuildGraph_SingleEdge(t *testing.T) {
	edges := []models.Edge{
		{FromState: "state-a", ToState: "state-b", FromOutput: "vpc_id"},
	}

	g, guidToNodeID, err := BuildGraph(edges)
	require.NoError(t, err)
	assert.Len(t, guidToNodeID, 2)
	assert.Equal(t, 2, g.Nodes().Len())
	assert.Equal(t, 1, g.Edges().Len())

	fromID := guidToNodeID["state-a"]
	toID := guidToNodeID["state-b"]
	assert.True(t, g.HasEdgeFromTo(fromID, toID))
}

func TestBuildGraph_MultipleEdges(t *testing.T) {
	// A -> B, B -> C, A -> C
	edges := []models.Edge{
		{FromState: "state-a", ToState: "state-b"},
		{FromState: "state-b", ToState: "state-c"},
		{FromState: "state-a", ToState: "state-c"},
	}

	g, guidToNodeID, err := BuildGraph(edges)
	require.NoError(t, err)
	assert.Len(t, guidToNodeID, 3)
	assert.Equal(t, 3, g.Nodes().Len())

	aID := guidToNodeID["state-a"]
	bID := guidToNodeID["state-b"]
	cID := guidToNodeID["state-c"]

	assert.True(t, g.HasEdgeFromTo(aID, bID))
	assert.True(t, g.HasEdgeFromTo(bID, cID))
	assert.True(t, g.HasEdgeFromTo(aID, cID))
}

func TestBuildGraph_MultiEdge(t *testing.T) {
	// Multiple edges from same source to same destination (multigraph)
	edges := []models.Edge{
		{FromState: "state-a", ToState: "state-b", FromOutput: "vpc_id"},
		{FromState: "state-a", ToState: "state-b", FromOutput: "subnet_id"},
	}

	g, guidToNodeID, err := BuildGraph(edges)
	require.NoError(t, err)
	assert.Len(t, guidToNodeID, 2)

	aID := guidToNodeID["state-a"]
	bID := guidToNodeID["state-b"]

	// Graph only stores one edge per from->to pair
	assert.True(t, g.HasEdgeFromTo(aID, bID))
	assert.Equal(t, 1, g.Edges().Len())
}

func TestNodeIDToGUID_Success(t *testing.T) {
	guidToNodeID := map[string]int64{
		"state-a": 0,
		"state-b": 1,
		"state-c": 2,
	}

	guid, err := NodeIDToGUID(1, guidToNodeID)
	require.NoError(t, err)
	assert.Equal(t, "state-b", guid)
}

func TestNodeIDToGUID_NotFound(t *testing.T) {
	guidToNodeID := map[string]int64{
		"state-a": 0,
	}

	guid, err := NodeIDToGUID(999, guidToNodeID)
	assert.Error(t, err)
	assert.Empty(t, guid)
	assert.Contains(t, err.Error(), "node ID 999 not found")
}

func TestDetectCycle_NoCycle(t *testing.T) {
	// A -> B -> C (DAG)
	edges := []models.Edge{
		{FromState: "state-a", ToState: "state-b"},
		{FromState: "state-b", ToState: "state-c"},
	}

	hasCycle, err := DetectCycle(edges)
	require.NoError(t, err)
	assert.False(t, hasCycle)
}

func TestDetectCycle_SimpleCycle(t *testing.T) {
	// A -> B -> C -> A (cycle)
	edges := []models.Edge{
		{FromState: "state-a", ToState: "state-b"},
		{FromState: "state-b", ToState: "state-c"},
		{FromState: "state-c", ToState: "state-a"},
	}

	hasCycle, err := DetectCycle(edges)
	require.NoError(t, err)
	assert.True(t, hasCycle)
}

func TestDetectCycle_SelfLoop(t *testing.T) {
	// A -> A (self loop)
	// Note: gonum simple.DirectedGraph panics on self-loops
	// This is expected behavior - self-loops should be prevented at repository level
	edges := []models.Edge{
		{FromState: "state-a", ToState: "state-a"},
	}

	// This should panic, which we can recover from
	defer func() {
		if r := recover(); r != nil {
			// Expected panic from gonum
			assert.Contains(t, fmt.Sprint(r), "self edge")
		}
	}()

	_ = DetectCycle(edges)
	t.Fatal("Expected panic for self-loop, but none occurred")
}

func TestGetTopologicalOrder_Downstream_LinearChain(t *testing.T) {
	// A -> B -> C
	edges := []models.Edge{
		{FromState: "state-a", ToState: "state-b"},
		{FromState: "state-b", ToState: "state-c"},
	}

	layers, err := GetTopologicalOrder(edges, "state-a", "downstream")
	require.NoError(t, err)
	require.Len(t, layers, 3)

	assert.Equal(t, 0, layers[0].Level)
	assert.Contains(t, layers[0].States, "state-a")

	assert.Equal(t, 1, layers[1].Level)
	assert.Contains(t, layers[1].States, "state-b")

	assert.Equal(t, 2, layers[2].Level)
	assert.Contains(t, layers[2].States, "state-c")
}

func TestGetTopologicalOrder_Upstream_LinearChain(t *testing.T) {
	// A -> B -> C (traverse upstream from C)
	edges := []models.Edge{
		{FromState: "state-a", ToState: "state-b"},
		{FromState: "state-b", ToState: "state-c"},
	}

	layers, err := GetTopologicalOrder(edges, "state-c", "upstream")
	require.NoError(t, err)
	require.Len(t, layers, 3)

	assert.Equal(t, 0, layers[0].Level)
	assert.Contains(t, layers[0].States, "state-c")

	assert.Equal(t, 1, layers[1].Level)
	assert.Contains(t, layers[1].States, "state-b")

	assert.Equal(t, 2, layers[2].Level)
	assert.Contains(t, layers[2].States, "state-a")
}

func TestGetTopologicalOrder_IsolatedRoot(t *testing.T) {
	// State with no edges
	edges := []models.Edge{}

	layers, err := GetTopologicalOrder(edges, "isolated-state", "downstream")
	require.NoError(t, err)
	require.Len(t, layers, 1)

	assert.Equal(t, 0, layers[0].Level)
	assert.Contains(t, layers[0].States, "isolated-state")
}

func TestGetTopologicalOrder_RootNotInGraph(t *testing.T) {
	// A -> B, but requesting from C (not in graph)
	edges := []models.Edge{
		{FromState: "state-a", ToState: "state-b"},
	}

	layers, err := GetTopologicalOrder(edges, "state-c", "downstream")
	require.NoError(t, err)
	require.Len(t, layers, 1)

	// Root should be added even if not in graph
	assert.Contains(t, layers[0].States, "state-c")
}

func TestGetTopologicalOrder_Diamond(t *testing.T) {
	// A -> B, A -> C, B -> D, C -> D (diamond)
	edges := []models.Edge{
		{FromState: "state-a", ToState: "state-b"},
		{FromState: "state-a", ToState: "state-c"},
		{FromState: "state-b", ToState: "state-d"},
		{FromState: "state-c", ToState: "state-d"},
	}

	layers, err := GetTopologicalOrder(edges, "state-a", "downstream")
	require.NoError(t, err)
	require.Len(t, layers, 3)

	assert.Equal(t, 0, layers[0].Level)
	assert.Contains(t, layers[0].States, "state-a")

	assert.Equal(t, 1, layers[1].Level)
	assert.Len(t, layers[1].States, 2)
	assert.Contains(t, layers[1].States, "state-b")
	assert.Contains(t, layers[1].States, "state-c")

	assert.Equal(t, 2, layers[2].Level)
	assert.Contains(t, layers[2].States, "state-d")
}

func TestGetTopologicalOrder_InvalidDirection(t *testing.T) {
	edges := []models.Edge{
		{FromState: "state-a", ToState: "state-b"},
	}

	layers, err := GetTopologicalOrder(edges, "state-a", "sideways")
	assert.Error(t, err)
	assert.Nil(t, layers)
	assert.Contains(t, err.Error(), "invalid direction")
}

func TestGetTopologicalOrder_DefaultDirection(t *testing.T) {
	// Empty direction should default to downstream
	edges := []models.Edge{
		{FromState: "state-a", ToState: "state-b"},
	}

	layers, err := GetTopologicalOrder(edges, "state-a", "")
	require.NoError(t, err)
	require.Len(t, layers, 2)

	assert.Contains(t, layers[0].States, "state-a")
	assert.Contains(t, layers[1].States, "state-b")
}

func TestGetTopologicalOrder_CycleDetected(t *testing.T) {
	// A -> B -> C -> A (cycle)
	edges := []models.Edge{
		{FromState: "state-a", ToState: "state-b"},
		{FromState: "state-b", ToState: "state-c"},
		{FromState: "state-c", ToState: "state-a"},
	}

	layers, err := GetTopologicalOrder(edges, "state-a", "downstream")
	assert.Error(t, err)
	assert.Nil(t, layers)
	assert.Contains(t, err.Error(), "cycle detected")
}

func TestGetTopologicalOrder_ComplexGraph(t *testing.T) {
	// More complex dependency graph
	//     A
	//    / \
	//   B   C
	//   |\ /|
	//   | X |
	//   |/ \|
	//   D   E
	//    \ /
	//     F
	edges := []models.Edge{
		{FromState: "a", ToState: "b"},
		{FromState: "a", ToState: "c"},
		{FromState: "b", ToState: "d"},
		{FromState: "b", ToState: "e"},
		{FromState: "c", ToState: "d"},
		{FromState: "c", ToState: "e"},
		{FromState: "d", ToState: "f"},
		{FromState: "e", ToState: "f"},
	}

	layers, err := GetTopologicalOrder(edges, "a", "downstream")
	require.NoError(t, err)
	require.Len(t, layers, 4)

	// Level 0: A
	assert.Contains(t, layers[0].States, "a")

	// Level 1: B, C
	assert.Len(t, layers[1].States, 2)
	assert.Contains(t, layers[1].States, "b")
	assert.Contains(t, layers[1].States, "c")

	// Level 2: D, E
	assert.Len(t, layers[2].States, 2)
	assert.Contains(t, layers[2].States, "d")
	assert.Contains(t, layers[2].States, "e")

	// Level 3: F
	assert.Contains(t, layers[3].States, "f")
}

func TestGetTopologicalOrder_UpstreamComplex(t *testing.T) {
	// Same complex graph, but traverse upstream from F
	edges := []models.Edge{
		{FromState: "a", ToState: "b"},
		{FromState: "a", ToState: "c"},
		{FromState: "b", ToState: "d"},
		{FromState: "b", ToState: "e"},
		{FromState: "c", ToState: "d"},
		{FromState: "c", ToState: "e"},
		{FromState: "d", ToState: "f"},
		{FromState: "e", ToState: "f"},
	}

	layers, err := GetTopologicalOrder(edges, "f", "upstream")
	require.NoError(t, err)
	require.Len(t, layers, 4)

	// Level 0: F
	assert.Contains(t, layers[0].States, "f")

	// Level 1: D, E
	assert.Len(t, layers[1].States, 2)
	assert.Contains(t, layers[1].States, "d")
	assert.Contains(t, layers[1].States, "e")

	// Level 2: B, C
	assert.Len(t, layers[2].States, 2)
	assert.Contains(t, layers[2].States, "b")
	assert.Contains(t, layers[2].States, "c")

	// Level 3: A
	assert.Contains(t, layers[3].States, "a")
}

func TestGetTopologicalOrder_CaseInsensitive(t *testing.T) {
	// Direction should be case-insensitive
	edges := []models.Edge{
		{FromState: "state-a", ToState: "state-b"},
	}

	layers, err := GetTopologicalOrder(edges, "state-a", "DOWNSTREAM")
	require.NoError(t, err)
	assert.Len(t, layers, 2)
}

func TestGetNodeIDs(t *testing.T) {
	edges := []models.Edge{
		{FromState: "state-a", ToState: "state-b"},
		{FromState: "state-b", ToState: "state-c"},
	}

	g, _, err := BuildGraph(edges)
	require.NoError(t, err)

	nodeIDs := GetNodeIDs(g)
	assert.Len(t, nodeIDs, 3)
	assert.Contains(t, nodeIDs, int64(0))
	assert.Contains(t, nodeIDs, int64(1))
	assert.Contains(t, nodeIDs, int64(2))
}
