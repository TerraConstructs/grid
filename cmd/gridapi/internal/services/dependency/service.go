package dependency

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/services/graph"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/services/tfstate"
)

// Service handles dependency management operations
type Service struct {
	edgeRepo   repository.EdgeRepository
	stateRepo  repository.StateRepository
	outputRepo repository.StateOutputRepository
}

// NewService creates a new dependency service
func NewService(edgeRepo repository.EdgeRepository, stateRepo repository.StateRepository) *Service {
	return &Service{
		edgeRepo:  edgeRepo,
		stateRepo: stateRepo,
	}
}

// WithOutputRepository adds output repository (optional)
func (s *Service) WithOutputRepository(outputRepo repository.StateOutputRepository) *Service {
	s.outputRepo = outputRepo
	return s
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

	// Initialize edge if producer already has the referenced output
	// This handles the case where edge is created AFTER producer has outputs
	if err := s.initializeEdgeIfProducerHasOutput(ctx, edge); err != nil {
		// Non-fatal: log but don't fail the AddDependency operation
		// The edge will be initialized later when producer updates
		fmt.Printf("Warning: failed to initialize edge: %v\n", err)
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

// ListAllEdges returns all dependency edges in the system, ordered by ID.
func (s *Service) ListAllEdges(ctx context.Context) ([]models.Edge, error) {
	return s.edgeRepo.GetAllEdges(ctx)
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

	// Get unique producer GUIDs
	producerGUIDs := make(map[string]bool)
	for _, edge := range edges {
		producerGUIDs[edge.FromState] = true
	}

	// Batch fetch producer states (avoids N+1 queries)
	guids := make([]string, 0, len(producerGUIDs))
	for guid := range producerGUIDs {
		guids = append(guids, guid)
	}

	producerStates, err := s.stateRepo.GetByGUIDs(ctx, guids)
	if err != nil {
		return nil, fmt.Errorf("batch fetch producer states: %w", err)
	}

	// Build producer list
	producers := make([]ProducerState, 0, len(producerStates))
	for guid, state := range producerStates {
		producers = append(producers, ProducerState{
			GUID:    guid,
			LogicID: state.LogicID,
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

// initializeEdgeIfProducerHasOutput checks if producer has the referenced output
// and initializes InDigest and status if it does.
// This handles the case where an edge is created AFTER the producer already has outputs.
func (s *Service) initializeEdgeIfProducerHasOutput(ctx context.Context, edge *models.Edge) error {
	// Skip if edge already has InDigest (e.g., mock edges)
	if edge.InDigest != "" {
		return nil
	}

	// Get producer state
	producerState, err := s.stateRepo.GetByGUID(ctx, edge.FromState)
	if err != nil {
		return fmt.Errorf("get producer state: %w", err)
	}

	// Skip if no state content
	if len(producerState.StateContent) == 0 {
		return nil // Producer doesn't have outputs yet, stay pending
	}

	// Fast path: consult output cache (if available) to determine absence
	// If the referenced output key is not cached for this producer at the current serial,
	// mark the edge as missing-output and avoid parsing tfstate JSON.
	if s.outputRepo != nil {
		if cached, cacheErr := s.outputRepo.GetOutputsByState(ctx, producerState.GUID); cacheErr == nil {
			found := false
			for _, out := range cached {
				if out.Key == edge.FromOutput {
					found = true
					break
				}
			}
			if !found {
				edge.Status = models.EdgeStatusMissingOutput
				return s.edgeRepo.Update(ctx, edge)
			}
		}
		// On cache error or inconclusive result, fall back to parsing
	}

	// Parse tfstate to get outputs
	parsed, err := tfstate.ParseState(producerState.StateContent)
	if err != nil {
		return fmt.Errorf("parse producer state: %w", err)
	}

	// Check if the referenced output exists
	outputValue, exists := parsed.Values[edge.FromOutput]
	if !exists {
		// Output doesn't exist, edge should be missing-output
		edge.Status = models.EdgeStatusMissingOutput
		return s.edgeRepo.Update(ctx, edge)
	}

	// Output exists - compute fingerprint and set InDigest
	digest := tfstate.ComputeFingerprint(outputValue)
	edge.InDigest = digest
	now := time.Now()
	edge.LastInAt = &now

	// Set status to dirty (producer has output, consumer hasn't observed yet)
	edge.Status = models.EdgeStatusDirty

	// Update the edge in database
	return s.edgeRepo.Update(ctx, edge)
}
