package server

import (
	"context"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	statev1 "github.com/terraconstructs/grid/api/state/v1"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/db/models"
	"github.com/terraconstructs/grid/cmd/gridapi/internal/dependency"
	statepkg "github.com/terraconstructs/grid/cmd/gridapi/internal/state"
)

// mockEdgeRepository implements repository.EdgeRepository for testing
type mockEdgeRepository struct {
	edges       map[int64]*models.Edge
	nextID      int64
	cycleFunc   func(fromState, toState string) bool
	outputIndex map[string][]int64 // fromOutput -> edge IDs
}

func newMockEdgeRepository() *mockEdgeRepository {
	return &mockEdgeRepository{
		edges:       make(map[int64]*models.Edge),
		nextID:      1,
		outputIndex: make(map[string][]int64),
	}
}

func (m *mockEdgeRepository) Create(_ context.Context, edge *models.Edge) error {
	// Check for duplicate constraints
	for _, existing := range m.edges {
		// UNIQUE (from_state, from_output, to_state)
		if existing.FromState == edge.FromState && existing.FromOutput == edge.FromOutput && existing.ToState == edge.ToState {
			return fmt.Errorf("edge already exists")
		}
		// UNIQUE (to_state, to_input_name)
		if existing.ToState == edge.ToState && existing.ToInputName == edge.ToInputName {
			return fmt.Errorf("to_input_name conflict: input '%s' already exists for state %s", edge.ToInputName, edge.ToState)
		}
	}

	edge.ID = m.nextID
	m.nextID++
	if edge.CreatedAt.IsZero() {
		edge.CreatedAt = time.Now()
	}
	if edge.UpdatedAt.IsZero() {
		edge.UpdatedAt = edge.CreatedAt
	}
	copy := *edge
	m.edges[copy.ID] = &copy
	m.outputIndex[copy.FromOutput] = append(m.outputIndex[copy.FromOutput], copy.ID)
	return nil
}

func (m *mockEdgeRepository) GetByID(_ context.Context, id int64) (*models.Edge, error) {
	edge, ok := m.edges[id]
	if !ok {
		return nil, fmt.Errorf("edge with id %d not found", id)
	}
	copy := *edge
	return &copy, nil
}

func (m *mockEdgeRepository) Delete(_ context.Context, id int64) error {
	edge, ok := m.edges[id]
	if !ok {
		return fmt.Errorf("edge with id %d not found", id)
	}
	// Remove from output index
	edgeIDs := m.outputIndex[edge.FromOutput]
	for i, eid := range edgeIDs {
		if eid == id {
			m.outputIndex[edge.FromOutput] = append(edgeIDs[:i], edgeIDs[i+1:]...)
			break
		}
	}
	delete(m.edges, id)
	return nil
}

func (m *mockEdgeRepository) Update(_ context.Context, edge *models.Edge) error {
	_, ok := m.edges[edge.ID]
	if !ok {
		return fmt.Errorf("edge with id %d not found", edge.ID)
	}
	copy := *edge
	copy.UpdatedAt = time.Now()
	m.edges[edge.ID] = &copy
	return nil
}

func (m *mockEdgeRepository) GetOutgoingEdges(_ context.Context, fromStateGUID string) ([]models.Edge, error) {
	var result []models.Edge
	for _, edge := range m.edges {
		if edge.FromState == fromStateGUID {
			result = append(result, *edge)
		}
	}
	return result, nil
}

func (m *mockEdgeRepository) GetIncomingEdges(_ context.Context, toStateGUID string) ([]models.Edge, error) {
	var result []models.Edge
	for _, edge := range m.edges {
		if edge.ToState == toStateGUID {
			result = append(result, *edge)
		}
	}
	return result, nil
}

func (m *mockEdgeRepository) GetAllEdges(_ context.Context) ([]models.Edge, error) {
	result := make([]models.Edge, 0, len(m.edges))
	for _, edge := range m.edges {
		result = append(result, *edge)
	}
	return result, nil
}

