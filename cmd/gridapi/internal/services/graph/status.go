package graph

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
)

// StateStatus represents the computed status of a state based on its dependencies
type StateStatus struct {
	StateGUID string           `json:"state_guid"`
	LogicID   string           `json:"logic_id"`
	Status    string           `json:"status"` // "clean", "stale", "potentially-stale"
	Incoming  []IncomingEdgeView `json:"incoming"`
	Summary   StatusSummary    `json:"summary"`
}

// IncomingEdgeView shows incoming edge details for status computation
type IncomingEdgeView struct {
	EdgeID       int64      `json:"edge_id"`
	FromGUID     string     `json:"from_guid"`
	FromLogicID  string     `json:"from_logic_id"`
	FromOutput   string     `json:"from_output"`
	Status       string     `json:"status"`
	InDigest     string     `json:"in_digest,omitempty"`
	OutDigest    string     `json:"out_digest,omitempty"`
	LastInAt     *time.Time `json:"last_in_at,omitempty"`
	LastOutAt    *time.Time `json:"last_out_at,omitempty"`
}

// StatusSummary aggregates incoming edge counts
type StatusSummary struct {
	IncomingClean   int `json:"incoming_clean"`
	IncomingDirty   int `json:"incoming_dirty"`
	IncomingPending int `json:"incoming_pending"`
	IncomingUnknown int `json:"incoming_unknown"`
}

// ComputeStateStatus derives state status from all incoming edges + transitive propagation
func ComputeStateStatus(ctx context.Context, edgeRepo repository.EdgeRepository, stateRepo repository.StateRepository, stateGUID string) (*StateStatus, error) {
	// Get target state
	state, err := stateRepo.GetByGUID(ctx, stateGUID)
	if err != nil {
		return nil, fmt.Errorf("get state: %w", err)
	}

	// Fetch all edges
	allEdges, err := edgeRepo.GetAllEdges(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all edges: %w", err)
	}

	// Build adjacency map (for transitive propagation)
	adj := make(map[string][]string)
	for _, edge := range allEdges {
		adj[edge.FromState] = append(adj[edge.FromState], edge.ToState)
	}

	// Identify red states (any state with incoming dirty/pending edge)
	red := make(map[string]bool)
	for _, edge := range allEdges {
		if edge.Status == models.EdgeStatusDirty || edge.Status == models.EdgeStatusPending {
			red[edge.ToState] = true
		}
	}

	// Propagate yellow (potentially-stale) from red states via BFS
	yellow := make(map[string]bool)
	queue := []string{}
	seen := make(map[string]bool)

	for stateID := range red {
		queue = append(queue, stateID)
		seen[stateID] = true
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, next := range adj[current] {
			if seen[next] {
				continue
			}
			yellow[next] = true
			seen[next] = true
			queue = append(queue, next)
		}
	}

	// Compute target state status
	status := "clean"
	if red[stateGUID] {
		status = "stale" // Direct incoming dirty/pending
	} else if yellow[stateGUID] {
		status = "potentially-stale" // Transitive upstream dirty
	}

	// Collect incoming edges with producer logic IDs (using eager loading to avoid N+1)
	incoming := []IncomingEdgeView{}
	summary := StatusSummary{}

	incomingEdges, err := edgeRepo.GetIncomingEdgesWithProducers(ctx, stateGUID)
	if err != nil {
		return nil, fmt.Errorf("get incoming edges with producers: %w", err)
	}

	for _, edge := range incomingEdges {
		// Producer state is already loaded via eager loading
		if edge.FromStateRel == nil {
			// Skip if producer state not found (shouldn't happen with proper FK constraints)
			continue
		}

		view := IncomingEdgeView{
			EdgeID:      edge.ID,
			FromGUID:    edge.FromState,
			FromLogicID: edge.FromStateRel.LogicID,
			FromOutput:  edge.FromOutput,
			Status:      string(edge.Status),
			InDigest:    edge.InDigest,
			OutDigest:   edge.OutDigest,
			LastInAt:    edge.LastInAt,
			LastOutAt:   edge.LastOutAt,
		}

		incoming = append(incoming, view)

		// Update summary
		switch edge.Status {
		case models.EdgeStatusClean:
			summary.IncomingClean++
		case models.EdgeStatusDirty:
			summary.IncomingDirty++
		case models.EdgeStatusPending:
			summary.IncomingPending++
		default:
			summary.IncomingUnknown++
		}
	}

	// Sort incoming edges by status priority (pending first, then unknown, then others)
	sort.Slice(incoming, func(i, j int) bool {
		statusOrder := map[string]int{
			"pending": 0,
			"unknown": 1,
			"dirty":   2,
			"clean":   3,
		}

		orderI, okI := statusOrder[incoming[i].Status]
		orderJ, okJ := statusOrder[incoming[j].Status]

		if !okI {
			orderI = 1 // Treat unknown statuses as "unknown"
		}
		if !okJ {
			orderJ = 1
		}

		return orderI < orderJ
	})

	return &StateStatus{
		StateGUID: stateGUID,
		LogicID:   state.LogicID,
		Status:    status,
		Incoming:  incoming,
		Summary:   summary,
	}, nil
}

// ComputeStateSummary computes just the status string without full details
func ComputeStateSummary(ctx context.Context, edgeRepo repository.EdgeRepository, stateGUID string) (string, error) {
	// Fetch all edges
	allEdges, err := edgeRepo.GetAllEdges(ctx)
	if err != nil {
		return "", fmt.Errorf("get all edges: %w", err)
	}

	// Build adjacency map
	adj := make(map[string][]string)
	for _, edge := range allEdges {
		adj[edge.FromState] = append(adj[edge.FromState], edge.ToState)
	}

	// Identify red states
	red := make(map[string]bool)
	for _, edge := range allEdges {
		if edge.Status == models.EdgeStatusDirty || edge.Status == models.EdgeStatusPending {
			red[edge.ToState] = true
		}
	}

	// Propagate yellow
	yellow := make(map[string]bool)
	queue := []string{}
	seen := make(map[string]bool)

	for stateID := range red {
		queue = append(queue, stateID)
		seen[stateID] = true
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, next := range adj[current] {
			if seen[next] {
				continue
			}
			yellow[next] = true
			seen[next] = true
			queue = append(queue, next)
		}
	}

	// Compute status
	if red[stateGUID] {
		return "stale", nil
	} else if yellow[stateGUID] {
		return "potentially-stale", nil
	}

	return "clean", nil
}
