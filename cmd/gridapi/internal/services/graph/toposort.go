package graph

import (
	"fmt"
	"strings"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/graph/topo"
)

// Layer represents a level in the topological ordering
type Layer struct {
	Level  int      `json:"level"`
	States []string `json:"states"` // GUIDs
}

// GetTopologicalOrder computes layered ordering rooted at a given state
// Direction: "upstream" (dependencies) or "downstream" (dependents)
func GetTopologicalOrder(edges []models.Edge, rootGUID string, direction string) ([]Layer, error) {
	direction = strings.ToLower(direction)
	if direction == "" {
		direction = "downstream"
	}
	if direction != "downstream" && direction != "upstream" {
		return nil, fmt.Errorf("invalid direction %q", direction)
	}

	g, guidToNodeID, err := BuildGraph(edges)
	if err != nil {
		return nil, fmt.Errorf("build graph: %w", err)
	}

	// Ensure the root node exists in the graph even if it currently has no edges.
	rootNodeID, exists := guidToNodeID[rootGUID]
	if !exists {
		var maxID int64
		for _, id := range guidToNodeID {
			if id > maxID {
				maxID = id
			}
		}
		rootNodeID = maxID + 1
		guidToNodeID[rootGUID] = rootNodeID
		g.AddNode(simple.Node(rootNodeID))
	}

	// Verify graph is acyclic
	if _, err := topo.Sort(g); err != nil {
		return nil, fmt.Errorf("topological sort failed (cycle detected): %w", err)
	}

	// Build adjacency map for traversal
	adj := make(map[int64][]int64)
	for _, edge := range edges {
		fromNodeID := guidToNodeID[edge.FromState]
		toNodeID := guidToNodeID[edge.ToState]

		if direction == "upstream" {
			// For upstream, traverse from consumer to producer (reverse edge direction)
			adj[toNodeID] = append(adj[toNodeID], fromNodeID)
		} else {
			// For downstream, traverse from producer to consumer (forward edge direction)
			adj[fromNodeID] = append(adj[fromNodeID], toNodeID)
		}
	}

	// BFS to compute layers from root
	visited := make(map[int64]bool)
	layers := []Layer{{Level: 0, States: []string{rootGUID}}}
	visited[rootNodeID] = true

	currentLayer := []int64{rootNodeID}
	level := 0

	for len(currentLayer) > 0 {
		nextLayer := []int64{}
		for _, nodeID := range currentLayer {
			for _, neighborID := range adj[nodeID] {
				if !visited[neighborID] {
					visited[neighborID] = true
					nextLayer = append(nextLayer, neighborID)
				}
			}
		}

		if len(nextLayer) > 0 {
			level++
			stateGUIDs := make([]string, 0, len(nextLayer))
			for _, nodeID := range nextLayer {
				guid, err := NodeIDToGUID(nodeID, guidToNodeID)
				if err != nil {
					return nil, err
				}
				stateGUIDs = append(stateGUIDs, guid)
			}
			layers = append(layers, Layer{Level: level, States: stateGUIDs})
			currentLayer = nextLayer
		} else {
			break
		}
	}

	return layers, nil
}

// DetectCycle checks if the graph contains a cycle
func DetectCycle(edges []models.Edge) (bool, error) {
	g, _, err := BuildGraph(edges)
	if err != nil {
		return false, fmt.Errorf("build graph: %w", err)
	}

	_, err = topo.Sort(g)
	if err != nil {
		// topo.Sort returns error if cycle detected
		return true, nil
	}

	return false, nil
}