func (m *mockEdgeRepository) FindByOutput(_ context.Context, outputKey string) ([]models.Edge, error) {
	edgeIDs := m.outputIndex[outputKey]
	result := make([]models.Edge, 0, len(edgeIDs))
	for _, id := range edgeIDs {
		if edge, ok := m.edges[id]; ok {
			result = append(result, *edge)
		}
	}
	return result, nil
}

func (m *mockEdgeRepository) WouldCreateCycle(_ context.Context, fromState, toState string) (bool, error) {
	if m.cycleFunc != nil {
		return m.cycleFunc(fromState, toState), nil
	}

	// Simple cycle detection: check if path exists from toState to fromState
	visited := make(map[string]bool)
	var dfs func(current string) bool
	dfs = func(current string) bool {
		if current == fromState {
			return true
		}
		if visited[current] {
			return false
		}
		visited[current] = true
		for _, edge := range m.edges {
			if edge.FromState == current {
				if dfs(edge.ToState) {
					return true
				}
			}
		}
		return false
	}
	return dfs(toState), nil
}

// setupDependencyHandler creates a handler with both state and edge repositories
func setupDependencyHandler(baseURL string) (*StateServiceHandler, *mockRepository, *mockEdgeRepository) {
	stateRepo := newMockRepository()
	edgeRepo := newMockEdgeRepository()
	stateService := statepkg.NewService(stateRepo, baseURL)
	depService := dependency.NewService(edgeRepo, stateRepo)

	handler := &StateServiceHandler{
		service:    stateService,
		depService: depService,
	}
	return handler, stateRepo, edgeRepo
}

