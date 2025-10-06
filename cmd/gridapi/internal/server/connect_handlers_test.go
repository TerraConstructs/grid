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
	"github.com/terraconstructs/grid/cmd/gridapi/internal/repository"
	statepkg "github.com/terraconstructs/grid/cmd/gridapi/internal/state"
)

type mockRepository struct {
	states map[string]*models.State
}

func newMockRepository() *mockRepository {
	return &mockRepository{states: make(map[string]*models.State)}
}

func (m *mockRepository) Create(_ context.Context, state *models.State) error {
	for _, existing := range m.states {
		if existing.LogicID == state.LogicID {
			return fmt.Errorf("state with logic_id '%s' already exists", state.LogicID)
		}
	}
	copy := *state
	if copy.CreatedAt.IsZero() {
		copy.CreatedAt = time.Now()
	}
	if copy.UpdatedAt.IsZero() {
		copy.UpdatedAt = copy.CreatedAt
	}
	m.states[state.GUID] = &copy
	return nil
}

func (m *mockRepository) GetByGUID(_ context.Context, guid string) (*models.State, error) {
	state, ok := m.states[guid]
	if !ok {
		return nil, fmt.Errorf("state with guid '%s' not found", guid)
	}
	copy := *state
	return &copy, nil
}

func (m *mockRepository) GetByLogicID(ctx context.Context, logicID string) (*models.State, error) {
	for _, state := range m.states {
		if state.LogicID == logicID {
			return m.GetByGUID(ctx, state.GUID)
		}
	}
	return nil, fmt.Errorf("state with logic_id '%s' not found", logicID)
}

func (m *mockRepository) Update(_ context.Context, state *models.State) error {
	current, ok := m.states[state.GUID]
	if !ok {
		return fmt.Errorf("state with guid '%s' not found", state.GUID)
	}
	copy := *current
	copy.StateContent = state.StateContent
	copy.Locked = state.Locked
	copy.LockInfo = state.LockInfo
	copy.UpdatedAt = time.Now()
	m.states[state.GUID] = &copy
	return nil
}

func (m *mockRepository) List(_ context.Context) ([]models.State, error) {
	result := make([]models.State, 0, len(m.states))
	for _, state := range m.states {
		result = append(result, *state)
	}
	return result, nil
}

func (m *mockRepository) Lock(_ context.Context, guid string, lockInfo *models.LockInfo) error {
	state, ok := m.states[guid]
	if !ok {
		return fmt.Errorf("state with guid '%s' not found", guid)
	}
	if state.Locked {
		return fmt.Errorf("state is already locked")
	}
	copy := *state
	copy.Locked = true
	lockCopy := *lockInfo
	copy.LockInfo = &lockCopy
	copy.UpdatedAt = time.Now()
	m.states[guid] = &copy
	return nil
}

func (m *mockRepository) Unlock(_ context.Context, guid string, lockID string) error {
	state, ok := m.states[guid]
	if !ok {
		return fmt.Errorf("state with guid '%s' not found", guid)
	}
	if !state.Locked {
		return fmt.Errorf("state is not locked")
	}
	if state.LockInfo == nil || state.LockInfo.ID != lockID {
		return fmt.Errorf("lock ID mismatch: expected %s", state.LockInfo.ID)
	}
	copy := *state
	copy.Locked = false
	copy.LockInfo = nil
	copy.UpdatedAt = time.Now()
	m.states[guid] = &copy
	return nil
}

func (m *mockRepository) UpdateContentAndUpsertOutputs(_ context.Context, guid string, content []byte, lockID string, serial int64, outputs []repository.OutputKey) error {
	state, ok := m.states[guid]
	if !ok {
		return fmt.Errorf("state with guid '%s' not found", guid)
	}

	// Validate lock if state is locked
	if state.Locked {
		if lockID == "" || state.LockInfo == nil || state.LockInfo.ID != lockID {
			return fmt.Errorf("state is locked, cannot update")
		}
	}

	// Update state content
	copy := *state
	copy.StateContent = content
	copy.UpdatedAt = time.Now()
	m.states[guid] = &copy

	// Note: output cache operations omitted in mock since tests don't verify output repo
	return nil
}

func setupHandler(baseURL string) (*StateServiceHandler, *mockRepository) {
	repo := newMockRepository()
	service := statepkg.NewService(repo, baseURL)
	return NewStateServiceHandler(service), repo
}

