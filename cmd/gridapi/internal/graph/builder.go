package graph

import (
	"fmt"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
)

// BuildGraph constructs an in-memory directed graph from edge records
func BuildGraph(edges []models.Edge) (*simple.DirectedGraph, map[string]int64, error) {
	g := simple.NewDirectedGraph()

	// Map state GUIDs to node IDs
	guidToNodeID := make(map[string]int64)
	nodeIDCounter := int64(0)

	// Helper to get or create node ID for a state GUID
	getNodeID := func(guid string) int64 {
		if nodeID, exists := guidToNodeID[guid]; exists {
			return nodeID
		}
		nodeID := nodeIDCounter
		nodeIDCounter++
		guidToNodeID[guid] = nodeID
		g.AddNode(simple.Node(nodeID))
		return nodeID
	}

	// Add all edges to graph
	for _, edge := range edges {
		fromNodeID := getNodeID(edge.FromState)
		toNodeID := getNodeID(edge.ToState)

		// Add edge (in a multigraph, multiple edges from same from->to are allowed)
		// But for toposort, we only care about connectivity, not multiplicity
		if !g.HasEdgeFromTo(fromNodeID, toNodeID) {
			g.SetEdge(simple.Edge{F: simple.Node(fromNodeID), T: simple.Node(toNodeID)})
		}
	}

	return g, guidToNodeID, nil
}

// NodeIDToGUID returns the GUID for a given node ID (reverse lookup)
func NodeIDToGUID(nodeID int64, guidToNodeID map[string]int64) (string, error) {
	for guid, nid := range guidToNodeID {
		if nid == nodeID {
			return guid, nil
		}
	}
	return "", fmt.Errorf("node ID %d not found in mapping", nodeID)
}

// GetNodeIDs returns all node IDs in the graph
func GetNodeIDs(g graph.Graph) []int64 {
	nodes := g.Nodes()
	nodeIDs := make([]int64, 0, nodes.Len())
	for nodes.Next() {
		nodeIDs = append(nodeIDs, nodes.Node().ID())
	}
	return nodeIDs
}