func TestStateServiceHandler_AddDependency(t *testing.T) {
	handler, stateRepo, _ := setupDependencyHandler("http://localhost:8080")
	ctx := context.Background()

	// Create two states for testing
	producerGUID := uuid.Must(uuid.NewV7()).String()
	consumerGUID := uuid.Must(uuid.NewV7()).String()
	require.NoError(t, stateRepo.Create(ctx, &models.State{GUID: producerGUID, LogicID: "producer"}))
	require.NoError(t, stateRepo.Create(ctx, &models.State{GUID: consumerGUID, LogicID: "consumer"}))

	t.Run("success with logic_id", func(t *testing.T) {
		toInputName := "landing_zone_vpc_id"
		req := connect.NewRequest(&statev1.AddDependencyRequest{
			FromState:   &statev1.AddDependencyRequest_FromLogicId{FromLogicId: "producer"},
			FromOutput:  "vpc_id",
			ToState:     &statev1.AddDependencyRequest_ToLogicId{ToLogicId: "consumer"},
			ToInputName: &toInputName,
		})
		resp, err := handler.AddDependency(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp.Msg.Edge)
		assert.Equal(t, "producer", resp.Msg.Edge.FromLogicId)
		assert.Equal(t, "vpc_id", resp.Msg.Edge.FromOutput)
		assert.Equal(t, "consumer", resp.Msg.Edge.ToLogicId)
		require.NotNil(t, resp.Msg.Edge.ToInputName)
		assert.Equal(t, "landing_zone_vpc_id", *resp.Msg.Edge.ToInputName)
		assert.Equal(t, "pending", resp.Msg.Edge.Status)
		assert.False(t, resp.Msg.AlreadyExists)
	})

	t.Run("success with default to_input_name", func(t *testing.T) {
		req := connect.NewRequest(&statev1.AddDependencyRequest{
			FromState:  &statev1.AddDependencyRequest_FromLogicId{FromLogicId: "producer"},
			FromOutput: "subnet_ids",
			ToState:    &statev1.AddDependencyRequest_ToLogicId{ToLogicId: "consumer"},
			// ToInputName omitted - should generate "producer_subnet_ids"
		})
		resp, err := handler.AddDependency(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, resp.Msg.Edge.ToInputName)
		assert.Equal(t, "producer_subnet_ids", *resp.Msg.Edge.ToInputName)
	})

	t.Run("idempotent duplicate", func(t *testing.T) {
		toInputName := "landing_zone_vpc_id"
		req := connect.NewRequest(&statev1.AddDependencyRequest{
			FromState:   &statev1.AddDependencyRequest_FromLogicId{FromLogicId: "producer"},
			FromOutput:  "vpc_id",
			ToState:     &statev1.AddDependencyRequest_ToLogicId{ToLogicId: "consumer"},
			ToInputName: &toInputName,
		})
		resp, err := handler.AddDependency(ctx, req)

		require.NoError(t, err)
		assert.True(t, resp.Msg.AlreadyExists)
		assert.NotNil(t, resp.Msg.Edge)
	})

	t.Run("to_input_name conflict", func(t *testing.T) {
		toInputName := "landing_zone_vpc_id" // Already used
		req := connect.NewRequest(&statev1.AddDependencyRequest{
			FromState:   &statev1.AddDependencyRequest_FromLogicId{FromLogicId: "producer"},
			FromOutput:  "different_output",
			ToState:     &statev1.AddDependencyRequest_ToLogicId{ToLogicId: "consumer"},
			ToInputName: &toInputName,
		})
		_, err := handler.AddDependency(ctx, req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "conflict")
	})

	t.Run("cycle prevention", func(t *testing.T) {
		// Create fresh states for cycle test to avoid interference from previous tests
		stateAGUID := uuid.Must(uuid.NewV7()).String()
		stateBGUID := uuid.Must(uuid.NewV7()).String()
		require.NoError(t, stateRepo.Create(ctx, &models.State{GUID: stateAGUID, LogicID: "state-a"}))
		require.NoError(t, stateRepo.Create(ctx, &models.State{GUID: stateBGUID, LogicID: "state-b"}))

		// Create edge: state-a -> state-b
		toInputName1 := "test_input"
		req1 := connect.NewRequest(&statev1.AddDependencyRequest{
			FromState:   &statev1.AddDependencyRequest_FromLogicId{FromLogicId: "state-a"},
			FromOutput:  "test_output",
			ToState:     &statev1.AddDependencyRequest_ToLogicId{ToLogicId: "state-b"},
			ToInputName: &toInputName1,
		})
		_, err := handler.AddDependency(ctx, req1)
		require.NoError(t, err)

		// Now try to add state-b -> state-a (would create cycle)
		toInputName2 := "reverse_input"
		req2 := connect.NewRequest(&statev1.AddDependencyRequest{
			FromState:   &statev1.AddDependencyRequest_FromLogicId{FromLogicId: "state-b"},
			FromOutput:  "reverse_output",
			ToState:     &statev1.AddDependencyRequest_ToLogicId{ToLogicId: "state-a"},
			ToInputName: &toInputName2,
		})
		_, err = handler.AddDependency(ctx, req2)

		// Should detect cycle and reject
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cycle")
	})

	t.Run("state not found", func(t *testing.T) {
		toInputName := "input"
		req := connect.NewRequest(&statev1.AddDependencyRequest{
			FromState:   &statev1.AddDependencyRequest_FromLogicId{FromLogicId: "nonexistent"},
			FromOutput:  "output",
			ToState:     &statev1.AddDependencyRequest_ToLogicId{ToLogicId: "consumer"},
			ToInputName: &toInputName,
		})
		_, err := handler.AddDependency(ctx, req)

		require.Error(t, err)
		assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	})
}