func TestStateServiceHandler_CreateState(t *testing.T) {
	handler, _ := setupHandler("http://localhost:8080")

	t.Run("success", func(t *testing.T) {
		guid := uuid.Must(uuid.NewV7()).String()
		req := connect.NewRequest(&statev1.CreateStateRequest{Guid: guid, LogicId: "test-state"})
		resp, err := handler.CreateState(context.Background(), req)

		require.NoError(t, err)
		assert.Equal(t, guid, resp.Msg.Guid)
		assert.Equal(t, "test-state", resp.Msg.LogicId)
		assert.NotNil(t, resp.Msg.BackendConfig)
	})

	t.Run("duplicate logic_id", func(t *testing.T) {
		guid := uuid.Must(uuid.NewV7()).String()
		req := connect.NewRequest(&statev1.CreateStateRequest{Guid: guid, LogicId: "dup"})
		_, err := handler.CreateState(context.Background(), req)
		require.NoError(t, err)

		req2 := connect.NewRequest(&statev1.CreateStateRequest{Guid: uuid.Must(uuid.NewV7()).String(), LogicId: "dup"})
		_, err = handler.CreateState(context.Background(), req2)
		require.Error(t, err)
		assert.Equal(t, connect.CodeAlreadyExists, connect.CodeOf(err))
	})
}

func TestStateServiceHandler_ListStates(t *testing.T) {
	handler, repo := setupHandler("http://localhost:8080")

	for i := 0; i < 2; i++ {
		guid := uuid.Must(uuid.NewV7()).String()
		repo.Create(context.Background(), &models.State{GUID: guid, LogicID: fmt.Sprintf("state-%d", i)})
	}

	req := connect.NewRequest(&statev1.ListStatesRequest{})
	resp, err := handler.ListStates(context.Background(), req)

	require.NoError(t, err)
	assert.Len(t, resp.Msg.States, 2)
}

func TestStateServiceHandler_GetStateConfig(t *testing.T) {
	handler, repo := setupHandler("http://localhost:8080")

	guid := uuid.Must(uuid.NewV7()).String()
	require.NoError(t, repo.Create(context.Background(), &models.State{GUID: guid, LogicID: "config-test"}))

	req := connect.NewRequest(&statev1.GetStateConfigRequest{LogicId: "config-test"})
	resp, err := handler.GetStateConfig(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, guid, resp.Msg.Guid)
	assert.NotNil(t, resp.Msg.BackendConfig)
}

func TestStateServiceHandler_GetStateLock_and_Unlock(t *testing.T) {
	handler, repo := setupHandler("http://localhost:8080")

	guid := uuid.Must(uuid.NewV7()).String()
	require.NoError(t, repo.Create(context.Background(), &models.State{GUID: guid, LogicID: "lock-test"}))

	lockInfo := &models.LockInfo{ID: "lock-id", Operation: "apply", Created: time.Now(), Path: "lock-test"}
	require.NoError(t, repo.Lock(context.Background(), guid, lockInfo))

	lockResp, err := handler.GetStateLock(context.Background(), connect.NewRequest(&statev1.GetStateLockRequest{Guid: guid}))
	require.NoError(t, err)
	assert.True(t, lockResp.Msg.Lock.Locked)
	assert.NotNil(t, lockResp.Msg.Lock.Info)

	unlockResp, err := handler.UnlockState(context.Background(), connect.NewRequest(&statev1.UnlockStateRequest{Guid: guid, LockId: "lock-id"}))
	require.NoError(t, err)
	assert.False(t, unlockResp.Msg.Lock.Locked)
}

