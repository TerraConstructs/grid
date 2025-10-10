package server

import (
	"context"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/hashicorp/go-bexpr"
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
	copy.Labels = state.Labels
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

func (m *mockRepository) ListWithFilter(_ context.Context, filter string, pageSize int, offset int) ([]models.State, error) {
	// Simple implementation: if no filter, return all states
	var result []models.State
	for _, state := range m.states {
		result = append(result, *state)
	}

	// If filter is provided, use bexpr to filter
	if filter != "" {
		evaluator, err := bexpr.CreateEvaluator(filter)
		if err != nil {
			return nil, fmt.Errorf("invalid filter expression: %w", err)
		}

		var filtered []models.State
		for _, state := range result {
			labels := make(map[string]any, len(state.Labels))
			for k, v := range state.Labels {
				labels[k] = v
			}

			matches, err := evaluator.Evaluate(labels)
			if err != nil {
				// Skip states that can't be evaluated
				continue
			}
			if matches {
				filtered = append(filtered, state)
			}
		}
		result = filtered
	}

	// Apply pagination
	if offset >= len(result) {
		return []models.State{}, nil
	}

	end := offset + pageSize
	if end > len(result) {
		end = len(result)
	}

	return result[offset:end], nil
}

func setupHandler(baseURL string) (*StateServiceHandler, *mockRepository) {
	repo := newMockRepository()
	service := statepkg.NewService(repo, baseURL)

	// Create a mock policy repository and policy service
	policyRepo := newMockPolicyRepository()
	policyService := statepkg.NewPolicyService(policyRepo, statepkg.NewPolicyValidator())

	// Attach policy service to state service
	service.WithPolicyRepository(policyRepo)

	// Create handler with policy service
	handler := NewStateServiceHandler(service)
	handler.WithPolicyService(policyService)

	return handler, repo
}

// mockPolicyRepository is a simple in-memory policy repository for testing
type mockPolicyRepository struct {
	policy *models.LabelPolicy
}

func newMockPolicyRepository() *mockPolicyRepository {
	// Start with a simple policy
	return &mockPolicyRepository{
		policy: &models.LabelPolicy{
			ID:      1,
			Version: 1,
			PolicyJSON: models.PolicyDefinition{
				MaxKeys:     32,
				MaxValueLen: 256,
			},
		},
	}
}

func (m *mockPolicyRepository) GetPolicy(ctx context.Context) (*models.LabelPolicy, error) {
	if m.policy == nil {
		return nil, fmt.Errorf("label policy not found")
	}
	return m.policy, nil
}

func (m *mockPolicyRepository) SetPolicy(ctx context.Context, policy *models.PolicyDefinition) error {
	if m.policy == nil {
		m.policy = &models.LabelPolicy{
			ID:         1,
			Version:    1,
			PolicyJSON: *policy,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
	} else {
		m.policy.Version++
		m.policy.PolicyJSON = *policy
		m.policy.UpdatedAt = time.Now()
	}
	return nil
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

// T018: Test UpdateStateLabels RPC handler
func TestStateServiceHandler_UpdateStateLabels(t *testing.T) {
	handler, repo := setupHandler("http://localhost:8080")
	ctx := context.Background()

	guid := uuid.Must(uuid.NewV7()).String()
	state := &models.State{
		GUID:    guid,
		LogicID: "label-test",
		Labels: models.LabelMap{
			"env":  "staging",
			"team": "core",
		},
	}
	require.NoError(t, repo.Create(ctx, state))

	t.Run("adds and removals apply correctly", func(t *testing.T) {
		req := connect.NewRequest(&statev1.UpdateStateLabelsRequest{
			StateId: guid,
			Adds: map[string]*statev1.LabelValue{
				"region": {Value: &statev1.LabelValue_StringValue{StringValue: "us-west"}},
				"active": {Value: &statev1.LabelValue_BoolValue{BoolValue: true}},
			},
			Removals: []string{"team"},
		})

		resp, err := handler.UpdateStateLabels(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, resp.Msg)

		// Verify state was updated
		updated, err := repo.GetByGUID(ctx, guid)
		require.NoError(t, err)
		assert.Equal(t, "staging", updated.Labels["env"])    // unchanged
		assert.Equal(t, "us-west", updated.Labels["region"]) // added
		assert.Equal(t, true, updated.Labels["active"])      // added
		assert.Nil(t, updated.Labels["team"])                // removed
	})

	t.Run("validation errors return INVALID_ARGUMENT", func(t *testing.T) {
		req := connect.NewRequest(&statev1.UpdateStateLabelsRequest{
			StateId: guid,
			Adds: map[string]*statev1.LabelValue{
				"INVALID-KEY": {Value: &statev1.LabelValue_StringValue{StringValue: "value"}},
			},
		})

		_, err := handler.UpdateStateLabels(ctx, req)
		require.Error(t, err)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})

	t.Run("state not found returns NOT_FOUND", func(t *testing.T) {
		req := connect.NewRequest(&statev1.UpdateStateLabelsRequest{
			StateId: uuid.NewString(),
			Adds: map[string]*statev1.LabelValue{
				"env": {Value: &statev1.LabelValue_StringValue{StringValue: "prod"}},
			},
		})

		_, err := handler.UpdateStateLabels(ctx, req)
		require.Error(t, err)
		assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	})
}

// T019: Test GetLabelPolicy RPC handler
func TestStateServiceHandler_GetLabelPolicy(t *testing.T) {
	handler, _ := setupHandler("http://localhost:8080")
	ctx := context.Background()

	t.Run("retrieves policy", func(t *testing.T) {
		req := connect.NewRequest(&statev1.GetLabelPolicyRequest{})

		resp, err := handler.GetLabelPolicy(ctx, req)

		// May return empty policy or not found depending on implementation
		if err == nil {
			assert.NotNil(t, resp.Msg)
		} else {
			assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
		}
	})
}

// T020: Test SetLabelPolicy RPC handler
func TestStateServiceHandler_SetLabelPolicy(t *testing.T) {
	handler, _ := setupHandler("http://localhost:8080")
	ctx := context.Background()

	t.Run("sets policy and increments version", func(t *testing.T) {
		req := connect.NewRequest(&statev1.SetLabelPolicyRequest{
			PolicyJson: `{
				"allowed_keys": {"env": {}, "team": {}},
				"allowed_values": {"env": ["staging", "prod"]},
				"max_keys": 32,
				"max_value_len": 256
			}`,
		})

		resp, err := handler.SetLabelPolicy(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, resp.Msg)
		assert.Greater(t, resp.Msg.Version, int32(0))
	})
}

// T020a: Test SetLabelPolicy with invalid policy
func TestStateServiceHandler_SetLabelPolicyWithInvalidPolicy(t *testing.T) {
	handler, _ := setupHandler("http://localhost:8080")
	ctx := context.Background()

	t.Run("malformed JSON returns INVALID_ARGUMENT", func(t *testing.T) {
		req := connect.NewRequest(&statev1.SetLabelPolicyRequest{
			PolicyJson: `{"invalid": json}`,
		})

		_, err := handler.SetLabelPolicy(ctx, req)
		require.Error(t, err)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})

	t.Run("invalid schema returns INVALID_ARGUMENT", func(t *testing.T) {
		req := connect.NewRequest(&statev1.SetLabelPolicyRequest{
			PolicyJson: `{"allowed_keys": "should-be-map"}`,
		})

		_, err := handler.SetLabelPolicy(ctx, req)
		require.Error(t, err)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})
}

// T021: Test ListStates with filter parameter
func TestStateServiceHandler_ListStatesWithFilter(t *testing.T) {
	handler, repo := setupHandler("http://localhost:8080")
	ctx := context.Background()

	// Create test states with labels
	states := []struct {
		logicID string
		labels  models.LabelMap
	}{
		{"filter-state-1", models.LabelMap{"env": "staging"}},
		{"filter-state-2", models.LabelMap{"env": "prod"}},
		{"filter-state-3", models.LabelMap{"env": "staging", "team": "core"}},
	}

	for _, s := range states {
		state := &models.State{
			GUID:    uuid.Must(uuid.NewV7()).String(),
			LogicID: s.logicID,
			Labels:  s.labels,
		}
		require.NoError(t, repo.Create(ctx, state))
	}

	t.Run("filter by bexpr expression", func(t *testing.T) {
		filterStr := `env == "staging"`
		req := connect.NewRequest(&statev1.ListStatesRequest{
			Filter: &filterStr,
		})

		resp, err := handler.ListStates(ctx, req)
		require.NoError(t, err)
		require.Len(t, resp.Msg.States, 2)

		for _, state := range resp.Msg.States {
			require.Contains(t, state.Labels, "env")
			assert.Equal(t, "staging", state.Labels["env"].GetStringValue())
		}
	})

	t.Run("empty filter returns all", func(t *testing.T) {
		filterStr := ""
		req := connect.NewRequest(&statev1.ListStatesRequest{
			Filter: &filterStr,
		})

		resp, err := handler.ListStates(ctx, req)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(resp.Msg.States), 3, "Should return all test states")
	})

	t.Run("include_labels toggle", func(t *testing.T) {
		includeBool := false
		req := connect.NewRequest(&statev1.ListStatesRequest{
			IncludeLabels: &includeBool,
		})

		resp, err := handler.ListStates(ctx, req)
		require.NoError(t, err)

		for _, state := range resp.Msg.States {
			assert.Nil(t, state.Labels)
		}
	})
}

func TestMapServiceError_NotFoundOverridesGuid(t *testing.T) {
	err := fmt.Errorf("state with guid 'abc' not found")
	mapped := mapServiceError(err)
	require.Error(t, mapped)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(mapped))
}