func TestStateServiceHandler_RemoveDependency(t *testing.T) {
	handler, stateRepo, edgeRepo := setupDependencyHandler("http://localhost:8080")
	ctx := context.Background()

	producerGUID := uuid.Must(uuid.NewV7()).String()
	consumerGUID := uuid.Must(uuid.NewV7()).String()
	require.NoError(t, stateRepo.Create(ctx, &models.State{GUID: producerGUID, LogicID: "producer"}))
	require.NoError(t, stateRepo.Create(ctx, &models.State{GUID: consumerGUID, LogicID: "consumer"}))

	edge := &models.Edge{
		FromState:   producerGUID,
		FromOutput:  "vpc_id",
		ToState:     consumerGUID,
		ToInputName: "input",
		Status:      models.EdgeStatusPending,
	}
	require.NoError(t, edgeRepo.Create(ctx, edge))
	edgeID := edge.ID

	t.Run("success", func(t *testing.T) {
		req := connect.NewRequest(&statev1.RemoveDependencyRequest{EdgeId: edgeID})
		resp, err := handler.RemoveDependency(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, resp)

		// Verify edge is deleted
		_, err = edgeRepo.GetByID(ctx, edgeID)
		require.Error(t, err)
	})

	t.Run("not found", func(t *testing.T) {
		req := connect.NewRequest(&statev1.RemoveDependencyRequest{EdgeId: 9999})
		_, err := handler.RemoveDependency(ctx, req)

		require.Error(t, err)
		assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	})
}

func TestStateServiceHandler_ListDependencies(t *testing.T) {
	handler, stateRepo, edgeRepo := setupDependencyHandler("http://localhost:8080")
	ctx := context.Background()

	producerGUID := uuid.Must(uuid.NewV7()).String()
	consumerGUID := uuid.Must(uuid.NewV7()).String()
	require.NoError(t, stateRepo.Create(ctx, &models.State{GUID: producerGUID, LogicID: "producer"}))
	require.NoError(t, stateRepo.Create(ctx, &models.State{GUID: consumerGUID, LogicID: "consumer"}))

	// Create incoming edges (consumer depends on producer)
	require.NoError(t, edgeRepo.Create(ctx, &models.Edge{
		FromState:   producerGUID,
		FromOutput:  "vpc_id",
		ToState:     consumerGUID,
		ToInputName: "vpc",
		Status:      models.EdgeStatusClean,
	}))

	t.Run("list incoming by logic_id", func(t *testing.T) {
		req := connect.NewRequest(&statev1.ListDependenciesRequest{
			State: &statev1.ListDependenciesRequest_LogicId{LogicId: "consumer"},
		})
		resp, err := handler.ListDependencies(ctx, req)

		require.NoError(t, err)
		assert.Len(t, resp.Msg.Edges, 1)
		assert.Equal(t, "producer", resp.Msg.Edges[0].FromLogicId)
		assert.Equal(t, "vpc_id", resp.Msg.Edges[0].FromOutput)
		require.NotNil(t, resp.Msg.Edges[0].ToInputName)
		assert.Equal(t, "vpc", *resp.Msg.Edges[0].ToInputName)
		assert.Equal(t, "clean", resp.Msg.Edges[0].Status)
	})
}

func TestStateServiceHandler_ListDependents(t *testing.T) {
	handler, stateRepo, edgeRepo := setupDependencyHandler("http://localhost:8080")
	ctx := context.Background()

	producerGUID := uuid.Must(uuid.NewV7()).String()
	consumerGUID := uuid.Must(uuid.NewV7()).String()
	require.NoError(t, stateRepo.Create(ctx, &models.State{GUID: producerGUID, LogicID: "producer"}))
	require.NoError(t, stateRepo.Create(ctx, &models.State{GUID: consumerGUID, LogicID: "consumer"}))

	// Create outgoing edges (producer is depended upon by consumer)
	require.NoError(t, edgeRepo.Create(ctx, &models.Edge{
		FromState:   producerGUID,
		FromOutput:  "vpc_id",
		ToState:     consumerGUID,
		ToInputName: "vpc",
		Status:      models.EdgeStatusDirty,
	}))

	t.Run("list outgoing by logic_id", func(t *testing.T) {
		req := connect.NewRequest(&statev1.ListDependentsRequest{
			State: &statev1.ListDependentsRequest_LogicId{LogicId: "producer"},
		})
		resp, err := handler.ListDependents(ctx, req)

		require.NoError(t, err)
		assert.Len(t, resp.Msg.Edges, 1)
		assert.Equal(t, "consumer", resp.Msg.Edges[0].ToLogicId)
		assert.Equal(t, "vpc_id", resp.Msg.Edges[0].FromOutput)
		assert.Equal(t, "dirty", resp.Msg.Edges[0].Status)
	})
}