func TestStateServiceHandler_ListStateOutputs(t *testing.T) {
	handler, repo := setupHandler("http://localhost:8080")
	ctx := context.Background()

	guid := uuid.Must(uuid.NewV7()).String()
	logicID := "outputs-test"
	state := &models.State{
		GUID:    guid,
		LogicID: logicID,
	}
	require.NoError(t, repo.Create(ctx, state))

	t.Run("list outputs by logic_id", func(t *testing.T) {
		// Add state content with outputs
		stateContent := []byte(`{"version": 4, "serial": 1, "outputs": {"vpc_id": {"value": "vpc-123", "sensitive": false}, "db_password": {"value": "secret", "sensitive": true}}}`)
		outputs := []repository.OutputKey{
			{Key: "vpc_id", Sensitive: false},
			{Key: "db_password", Sensitive: true},
		}
		err := repo.UpdateContentAndUpsertOutputs(ctx, guid, stateContent, "", 1, outputs)
		require.NoError(t, err)

		req := connect.NewRequest(&statev1.ListStateOutputsRequest{
			State: &statev1.ListStateOutputsRequest_LogicId{LogicId: logicID},
		})
		resp, err := handler.ListStateOutputs(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, guid, resp.Msg.StateGuid)
		assert.Equal(t, logicID, resp.Msg.StateLogicId)
		assert.Len(t, resp.Msg.Outputs, 2)

		// Verify output keys and sensitive flags
		keys := make(map[string]bool)
		for _, out := range resp.Msg.Outputs {
			keys[out.Key] = out.Sensitive
		}
		assert.False(t, keys["vpc_id"])
		assert.True(t, keys["db_password"])
	})

	t.Run("list outputs by guid", func(t *testing.T) {
		req := connect.NewRequest(&statev1.ListStateOutputsRequest{
			State: &statev1.ListStateOutputsRequest_Guid{Guid: guid},
		})
		resp, err := handler.ListStateOutputs(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, guid, resp.Msg.StateGuid)
		assert.Equal(t, logicID, resp.Msg.StateLogicId)
		assert.Len(t, resp.Msg.Outputs, 2)
	})

	t.Run("state with no outputs", func(t *testing.T) {
		emptyGUID := uuid.Must(uuid.NewV7()).String()
		emptyLogicID := "empty-outputs"
		emptyState := &models.State{
			GUID:    emptyGUID,
			LogicID: emptyLogicID,
		}
		require.NoError(t, repo.Create(ctx, emptyState))

		req := connect.NewRequest(&statev1.ListStateOutputsRequest{
			State: &statev1.ListStateOutputsRequest_LogicId{LogicId: emptyLogicID},
		})
		resp, err := handler.ListStateOutputs(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, emptyGUID, resp.Msg.StateGuid)
		assert.Len(t, resp.Msg.Outputs, 0)
	})

	t.Run("state not found", func(t *testing.T) {
		req := connect.NewRequest(&statev1.ListStateOutputsRequest{
			State: &statev1.ListStateOutputsRequest_LogicId{LogicId: "nonexistent"},
		})
		_, err := handler.ListStateOutputs(ctx, req)

		require.Error(t, err)
		assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	})
}

func TestStateServiceHandler_GetStateInfo(t *testing.T) {
	handler, repo := setupHandler("http://localhost:8080")
	ctx := context.Background()

	guid := uuid.Must(uuid.NewV7()).String()
	logicID := "info-test"
	state := &models.State{
		GUID:    guid,
		LogicID: logicID,
	}
	require.NoError(t, repo.Create(ctx, state))

	// Add state content with outputs
	stateContent := []byte(`{"version": 4, "serial": 1, "outputs": {"vpc_id": {"value": "vpc-123", "sensitive": false}}}`)
	outputs := []repository.OutputKey{
		{Key: "vpc_id", Sensitive: false},
	}
	err := repo.UpdateContentAndUpsertOutputs(ctx, guid, stateContent, "", 1, outputs)
	require.NoError(t, err)

	t.Run("get info by logic_id", func(t *testing.T) {
		req := connect.NewRequest(&statev1.GetStateInfoRequest{
			State: &statev1.GetStateInfoRequest_LogicId{LogicId: logicID},
		})
		resp, err := handler.GetStateInfo(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, guid, resp.Msg.Guid)
		assert.Equal(t, logicID, resp.Msg.LogicId)
		assert.NotNil(t, resp.Msg.BackendConfig)
		assert.NotNil(t, resp.Msg.BackendConfig.Address)
		assert.Contains(t, resp.Msg.BackendConfig.Address, guid)

		// Verify outputs are included
		assert.Len(t, resp.Msg.Outputs, 1)
		assert.Equal(t, "vpc_id", resp.Msg.Outputs[0].Key)
		assert.False(t, resp.Msg.Outputs[0].Sensitive)
	})

	t.Run("get info by guid", func(t *testing.T) {
		req := connect.NewRequest(&statev1.GetStateInfoRequest{
			State: &statev1.GetStateInfoRequest_Guid{Guid: guid},
		})
		resp, err := handler.GetStateInfo(ctx, req)

		require.NoError(t, err)
		assert.Equal(t, guid, resp.Msg.Guid)
		assert.Equal(t, logicID, resp.Msg.LogicId)
	})

	t.Run("state not found", func(t *testing.T) {
		req := connect.NewRequest(&statev1.GetStateInfoRequest{
			State: &statev1.GetStateInfoRequest_LogicId{LogicId: "nonexistent"},
		})
		_, err := handler.GetStateInfo(ctx, req)

		require.Error(t, err)
		assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	})
}
