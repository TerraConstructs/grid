package dependency

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/graph"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
)

// Service handles dependency management operations
type Service struct {
	edgeRepo  repository.EdgeRepository
	stateRepo repository.StateRepository
}

// NewService creates a new dependency service
func NewService(edgeRepo repository.EdgeRepository, stateRepo repository.StateRepository) *Service {
	return &Service{
		edgeRepo:  edgeRepo,
		stateRepo: stateRepo,
	}
}

// AddDependencyRequest represents a request to add a dependency
type AddDependencyRequest struct {
	FromLogicID   string
	FromGUID      string
	FromOutput    string
	ToLogicID     string
	ToGUID        string
	ToInputName   string
	MockValueJSON string
}

// AddDependency creates a new dependency edge with validation
func (s *Service) AddDependency(ctx context.Context, req *AddDependencyRequest) (*models.Edge, bool, error) {
	// Resolve from state
	fromState, err := s.resolveState(ctx, req.FromLogicID, req.FromGUID)
	if err != nil {
		return nil, false, fmt.Errorf("resolve from state: %w", err)
	}

	// Resolve to state
	toState, err := s.resolveState(ctx, req.ToLogicID, req.ToGUID)
	if err != nil {
		return nil, false, fmt.Errorf("resolve to state: %w", err)
	}

	// Generate default to_input_name if not provided
	toInputName := req.ToInputName
	if toInputName == "" {
		toInputName = slugify(fromState.LogicID) + "_" + slugify(req.FromOutput)
	}

	// Check for cycle
	wouldCycle, err := s.edgeRepo.WouldCreateCycle(ctx, fromState.GUID, toState.GUID)
	if err != nil {
		return nil, false, fmt.Errorf("cycle detection: %w", err)
	}
	if wouldCycle {
		return nil, false, fmt.Errorf("adding edge would create a cycle")
	}

	// Create edge
	edge := &models.Edge{
		FromState:   fromState.GUID,
		FromOutput:  req.FromOutput,
		ToState:     toState.GUID,
		ToInputName: toInputName,
		Status:      models.EdgeStatusPending,
	}

	// Set mock value if provided
	if req.MockValueJSON != "" {
		edge.MockValue = []byte(req.MockValueJSON)
		edge.Status = models.EdgeStatusMock
	}

	// Try to create (handle idempotent duplicate)
	err = s.edgeRepo.Create(ctx, edge)
	if err != nil {
		// Check if it's a duplicate error
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "conflicts") {
			// Find existing edge
			existingEdges, err := s.edgeRepo.GetOutgoingEdges(ctx, fromState.GUID)
			if err != nil {
				return nil, false, fmt.Errorf("query existing edges: %w", err)
			}

			for _, existing := range existingEdges {
				if existing.FromOutput == req.FromOutput && existing.ToState == toState.GUID {
					return &existing, true, nil // Return existing edge, already_exists=true
				}
			}
		}
		return nil, false, fmt.Errorf("create edge: %w", err)
	}

	return edge, false, nil
}

// RemoveDependency deletes an edge by ID
func (s *Service) RemoveDependency(ctx context.Context, edgeID int64) error {
	return s.edgeRepo.Delete(ctx, edgeID)
}

// ListDependencies returns incoming edges for a consumer state
func (s *Service) ListDependencies(ctx context.Context, logicID, guid string) ([]models.Edge, error) {
	state, err := s.resolveState(ctx, logicID, guid)
	if err != nil {
		return nil, fmt.Errorf("resolve state: %w", err)
	}

	return s.edgeRepo.GetIncomingEdges(ctx, state.GUID)
}

// ListDependents returns outgoing edges for a producer state
func (s *Service) ListDependents(ctx context.Context, logicID, guid string) ([]models.Edge, error) {
	state, err := s.resolveState(ctx, logicID, guid)
	if err != nil {
		return nil, fmt.Errorf("resolve state: %w", err)
	}

	return s.edgeRepo.GetOutgoingEdges(ctx, state.GUID)
}

// SearchByOutput finds edges by output key
func (s *Service) SearchByOutput(ctx context.Context, outputKey string) ([]models.Edge, error) {
	return s.edgeRepo.FindByOutput(ctx, outputKey)
}

// GetTopologicalOrder computes layered ordering
func (s *Service) GetTopologicalOrder(ctx context.Context, logicID, guid, direction string) ([]graph.Layer, error) {
	state, err := s.resolveState(ctx, logicID, guid)
	if err != nil {
		return nil, fmt.Errorf("resolve state: %w", err)
	}

	edges, err := s.edgeRepo.GetAllEdges(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all edges: %w", err)
	}

	return graph.GetTopologicalOrder(edges, state.GUID, direction)
}

// GetStateStatus computes on-demand status for a state
func (s *Service) GetStateStatus(ctx context.Context, logicID, guid string) (*graph.StateStatus, error) {
	state, err := s.resolveState(ctx, logicID, guid)
	if err != nil {
		return nil, fmt.Errorf("resolve state: %w", err)
	}

	return graph.ComputeStateStatus(ctx, s.edgeRepo, s.stateRepo, state.GUID)
}

// GetDependencyGraph returns graph data for HCL generation
func (s *Service) GetDependencyGraph(ctx context.Context, logicID, guid string) (*DependencyGraph, error) {
	consumerState, err := s.resolveState(ctx, logicID, guid)
	if err != nil {
		return nil, fmt.Errorf("resolve consumer state: %w", err)
	}

	// Get incoming edges
	edges, err := s.edgeRepo.GetIncomingEdges(ctx, consumerState.GUID)
	if err != nil {
		return nil, fmt.Errorf("get incoming edges: %w", err)
	}

	// Get unique producer states
	producerGUIDs := make(map[string]bool)
	for _, edge := range edges {
		producerGUIDs[edge.FromState] = true
	}

	producers := []ProducerState{}
	for producerGUID := range producerGUIDs {
		producerState, err := s.stateRepo.GetByGUID(ctx, producerGUID)
		if err != nil {
			continue // Skip if not found
		}

		producers = append(producers, ProducerState{
			GUID:    producerState.GUID,
			LogicID: producerState.LogicID,
		})
	}

	return &DependencyGraph{
		ConsumerGUID:    consumerState.GUID,
		ConsumerLogicID: consumerState.LogicID,
		Producers:       producers,
		Edges:           edges,
	}, nil
}

// DependencyGraph represents the full graph for a consumer state
type DependencyGraph struct {
	ConsumerGUID    string
	ConsumerLogicID string
	Producers       []ProducerState
	Edges           []models.Edge
}

// ProducerState represents a unique producer state
type ProducerState struct {
	GUID    string
	LogicID string
}

// resolveState resolves a state by logic_id or GUID
func (s *Service) resolveState(ctx context.Context, logicID, guid string) (*models.State, error) {
	if logicID != "" {
		return s.stateRepo.GetByLogicID(ctx, logicID)
	}
	if guid != "" {
		return s.stateRepo.GetByGUID(ctx, guid)
	}
	return nil, fmt.Errorf("either logic_id or guid must be provided")
}

var nonAlphanumericRegex = regexp.MustCompile(`[^a-z0-9]+`)

// slugify converts a string to a valid slug format
func slugify(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)

	// Replace non-alphanumeric with underscore
	s = nonAlphanumericRegex.ReplaceAllString(s, "_")

	// Trim leading/trailing underscores
	s = strings.Trim(s, "_")

	// Collapse repeated underscores
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}

	return s
}