func TestStateServiceHandler_SearchByOutput(t *testing.T) {
	handler, stateRepo, edgeRepo := setupDependencyHandler("http://localhost:8080")
	ctx := context.Background()

	producer1GUID := uuid.Must(uuid.NewV7()).String()
	producer2GUID := uuid.Must(uuid.NewV7()).String()
	consumerGUID := uuid.Must(uuid.NewV7()).String()
	require.NoError(t, stateRepo.Create(ctx, &models.State{GUID: producer1GUID, LogicID: "producer1"}))
	require.NoError(t, stateRepo.Create(ctx, &models.State{GUID: producer2GUID, LogicID: "producer2"}))
	require.NoError(t, stateRepo.Create(ctx, &models.State{GUID: consumerGUID, LogicID: "consumer"}))

	// Both producers export "vpc_id"
	require.NoError(t, edgeRepo.Create(ctx, &models.Edge{
		FromState:   producer1GUID,
		FromOutput:  "vpc_id",
		ToState:     consumerGUID,
		ToInputName: "vpc1",
		Status:      models.EdgeStatusClean,
	}))
	require.NoError(t, edgeRepo.Create(ctx, &models.Edge{
		FromState:   producer2GUID,
		FromOutput:  "vpc_id",
		ToState:     consumerGUID,
		ToInputName: "vpc2",
		Status:      models.EdgeStatusClean,
	}))

	t.Run("search by output key", func(t *testing.T) {
		req := connect.NewRequest(&statev1.SearchByOutputRequest{OutputKey: "vpc_id"})
		resp, err := handler.SearchByOutput(ctx, req)

		require.NoError(t, err)
		assert.Len(t, resp.Msg.Edges, 2)

		// Verify both producers are found
		logicIDs := []string{resp.Msg.Edges[0].FromLogicId, resp.Msg.Edges[1].FromLogicId}
		assert.Contains(t, logicIDs, "producer1")
		assert.Contains(t, logicIDs, "producer2")
	})
}

func TestStateServiceHandler_GetTopologicalOrder(t *testing.T) {
	handler, stateRepo, edgeRepo := setupDependencyHandler("http://localhost:8080")
	ctx := context.Background()

	// Create chain: A -> B -> C
	guidA := uuid.Must(uuid.NewV7()).String()
	guidB := uuid.Must(uuid.NewV7()).String()
	guidC := uuid.Must(uuid.NewV7()).String()
	require.NoError(t, stateRepo.Create(ctx, &models.State{GUID: guidA, LogicID: "a"}))
	require.NoError(t, stateRepo.Create(ctx, &models.State{GUID: guidB, LogicID: "b"}))
	require.NoError(t, stateRepo.Create(ctx, &models.State{GUID: guidC, LogicID: "c"}))

	require.NoError(t, edgeRepo.Create(ctx, &models.Edge{
		FromState:   guidA,
		FromOutput:  "out",
		ToState:     guidB,
		ToInputName: "in",
		Status:      models.EdgeStatusClean,
	}))
	require.NoError(t, edgeRepo.Create(ctx, &models.Edge{
		FromState:   guidB,
		FromOutput:  "out",
		ToState:     guidC,
		ToInputName: "in",
		Status:      models.EdgeStatusClean,
	}))

	t.Run("upstream from C", func(t *testing.T) {
		direction := "upstream"
		req := connect.NewRequest(&statev1.GetTopologicalOrderRequest{
			State:     &statev1.GetTopologicalOrderRequest_LogicId{LogicId: "c"},
			Direction: &direction,
		})
		resp, err := handler.GetTopologicalOrder(ctx, req)

		require.NoError(t, err)
		assert.NotEmpty(t, resp.Msg.Layers)
		// Should include a, b, c in topological order
	})

	t.Run("downstream from A", func(t *testing.T) {
		direction := "downstream"
		req := connect.NewRequest(&statev1.GetTopologicalOrderRequest{
			State:     &statev1.GetTopologicalOrderRequest_LogicId{LogicId: "a"},
			Direction: &direction,
		})
		resp, err := handler.GetTopologicalOrder(ctx, req)

		require.NoError(t, err)
		assert.NotEmpty(t, resp.Msg.Layers)
	})
}

