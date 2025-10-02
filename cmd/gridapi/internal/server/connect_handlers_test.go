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