func TestStateServiceHandler_GetStateStatus(t *testing.T) {
	handler, stateRepo, edgeRepo := setupDependencyHandler("http://localhost:8080")
	ctx := context.Background()

	producerGUID := uuid.Must(uuid.NewV7()).String()
	consumerGUID := uuid.Must(uuid.NewV7()).String()
	require.NoError(t, stateRepo.Create(ctx, &models.State{GUID: producerGUID, LogicID: "producer"}))
	require.NoError(t, stateRepo.Create(ctx, &models.State{GUID: consumerGUID, LogicID: "consumer"}))

	t.Run("clean status", func(t *testing.T) {
		// Create clean edge
		require.NoError(t, edgeRepo.Create(ctx, &models.Edge{
			FromState:   producerGUID,
			FromOutput:  "vpc_id",
			ToState:     consumerGUID,
			ToInputName: "vpc",
			Status:      models.EdgeStatusClean,
		}))

		req := connect.NewRequest(&statev1.GetStateStatusRequest{
			State: &statev1.GetStateStatusRequest_LogicId{LogicId: "consumer"},
		})
		resp, err := handler.GetStateStatus(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, "clean", resp.Msg.Status)
	})

	t.Run("stale status with dirty edge", func(t *testing.T) {
		// Update edge to dirty
		edges, _ := edgeRepo.GetIncomingEdges(ctx, consumerGUID)
		edge := edges[0]
		edge.Status = models.EdgeStatusDirty
		require.NoError(t, edgeRepo.Update(ctx, &edge))

		req := connect.NewRequest(&statev1.GetStateStatusRequest{
			State: &statev1.GetStateStatusRequest_LogicId{LogicId: "consumer"},
		})
		resp, err := handler.GetStateStatus(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, "stale", resp.Msg.Status)
		assert.Greater(t, resp.Msg.Summary.IncomingDirty, int32(0))
	})
}

func TestStateServiceHandler_GetDependencyGraph(t *testing.T) {
	handler, stateRepo, edgeRepo := setupDependencyHandler("http://localhost:8080")
	ctx := context.Background()

	producerGUID := uuid.Must(uuid.NewV7()).String()
	consumerGUID := uuid.Must(uuid.NewV7()).String()
	require.NoError(t, stateRepo.Create(ctx, &models.State{GUID: producerGUID, LogicID: "producer"}))
	require.NoError(t, stateRepo.Create(ctx, &models.State{GUID: consumerGUID, LogicID: "consumer"}))

	require.NoError(t, edgeRepo.Create(ctx, &models.Edge{
		FromState:   producerGUID,
		FromOutput:  "vpc_id",
		ToState:     consumerGUID,
		ToInputName: "vpc",
		Status:      models.EdgeStatusClean,
	}))

	t.Run("get graph for consumer", func(t *testing.T) {
		req := connect.NewRequest(&statev1.GetDependencyGraphRequest{
			State: &statev1.GetDependencyGraphRequest_LogicId{LogicId: "consumer"},
		})
		resp, err := handler.GetDependencyGraph(ctx, req)

		require.NoError(t, err)
		assert.Len(t, resp.Msg.Edges, 1)
		assert.Len(t, resp.Msg.Producers, 1)
		assert.Equal(t, "producer", resp.Msg.Producers[0].LogicId)
		assert.NotNil(t, resp.Msg.Producers[0].BackendConfig)
	})
}
